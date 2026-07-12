---
name: tamga-sdlc
description: "Run Tamga's architect-led SDLC task board: initialize tasks, plan features, bugs, or sprints, advance developer-reviewer-builder-tester-committer gates, and report status. Use for tracked implementation work; not for untracked ad-hoc edits."
---

# Tamga SDLC

Act as architect and state-machine owner. Do not implement task code in the root thread. Delegate every implementation, review, runtime-test, and commit stage to the configured custom agents. Keep the root context limited to requirements, routing decisions, and distilled reports.

## Commands

- `init`: ensure `tasks/{active,review,test,done}` and `sprints/` exist.
- `sprint <goal>`: audit an open-ended goal, create a sprint, then file findings-driven tasks.
- `plan <description>`: inspect real code and create decision-complete task files.
- `run`: advance the oldest eligible task through the gates, one task at a time.
- `status`: summarize stages, blockers, clusters, and sprint progress.
- `help`: describe this workflow without mutating the board.

Read the matching template in `templates/` before creating an artifact. The architect alone edits task frontmatter, history, assignee, status, and stage path.

## Board invariants

- `tasks/active`: pending, in-development, changes-requested.
- `tasks/review`: awaiting/in review, including reviewed cluster parts held for integration.
- `tasks/test`: awaiting/in runtime test.
- `tasks/done`: passed tasks ready for or finished with commit.
- Before a move, ensure all four directories exist; Git drops empty directories.
- Select the oldest active task with `pending` or `changes-requested`.
- Append an ISO-date history entry for every transition. Never erase older Review/Test Notes.
- Scan every task directory before allocating a BUG/FEAT/TEST ID. Never reuse an ID.
- Preserve ambient WIP. Never use broad staging, destructive cleanup, or parallel write stages.

## Complexity and model routing

Explore before filing. `simple` means one localized seam, a known solution, a small diff, and no unresolved architecture, lifecycle, networking, concurrency, or data-model decision. Otherwise use `standard`.

Work larger than standard must be decomposed. Cluster parts carry `**Part of:** <cluster>` and `**Depends on:** <IDs>`. Each implementation part runs developer and reviewer, then remains uncommitted in review. One `type:test` task verifies the combined cluster. PASS moves and commits the cluster together; FAIL returns only implicated parts.

- Root architect: `gpt-5.6-terra`, high (project config).
- Simple developer/reviewer: `sdlc_developer_simple`, `sdlc_reviewer_simple` (Luna, medium).
- Standard developer/reviewer: `sdlc_developer_standard`, `sdlc_reviewer_standard` (Terra, medium).
- Environment: `builder` (GPT-5.4 mini, low).
- Runtime QA: `sdlc_tester` (Luna, medium).
- Git: `committer` (GPT-5.4 mini, medium).

## Run state machine

### Develop

1. Set `in-development`, set assignee to the chosen developer, append history.
2. Spawn that custom agent with the full task file and path. For rework, include latest Review/Test Notes.
3. Verify Root Cause/Proposed Solution and Implementation Notes landed and claimed files changed. A blocker routes to the blocker rule, not review.

### Review

1. Move to review, set `in-review`, append history.
2. Spawn the complexity-matched reviewer with task content/path and read its appended verdict.
3. `CHANGES_REQUESTED`: move to active, set `changes-requested`, record the request, redevelop.
4. `PASS`: hold a cluster implementation part in review; otherwise continue to runtime test.

### Runtime test

1. Move to test, set `in-test`, append history.
2. Spawn `builder`; retain its agent thread identifier.
3. When ready, spawn `sdlc_tester` with task content/path, builder report verbatim, and reviewer-established static facts.
4. On PASS, FAIL, interruption, or tester error, send teardown to the same builder thread and wait for cleanup.
5. `FAIL`: move to active, set `changes-requested`, record concrete evidence, redevelop.
6. `PASS`: move to done, set `done`, append history, then commit.

Skip builder/tester only if there is no runtime surface and the entire Test Plan was already directly executed (such as documentation-only work or a test-suite-only task). Record the reason.

### Commit

Spawn `committer` with the full done-task file and path. The architect never stages or commits. A scope ambiguity or failed clean-HEAD build is a blocker.

### Loop and blockers

Continue sequentially while eligible work exists. Retry a transient agent/tool failure once with a corrected prompt. If the same failure repeats, record a blocker and stop. Fix persistent pipeline instructions/config at their source.

## Sprint behavior

Use an audit/test phase before implementation when current-state facts are unknown. Keep the sprint manifest current, while task frontmatter remains authoritative. When no sprint task remains active/review/test, mark the sprint done and write user-facing Added/Changed/Fixed release notes.

## Context and telemetry

Pass only the task file, relevant previous-stage report, and exact required paths to each agent. Do not paste raw build/test logs into the root thread.

Each stage adds one row under `## Pipeline Telemetry`:

`| date | role | model | effort | result | duration | rework |`

Use only surfaced duration/token data. After 15-20 new tasks, compare first-pass review, test pass, rework count, and duration before changing model/effort.

## Deterministic environment lifecycle

Builder must use the project helper `scripts/sdlc-environment.sh`: `prepare <manifest> <repo-root>` to ready the shared stack, `smoke` for the baseline check, `record` for every task-owned resource, and `cleanup <manifest>` after testing. Cleanup removes only exact recorded names and never infers ownership from patterns. The shared compose stack is never task-owned.
