# Tamga — Implementation Plan

## 1. Overview

Tamga is a self-hosted DevOps panel that automates application deployment from Git repositories. It manages Docker containers, configures Caddy reverse proxy with automatic HTTPS, and provides AI-powered remote development via agent containers.

### Core Workflow

```
User clones Tamga → make setup → make up
  → Caddy starts (SSL ready)
  → Backend starts (SQLite, API)
  → Frontend starts (Next.js)

User creates project → enters repo URL
  → Backend clones repo
  → Builds Docker image
  → Starts container
  → Configures Caddy route
  → Project is live at project.domain.com

User opens project → Agent tab
  → Sends message via chat
  → Backend creates task → agent container processes
  → Frontend polls for result
  → Agent response + diff shown in UI
```

---

## 2. Architecture

### System Components

```
┌─────────────────────────────────────────────────┐
│                    Internet                       │
└──────────────────┬──────────────────────────────┘
                   │
              ┌────▼────┐
              │  Caddy   │  ← Reverse Proxy, SSL (Let's Encrypt)
              │ :443/80  │
              └───┬─────┘
                   │
          ┌────────┼────────┐
          │        │        │
     ┌────▼──┐ ┌──▼───┐ ┌──▼──────────┐
     │Backend│ │Front.│ │Project :1..N│  ← Deployed containers
     │:8080  │ │:3000 │ │:xxxx        │
     └───────┘ └──────┘ └─────────────┘
          │
     ┌────▼────┐
     │ SQLite  │
     └─────────┘
          │
     ┌────▼──────────┐
     │ Agent Container│  ← opencode server
     │ :9xxx         │
     └───────────────┘
```

### Key Design Decisions

| Karar | Sebep |
|-------|-------|
| Chi (Go) | Idiomatic, net/http compatible, minimal |
| SQLite | Zero ops, tek dosya, migration kolay |
| Caddy | Go ile yazılmış, auto HTTPS, basit admin API |
| Docker SDK (no compose) | Container yönetimi panelden yapılacak |
| Polling (Task ID) | WebSocket'siz agent iletişimi |
| Per-project agent container | İzolasyon, kaynak kontrolü |

---

## 3. Data Model

### Domain Entities

```
User
├── ID (int64)
├── PasswordHash (string)    ← bcrypt
├── CreatedAt
└── UpdatedAt

Project
├── ID (int64)
├── Name (string)
├── RepoURL (string)
├── Branch (string)          ← default: main
├── Domain (string)          ← project-domain.com
├── Status (enum)            ← created | cloning | building | running | error
├── ContainerID (string)     ← Docker container ID
├── EnvVars (map)            ← key-value pairs
├── CreatedAt
└── UpdatedAt

Deployment
├── ID (int64)
├── ProjectID (int64)        ← FK
├── Status (enum)            ← pending | running | success | failed
├── CommitSHA (string)
├── Logs (string)
├── CreatedAt
└── UpdatedAt

AgentTask
├── ID (string/uuid)
├── ProjectID (int64)        ← FK (hangi proje context'inde)
├── Message (string)         ← kullanıcının mesajı
├── Status (enum)            ← pending | processing | completed | failed
├── Response (string)        ← agent yanıtı
├── Diff (string)            ← yapılan değişikliklerin diff'i
├── CreatedAt
└── CompletedAt
```

---

## 4. API Design

### Auth
```
POST   /api/auth/setup         İlk kullanıcı oluşturma (setup flag)
POST   /api/auth/login         JWT token al
GET    /api/auth/me            Token'dan kullanıcı bilgisi
```

### Projects
```
POST   /api/projects                   Yeni proje oluştur + deploy
GET    /api/projects                   Projeleri listele
GET    /api/projects/:id               Proje detayı
PUT    /api/projects/:id               Projeyi güncelle (env vars, domain)
DELETE /api/projects/:id               Projeyi sil + container durdur
POST   /api/projects/:id/deploy        Re-deploy (yeniden build + run)
POST   /api/projects/:id/restart       Container restart
GET    /api/projects/:id/logs          Container logları
```

### Agent
```
POST   /api/projects/:id/agent/chat    Agent'a mesaj gönder → Task ID döner
GET    /api/projects/:id/agent/tasks/:task_id  Task status + response
```

### System
```
GET    /health                         Health check
GET    /api/system/info                Sistem bilgisi (Docker version, disk)
```

---

## 5. Backend Package Structure

