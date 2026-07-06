# Tamga

Docker orchestration layer with a developer-friendly UI — deploy applications from Git repositories with automatic HTTPS.

## Architecture

```
┌─────────┐   HTTP/HTTPS   ┌──────────┐
│  Client  │ ─────────────> │  Caddy   │
└─────────┘                 └──────────┘
                                  │
                 ┌────────────────┼────────────┐
                 ▼                ▼            │
          ┌────────────┐  ┌─────────────┐      │
          │  Backend   │  │  Frontend   │      │
          │  Go/chi    │  │  Next.js    │      │
          │  SQLite    │  │  (App Dir)  │      │
          │  :8080     │  │  :3000      │      │
          └────────────┘  └─────────────┘      │
                 │                              │
                 ▼                              │
         ┌──────────────┐                      │
         │ Docker Socket │◄─────────────────────┘
         │ (sibling)     │
         └──────────────┘
```

- **Caddy** — reverse proxy, automatic HTTPS via Let's Encrypt, routes `/api/*` to backend, everything else to frontend.
- **Backend** — Go server using `chi` router, SQLite via `modernc.org/sqlite` (pure Go, no CGO), Docker SDK for container management.
- **Frontend** — Next.js App Router, Tailwind CSS, shadcn/ui components.
- **Docker SDK** — sibling-container model: the backend binds `/var/run/docker.sock` to manage project containers directly.

## Quick Start

```bash
cp .env.example .env
docker compose up -d
```

The backend auto-runs database migrations on startup. An admin user is created automatically using `ADMIN_PASSWORD` from `.env`.

| Service   | URL                          | Notes                          |
|-----------|------------------------------|---------------------------------|
| Frontend  | `http://localhost`           | Served via Caddy reverse proxy  |
| API       | `http://localhost/api`       | Proxied through Caddy           |
| Caddy     | `:2019` (in-network only)    | Admin API — not published to the host, only reachable from other containers on `tamga-network` |

## API Endpoints

### Health

| Method | Path      | Auth | Description   |
|--------|-----------|------|---------------|
| GET    | `/health` | No   | Health check — registered outside `/api`, so it's only reachable directly against the backend container (or from inside `tamga-network`), not through Caddy's public routing, which only proxies `/api/*` and the frontend |

### Auth

| Method | Path                  | Auth    | Description         |
|--------|-----------------------|---------|---------------------|
| GET    | `/api/auth/status`    | No      | Setup status        |
| POST   | `/api/auth/setup`     | No      | Initial setup       |
| POST   | `/api/auth/login`     | No      | Login, returns JWT  |
| GET    | `/api/auth/me`        | Bearer  | Current user info   |

### Projects

| Method | Path                               | Auth    | Description          |
|--------|------------------------------------|---------|----------------------|
| POST   | `/api/projects`                    | Bearer  | Create project       |
| GET    | `/api/projects`                    | Bearer  | List user's projects |
| GET    | `/api/projects/{id}`               | Bearer  | Get project details  |
| PUT    | `/api/projects/{id}`               | Bearer  | Update project       |
| DELETE | `/api/projects/{id}`               | Bearer  | Delete project       |
| POST   | `/api/projects/{id}/restart`       | Bearer  | Restart project      |
| GET    | `/api/projects/{id}/logs`          | Bearer  | Get project logs     |
| GET    | `/api/projects/{id}/deployments`   | Bearer  | List deployments     |
| GET    | `/api/projects/{id}/env-vars`      | Bearer  | List env vars        |
| POST   | `/api/projects/{id}/env-vars`      | Bearer  | Add env var          |
| DELETE | `/api/projects/{id}/env-vars/{id}` | Bearer  | Remove env var       |

### Agent Terminal

A WebSocket-backed terminal (xterm.js on the frontend) into an on-demand
sandbox container. The container is created when the terminal connects and
stopped + removed when it disconnects. Run whatever agent CLI you like
(`opencode`, etc.) by hand inside it - the backend just proxies a shell, it
doesn't speak any agent-specific protocol.

| Method | Path                                       | Auth    | Description                        |
|--------|---------------------------------------------|---------|------------------------------------|
| GET    | `/api/projects/{id}/agent/terminal`         | Bearer* | WebSocket: PTY into sandbox         |

\* WebSocket connections can't set an `Authorization` header, so the token is
passed as a `?token=` query param instead.

### Code

