---
id: FEAT-016
type: feature
title: Egress modes — Open / Whitelist / Blacklist, user-selectable, default Open
status: pending
complexity: standard
assignee: unassigned
sprint: SPRINT-003
created: 2026-07-08
history:
  - {date: 2026-07-08, stage: created, by: architect, note: "task created from TEST-009 findings §4"}
---

## Summary
Egress from sandboxes is currently whitelist-only (`ALLOWED_DOMAINS` env
on the egress-proxy, pure allow-list membership check — TEST-009 §4).
Per the user's decision, egress becomes a three-mode choice: **Open**
(everything allowed — the default on fresh installs), **Whitelist** (only
listed domains), **Blacklist** (everything except listed domains). The
whitelist and blacklist are stored as two separate lists so switching
modes never loses entries. Backend + proxy + API only — the new Settings
Network UI lands with FEAT-017.

## Requirements
- Storage (per TEST-009 §4's concrete design): keep `egress_whitelist` as
  is; new migration adding `egress_blacklist` (same shape) and
  `egress_settings(id INTEGER PRIMARY KEY CHECK (id=1), mode TEXT NOT
  NULL DEFAULT 'open')` — same single-row pattern as `resource_limits`.
- Service layer: mode get/set; blacklist CRUD mirroring the existing
  whitelist CRUD (including the same `normalizeDomain` + duplicate-domain
  409 behavior from BUG-017).
- API: extend the existing whitelist route group into an egress group —
  `GET/PUT` mode endpoint + `GET/POST/DELETE` for each list. Keep or
  cleanly rename the existing whitelist endpoints; if renamed, update the
  frontend `api.ts` callers and the Settings whitelist card in place (the
  card's relocation to /settings/network is FEAT-017's job).
- Proxy (`backend/cmd/egress-proxy/main.go`): add `MODE` and
  `DENIED_DOMAINS` env vars; `isAllowed` becomes a 3-way branch — open:
  always true; whitelist: current membership check; blacklist: inverted
  membership check. CONNECT/forward handling stays mode-agnostic.
- Reload: extend `ensureEgressProxy`'s wanted-env diff
  (agent_service.go:117-171) to include `MODE` and (in blacklist mode)
  `DENIED_DOMAINS`, so mode/list changes are picked up by the existing
  diff-and-recreate mechanism. Document (as today) that changes apply on
  next sandbox start, not to live sandboxes.
- Default: fresh installs get mode `open`. Existing installs: migration
  default is also `open` — this is a deliberate behavior change the user
  chose (previously-effective whitelist stops filtering until the user
  picks whitelist mode again); note it in Implementation Notes for the
  release notes.

## Out of Scope
- The Settings > Network UI (mode selector + two list editors) —
  FEAT-017.
- Per-project egress modes — global only, as today.
- Live reconfiguration of already-running sandboxes/proxy beyond the
  existing recreate-on-next-start mechanism.

## Proposed Solution / Approach
<filled in by developer>

## Affected Areas
<filled in by developer>

## Acceptance Criteria / Definition of Done
- [ ] Fresh DB: mode is `open`; a sandbox can reach an arbitrary domain (e.g. `curl https://example.com` through the proxy succeeds)
- [ ] Whitelist mode: only whitelisted domains resolve through the proxy; a non-listed domain is blocked (curl fails with the proxy's block response)
- [ ] Blacklist mode: a blacklisted domain is blocked, everything else succeeds
- [ ] Switching modes preserves both lists (add to both, flip modes, both lists still return their entries)
- [ ] Mode/list changes take effect on next sandbox start via the existing env-diff recreate (verify proxy container env changes)
- [ ] Duplicate domain on either list returns 409, not 500
- [ ] `go build ./...`, `go vet ./...`, `go test ./...` pass; service tests cover mode + blacklist CRUD like the existing whitelist tests
- [ ] Code follows KISS/YAGNI — no speculative abstraction

## Test Plan
On the compose stack with rebuilt backend + egress-proxy images: curl the
new endpoints through the API; for each of the three modes, exec a curl
inside a fresh sandbox against an allowed and a blocked domain and verify
proxy behavior; verify `docker inspect tamga-egress-proxy` shows the
expected MODE/ALLOWED_DOMAINS/DENIED_DOMAINS env after a mode change +
sandbox restart.

## Implementation Notes
<filled in by developer>

## Review Notes
<filled in by reviewer>

## Test Notes
<filled in by tester>
