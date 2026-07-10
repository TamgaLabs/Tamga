---
id: FEAT-029
type: feature
title: Compose-project create/deploy UI
status: pending
complexity: standard
assignee: unassigned
sprint: SPRINT-004
created: 2026-07-10
history:
  - {date: 2026-07-10, stage: created, by: architect, note: "SPRINT-004 C2 cluster"}
---

**Part of:** C2-compose-deploy
**Depends on:** FEAT-025, FEAT-028

## Summary
The frontend for the unified compose model: create a project from a compose
file (a project is now a compose stack), and see its services once deployed.
Extends the existing project-create flow (dashboard/new) + project detail
(FEAT-018's secondary sidebar).

## Requirements
- Project create (`dashboard/new`): alongside the existing git-repo source,
  allow creating a **compose project** — paste/enter a docker-compose YAML +
  a domain. On submit, the backend (FEAT-028) parses + deploys. Surface
  parse/validation errors from FEAT-027 clearly inline (the "build: not
  supported" class of message). Keep the existing git-build create path
  working (it now deploys as a 1-service compose under the hood, but the UI
  for it is unchanged).
- Project detail: the Overview + Containers views (FEAT-018/019) should show
  the project's N services/containers (from `project_service_containers` /
  the topology) rather than assuming one. If FEAT-018's containers views
  already filter by project_id, confirm they render multiple service
  containers correctly; adjust if they assumed single-container.
- Exposed-service / domain: show which service holds the project domain
  (read-only here — the interactive domain-binding edit is C6/FEAT later,
  not this task). Just display it.
- api.ts: whatever new/changed endpoints FEAT-028 exposes for compose create
  + service listing.
- Keep it consistent with the existing shadcn UI + the FEAT-017/018 patterns.

## Out of Scope
- The interactive domain-binding edit action (later cluster C6).
- The infra map (C5) and analytics (C4).
- Backend deploy logic (FEAT-028).

## Proposed Solution / Approach
<filled in by developer>

## Affected Areas
<filled in by developer>

## Acceptance Criteria / Definition of Done
- [ ] dashboard/new can create a compose project (YAML + domain); parse/validation errors show inline; the git-repo path still works
- [ ] Project detail shows the project's N service containers (not just one); the exposed service/domain is displayed
- [ ] api.ts wired to FEAT-028's endpoints
- [ ] `npx tsc --noEmit` and `npm run build` pass
- [ ] Consistent with existing UI patterns; KISS/YAGNI

## Test Plan
Browser (C2 integration TEST-014 + a focused check): create a compose
project, see it deploy, see its services on the detail page; a bad compose
shows the error inline.

## Implementation Notes
<filled in by developer>

## Review Notes
<filled in by reviewer>

## Test Notes
<filled in by tester>