| Method | Path                                                              | Auth    | Description               |
|--------|-------------------------------------------------------------------|---------|---------------------------|
| GET    | `/api/code/projects`                                              | Bearer  | List codebases            |
| GET    | `/api/code/{projectId}/tree`                                      | Bearer  | File tree                 |
| GET    | `/api/code/{projectId}/file`                                      | Bearer  | Read file                 |
| PUT    | `/api/code/{projectId}/file`                                      | Bearer  | Write file                |

### Agent Providers

| Method | Path                         | Auth    | Description                |
|--------|------------------------------|---------|----------------------------|
| GET    | `/api/agent-providers`       | Bearer  | List providers             |
| GET    | `/api/agent-providers/{id}`  | Bearer  | Get provider               |
| POST   | `/api/agent-providers`       | Bearer  | Create provider            |
| PUT    | `/api/agent-providers/{id}`  | Bearer  | Update provider            |
| DELETE | `/api/agent-providers/{id}`  | Bearer  | Delete provider            |

### System / Docker

| Method | Path                                   | Auth    | Description               |
|--------|----------------------------------------|---------|---------------------------|
| GET    | `/api/system/containers`               | Bearer  | List containers           |
| GET    | `/api/system/containers/{id}`          | Bearer  | Inspect container         |
| POST   | `/api/system/containers/{id}/start`    | Bearer  | Start container           |
| POST   | `/api/system/containers/{id}/stop`     | Bearer  | Stop container            |
| POST   | `/api/system/containers/{id}/restart`  | Bearer  | Restart container         |
| DELETE | `/api/system/containers/{id}`          | Bearer  | Remove container          |
| GET    | `/api/system/containers/{id}/logs`     | Bearer  | Container logs            |
| GET    | `/api/system/containers/{id}/stats`    | Bearer  | Container stats           |
| PUT    | `/api/system/containers/{id}/resources`| Bearer  | Update resources          |
| POST   | `/api/system/prune`                    | Bearer  | Prune Docker system       |
| GET    | `/api/system/info`                     | Bearer  | Docker info               |

### API Keys

| Method | Path                    | Auth    | Description       |
|--------|-------------------------|---------|-------------------|
| GET    | `/api/system/api-keys`  | Bearer  | List API keys     |
| POST   | `/api/system/api-keys`  | Bearer  | Set API key       |
| DELETE | `/api/system/api-keys/{id}` | Bearer | Delete API key  |

## Environment Variables

See `.env.example` for all configurable variables and defaults.

**Note on `HOST_DATA_DIR`**: This is the absolute host-side path to the data directory (the same directory that docker-compose.yml mounts as `./data:/data`). When using `docker compose up`, this is automatically set by the compose file to `${PWD}/data`. If running the backend outside of docker-compose, you must set this to the absolute path of your data directory; otherwise, agent sandbox container creation will fail.

## Project Layout

```
├── backend/
│   ├── cmd/api/main.go            # Go entrypoint
│   ├── cmd/egress-proxy/main.go   # Agent sandbox egress-whitelist proxy (FEAT-006)
│   └── internal/
│       ├── config/                # Config loader (env vars)
│       ├── handler/               # HTTP handlers
│       ├── repository/
│       │   ├── caddy/             # Caddy admin API client
│       │   ├── docker/            # Docker SDK client
│       │   └── sqlite/            # SQLite database + migrations
│       ├── router/                # chi router setup
│       └── service/               # Business logic
├── deploy/
│   ├── Dockerfile.backend         # Go production image
│   ├── Dockerfile.frontend        # Next.js production image
│   ├── Dockerfile.agent           # Agent runner image
│   └── Dockerfile.egress-proxy    # Agent sandbox egress-whitelist proxy (FEAT-006)
├── frontend/                      # Next.js App Router
│   └── src/
│       ├── app/                   # Pages (login, setup, dashboard, projects)
│       ├── components/            # UI components (shadcn/ui)
│       └── lib/                   # API client + types
├── Caddyfile                      # Caddy config template
├── docker-compose.yml             # Production stack
└── Makefile
```

## Makefile

| Command      | Description                        |
|--------------|------------------------------------|
| `make setup` | Copy `.env.example` to `.env`      |
| `make build` | Build Docker images                |
| `make up`    | Start production stack             |
| `make down`  | Stop production stack              |
| `make logs`  | Tail backend container logs        |
| `make test`  | Run Go tests                       |
| `make clean` | Stop and remove volumes            |