```
backend/
├── cmd/
│   └── api/
│       └── main.go                    ← Entry point, setup config, DB, start server
├── internal/
│   ├── domain/
│   │   ├── project.go                 ← Project entity + status enum
│   │   ├── user.go                    ← User entity
│   │   ├── deployment.go             ← Deployment entity
│   │   ├── agent.go                  ← AgentTask entity
│   │   └── errors.go                 ← Domain-specific errors (ErrNotFound, etc)
│   ├── service/
│   │   ├── auth_service.go           ← Register, Login, JWT generate/validate
│   │   ├── project_service.go        ← CRUD, clone, build, run, stop
│   │   ├── deploy_service.go         ← Deploy pipeline orchestration
│   │   └── agent_service.go          ← Agent task management
│   ├── repository/
│   │   ├── sqlite/
│   │   │   ├── db.go                 ← DB connection, migration runner
│   │   │   ├── user_repo.go          ← User CRUD
│   │   │   ├── project_repo.go       ← Project CRUD
│   │   │   ├── deployment_repo.go    ← Deployment CRUD
│   │   │   └── agent_repo.go         ← AgentTask CRUD
│   │   ├── docker/
│   │   │   └── client.go             ← Docker SDK wrapper (build, run, stop, logs)
│   │   └── caddy/
│   │       └── client.go             ← Caddy Admin API client (add/remove route)
│   ├── handler/
│   │   ├── auth_handler.go           ← /api/auth/*
│   │   ├── project_handler.go        ← /api/projects/*
│   │   ├── agent_handler.go          ← /api/projects/:id/agent/*
│   │   └── system_handler.go         ← /health, /api/system/*
│   └── config/
│       └── config.go                 ← Env-based config struct
├── migrations/
│   ├── 000001_create_users.up.sql
│   ├── 000001_create_users.down.sql
│   ├── 000002_create_projects.up.sql
│   ├── 000002_create_projects.down.sql
│   ├── 000003_create_deployments.up.sql
│   ├── 000003_create_deployments.down.sql
│   ├── 000004_create_agent_tasks.up.sql
│   └── 000004_create_agent_tasks.down.sql
├── go.mod
└── go.sum
```

---

## 6. Frontend Pages & Components

```
frontend/
├── src/
│   ├── app/
│   │   ├── layout.tsx                    ← Root layout (shadcn providers)
│   │   ├── page.tsx                      ← Redirect to /dashboard or /setup
│   │   ├── setup/
│   │   │   └── page.tsx                  ← İlk kullanıcı oluşturma (setup flag ise)
│   │   ├── login/
│   │   │   └── page.tsx                  ← Giriş sayfası
│   │   ├── dashboard/
│   │   │   ├── page.tsx                  ← Proje listesi, create button
│   │   │   └── create-project.tsx        ← Yeni proje formu (modal/page)
│   │   └── projects/
│   │       └── [id]/
│   │           ├── page.tsx              ← Proje detay sayfası
│   │           ├── overview.tsx          ← Overview tab (status, domain, logs)
│   │           ├── settings.tsx          ← Settings tab (env vars, domain)
│   │           └── agent.tsx             ← Agent tab (chat + diff paneli)
│   ├── components/
│   │   ├── ui/                           ← shadcn/ui components
│   │   ├── project-card.tsx
│   │   ├── project-status-badge.tsx
│   │   ├── log-viewer.tsx
│   │   ├── agent-chat.tsx               ← Chat input + message list
│   │   ├── agent-diff.tsx               ← Diff görüntüleyici
│   │   └── create-project-form.tsx
│   ├── lib/
│   │   ├── api.ts                       ← Backend API client
│   │   ├── auth.ts                      ← JWT storage, auth context
│   │   └── utils.ts                     ← shadcn cn() helper
│   └── middleware.ts                    ← Next.js auth middleware (route protection)
```

---

## 7. Implementation Phases

### Phase 1: Foundation (Core Backend + Caddy Setup)
**Goal:** Backend ayağa kalkar, Caddy ile SSL çalışır, basit health endpoint'i vardır.

