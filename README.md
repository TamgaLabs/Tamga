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
make up
```

The `make up` command:
1. Copies `.env.example` to `.env` (if `.env` doesn't exist)
2. Builds the on-demand agent sandbox and egress-proxy images (required for agent terminals to work)
3. Starts the stack with `docker compose up -d`

The backend auto-runs database migrations on startup. An admin user is created automatically using `ADMIN_PASSWORD` from `.env`.

**Why `make up` instead of `docker compose up -d` directly?** The agent sandbox (`tamga-agent`) and egress-proxy (`tamga-egress-proxy`) images are excluded from bare `docker compose up -d` by design—they're not persistent compose-managed containers, but rather created on-demand by the backend via the Docker API. The `make up` target explicitly builds these images before starting the stack. If you run bare `docker compose up -d`, the main services (caddy, backend, frontend) will work, but agent terminals will fail with an image-not-found error until the images are built via `make up`.

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
sandbox container. Each interactive Bash session remains alive when its tab
detaches and ends only when it is terminated or its shell exits. Command
history is shared by tabs attached to the same live project sandbox, kept only
inside that sandbox, and is removed when its final session ends and the
sandbox is removed. The browser does not store terminal history. Run whatever
agent CLI you like (`opencode`, etc.) by hand inside it - the backend just
proxies a shell; it doesn't speak any agent-specific protocol.

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

| Command | Description |
|---|---|
| `make setup` | Copy `.env.example` to `.env` |
| `make build` | Build Docker images |
| `make up` | Start production stack |
| `make down` | Stop production stack |
| `make logs` | Tail backend container logs |
| `make clean` | Stop and remove volumes |

## Test Commands

The test lanes are intentionally separate. Fast commands do not require
Docker and never start or change a compose stack; every command returns a
single process exit status suitable for local use and CI.

| Lane | Command | Contract |
|---|---|---|
| Backend unit | `make test` or `make test-backend-unit` | Docker-free Go tests. Docker-reaching tests are selected only by the dedicated Docker lane. |
| Frontend static | `make test-frontend-static` | Runs lint plus an offline production build; no API or Docker is used. |
| Isolated API | `make test-backend-api` | Runs the auth and project scripts sequentially with their own temporary databases, ports, and cleanup. Requires `go`, `curl`, `git`, and `sqlite3`. |
| Docker integration | `TAMGA_TEST_DOCKER_OWNED=1 make test-backend-docker` | Requires a fresh, CI/job-owned Docker daemon. It refuses known terminal fixture names, builds the two required images, runs serially, and removes only resources it owns. It never runs against a shared developer daemon. |
| Frontend unit | `make test-frontend-unit` | Stable entry point for the Vitest suite supplied by FEAT-052. |
| Browser E2E | `E2E_BASE_URL=https://… make test-e2e` | Stable entry point for FEAT-053. The URL must name a CI-owned stack; no stack is inferred or started. |
| Live-stack smoke | `CADDY_HOST=https://… ADMIN_PASSWORD=… make test-live-smoke` | Operator-only check against an explicitly supplied disposable stack. It can mutate auth/project data and is never a default or CI lane. |

`make smoke-test` remains a deprecated compatibility alias for
`make test-live-smoke` and has the same explicit environment requirements.

## Smoke Tests

Verify that a running (or freshly-brought-up) Tamga stack is healthy and basic functionality works. The smoke test script checks:
- Backend health endpoint reachability
- Frontend availability through Caddy proxy
- Auth flow (setup, login, token generation)
- Basic project CRUD round-trip (create, list, delete)

Run the smoke tests only against an explicitly selected, disposable running stack:

```bash
CADDY_HOST=https://localhost ADMIN_PASSWORD=... make test-live-smoke
```

Do not use `--up` as a CI default: the smoke script is a live-stack check and
creates authentication/project state in its target database.

The script exits 0 on success with per-step output, or exits 1+ with a clear error message identifying which check failed. Example output:

```
→ Waiting for backend health endpoint...
✓ Backend health check passed
✓ Frontend is reachable through Caddy
✓ Auth status check passed
→ Logging in to get authentication token...
✓ Login successful, obtained authentication token
→ Creating test project...
✓ Test project created (ID: abc123)
✓ Test project found in project list
✓ Test project deleted successfully

✓ All smoke tests passed!
```

Useful environment variables (can be set in `.env` or passed to the script):
- `ADMIN_PASSWORD` — Password for admin user (defaults to `admin` if not set)
- `BACKEND_HOST` — Backend address (defaults to `localhost:8080`)
- `CADDY_HOST` — Caddy/frontend address (defaults to `https://localhost`)

## License

Tamga is dual-licensed under the **AGPL-3.0** and a **Commercial License**.

- **AGPL-3.0:** Free for personal, non-commercial, and open-source use. See the [LICENSE](LICENSE) file for full details.
- **Commercial License:** Required for companies using Tamga for internal business operations, revenue-generating use, embedding in commercial products, or SaaS/managed service offerings.

See [LICENSE](LICENSE) for full licensing details and the commercial licensing model.
