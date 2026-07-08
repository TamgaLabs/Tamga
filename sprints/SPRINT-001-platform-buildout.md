---
id: SPRINT-001
name: Platform Build-out
status: complete
created: 2026-07-04
completed: 2026-07-06
---

## Goal
Take Tamga from an early prototype to a real, self-hostable Docker
orchestration platform: single-command deployment via docker-compose,
a working git-based deploy pipeline, an ephemeral agent sandbox with a
real terminal (replacing the earlier ACP-bridge approach), egress
control, per-sandbox resource limits, a global git credential mechanism
for private repos, licensing, a real backend test suite, and the
frontend pages/flows needed to actually use all of it. "Done" for this
sprint meant: a fresh clone can `docker-compose up` and deploy a project
end to end through the UI.

## Scope
- Docker-compose deployment (backend, frontend, Caddy, egress proxy)
- Agent sandbox lifecycle: ephemeral WebSocket PTY terminal, multi-CLI
  image (Claude Code/Codex/Gemini), network egress whitelist, resource
  limits
- Global git credential storage for clone/pull + sandbox commit/push
- Licensing (AGPL-3.0 + commercial exception)
- Backend test suite and a docker-compose smoke-test script
- Frontend pages needed to drive all of the above (container detail/logs,
  resource controls, deployment history)
- Bugs found and fixed along the way as each feature landed

## Out of Scope
- Systematic, dedicated verification of the resulting platform end to end
  (that became its own sprint — see `SPRINT-002`)
- Any visual/design refactor of the frontend (not yet audited at this
  point — also `SPRINT-002`)

## Phases
This sprint predates the sprint/phase convention itself — it ran as a
single continuous stream of `feature`/`bug` tasks, each landing and
getting fixed forward as issues surfaced during implementation, rather
than a formal explore-then-build split. Retroactively tagged into this
sprint file after the fact.

## Tasks
- FEAT-001 — docker-compose.yml for single-command deployment — done
- FEAT-002 — Backend config: CADDY_AUTO_SSL, UI_DOMAIN, API_DOMAIN — done
- FEAT-003 — Rewrite README and .env.example to match the real stack — done
- FEAT-004 — Remove ACP bridge, add WebSocket PTY terminal with ephemeral sandbox lifecycle — done
- FEAT-005 — Bundle Claude Code/Codex/Gemini CLI into agent sandbox image — done
- FEAT-006 — Agent network: isolated with whitelist-only egress — done
- FEAT-007 — Agent sandbox resource limits (default + Settings override) — done
- FEAT-008 — Global git credential (clone/pull + sandbox commit/push) — done
- FEAT-009 — Add LICENSE (AGPL-3.0 + Commercial Exception) — done
- FEAT-010 — Backend test suite (auth, project CRUD, env vars, agent terminal, git credential) — done
- FEAT-011 — Frontend: container detail/logs, resource update UI, deployment history — done
- FEAT-012 — Add a docker-compose smoke-test script for builder/tester tooling — done
- BUG-001 — Fix agent_provider Command mismatch / simplify fields post-ACP — done
- BUG-002 — Replace confirm() with shadcn AlertDialog — done
- BUG-003 — Remove unused agent-bridge (Go) directory — done
- BUG-004 — ApiKeyService.Set panics on nil pointer when no existing key for provider — done
- BUG-005 — setupCaddyRoutes posts to nonexistent Caddy admin /reload endpoint — done
- BUG-006 — Agent sandbox bind-mount uses relative DATA_DIR, breaking with stock .env.example — done
- BUG-007 — AgentProvidersCard create/update never sends `type`, backend rejects it — done
- BUG-008 — ApiKeyRepo can't scan created_at/updated_at back into time.Time — done
- BUG-009 — tamga-agent and tamga-egress-proxy images aren't built by docker-compose/Makefile — done

## Release Notes

### Added
- Single-command deployment via `docker-compose up` (backend, frontend,
  Caddy reverse proxy, egress proxy).
- Ephemeral agent sandbox terminal over a real WebSocket PTY connection,
  replacing the earlier ACP-bridge integration.
- Agent sandbox image bundling Claude Code, Codex, and Gemini CLIs so a
  deployed agent can use any of them.
- Network-isolated agent sandboxes with whitelist-only egress — an agent
  can only reach domains an operator has explicitly allowed.
- Per-sandbox resource limits (memory/CPU), with a configurable default
  and a per-project override in Settings.
- Global git credential storage, used both for cloning/pulling private
  project repos and for an agent sandbox's own commit/push access.
- AGPL-3.0 licensing with a commercial-license carve-out.
- A backend test suite covering auth, project CRUD, env vars, the agent
  terminal, and git credentials, plus a docker-compose smoke-test script.
- Frontend: container detail/logs view, a resource-limit update UI, and
  deployment history.

### Fixed
- Agent provider config no longer sends a field the backend rejects on
  create/update.
- API key creation no longer panics when no key previously existed for a
  provider.
- Caddy route reload now posts to the endpoint that actually exists.
- Agent sandbox bind-mounts now resolve correctly with the stock
  `.env.example` (previously broke on a relative `DATA_DIR`).
- API key timestamps now read back correctly instead of failing to scan.
- The agent and egress-proxy Docker images are now actually built by
  `docker-compose`/`Makefile` instead of silently missing.
- Removed dead code (the old ACP bridge, an unused `agent-bridge`
  directory) and a native `confirm()` dialog replaced with the app's own
  `AlertDialog` component.

### Changed
- N/A for this sprint — no breaking changes to already-shipped behavior;
  this sprint took the platform from prototype to first usable release.
