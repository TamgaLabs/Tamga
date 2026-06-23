# Tamga — Agent Guidelines

## Proje Özeti

Tamga, bir Git repo URL'si vererek otomatik deploy yapmayı sağlayan, kendi kendini host eden bir DevOps panelidir. Caddy reverse proxy, Docker container orchestration ve AI agent entegrasyonu ile çalışır.

## Tech Stack

| Bileşen | Teknoloji |
|---------|-----------|
| Backend | Go 1.26+, Chi router |
| Frontend | Next.js 14+, shadcn/ui, Tailwind CSS |
| Database | SQLite (via modernc.org/sqlite) |
| Migration | golang-migrate (embed) |
| Reverse Proxy | Caddy (Admin API ile dinamik config) |
| Container | Docker SDK (docker-compose yok) |
| Auth | JWT (golang-jwt) |
| Agent | opencode server modu (container içinde) |

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
│   ├── migrations/         # SQL migration files (embed)
│   ├── go.mod
│   └── go.sum
├── frontend/               # Next.js + shadcn
├── deploy/                 # Dockerfiles, Caddy config templates
└── Makefile
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
- **shadcn/ui** komponentleri `components/ui/` altında
- API çağrıları için `lib/api.ts`
- Server Components öncelikli, client sadece interaktivite gereken yerde

### Genel
- **Monorepo** — backend/ ve frontend/ aynı repo
- **Docker-only runtime** — docker-compose kullanma, Docker SDK ile yönet
- **Caddy** reverse proxy, backend admin API ile dinamik route ekler
- **.env** üzerinden konfigürasyon (domain, auth secret, db path)
- **Tek kullanıcılı** sistem, JWT auth

## Kod Yazım Kuralları

### Go

```go
// Domain entity — saf Go, no tags no deps
type Project struct {
    ID        int64
    Name      string
    RepoURL   string
    Domain    string
    Status    ProjectStatus
    CreatedAt time.Time
}
```

```go
// Handler — sadece HTTP işleri
func (h *ProjectHandler) Create(w http.ResponseWriter, r *http.Request) {
    var req CreateProjectRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, err.Error(), http.StatusBadRequest)
        return
    }
    project, err := h.service.Create(r.Context(), req)
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }
    json.NewEncoder(w).Encode(project)
}
```

```go
// Service — iş mantığı
func (s *ProjectService) Create(ctx context.Context, req CreateProjectRequest) (*domain.Project, error) {
    // 1. Validate
    // 2. Clone repo
    // 3. Build image
    // 4. Start container
    // 5. Register Caddy route
    // 6. Save to DB
}
```

- Hata yönetimi: `fmt.Errorf("context: %w", err)` ile wrap et
- Log: `slog` standard library
- Config: `os.Getenv` + struct (viper'a gerek yok)
- SQL: Raw SQL + `database/sql` (orm yok)

### TypeScript/React

```typescript
// API client — tek bir fetch instance
export async function createProject(data: CreateProjectInput): Promise<Project> {
  const res = await fetch("/api/projects", { ... })
  return res.json()
}
```

```tsx
// Component — shadcn/ui pattern
export function ProjectCard({ project }: { project: Project }) {
  return <Card>...</Card>
}
```

## Commit Conventions

Format: `type(scope): description`

Types: `feat`, `fix`, `refactor`, `chore`, `docs`, `test`, `style`

Scopes: `backend`, `frontend`, `deploy`, `agent`, `docs`

Examples:
```
feat(backend): add project deploy pipeline
fix(frontend): handle empty project list
chore(deploy): update Dockerfile for multi-stage build
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
