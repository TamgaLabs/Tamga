---
id: SPRINT-002
name: Full-Stack Verification & Hardening
status: complete
created: 2026-07-07
completed: 2026-07-08
---

## Goal
Systematically verify the entire platform built in `SPRINT-001` actually
works end to end, backend first, then frontend/backend compatibility,
then frontend UI consistency — and fix every real defect found along the
way, rather than fixing bugs opportunistically. "Done" for this sprint
meant: every backend endpoint exercised live against a real Docker
daemon, the frontend's API contract diffed against the real backend, the
single most important user journey (setup → deploy → teardown) walked as
one continuous sequence, and the frontend's shadcn/ui usage audited for
consistency — with every discovered defect fixed and verified, not just
logged.

## Scope
- Phase 1 — Backend verification: auth/session, projects/env-vars/code/git
  credentials, container lifecycle/resources, agent providers/API
  keys/egress whitelist — all exercised live against a real backend +
  Docker daemon, not just read from source.
- Phase 2 — Frontend/backend compatibility: a static contract audit of
  every frontend API call against the real backend route table, then a
  single continuous end-to-end critical-path run (setup through
  deploy to teardown) to catch integration-order bugs per-endpoint
  testing can't.
- Phase 3 — Frontend UI audit: shadcn/ui component usage and styling
  consistency across the whole frontend, producing a findings backlog for
  a future visual-refactor sprint (not this sprint's job to execute).
- Every bug found during any phase, fixed and independently re-verified
  before the sprint's board could close.

## Out of Scope
- Actually executing the visual refactor implied by the Phase 3 findings
  — that's deliberately left as a backlog for a future sprint, since
  guessing its task list before the audit landed would have defeated the
  point of auditing first.

## Phases
1. **Explore/audit** (`TEST-001`–`TEST-004`) — exercise the entire backend
   route surface live, one subsystem at a time.
2. **Findings-driven fixes** (`BUG-010`–`BUG-018`) — every defect Phase 1
   surfaced, fixed and re-verified.
3. **Compatibility audit** (`TEST-005`, `TEST-006`) — frontend/backend
   contract diff, then a real end-to-end critical-path run.
4. **Findings-driven fixes** (`BUG-019`–`BUG-021`, `FEAT-013`) — every
   defect Phase 3 surfaced (including a missing frontend feature),
   fixed/built and re-verified.
5. **UI audit** (`TEST-007`) — shadcn/styling consistency audit, output
   is a findings backlog rather than an immediate refactor.

## Tasks
- TEST-001 — Auth, setup & session verification — done
- TEST-002 — Projects, env vars, code editor & git credential verification — done
- TEST-003 — Container lifecycle, system endpoints & resource limits verification — done
- TEST-004 — Agent providers, API keys & egress whitelist verification — done
- BUG-010 — Project handler returns 500 instead of 404 for a nonexistent project ID — done
- BUG-011 — Data race between ProjectService.Create's response and its background deploy goroutine — done
- BUG-012 — PUT /system/containers/{id}/resources fails for any container without a pre-existing memory-swap limit — done
- BUG-013 — Container handler returns 500 instead of 404 for a nonexistent container ID — done
- BUG-014 — POST /system/prune's malformed-body fallback silently prunes everything — done
- BUG-015 — PUT /agent-providers/{id} returns 500 instead of 404 for a nonexistent provider — done
- BUG-016 — Agent provider Create/Update doesn't enforce is_default exclusivity — done
- BUG-017 — POST /system/egress-whitelist returns 500 instead of 400/409 for a duplicate domain — done
- BUG-018 — SQLite DSN's WAL/busy_timeout query params are silently ignored by the driver — done
- TEST-005 — Frontend/backend API contract audit — done
- TEST-006 — End-to-end critical path verification (setup through deploy to teardown) — done
- BUG-019 — Frontend api() helper throws on empty-body 200/204 responses, silently aborting ~12 post-call UI actions — done
- FEAT-013 — Add settings UI for the agent egress whitelist — done
- BUG-020 — Project deploy hardcodes a Docker network ("tamga-net") that is never created — breaks every first deploy on a fresh install — done
- BUG-021 — Project env vars are stored in the DB but never actually applied to the running container — done
- TEST-007 — shadcn/component usage & styling audit — done

## Release Notes

### Fixed
- **Deploy was broken on every fresh install**: project deploy hardcoded
  a Docker network name nothing ever created, so the very first deploy
  on a clean install failed permanently. Deploy now provisions its own
  network on demand.
- **Project environment variables had no effect**: a project's env vars
  were stored but never actually passed to its container, at creation or
  on restart. Restart now recreates the container with the current env
  vars, so changes actually apply.
- **Frontend silently swallowed the result of ~12 actions**: deleting or
  restarting a project, starting/stopping/restarting/removing a
  container, deleting an API key/provider/git credential, and saving a
  code file all failed to refresh the UI afterward, because the shared
  API helper choked on empty success responses. Fixed once, centrally.
- Four different backend endpoints (`projects`, `containers`, `agent
  providers`, `whitelist`) returned `500` instead of a proper `404`/`409`
  for not-found or conflicting requests — now return the correct status
  with a clear message.
- A duplicate egress-whitelist domain no longer 500s; it returns a clean
  `409 Conflict`.
- A container resource-limit update (memory) no longer fails on
  containers without a pre-existing memory-swap limit — i.e. it no
  longer fails on effectively every container.
- Setting a second agent provider as default no longer leaves two
  providers simultaneously marked default; exclusivity is now enforced
  atomically.
- A rare data race between a just-created project's API response and its
  background deploy goroutine (which could leak an in-progress status
  into the create response) is fixed.
- `POST /system/prune` no longer silently prunes everything
  daemon-wide on a malformed/empty request body — it now rejects the
  request instead.
- The SQLite connection string's intended WAL mode and busy-timeout
  settings were silently no-ops due to a driver-specific syntax mismatch
  — the database has been running without either since it was
  introduced. Both now actually take effect.

### Added
- A Settings UI for managing the agent egress whitelist (view/add/remove
  allowed domains) — the backend supported this since `SPRINT-001` but
  had no frontend surface until now.

### Changed
- No user-facing behavior changes beyond the fixes above — this sprint
  was verification and hardening, not new functionality (aside from the
  whitelist UI).

### Internal
- Backend verification scripts added under `backend/scripts/` (auth,
  projects, containers, providers, and a full end-to-end critical-path
  script) — reusable for regression-checking future changes.
- A full shadcn/ui component-usage and styling audit is now on record
  (see `TEST-007`), seeding a backlog for a future UI-refactor sprint.
