# Tamga — Implementation Plan (Phase 5 Complete)

## Page Structure

```
/                    → redirect to /dashboard
/dashboard           → Project grid (existing)
/dashboard/new       → Create project form (existing)

/projects/[id]       → Project detail + 4 tabs
    Overview / Settings / Agent / Code (Monaco Editor + file tree)

/containers          → Container list + filters (project/system toggle, search)
/containers/[id]     → Container detail (Inspect/Logs/Stats/Actions tabs)

/code                → Codebase list (projects + system filter toggle)
/code/[id]           → Code IDE (file tree + Monaco Editor + Agent Chat + Diff)

/settings            → System info (Docker version, resources)
```

## Backend — New Files & Changes

| File | Change |
|------|--------|
| `repository/docker/client.go` | Added: ListContainers, InspectContainer, ContainerStats, RestartContainer, UpdateContainerResources, PruneContainers/Images/Volumes/Networks, DockerInfo, CreateContainerOpts, ContainerLogsSince |
| `handler/container_handler.go` | **NEW** — All container endpoints (list, inspect, start/stop/restart, logs, stats, update resources, prune, system info) |
| `handler/code_handler.go` | **NEW** — Codebase list, file tree, file read/write, code agent chat/task |
| `config/config.go` | Added `SystemCodeDir string` |
| `service/agent_service.go` | Added ChatForDir, GetTaskForDir, pollAgentTask, refactored forwardTask with port/dir params |
| `router/router.go` | Added container routes under /api/system/, code routes under /api/code/ |
| `cmd/api/main.go` | Wired ContainerHandler + CodeHandler |

## Frontend — New Files & Changes

| File | Change |
|------|--------|
| `components/sidebar.tsx` | **NEW** — Navigation sidebar (Projects, Containers, Code, Settings) |
| `app/(main)/layout.tsx` | **NEW** — Auth layout with Sidebar |
| `app/(main)/containers/page.tsx` | **NEW** — Container list with filters |
| `app/(main)/containers/[id]/page.tsx` | **NEW** — Container detail (inspect, logs, stats) |
| `app/(main)/code/page.tsx` | **NEW** — Codebase list |
| `app/(main)/code/[id]/page.tsx` | **NEW** — Code IDE (file tree + Monaco + Agent Chat + Diff) |
| `app/(main)/settings/page.tsx` | **NEW** — System info/settings |
| `app/(main)/projects/[id]/page.tsx` | Added Code tab |
| `lib/api.ts` | Added container, code, info API functions + types |

## Docker / Makefile

| Change | Detail |
|--------|--------|
| `SYSTEM_CODE_DIR` volume mount | Backend container mounts `$(pwd):$(pwd):ro`, env var set |
| `.env.example` | Added `SYSTEM_CODE_DIR=.` |
| System agent container | `agent-system` on port 9001, mounts system code dir |

## Implementation Order (Completed)

1. Backend: Docker Client — ListContainers, Inspect, Stats, Restart, Prune
2. Backend: Container Handler
3. Backend: Code Handler + Codebase list, file operations
4. Backend: Agent Service — ChatForDir for system codebase
5. Backend: Config, Router updates
6. Frontend: Monaco Editor install, Sidebar + Layout
7. Frontend: Containers list + detail
8. Frontend: Code list + IDE with Monaco
9. Frontend: Project Code tab, Settings page
10. Frontend: API client additions
11. Makefile volume mount
12. agents.md + plan.md updates
