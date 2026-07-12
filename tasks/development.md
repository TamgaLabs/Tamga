# Development Guide

How to work on Tamga by hand — bring the system up, make a change, verify
it actually works, and clean up after yourself. This mirrors, in manual
form, what the automated `/sdlc` pipeline does for each task.

---

## 1. Prerequisites

- **Docker** + **Docker Compose** (the whole stack runs in containers).
- **Go** (backend; check `backend/go.mod` for the version).
- **Node.js + npm** (frontend; Next.js 15 / React 19).
- A POSIX shell. `curl`, `git`, and `sqlite3` are handy for poking at things.

> Sandbox networking note: on some hosts the default Docker bridge can't
> create veth pairs and `docker build`/`run`/`compose up` fails with a
> network error. If that happens, retry the same command with
> `--network host`.

---

## 2. Bring the system up

From the repo root:

```bash
docker compose up -d          # builds (first time) + starts all services
docker compose ps             # confirm the four services are healthy
```

The stack is four services:

| Service              | Role                                   | Reachable at            |
|----------------------|----------------------------------------|-------------------------|
| `tamga-caddy-1`      | reverse proxy + TLS (self-signed)      | `https://localhost`     |
| `tamga-frontend-1`   | Next.js UI (baked production image)    | via caddy `/`           |
| `tamga-backend-1`    | Go API + WebSocket terminal            | via caddy `/api/*`      |
| `tamga-egress-proxy` | sandbox internet egress filter         | internal only           |

- Everything is served at **`https://localhost`** through caddy. The cert
  is self-signed — your browser will warn; accept it (or use `curl -k`).
- The **admin password** comes from `ADMIN_PASSWORD` in `.env` (dev
  default: `admin`). `.env` is git-ignored; copy `.env.example` if you
  don't have one.
- The DB is SQLite at `./data/tamga.db` (bind-mounted). **Migrations run
  automatically on backend boot** — you don't run them by hand.
- The Tamga source is mounted read-only into the backend at `/tamga-src`
  (`SYSTEM_CODE_DIR`) so the "system" codebase shows up in the Code UI.

Log in:

```bash
curl -sk -X POST https://localhost/api/auth/login \
  -H 'Content-Type: application/json' -d '{"password":"admin"}'
# -> {"token":"<JWT>"}   use as:  Authorization: Bearer <JWT>
```

---

## 3. Repo layout

```
backend/                 Go API
  cmd/api/               main entrypoint
  cmd/egress-proxy/      the egress proxy binary
  internal/
    handler/             HTTP + WebSocket handlers
    service/             business logic (agent/session, egress, projects, …)
    repository/sqlite/   DB access + migrations/
    domain/              types
  scripts/               bash smoke/integration test scripts
frontend/                Next.js app (App Router)
  src/app/(main)/…       the authenticated pages
  src/components/        shared components + ui/ (shadcn primitives)
  src/lib/               api client, auth, theme, settings
deploy/                  Dockerfile.{backend,frontend,agent,egress-proxy}
docker-compose.yml       the stack
Caddyfile                reverse-proxy config
sprints/ , tasks/        the /sdlc pipeline's planning + task board
```

---

## 4. Running the tests

### Backend

```bash
cd backend
go build ./...     # compiles everything
go vet ./...       # static checks
go test ./...      # unit + some Docker-backed integration tests
```

> Some backend tests spin up real containers (terminal session lifecycle,
> orphan-cleanup regression). They are Docker-gated and skip cleanly if no
> daemon is available, but when Docker *is* present they will create and
> remove throwaway `agent-*` containers. Run them with a working daemon and
> check `docker ps -a` afterwards for anything left behind (there shouldn't
> be).

### Frontend

```bash
cd frontend
npx tsc --noEmit   # typecheck
npm run build      # production build (also runs lint)
```

### Bash smoke / integration scripts

`backend/scripts/` holds end-to-end scripts (auth, projects, containers,
providers, critical path). Prefer these over improvising `curl` chains:

```bash
bash backend/scripts/test-e2e-critical-path.sh
# and the more focused test-auth.sh / test-projects.sh / test-containers.sh
```

---

## 5. Testing a change manually

The discipline that matters: **drive the actual flow and watch it behave**,
not just typecheck. Pick the smallest environment that proves the change.

### Backend change

Rebuild and recreate only the backend (migrations re-run on boot):

```bash
docker compose build backend && docker compose up -d backend
docker logs -f tamga-backend-1        # watch it start + apply migrations
```

Then exercise the affected endpoint through caddy, e.g.:

```bash
TOKEN=$(curl -sk -X POST https://localhost/api/auth/login \
  -H 'Content-Type: application/json' -d '{"password":"admin"}' \
  | python3 -c 'import sys,json;print(json.load(sys.stdin)["token"])')

curl -sk https://localhost/api/projects -H "Authorization: Bearer $TOKEN"
```

If you changed the **egress proxy** or **agent sandbox image**, also
rebuild those images (`docker compose build egress-proxy`, or rebuild
`tamga-agent` from `deploy/Dockerfile.agent`) — the backend recreates the
egress-proxy container on the next sandbox start when its env changes, and
new sandboxes use `tamga-agent:latest`.

### Frontend change