- [x] Go module init, Chi router setup
- [ ] `internal/config/config.go` — env-based config
- [ ] `internal/domain/user.go` — User entity
- [ ] `internal/repository/sqlite/db.go` — SQLite bağlantısı + migration runner
- [ ] `migrations/000001_create_users.up.sql` — users table
- [ ] `internal/service/auth_service.go` — Register, Login, JWT
- [ ] `internal/handler/auth_handler.go` — `/api/auth/*`
- [ ] `internal/handler/system_handler.go` — `/health`
- [ ] `deploy/Caddyfile` — Caddy base config
- [ ] `deploy/Dockerfile.backend` — Backend Docker image
- [ ] `deploy/Dockerfile.frontend` — Frontend Docker image
- [ ] `internal/service/setup_service.go` — First-run setup flow
- [ ] Makefile: `make setup`, `make up`, `make down`
- [ ] E2E: Backend + Caddy + Frontend ayakta, login akışı çalışıyor

### Phase 2: Core Deploy Pipeline
**Goal:** Proje oluşturup deploy edebiliyorum. Container build oluyor, başlıyor, Caddy route'u atanıyor.

- [ ] `internal/domain/project.go` — Project entity
- [ ] `internal/repository/sqlite/project_repo.go` — Project CRUD
- [ ] `migrations/000002_create_projects.up.sql`
- [ ] `internal/repository/docker/client.go` — Docker SDK: BuildImage, CreateContainer, StartContainer, StopContainer, Logs
- [ ] `internal/repository/caddy/client.go` — Caddy Admin API: add/remove route
- [ ] `internal/service/project_service.go` — Create (clone→build→run→route), Delete, List, Get
- [ ] `internal/handler/project_handler.go` — `/api/projects/*`
- [ ] `internal/domain/deployment.go` — Deployment entity
- [ ] `migrations/000003_create_deployments.up.sql`
- [ ] `internal/service/deploy_service.go` — Deploy pipeline
- [ ] Frontend: Dashboard, Project List, Create Project modal, Project Detail (overview)
- [ ] E2E: Repo URL verince proje deploy oluyor, domain'den erişilebiliyor

### Phase 3: Project Management (Settings, Logs, Env Vars)
**Goal:** Projeyi yönetebiliyorum (env var ekle, log izle, restart, delete).

- [ ] `internal/service/project_service.go` — Update, Restart, Delete (with container cleanup)
- [ ] `internal/handler/project_handler.go` — PUT, DELETE, POST restart, GET logs
- [ ] Frontend: Project Settings tab (env vars, domain), Log viewer, Restart/Delete actions
- [ ] Frontend: Project Status badge, real-time log polling
- [ ] E2E: Proje env var ekleme, log görüntüleme, restart silme çalışıyor

### Phase 4: Agent System
**Goal:** Her proje için agent container'ı, chat arayüzü, diff paneli.

- [ ] `internal/domain/agent.go` — AgentTask entity
- [ ] `migrations/000004_create_agent_tasks.up.sql`
- [ ] `internal/repository/docker/client.go` — Agent container management (start/stop per project)
- [ ] `internal/service/agent_service.go` — Task oluştur, agent container'a HTTP isteği yap, sonucu kaydet
- [ ] `internal/handler/agent_handler.go` — `/api/projects/:id/agent/chat`, `/api/projects/:id/agent/tasks/:task_id`
- [ ] `deploy/Dockerfile.agent` — Agent container (opencode + Node.js + git)
- [ ] Agent container HTTP API design: POST `/chat` (message → task), GET `/tasks/:id` (status+result)
- [ ] Frontend: Agent tab (chat input, message list, diff panel)
- [ ] Frontend: Polling logic for task status
- [ ] E2E: Agent'a mesaj atıp yanıt alabiliyorum, diff görüntüleyebiliyorum

### Phase 5: Polish & Production Readiness
**Goal:** Sistem production-ready, güvenli, gözlemlenebilir.

