---
id: FEAT-027
type: feature
title: Parse the supported docker-compose subset (compose-go/v2)
status: pending
complexity: standard
assignee: unassigned
sprint: SPRINT-004
created: 2026-07-10
history:
  - {date: 2026-07-10, stage: created, by: architect, note: "SPRINT-004 C2 cluster"}
---

**Part of:** C2-compose-deploy
**Depends on:** (none — parallel with FEAT-025/026)

## Summary
Parse a project's compose YAML into an internal service model the deploy
engine (FEAT-028) consumes, using `github.com/compose-spec/compose-go/v2`
(TEST-011's recommendation — the library `docker compose` itself uses).
Only the supported subset is honored; unsupported features are rejected
with clear errors.

## Requirements
- Add `compose-spec/compose-go/v2` and parse a compose YAML string into the
  project's services. Extract, per service: `image`, `ports`,
  `environment`, `volumes`, `networks`, `depends_on`. Normalize both the
  short and long syntaxes compose allows (e.g. `"8080:80"` vs the long port
  mapping; list vs map env) — this is exactly why we use the library, don't
  hand-parse.
- **Reject the out-of-scope features with a clear, actionable error** (not
  silent ignore): `build:`, `profiles:`, `secrets:`, healthcheck
  conditions, and any other unsupported key that would change runtime
  behavior. A user submitting `build:` must get "build is not supported yet;
  use a prebuilt image" rather than a broken deploy.
- Output an internal `[]ComposeService` (or reuse FEAT-025's domain type if
  it fits) with the extracted, normalized fields — the contract FEAT-028
  builds containers from. Coordinate the type with FEAT-025 (if FEAT-025
  defined ComposeService, produce that; else define here and FEAT-025/028
  consume it — note the seam).
- Validate: at least one service; a referenced `depends_on` target exists;
  duplicate service names rejected.
- Tests: black-box — parse valid subset composes (short+long syntax), assert
  the normalized model; assert each unsupported feature yields the expected
  error; validation errors (missing service, bad depends_on).

## Out of Scope
- Deploying the parsed model (FEAT-028); docker primitives (FEAT-026);
  schema (FEAT-025). Just YAML → internal model + validation.

## Proposed Solution / Approach
<filled in by developer>

## Affected Areas
<filled in by developer>

## Acceptance Criteria / Definition of Done
- [ ] compose-go/v2 added; a valid subset compose (both short + long syntaxes) parses into the normalized internal model
- [ ] Each unsupported feature (build/profiles/secrets/...) is rejected with a clear error, not silently dropped
- [ ] Validation: ≥1 service, depends_on targets exist, no dup names
- [ ] `go build/vet/test` pass; black-box tests cover valid parse + each rejection + validation
- [ ] Code follows KISS/YAGNI (lean on the library; don't reimplement compose)

## Test Plan
Black-box: a table of compose inputs → expected model or expected error.

## Implementation Notes
<filled in by developer>

## Review Notes
<filled in by reviewer>

## Test Notes
<filled in by tester>
