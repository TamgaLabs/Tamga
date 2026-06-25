# Tamga — Agent Guidelines

## Proje Özeti

Tamga, bir Git repo URL'si vererek otomatik deploy yapmayı sağlayan, kendi kendini host eden bir DevOps panelidir. Caddy reverse proxy, Docker container orchestration ve AI agent entegrasyonu ile çalışır.

## Tech Stack

| Bileşen | Teknoloji |
|---------|-----------|
| Backend | Go 1.26+, Chi router |
| Frontend | Next.js 15+, shadcn/ui, Tailwind CSS, Monaco Editor |
| Database | SQLite (via modernc.org/sqlite) |
| Migration | golang-migrate (embed) |
| Reverse Proxy | Caddy (Admin API ile dinamik config) |
| Container | Docker SDK (docker-compose yok) |
| Auth | JWT (golang-jwt) |
| Agent | opencode server modu (container içinde) |
| Code Editor | @monaco-editor/react |

## Dizin Yapısı

```
/
├── backend/
│   ├── cmd/api/            # Entry point
│   ├── internal/
│   │   ├── domain/         # Core entities & business rules
│   │   ├── service/        # Use cases / business logic
│   │   ├── repository/     # Data access (SQLite, Docker, Caddy)
│   │   └── handler/        # HTTP handlers (Chi)
│   │       ├── auth_handler.go
│   │       ├── project_handler.go
│   │       ├── agent_handler.go
│   │       ├── system_handler.go
│   │       ├── container_handler.go   ← Docker container management
│   │       └── code_handler.go        ← Code IDE (file tree, read/write, agent)
│   ├── migrations/         # SQL migration files (embed)
│   ├── go.mod
│   └── go.sum
├── frontend/
│   ├── src/app/
│   │   ├── layout.tsx      # Root layout (AuthProvider)
│   │   ├── page.tsx        # Redirect to /dashboard or /login
│   │   ├── (auth)/         # Login, Setup (no sidebar)
│   │   └── (main)/         # Authenticated pages with sidebar
│   │       ├── layout.tsx  # Sidebar wrapper
│   │       ├── dashboard/  # Project list + create
│   │       ├── projects/[id]/  # Project detail (4 tabs: Overview/Settings/Agent/Code)
│   │       ├── containers/ # Docker container list + detail
│   │       ├── code/       # Code IDE (Monaco + Agent Chat + Diff)
│   │       └── settings/   # System info
│   ├── components/
│   │   ├── ui/             # shadcn/ui components
│   │   └── sidebar.tsx     # Navigation sidebar
│   └── lib/
│       ├── api.ts          # Backend API client
│       ├── auth.tsx        # JWT storage, auth context
│       └── utils.ts        # shadcn cn() helper
├── deploy/                 # Dockerfiles, Caddy config
└── Makefile
```

## Sayfa Yapısı

```
/                    → redirect to /dashboard
/dashboard           → Project grid
/dashboard/new       → Create project form
/projects/[id]       → Project detail:
    Overview         →   Status, domain, logs, deployments
    Settings         →   Name, domain, branch, env vars
    Agent            →   Chat + Diff panel
    Code             →   Monaco Editor + file tree

/containers          → Docker container list + filters
/containers/[id]     → Container detail: Inspect, Logs, Stats, Actions

/code                → Codebase list (projects + system toggle)
/code/[id]           → Code IDE: file tree + Monaco Editor + Agent Chat + Diff

/settings            → System info (Docker, CPU, memory, etc.)
```

## Mimari İlkeler

### Go (Backend)
- **Domain Driven Design** — domain katmanı dışa bağımlılık içermez, saf Go
- **Chi router** kullan (idiomatic, net/http compatible)
- **Gereksiz soyutlama yok** — ihtiyaç olmayan yere interface ekleme
- **Service layer** iş mantığını içerir, handler'lar sadece HTTP taşıyıcısıdır
- **Repository pattern** veri erişimini soyutlar ama her repository için interface şart değil
- **Testler** `_test.go` ile bitişik, `testing` package + `httptest`

### Frontend
- **Next.js App Router** kullan
- **Route groups**: `(auth)` login/setup, `(main)` authenticated pages
- **shadcn/ui** komponentleri `components/ui/` altında
- API çağrıları için `lib/api.ts`
- Monaco Editor için `@monaco-editor/react` (dynamic import, ssr: false)
- Client components sadece interaktivite gereken yerde