The compose `frontend` service runs a **baked production image**, so it
does *not* reflect your working-tree edits. To see them, run a dev server:

```bash
cd frontend
npm run dev -- --port 3001        # serves your working tree at :3001
```

Open **`http://localhost:3001`** in a real browser and log in (password
`admin`). The dev server calls the same-origin API; since `:3001` has no
API of its own, API/WebSocket calls resolve against the backend the app is
configured for. For pure UI work `:3001` is enough; for anything hitting
`/api` or the terminal WebSocket, test in a real browser at
`https://localhost` after building the frontend image, **or** point your
testing at the running backend. (The self-signed cert means the terminal
WebSocket won't open from an unrelated origin — a real browser at
`https://localhost` that has accepted the cert works fine.)

### The terminal (WebSocket) flow

Opening a project's terminal creates a **session** and spins up a per-project
sandbox container `agent-<projectId>`:

- WS endpoint: `wss://localhost/api/projects/<id>/agent/terminal?token=<JWT>`
  (optional `&session=<id>` to reattach to an existing session).
- Sessions **persist** after you close the tab (detached), and replay their
  scrollback on reattach. They end only when you explicitly terminate them.
- REST: `GET /api/projects/<id>/agent/sessions` lists them;
  `DELETE /api/projects/<id>/agent/sessions/<sid>` terminates one.
- Terminating a project's **last** session stops its sandbox container.
- Cap: **10** concurrent sessions per project.

To exercise it from the CLI you can use a small Node `ws` client (there's
one in `frontend/node_modules`):

```bash
cd frontend && node -e '
const WebSocket=require("ws");
const ws=new WebSocket("wss://localhost/api/projects/1/agent/terminal?token=<JWT>",{rejectUnauthorized:false});
ws.on("open",()=>ws.send(JSON.stringify({type:"input",data:"echo hi\n"})));
ws.on("message",m=>process.stdout.write(m.toString()));'
```

Wire protocol: client → server sends JSON text frames
(`{"type":"input","data":"…"}` / `{"type":"resize","cols":N,"rows":N}`);
server → client streams **raw bytes** of shell output.

### Egress modes

Egress is global with three modes (Settings → Network in the UI, or the API):

```bash
curl -sk https://localhost/api/system/egress/mode -H "Authorization: Bearer $TOKEN"
curl -sk -X PUT https://localhost/api/system/egress/mode \
  -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -d '{"mode":"whitelist"}'   # open | whitelist | blacklist
```

Mode/list changes take effect on the **next** sandbox start (the proxy
container is recreated with new env), not for already-running sandboxes.
`open` is the default.

---

## 6. Cleaning up resources

Leaving sandboxes, sessions, and dev servers around wastes resources and
confuses the next test run. After testing:

### Terminal sessions & sandboxes

```bash
# terminate sessions (this also stops the sandbox when the last one goes)
curl -sk https://localhost/api/projects/1/agent/sessions -H "Authorization: Bearer $TOKEN"
curl -sk -X DELETE https://localhost/api/projects/1/agent/sessions/<sid> -H "Authorization: Bearer $TOKEN"

# belt-and-braces: remove any stray agent containers/networks directly
docker ps -a --format '{{.Names}}' | grep '^agent-' | xargs -r docker rm -f
docker network ls --format '{{.Name}}' | grep '^agent-net-' | xargs -r docker network rm
```

### Throwaway test projects / data

Delete test projects you created via the API:

```bash
curl -sk -X DELETE https://localhost/api/projects/<id> -H "Authorization: Bearer $TOKEN"
```

### Frontend dev server

```bash
# find and stop the dev server you started on :3001
ss -ltnp | grep :3001      # note the PID
kill <pid>
```

### Stopping / resetting the stack

```bash
docker compose down          # stop + remove the stack containers (keeps ./data)
docker compose down -v       # ALSO removes volumes — destroys DB/state; only if you mean it
```

> **Safety rules that bit us before, worth internalizing:**
> - Never `rm` untracked files that merely *look* like test artifacts —
>   they may be someone's uncommitted work, and git can't restore what was
>   never committed.
> - The shared compose stack (`tamga-*`) is long-lived infrastructure.
>   Don't tear it down as "cleanup" just because you ran `docker compose up`
>   — only remove the throwaway resources a specific test created.
> - Reverting a runtime-rewritten config (e.g. a temporary `Caddyfile`
>   edit) is part of cleanup; check `git status` for anything you changed
>   to make a test work and put it back.

---

## 7. A full manual pass, end to end

Roughly what the pipeline does for one change, by hand:

1. **Understand** the change and the smallest way to prove it.
2. **Implement** it (backend `internal/…`, frontend `src/…`).
3. **Static checks**: `go build/vet/test` and/or `tsc --noEmit && npm run build`.
4. **Bring up** the minimal environment (rebuild the one service you
   touched; dev server for frontend).
5. **Drive the flow** in a real browser / with curl / with a `ws` probe and
   watch the actual behavior — success *and* failure paths.
6. **Clean up** sessions, sandboxes, throwaway projects, dev servers; put
   back any config you temporarily changed; confirm `docker ps` shows only
   the four compose services.
7. **Commit** only the files your change actually touched (this repo often
   carries unrelated WIP — never `git add -A`).
