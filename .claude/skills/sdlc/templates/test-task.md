---
id: <ID>
type: test
title: <TITLE>
status: pending
complexity: <simple|standard>
assignee: unassigned
sprint: <SPRINT-ID or "none">
created: <DATE>
history:
  - {date: <DATE>, stage: created, by: architect, note: "task created"}
---

## Summary
<What subsystem/area this verifies, and why it needs dedicated verification
now (e.g. "confirm the auth flow actually works end-to-end before we build
anything else on top of it").>

## Scope
- <specific endpoint/flow/module in scope>
- <specific endpoint/flow/module in scope>

## Out of Scope
<Explicitly excluded — areas covered by a different test task, or areas
this one deliberately doesn't touch.>

## Test Approach
<Filled in by the developer before writing anything: what will be
exercised and how (e.g. a Go test file hitting the service layer, a bash+curl
script hitting live endpoints, a manual checklist) — pick the simplest
approach that gives real confidence, not the most elaborate one.>

## Affected Areas
<Files/modules read or added — test scripts/files created, source files
this touches if any (a test task should not need to modify production code;
if it turns out to require a source change, that's a signal to file a
separate bug task instead).>

## Acceptance Criteria
- [ ] Every item in Scope has been exercised for both success and failure
      paths (not just the happy path)
- [ ] Each result is a concrete, checkable observation (status code, DB
      row, log line) — not "looks fine"
- [ ] Any defect found is filed as its own `BUG-XXX` task with repro steps,
      not fixed inline as part of this task
- [ ] <add scope-specific measurable criteria>

## Test Plan
<How the tester will actually run this: exact commands/scripts, what
"pass" looks like for each one.>

## Implementation Notes
<Filled in by the developer after writing the test approach: what was
actually built (script/test file), how to run it, and anything that
diverged from the Test Approach above.>

## Review Notes
<Filled in by the reviewer: does the test approach actually cover the
Scope, is it testing real behavior rather than tautologically re-asserting
the code, any gaps.>

## Test Notes
<Filled in by the tester: results per Scope item, pass/fail, and a list of
any `BUG-XXX` tasks filed as a result of what was found here.>
