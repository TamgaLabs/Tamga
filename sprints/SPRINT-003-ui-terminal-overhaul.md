---
id: SPRINT-003
name: UI Restructure & Persistent Terminals
status: complete
created: 2026-07-08
completed: 2026-07-10
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

A substantial restructure of Tamga's UI plus a ground-up rework of the
in-browser terminal.

### Added
- **Persistent terminal sessions with tabs.** The code page now supports
  multiple terminal tabs per project. Sessions run **bash** and survive
  closing the tab or the whole browser — reopen the page and your sessions
  are still there, with their scrollback replayed. Sessions end only when
  you explicitly terminate them (with a confirm), and terminating a
  project's last session stops its sandbox. Up to 10 concurrent sessions
  per project. (FEAT-015, FEAT-020)
- **Configurable idle timeout for detached sessions.** Settings → Sandbox
  now has a "Session Idle Timeout" — how long a session with no open tab
  may sit before it's auto-terminated. Defaults to **Never** (sessions
  persist until you end them); pick 30m/1h/8h/24h if you'd rather they
  self-clean. (FEAT-022)
- **Selectable egress modes.** Settings → Network lets you choose how
  sandboxes reach the internet: **Open** (everything allowed — the new
  default), **Whitelist** (only listed domains), or **Blacklist**
  (everything except listed domains). The whitelist and blacklist are kept
  separately, so switching modes never loses your entries. (FEAT-016)
- **Light / Dark / System theme.** The theme control moved into Settings →
  Appearance and now offers a System option that follows your OS
  preference live. (FEAT-017)
- **Project switcher & richer project pages.** Each project detail page has
  a secondary sidebar with a searchable project switcher at the top and
  Overview / Containers / Settings / Environment / Actions / Code sections.
  A project's containers now appear right on the project — a summary on the
  Overview plus a full Containers list. (FEAT-018)
- The code editor's file tree is now open by default. (FEAT-020)

### Changed
- **Settings is now sectioned.** The single long settings page became five
  sub-pages — Appearance, Sandbox, Network, Git, System — each with its own
  URL you can link to and refresh. (FEAT-017)
- **Containers are grouped by project.** The Containers list now groups
  running containers under the project they belong to (with a "Non-project"
  group for the rest), and each container's detail page is split into
  Inspect / Logs / Stats / Resources sub-pages with a back-link to its
  project. (FEAT-019)
- **Sandboxes stop almost instantly.** Closing a terminal used to leave the
  sandbox lingering ~10 seconds; it now stops in a fraction of a second.
  (BUG-025)
- **Faster, clearer terminal shell.** Sandboxes now use bash instead of a
  minimal shell. (FEAT-015)
- Deleting a project now redirects you back to the dashboard with a
  confirmation instead of leaving you on the dead page. (BUG-022)

### Removed
- **Agent Providers and API Keys** are gone — leftover concepts from the
  removed agent bridge that nothing used. Their settings sections,
  endpoints, and stored data are removed. On upgrade, the related tables
  are dropped automatically. (FEAT-014)

### Fixed
- The Tamga system codebase now actually shows on the Code page when "Show
  Tamga System" is on, and opens in the editor. (BUG-023)
- Opening a project that failed to load no longer shows a permanently blank
  page — you get a clear "Project not found" instead. (BUG-024)
- Saving a file in the editor now surfaces failures with a visible,
  dismissible error instead of failing silently; the read-only system
  codebase rejects writes with a clear message rather than an opaque
  server error. (BUG-026)
- Terminal sessions can no longer be orphaned by a dropped connection
  during setup (which previously ate a slot toward the session cap and kept
  a sandbox alive). (BUG-027)

### Internal
- Upgrade behavior note: existing installs move to egress **Open** mode on
  upgrade — if you relied on the old always-on whitelist, re-select
  Whitelist mode in Settings → Network. (FEAT-016)
- Backend Go tests were relocated out of the production packages into a
  dedicated `internal/tests/` tree (black-box), with a few unavoidable
  exceptions kept in place and documented. (FEAT-021)
- This sprint began with two codebase audits (frontend architecture and
  backend terminal/sandbox/egress) that the implementation tasks were
  planned from. (TEST-008, TEST-009)
