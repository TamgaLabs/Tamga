---
id: SPRINT-003
name: UI Restructure & Persistent Terminals
status: active
created: 2026-07-08
completed:
---

## Goal
User took a frontend build and found the UI needs a substantial restructure
plus a terminal-experience rework. Done means:

**Navigation / layout** (all secondary-sidebar sections are URL sub-routes,
Next.js nested layouts — decided with user):
- Dark-mode toggle moves out of the primary sidebar into Settings >
  Appearance (Light/Dark/System three-way).
- Project detail gets a secondary sidebar: project switcher dropdown at
  the top (Vercel-style popover: search + other projects + New Project
  shortcut; switching lands on the same sub-section), then Overview,
  Settings, Environment, Actions, Code entries. Code deep-links to the
  existing `/code/[id]` page; top-level `/code` list stays in the primary
  sidebar (it's also the home of the Tamga system codebase).
- Project's containers appear on the project detail: summary in Overview
  (status + quick start/stop/restart) AND a full Containers section in the
  project secondary sidebar ("ikisi de" — user chose both).
- Agent-provider selection on project detail is removed.
- Containers list page groups containers by project — open (non-collapsible)
  section headers, project name clickable to the project page, plus a
  "Non-project" group at the bottom.
- Container detail gets a secondary sidebar: Inspect, Logs, Stats,
  Resources as sub-routes.
- Settings gets a secondary sidebar with five sub-routes:
  /settings/appearance, /settings/sandbox (image + resource limits),
  /settings/network (egress), /settings/git, /settings/system (info +
  prune + Show Tamga System). Agent Providers and API Keys sections are
  removed from Settings — and removed entirely (backend endpoints,
  services, repos, domain, DB tables via migration too; user chose full
  removal).

**Terminal** (backend + frontend):
- Shell is bash, not sh (image is node:22-alpine — add bash to the image).
- Sessions survive the UI closing: backend session manager keeps each
  PTY + scrollback ring buffer alive after WebSocket disconnect; reattach
  replays the buffer. Sessions end only via explicit Terminate in the code
  page UI.
- Multiple terminal tabs per project, capped at 10 concurrent sessions.
- When a project's last session is terminated, its sandbox container
  auto-stops (re-created on demand next time, as today).
- Code page editor opens with the files sidebar visible by default.

**Egress**: three user-selectable modes in Settings > Network — Open
(everything allowed; the default on fresh installs), Whitelist (only
listed domains), Blacklist (everything except listed domains). The two
domain lists are stored separately so switching modes never loses entries.

**Known bugs to fix under this sprint**:
- Deleting a project leaves you on the deleted project's page with no
  feedback — should redirect to /dashboard with a toast.
- The Tamga system codebase never shows on the /code page, even with
  "Show Tamga System" enabled.

## Scope
- Frontend: (main) layout, primary sidebar, project detail, containers
  list + detail, code list + editor page, settings page, theme handling.
- Backend: terminal/session lifecycle (terminal_handler, agent_service),
  sandbox image (Dockerfile.agent), egress proxy modes
  (whitelist_service and friends), agent-provider + API-key removal,
  code_handler system-codebase bug.

## Out of Scope
- Deployment pipeline, git credential flows, auth — untouched except
  where the settings restructure moves their existing UI.
- New editor features beyond the files-sidebar default.
- Multi-user/permission concepts around egress modes.

## Phases
1. Explore/audit (`type: test`) — map the current frontend layout/routing/
   state architecture and the backend terminal + egress architecture, so
   the restructure tasks are planned against reality; plus the two
   already-reproducible bugs filed directly (no audit needed to know
   they're bugs).
2. Findings-driven implementation (planned 2026-07-08 from TEST-008 +
   TEST-009 findings). Intended order (bugs first, then features whose
   dependencies point backward):
   BUG-022/023/024/025 → FEAT-014 (remove providers/API keys; unblocks
   the settings + project-detail restructures) → FEAT-015 (terminal
   session manager; unblocks FEAT-020) → FEAT-016 (egress modes backend;
   unblocks FEAT-017's Network UI) → FEAT-017 (settings restructure +
   theme) → FEAT-018 (project detail sidebar) → FEAT-019 (containers
   grouping + detail sidebar) → FEAT-020 (terminal tabs UI).

## Tasks
- TEST-008 — Frontend layout/routing architecture audit for the restructure — done
- TEST-009 — Backend terminal, sandbox and egress architecture audit — done
- BUG-022 — Project delete leaves user on deleted project's page — done
- BUG-023 — Tamga system codebase never listed on /code even with Show Tamga System on — done
- BUG-024 — Project detail page renders permanently blank on fetch failure — done
- BUG-025 — Sandbox container stop takes ~10s after terminal WS closes — done
- FEAT-014 — Remove Agent Providers and API Keys entirely — done
- FEAT-015 — Backend terminal session manager (persistent bash sessions) — done
- FEAT-016 — Egress modes: Open / Whitelist / Blacklist — done
- FEAT-017 — Settings secondary sidebar + Light/Dark/System theme — done
- FEAT-018 — Project detail secondary sidebar + project switcher — done
- FEAT-019 — Containers grouped by project + detail secondary sidebar — done
- FEAT-020 — Code page terminal tabs + files sidebar default open — done
- BUG-026 — Code editor save failures silent; system codebase saves 500 invisibly — done
- FEAT-021 — Relocate backend Go tests out of production packages into internal/tests — done
- FEAT-022 — Detached terminal session idle-timeout setting (default Never) — done
- BUG-027 — Terminal session orphaned when WS upgrade fails after CreateSession — done

## Release Notes
<filled in at sprint completion>
