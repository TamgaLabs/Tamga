---
id: BUG-007
type: bug
title: AgentProvidersCard create/update never sends `type`, backend rejects it
status: pending
complexity: simple
assignee: unassigned
created: 2026-07-05
history:
  - {date: 2026-07-05, stage: created, by: architect, note: "found by sdlc-reviewer during BUG-001 review; pre-existing (since f3be4db), unrelated to that task, filed separately"}
---

## Summary
`AgentProvidersCard` (`frontend/src/app/(main)/settings/page.tsx`) never
includes a `type` field in its create/update payload to
`POST /api/agent-providers` / `PUT /api/agent-providers/{id}`. The backend
handler (`agent_provider_handler.go`) rejects any create where
`Type != domain.ProviderTypeDocker` (now the only valid type post-FEAT-004).
An empty/missing `type` in the JSON body means `p.Type` is the zero value
(`""`), which fails that check — so creating an agent provider via the
Settings UI has likely never worked since it was introduced.

## Steps to Reproduce
1. Go to Settings, open the Agent Providers card
2. Fill in the create form (name, image, etc.) and submit
3. Observe the request fails (400 "type must be 'docker'") or check the
   network tab for the outgoing payload — no `type` key present

## Expected Behavior
Creating an agent provider from the Settings UI succeeds.

## Actual Behavior
The create request is rejected by the backend because `type` is missing
from the payload.

## Environment / Context
Found by the sdlc-reviewer agent while reviewing BUG-001 (unrelated field
cleanup). Confirmed pre-existing since the original `f3be4db` commit
("add agent provider UI") — not a regression from BUG-001 or FEAT-004.

## Root Cause
<Filled in by the developer after investigation — likely the frontend form
simply never had a type selector/hidden field, since `ProviderTypeDocker`
was probably assumed to be the only option at the time and someone forgot
to actually include it in the payload.>

## Proposed Solution
<Filled in by the developer: since `docker` is now the only valid
`ProviderType` (post-FEAT-004's removal of `http`), the simplest fix is
likely to just always send `type: "docker"` in the create/update payload
from the frontend — no need for a type selector UI if there's only one
valid value. Confirm this doesn't need a broader form change.>

## Affected Areas
- `frontend/src/app/(main)/settings/page.tsx` (`AgentProvidersCard`)
- `frontend/src/lib/api.ts` (`createAgentProvider`/`updateAgentProvider` if they need a signature change)

## Acceptance Criteria
- [ ] Creating an agent provider from the Settings UI succeeds
- [ ] Updating an existing agent provider from the Settings UI succeeds

## Test Plan
In the browser, go to Settings > Agent Providers, create a new provider,
confirm it appears in the list without error. Edit it and confirm the
update succeeds too.

## Implementation Notes
<Filled in by the developer after coding.>

## Review Notes
<Filled in by the reviewer.>

## Test Notes
<Filled in by the tester.>