- [ ] Error handling improvements (structured errors)
- [ ] Input validation (repo URL format, domain format)
- [ ] Rate limiting (middleware)
- [ ] Graceful shutdown (signal handling)
- [ ] Docker image optimizasyonu (multi-stage, small images)
- [ ] Caddy SSL auto-configuration (Let's Encrypt)
- [ ] Log management (log rotation, log levels)
- [ ] Backup strategy (SQLite backup)
- [ ] README.md güncellemesi
- [ ] Test coverage (backend unit + integration tests)

---

## 8. Key Flows

### 8.1 Initial Setup (`make up`)

```
1. Check .env exists (DOMAIN, ADMIN_PASSWORD, JWT_SECRET)
2. Build backend Docker image
3. Build frontend Docker image
4. Build agent Docker image
5. Start Caddy container (ports 80, 443, admin API)
6. Start backend container (port 8080, SQLite volume)
7. Backend runs DB migrations
8. Backend registers Caddy route: api.{DOMAIN} → backend:8080
9. Backend registers Caddy route: {DOMAIN} → frontend:3000
10. Start frontend container (port 3000)
11. Health check → System ready
```

### 8.2 Project Deploy

```
User: POST /api/projects { name, repo_url, domain, branch }
1. Validate inputs
2. Create project record (status: created)
3. Clone repo to /data/projects/{id}/
   - git clone --branch {branch} {repo_url} /data/projects/{id}/
   - Update status: cloning
4. Build Docker image
   - docker build -t tamga-project-{id} /data/projects/{id}/
   - Update status: building
5. Start container
   - docker run -d --name project-{id} --restart unless-stopped \
       --network tamga-net tamga-project-{id}
   - Save container ID
   - Update status: running
6. Register Caddy route
   - POST Caddy Admin API: {domain} → project-{id}:{port}
7. Create deployment record
8. Return project details with domain URL
```

### 8.3 Agent Chat

```
User: POST /api/projects/:id/agent/chat { message }
1. Validate project exists + is running
2. Check if agent container exists for this project
   - If not: create agent container (opencode server)
3. Create AgentTask record (status: pending)
4. Forward message to agent container:
   - POST http://agent-{id}:{port}/chat { task_id, message }
   - Agent starts processing
   - Agent updates task status when done
5. Return { task_id }

Frontend: GET /api/projects/:id/agent/tasks/:task_id (poll every 2s)
6. Return { status, response (if completed), diff (if completed) }
7. When completed → show response in chat + diff in diff panel
```

---

## 9. Tech Details

### Caddy Admin API Usage

```bash
# Add route
curl -X POST "http://localhost:2019/config/apps/http/servers/srv0/routes/" \
  -H "Content-Type: application/json" \
  -d '{
    "match": [{"host": ["example.com"]}],
    "handle": [{
      "handler": "reverse_proxy",
      "upstreams": [{"dial": "localhost:3000"}]
    }]
  }'

# Remove route
curl -X DELETE "http://localhost:2019/config/apps/http/servers/srv0/routes/0"
```

### Docker SDK Usage (Key Calls)

```go
// Build image
buildOpts := types.ImageBuildOptions{
    Dockerfile: "Dockerfile",
    Tags:       []string{"tamga-project-123"},
    Remove:     true,
}

// Create container
resp, err := cli.ContainerCreate(ctx, &container.Config{
    Image: "tamga-project-123",
    Env:   []string{"PORT=8080"},
}, &container.HostConfig{
    NetworkMode: "tamga-net",
    RestartPolicy: container.RestartPolicy{Name: "unless-stopped"},
}, nil, nil, "project-123")

// Start container
cli.ContainerStart(ctx, resp.ID, types.ContainerStartOptions{})

// Get logs
cli.ContainerLogs(ctx, containerID, types.ContainerLogsOptions{
    ShowStdout: true,
    ShowStderr: true,
    Follow:     false,
})
```

### Environment Variables (.env)

```bash
DOMAIN=tamga.example.com          # Ana domain (frontend buradan erişilir)
ADMIN_PASSWORD=changeme           # İlk setup şifresi
JWT_SECRET=random-64-char-string  # JWT imzalama secretı
DB_PATH=./data/tamga.db           # SQLite dosya yolu
CADDY_ADMIN_URL=http://caddy:2019 # Caddy Admin API adresi
DATA_DIR=./data                   # Proje clone'larının tutulacağı dizin
```

---

## 10. Future Considerations (Phase 6+)

- **Auto Dockerfile Generation** — Repo'da Dockerfile yoksa otomatik oluştur (detect Node.js, Go, Python, etc.)
- **Multiple Users** — Multi-tenant support, team management
- **Multiple Servers** — SSH üzerinden remote server'lara deploy
- **Build Queue** — Concurrent build limit, queue management
- **Webhook Support** — GitHub/GitLab webhook ile auto-deploy
- **Metrics & Monitoring** — Prometheus metrics, Grafana dashboard
- **WebSocket Logs** — Live log streaming (replaces polling)
- **Template Projects** — One-click deploy from templates (Next.js, Go API, etc.)
- **Let's Encrypt Wildcard** — Wildcard SSL support via DNS challenge
- **SSH Key Management** — For private repo access
