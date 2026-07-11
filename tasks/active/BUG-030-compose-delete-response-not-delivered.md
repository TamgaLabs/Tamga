---
id: BUG-030
type: bug
title: Deleting a compose project succeeds server-side but the HTTP response isn't delivered (client sees a network error)
status: pending
complexity: standard
assignee: unassigned
sprint: SPRINT-004
created: 2026-07-11
history:
  - {date: 2026-07-11, stage: created, by: architect, note: "surfaced during TEST-014's item-5 re-verification"}
---

## Summary
`DELETE /api/projects/<id>` for a deployed compose project completes
correctly on the backend (containers removed, project-net-<id> removed,
Traefik route removed, DB row + child rows gone — all verified), but the
client never receives the HTTP response: curl reports HTTP 000, and the
frontend delete flow would see a network error and show its failure banner
(BUG-022) + no redirect, even though the delete actually succeeded — the
user is left confused, retries, and gets a 404.

## Steps to Reproduce
1. Deploy a compose project with a routed exposed service (so Traefik is
   connected to project-net-<id>).
2. `curl -sk -X DELETE https://localhost/api/projects/<id>` → HTTP 000
   (connection dropped), yet the backend log shows the delete completing
   and all resources are gone afterward.

## Expected Behavior
The DELETE returns its 204 (or 200) to the client so the UI can show the
success confirmation + redirect (BUG-022 behavior).

## Actual Behavior
The client connection drops mid-request (HTTP 000); the response is lost.

## Environment / Context
Almost certainly the teardown disconnecting Traefik from the project
network (and/or removing that network) disrupts the very
Traefik-proxied DELETE connection in flight. Now that teardown runs on a
detached context (FEAT-028 rework 2), the server-side work COMPLETES — but
the in-flight response to the client is still lost. Likely fix directions:
respond to the client BEFORE the Traefik-disconnect/network-remove (do the
docker teardown fully async after sending 204), OR ensure disconnecting
Traefik from a project network can't drop connections the API entrypoint
is serving (the API rides the core network, not the project net — confirm
why the connection drops at all; it may be a Traefik behavior on network
membership change). Investigate the real drop cause before fixing.

## Root Cause
<filled in by developer>

## Proposed Solution
<filled in by developer>

## Affected Areas
<filled in by developer>

## Acceptance Criteria
- [ ] Deleting a compose project returns its HTTP response to the client (no 000/network error)
- [ ] The frontend delete flow shows success + redirect (BUG-022) for a compose project, not a failure banner
- [ ] Server-side cleanup still completes fully (containers/network/route/rows gone) — no regression of FEAT-028's teardown
- [ ] Non-compose/base API requests unaffected

## Test Plan
Delete a deployed compose project via curl AND the browser; assert a 2xx
response reaches the client and the UI shows success+redirect; confirm all
docker resources are still cleaned up.

## Implementation Notes
<filled in by developer>

## Review Notes
<filled in by reviewer>

## Test Notes
<filled in by tester>
