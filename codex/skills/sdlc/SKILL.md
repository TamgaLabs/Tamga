---
name: sdlc
description: "Run Tamga's architect-led SDLC task board: initialize tasks, plan features, bugs, or sprints, advance developer-reviewer-builder-tester-committer gates, and report status. Use for tracked implementation work; not for untracked ad-hoc edits."
---

# SDLC

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
- Select the oldest **eligible** active task with `pending` or `changes-requested`. A task with unfinished `Depends on` IDs is not eligible; an integration `type:test` is eligible only when every listed part has review PASS and is held in review. Advance the dependency that unblocks the earliest cluster instead of testing or implementing around it.
- Append an ISO-date history entry for every transition. Never erase older Review/Test Notes.
- Scan every task directory before allocating a BUG/FEAT/TEST ID. Never reuse an ID.
- Preserve ambient WIP. Never use broad staging, destructive cleanup, or parallel write stages.

## Complexity and model routing

Explore before filing. `simple` means one localized seam, a known solution, a small diff, and no unresolved architecture, lifecycle, networking, concurrency, or data-model decision. Otherwise use `standard`.

Work larger than standard must be decomposed before implementation. The architect must turn every hard/multi-seam request into the smallest independent `simple` or `standard` parts, write their dependencies, and create exactly one integration `type:test` task for the resulting cluster. Never leave a task as one broad `standard` item merely because it is difficult.

## Mandatory cluster gate

Create a cluster whenever two or more changes share a runtime boundary, service/image build, data fixture, browser journey, deployment, or one user-visible acceptance flow. This includes a small bug plus a packaging/auth/UI change when their correctness is only meaningful together. Record `**Part of:** <cluster>`, `**Depends on:** <IDs>`, and the single integration-test ID on every part and in the sprint manifest.

- Develop and review each cluster part sequentially. A review PASS means **ready for integration**, not runtime-passed.
- Hold every passing part in `tasks/review`; do not invoke builder/tester, commit, or run a per-part browser/integration test.
- Start builder/tester exactly once, only after every cluster part has a review PASS. Pass the integration-test task, all part paths, and the reviewed static facts. Use one environment manifest for that cluster.
- A cluster test PASS moves every part and its integration test to done, then commits the cluster together. A FAIL returns only implicated parts to active; keep unrelated reviewed parts held in review and rerun the one integration test after rework.
- Do not file a duplicate bug for a failure already owned by a planned cluster part; cross-link the concrete evidence to that part instead.

- Root architect: `gpt-5.6-terra`, medium (project config).
- Simple developer/reviewer: `developer_simple`, `reviewer_simple` (Luna, medium).
- Standard developer/reviewer: `developer_standard`, `reviewer_standard` (Terra, medium).
- Environment: `builder` (GPT-5.4 mini, low).
- Runtime QA: `tester` (Luna, medium).
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

0. First check cluster membership. For an implementation part, hold it in review after PASS; only its cluster `type:test` task may enter runtime test.
1. Move to test, set `in-test`, append history.
2. Spawn `builder`; retain its agent thread identifier.
3. When ready, spawn `tester` with task content/path, builder report verbatim, and reviewer-established static facts.
4. On PASS, FAIL, interruption, or tester error, send teardown to the same builder thread and wait for cleanup.
5. `FAIL`: move to active, set `changes-requested`, record concrete evidence, redevelop.
6. `PASS`: move to done, set `done`, append history, then commit.

Skip builder/tester only if there is no runtime surface and the entire Test Plan was already directly executed (such as documentation-only work or a test-suite-only task). Record the reason.

### Commit

For a standalone task, spawn `committer` with the full done-task file and path. For a cluster, spawn it once with the integration-test file plus every held part; it stages and commits the cluster together. Before every commit, the committer reads global `user.name`/`user.email` and sets the repository-local identity to those exact values; never commit as a placeholder/test identity. The architect never stages or commits. A scope ambiguity or failed clean-HEAD build is a blocker.

### Loop and blockers

Continue sequentially while eligible work exists. Retry a transient agent/tool failure once with a corrected prompt. If the same failure repeats, record a blocker and stop. Fix persistent pipeline instructions/config at their source.

## Sprint behavior

Use an audit/test phase before implementation when current-state facts are unknown. Keep the sprint manifest current, while task frontmatter remains authoritative. When no sprint task remains active/review/test, mark the sprint done and write user-facing Added/Changed/Fixed release notes.

### Scope lock and discovery policy

Before filing any implementation cluster, record a compact scope lock in the
sprint/task: authorized user outcome, explicit non-goals, CI/deployment
authority, the integration-test owner, and required runtime fixture owner.
Do not add a CI platform, hosted service, deployment flow, or adjacent product
surface unless the user explicitly authorized it. Default to local repository
automation only.

Treat a legacy/stale/unrelated discovery as a note, not a new BUG or a source
change. File or fix it only when it blocks the authorized outcome, is already
owned by a planned cluster part, or the user explicitly asks. Ask for direction
before expanding scope; never "clean up" an adjacent retired surface by default.

### Runtime readiness gate

Before an E2E/browser/image-backed part may receive review PASS, prove its
integration test has an explicit safe target: lifecycle owner, unique resource
names/ports/data, ephemeral credentials where needed, exact cleanup, and a
current-image build/start path. If any item is absent, file the smallest
fixture/lifecycle prerequisite first and make the E2E part depend on it. A
shared developer stack is never an implicit test target.

## Context and telemetry

Pass only the task path, current-gate decision, relevant previous-stage verdict, and exact required paths to each agent. The agent reads the task file itself; do not paste its full body, historical telemetry, or raw logs into prompts. The builder report passed to tester is the one exception and must remain concise and evidence-only.

Agents read only frontmatter, Summary, Requirements/Acceptance Criteria,
Affected Areas, Test Plan, and the latest relevant Review/Test Notes. Do not
read or repeat historical notes, raw logs, or telemetry unless the current gate
depends on them. Keep reports to changed paths, commands, verdict, and blocker.

## Context and cost controls

- Do static developer/reviewer work before any environment action. Never spend a builder/tester cycle on a cluster part.
- For image-backed runtime QA, builder records the prior image, builds current source once, starts only the required service, and proves the served image/container is current and healthy before invoking tester. If that gate fails, skip browser QA, restore only the recorded prior service image when needed, and return the owning part for rework.
- Use task-local facts in prompts, not source dumps. Keep implementation/review/test notes to concise findings and paths; raw command logs stay out of task files unless a short excerpt is the evidence.
- Record `n/a` when a tool does not surface a duration or token count; never estimate either. After five completed clusters, compare rework count, duration, and surfaced tokens by role before changing model/effort routing.

Each stage adds one row under `## Pipeline Telemetry`:

`| date | role | model | effort | result | duration | tokens | rework |`

Use only surfaced duration/token data.

## Deterministic environment lifecycle

Builder must use the project helper `scripts/sdlc-environment.sh`: `prepare <manifest> <repo-root>` to ready the shared stack, `smoke` for the baseline check, `record` for every task-owned resource, and `cleanup <manifest>` after testing. Cleanup removes only exact recorded names and never infers ownership from patterns. The shared compose stack is never task-owned.
