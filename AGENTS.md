# Tamga — Agent Guidelines

## Project Summary

Tamga is a self-hosted DevOps panel that auto-deploys from a Git repo URL. It uses Caddy reverse proxy, Docker container orchestration, and AI agent integration.

## Tech Stack

| Component | Technology |
|-----------|-----------|
| Backend | Go 1.26+, Chi router |
| Frontend | Next.js 15+, shadcn/ui, Tailwind CSS, Monaco Editor |
| Database | SQLite (via modernc.org/sqlite) |
| Migration | golang-migrate (embed) |
| Reverse Proxy | Caddy (dynamic config via Admin API) |
| Container | Docker SDK (no docker-compose) |
| Auth | JWT (golang-jwt) |
| Agent | opencode server mode (inside container) |
| Code Editor | @monaco-editor/react |

## Directory Structure

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
│   │       ├── container_handler.go
│   │       └── code_handler.go
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
│   │       ├── projects/[id]/  # Project detail (4 tabs)
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

## Page Structure

```
/                    -> redirect to /dashboard
/dashboard           -> Project grid
/dashboard/new       -> Create project form
/projects/[id]       -> Project detail:
    Overview         ->   Status, domain, logs, deployments
    Settings         ->   Name, domain, branch, env vars
    Agent            ->   Chat + Diff panel
    Code             ->   Monaco Editor + file tree

/containers          -> Docker container list + filters
/containers/[id]     -> Container detail: Inspect, Logs, Stats, Actions

/code                -> Codebase list (projects + system toggle)
/code/[id]           -> Code IDE: file tree + Monaco + Agent Chat + Diff

/settings            -> System info (Docker, CPU, memory, etc.)
```

## Architecture Principles

### Go (Backend)
- **Domain Driven Design** — domain layer has zero external dependencies, pure Go
- **Chi router** — idiomatic, net/http compatible
- **No unnecessary abstraction** — don't add interfaces where not needed
- **Service layer** holds business logic; handlers are just HTTP carriers
- **Repository pattern** abstracts data access, but interfaces are optional per repo
- **Tests** use `_test.go` naming, `testing` package + `httptest`

### Frontend
- **Next.js App Router**
- **Route groups**: `(auth)` for login/setup, `(main)` for authenticated pages
- **shadcn/ui** components under `components/ui/`
- API calls via `lib/api.ts`
- Monaco Editor via `@monaco-editor/react` (dynamic import, ssr: false)
- Client components only where interactivity is needed

### General
- **Monorepo** — backend/ and frontend/ in the same repo
- **Docker-only runtime** — no docker-compose, managed via Docker SDK
- **Caddy** reverse proxy, adds dynamic routes via backend Admin API
- **.env** for configuration (domain, auth secret, db path, system code dir)
- **Single-user** system, JWT auth
- **SYSTEM_CODE_DIR** (mounted via Makefile) -> agent container accesses system code

## Code Style

### Go

```go
// Handler — HTTP only, one file per endpoint group
func (h *ContainerHandler) List(w http.ResponseWriter, r *http.Request) {
    if !h.requireDocker(w) { return }
    containers, err := h.docker.ListContainers(r.Context())
    ...
}
```

- Error wrapping: `fmt.Errorf("context: %w", err)`
- Logging: `slog` standard library
- Config: `os.Getenv` + struct
- SQL: Raw SQL + `database/sql` (no ORM)
- Docker v28.5.2 API: `system.Info`, `container.Stats`, `container.UpdateConfig`

### TypeScript/React

```typescript
// API client — single api<T>() function
export const listContainers = () => api<ContainerInfo[]>("/system/containers");

// Component — shadcn/ui pattern
export function Sidebar() { ... }
```

- Monaco Editor: `import dynamic from "next/dynamic"; const Editor = dynamic(() => import("@monaco-editor/react"), { ssr: false });`
- Language detection: `detectLanguage(path)` based on file extension

## Container Naming

| Container | Purpose | Network |
|-----------|---------|---------|
| `caddy` | Reverse proxy | tamga-net |
| `tamga-backend` | API server | tamga-net |
| `tamga-frontend` | Next.js UI | tamga-net |
| `project-{id}` | User projects | tamga-net |
| `agent-{id}` | Project agents | tamga-net (port 9000) |
| `agent-system` | System code agent | tamga-net (port 9001) |

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
make setup           # Create .env, check dependencies
make up              # Start all services (Docker)
make down            # Stop
make logs            # Tail logs
make test            # Run tests
make frontend-dev    # Frontend dev server
```

## Language

- Code comments and commit messages in English
- UI text in English (i18n can be added later)
- Agent responses in the user's language