### Genel
- **Monorepo** — backend/ ve frontend/ aynı repo
- **Docker-only runtime** — docker-compose kullanma, Docker SDK ile yönet
- **Caddy** reverse proxy, backend admin API ile dinamik route ekler
- **.env** üzerinden konfigürasyon (domain, auth secret, db path, system code dir)
- **Tek kullanıcılı** sistem, JWT auth
- **SYSTEM_CODE_DIR** (Makefile ile mount edilir) → agent container'ı system koduna erişir

## Kod Yazım Kuralları

### Go

```go
// Handler — sadece HTTP işleri, her yeni endpoint için yeni handler dosyası
func (h *ContainerHandler) List(w http.ResponseWriter, r *http.Request) {
    if !h.requireDocker(w) { return }
    containers, err := h.docker.ListContainers(r.Context())
    ...
}

// Docker client wrapper — repository/docker/client.go
// Her yeni Docker API çağrısı buraya eklenir
func (c *Client) ListContainers(ctx context.Context) ([]ContainerInfo, error)
func (c *Client) InspectContainer(ctx context.Context, id string) (types.ContainerJSON, error)
func (c *Client) ContainerStats(ctx context.Context, id string) (*container.Stats, error)
```

- Hata yönetimi: `fmt.Errorf("context: %w", err)` ile wrap et
- Log: `slog` standard library
- Config: `os.Getenv` + struct
- SQL: Raw SQL + `database/sql` (orm yok)
- Docker v28.5.2 API: `system.Info`, `container.Stats`, `container.UpdateConfig`

### TypeScript/React

```typescript
// API client — tek bir api<T>() fonksiyonu
export const listContainers = () => api<ContainerInfo[]>("/system/containers");

// Component — shadcn/ui pattern
export function Sidebar() { ... }
```

- Monaco Editor: `import dynamic from "next/dynamic"; const Editor = dynamic(() => import("@monaco-editor/react"), { ssr: false });`
- Dil tespiti: dosya uzantısına göre `detectLanguage(path)` fonksiyonu

## Container İsimlendirme

| Container | Amaç | Network |
|-----------|------|---------|
| `caddy` | Reverse proxy | tamga-net |
| `tamga-backend` | API server | tamga-net |
| `tamga-frontend` | Next.js UI | tamga-net |
| `project-{id}` | Kullanıcı projeleri | tamga-net |
| `agent-{id}` | Proje agent'ları | tamga-net (port 9000) |
| `agent-system` | System code agent'ı | tamga-net (port 9001) |

## API Routes (Auth Required)

### Projects
```
GET/POST   /api/projects
GET/PUT/DELETE /api/projects/{id}
POST /api/projects/{id}/restart
GET  /api/projects/{id}/logs
GET  /api/projects/{id}/deployments
GET/POST/DELETE /api/projects/{id}/env-vars
POST /api/projects/{id}/agent/chat
GET  /api/projects/{id}/agent/tasks/{taskId}
```

### Containers (System)
```
GET  /api/system/containers
GET  /api/system/containers/{id}
POST /api/system/containers/{id}/start|stop|restart
DELETE /api/system/containers/{id}
GET  /api/system/containers/{id}/logs
GET  /api/system/containers/{id}/stats
PUT  /api/system/containers/{id}/resources
POST /api/system/prune
GET  /api/system/info
```

### Code
```
GET  /api/code/projects?system=true
GET  /api/code/{projectID}/tree
GET|PUT /api/code/{projectID}/file?path=...
POST /api/code/{projectID}/agent/chat
GET  /api/code/{projectID}/agent/tasks/{taskId}
```

## Commit Conventions

Format: `type(scope): description`

Types: `feat`, `fix`, `refactor`, `chore`, `docs`, `test`, `style`

Scopes: `backend`, `frontend`, `deploy`, `agent`, `docs`

Examples:
```
feat(backend): add container management endpoints
feat(frontend): add code IDE with Monaco editor
chore(deploy): mount SYSTEM_CODE_DIR in Makefile
```

## Common Commands

```bash
make setup           # .env oluştur, bağımlılıkları kontrol et
make up              # Tüm sistemi ayağa kaldır (Docker)
make down            # Durdur
make logs            # Logları izle
make test            # Testleri çalıştır
make frontend-dev    # Frontend dev server
```

## Dil

- Kod içi yorumlar ve commit mesajları İngilizce
- Kullanıcıya hitap eden UI metinleri İngilizce (ileride i18n eklenebilir)
- Agent yanıtları kullanıcının dilinde
