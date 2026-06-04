# Tamga

Docker orchestration layer with a developer-friendly UI — deploy applications from Git repositories with automatic HTTPS.

## Prerequisites

- Docker & Docker Compose
- Node.js 18+

## Quick Start — Development

Terminal 1 — backend:
```bash
make dev
```

Terminal 2 — frontend:
```bash
make frontend-dev
```

Open `http://localhost:3000`.

| Service   | Location                      | Notes                          |
|-----------|-------------------------------|---------------------------------|
| Frontend  | `http://localhost:3000`       | Next.js dev server, proxies API |
| API       | `http://localhost:8080`       | Go backend                      |
| Postgres  | `localhost:5432`              | Volume persisted                |
| Traefik   | `localhost:80`, `localhost:443` | Let's Encrypt                  |

## Quick Start — Production

```bash
cp .env.example .env
make frontend-build
make build && make up
```

The API auto-runs database migrations on startup.

## API Endpoints

### Auth

| Method | Path                  | Auth    | Description         |
|--------|-----------------------|---------|---------------------|
| GET    | `/health`             | No      | Health check        |
| POST   | `/api/auth/register`  | No      | Create account      |
| POST   | `/api/auth/login`     | No      | Login, returns JWT  |
| GET    | `/api/auth/me`        | Bearer  | Current user info   |

### Projects

| Method | Path                        | Auth    | Description          |
|--------|-----------------------------|---------|----------------------|
| POST   | `/api/projects`             | Bearer  | Create project       |
| GET    | `/api/projects`             | Bearer  | List user's projects |
| GET    | `/api/projects/:id`         | Bearer  | Get project details  |
| PUT    | `/api/projects/:id`         | Bearer  | Update project       |
| DELETE | `/api/projects/:id`         | Bearer  | Delete project       |

### Domains

| Method | Path                                    | Auth    | Description          |
|--------|-----------------------------------------|---------|----------------------|
| POST   | `/api/projects/:projectId/domains`      | Bearer  | Add domain           |
| GET    | `/api/projects/:projectId/domains`      | Bearer  | List domains         |
| DELETE | `/api/domains/:id`                      | Bearer  | Remove domain        |

### Environment Variables

| Method | Path                                    | Auth    | Description          |
|--------|-----------------------------------------|---------|----------------------|
| POST   | `/api/projects/:projectId/env-vars`     | Bearer  | Add env var          |
| GET    | `/api/projects/:projectId/env-vars`     | Bearer  | List env vars        |
| PUT    | `/api/env-vars/:id`                     | Bearer  | Update env var       |
| DELETE | `/api/env-vars/:id`                     | Bearer  | Remove env var       |

### Git Repositories

| Method | Path                                    | Auth    | Description               |
|--------|-----------------------------------------|---------|---------------------------|
| POST   | `/api/projects/:projectId/git`          | Bearer  | Connect git repository    |
| GET    | `/api/projects/:projectId/git`          | Bearer  | Get git repository info   |
| DELETE | `/api/git/:id`                          | Bearer  | Disconnect git repository |

### Deployments

| Method | Path                                        | Auth    | Description               |
|--------|---------------------------------------------|---------|---------------------------|
| POST   | `/api/projects/:projectId/deployments`      | Bearer  | Trigger deployment        |
| GET    | `/api/projects/:projectId/deployments`      | Bearer  | List deployments          |
| GET    | `/api/deployments/:id`                      | Bearer  | Get deployment details    |
| POST   | `/api/deployments/:id/restart`              | Bearer  | Restart/Redeploy          |
| GET    | `/api/deployments/:id/logs`                 | Bearer  | Get historical logs       |
| GET    | `/api/deployments/:id/logs/stream`          | Bearer  | WebSocket live log stream |

## Project Layout

```
├── cmd/api/main.go                    # Go entrypoint
├── internal/
│   ├── api/router.go                  # Gin router
│   ├── auth/                          # JWT auth
│   ├── config/                        # Viper config
│   ├── database/                      # pgx pool, sqlc queries
│   ├── deployments/                   # Pipeline + API
│   ├── docker/                        # Docker SDK wrapper + builder
│   ├── domain/                        # Domain CRUD
│   ├── envvar/                        # Env var CRUD
│   ├── git/                           # Git clone + API
│   ├── logs/                          # WebSocket log streaming
│   └── proxy/                         # Traefik label generation
├── migrations/                        # SQL migrations
├── sqlc/                              # sqlc schema + queries
├── frontend/                          # Next.js app
│   └── src/
│       ├── app/
│       │   ├── login/                 # Login page
│       │   ├── register/              # Register page
│       │   ├── dashboard/             # Project list
│       │   └── projects/[id]/         # Project detail with tabs
│       └── lib/api.ts                 # API client + types
├── Dockerfile                         # Production multi-stage
├── Dockerfile.dev                     # Dev image
├── docker-compose.yml                 # Production
├── docker-compose.dev.yml             # Dev overrides
└── Makefile
```

## Makefile

| Command            | Description                        |
|--------------------|------------------------------------|
| `make dev`         | Start backend stack (dev)          |
| `make dev-down`    | Stop backend stack                 |
| `make up`          | Start production stack             |
| `make down`        | Stop production stack              |
| `make build`       | Build production Docker images     |
| `make frontend-dev`| Start Next.js dev server           |
| `make frontend-build`| Build Next.js for production     |
| `make logs`        | Tail all container logs            |
| `make generate`    | Run sqlc code generation           |
| `make test`        | Run Go tests in container          |
| `make clean`       | Stop and remove volumes            |
