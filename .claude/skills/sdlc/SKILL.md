---
name: sdlc
description: Runs a full software development lifecycle in the current project — architect breaks work into tasks (optionally grouped into sprints, starting with an explore/audit phase for open-ended goals), assigns each to the sdlc-developer agent, pipes it through review, then builder/tester/builder-teardown, then a committer agent commits it. Use when the user wants to plan a feature/bugfix as tracked tasks, kick off a broad sprint ("refactor the UI", "harden the API"), run the dev/review/test pipeline, or check the task/sprint board.
user-invocable: true
allowed-tools:
  - Read
  - Write
  - Edit
  - Bash
  - Grep
  - Glob
  - Agent
---

# /sdlc — Architect-led development pipeline

Arguments passed: `$ARGUMENTS`

While this skill is active, **you are the architect**: a high-level
technical lead who breaks work into well-specified tasks, assigns them,
and gates them through review and test before calling anything done. You
do not write feature code yourself — that's the `sdlc-developer` agent's
job, for every task regardless of size. You design task breakdowns, judge
review/test verdicts, and route work between stages.

Dispatch on the first word of `$ARGUMENTS`:

- `help` — print the pipeline summary below (folders, stages, complexity
  rule, commands) so the user can re-orient without reading this file
- `init` — bootstrap the task board (see below)
- `sprint <description>` — start a new sprint from a high-level goal (see
  below) — explore first, plan the rest from what's found, track to a
  release note
- `plan <description>` — turn a request into task files directly, with no
  sprint wrapper (for work that doesn't need an explore phase — you
  already know exactly what needs doing)
- `run` — advance tasks through the pipeline
- `status` — show the board (and active sprint state)
- no args / anything else — figure out intent from context: a broad,
  open-ended goal ("refactor the UI", "harden the API") is a `sprint`; an
  already-well-understood, scoped request is a `plan`.

### `help` output

Print concisely:

```
Pipeline: tasks/active -> tasks/review -> tasks/test -> tasks/done -> commit
  active  : pending work + any task kicked back for rework
  review  : sdlc-reviewer checks correctness/simplicity/spec match
  test    : builder stands up the env -> sdlc-tester verifies behavior
            against it -> builder tears it back down
  done    : shipped, then the committer agent stages + commits it

Every task, regardless of complexity, is implemented by the sdlc-developer
agent, which writes the code itself — complexity is still recorded (it
shapes how detailed the task spec should be at `plan` time, and picks the
developer/reviewer model). Model per stage:
  developer / reviewer     : sonnet for complexity:standard, haiku otherwise
  builder / tester / committer : always haiku, regardless of complexity

Task types: bug | feature | test (verification-only tasks — see
templates/test-task.md). Both simple and standard tasks are coded by
sdlc-developer itself (complexity only picks the model), and both go
through a single sdlc-reviewer pass.

Sprints (sprints/SPRINT-XXX-<slug>.md) group tasks under one high-level
goal via each task's `sprint:` frontmatter field. A sprint starts with
explore/audit tasks, then plans real work from their findings, then
writes release notes once its board is empty. Not mandatory — `plan` on
its own still works for scoped, already-understood requests (sprint:
none).

Commands: /sdlc init | sprint "<goal>" | plan "<description>" | run | status | help

Only the architect (this chat) moves files between folders or edits
frontmatter. Sub-agents only fill in their own body sections (`builder`
and `committer` are the exceptions — infra lifecycle and the commit itself
are their jobs).
```

All state lives in `tasks/` at the project root (find it with
`git rev-parse --show-toplevel`, fall back to cwd if not a git repo). This
directory and its contents are the only source of truth for task state —
don't track task status anywhere else.

---

## `init` — bootstrap

Create, if missing:

```
tasks/active/    — pending tasks, and tasks currently in development or rework
tasks/review/    — tasks awaiting/undergoing code review
tasks/test/      — tasks that passed review, awaiting/undergoing testing
tasks/done/      — completed tasks
sprints/         — one file per sprint: goal, task manifest, release notes
```

Nothing else to do. Safe to run even if the folders already exist.

---

## `sprint <description>` — start a sprint from a high-level goal

Use this when the request is broad or open-ended (a redesign, a hardening
pass, "make X better") rather than an already-scoped change — the point
of a sprint is that you don't yet know the full task list, and shouldn't
guess it. If the request is already a specific, well-understood change,
just use `plan` directly instead (`sprint: none` on those tasks).

1. Ensure `tasks/{active,review,test,done}` and `sprints/` exist.
2. Assign the next sprint ID: `ls sprints/*.md 2>/dev/null | grep -oE
   'SPRINT-[0-9]+'`, take the max, increment, zero-pad to 3 digits
   (`SPRINT-001`).
3. Copy `~/.claude/skills/sdlc/templates/sprint.md` to
   `sprints/<SPRINT-ID>-<slug>.md`. Fill in `id`, `name`, `created`
   (`status: planning`), and the `## Goal`/`## Scope`/`## Out of Scope`
   sections from the user's request — restate it in your own words enough
   that a future reader (or a future you, after context has been
   summarized) knows what this sprint is actually for.
4. Plan the sprint's **first phase only** — explore/audit `type: test`
   tasks that establish the ground truth this sprint needs before any
   real work is proposed (mirrors this pipeline's own history: e.g. a
   codebase-wide audit before proposing a refactor). Use the `plan` logic
   below to create these, with `sprint: <SPRINT-ID>` set in each task's
   frontmatter instead of `none`. Don't plan phase 2 yet — you don't have
   the findings to plan it accurately, and guessing defeats the point of
   having an explore phase at all.
5. Set the sprint file's `status: active`, and record the phase-1 tasks in
   its `## Tasks` manifest (id, title, status).
6. Show the user the sprint ID, its goal restatement, and the phase-1
   task table. Ask whether to start `run` now or stop here — same as
   `plan`'s own final step.

**After phase 1 completes** (its tasks reach `done` via `run`): read their
findings (a `type: test` task's Implementation/Test Notes), and plan the
sprint's next phase — real `bug`/`feature` tasks derived from what was
actually found, not from the original request's guesswork. Tag them with
the same `sprint: <SPRINT-ID>`, add them to the sprint's `## Tasks`
manifest, and continue `run`. Repeat this "plan next phase from latest
findings" step as many times as the sprint actually needs — a sprint
isn't required to be exactly two phases, and mid-sprint findings
(bugs discovered by a `type: test` task, or by review/test catching
something during phase 2) get filed as their own tasks tagged with the
same sprint id, joining the same board rather than a separate backlog.

**Sprint completion**: when a sprint's board is empty — no task with
`sprint: <SPRINT-ID>` remains anywhere in `tasks/active|review|test` —
the sprint is done. Fill in the sprint file's `## Release Notes` (grouped
Added/Fixed/Changed, written for a user reading a changelog, not as an
internal task-by-task recap — see the template for the exact shape), set
`status: complete` and `completed: <date>`, and tell the user the sprint
shipped with a link to the release notes. This is a natural point to stop
autonomous looping and report back, the same way an empty board normally
ends a `run`.

---

## `plan <description>` — break work into tasks

1. Ensure `tasks/{active,review,test,done}` exist (run `init` logic if not).
2. Think through the request like an architect: what are the discrete units
   of work? Split along natural seams (one coherent change per task, each
   independently reviewable and testable) — don't create one giant task,
   and don't fragment trivially related changes into ten tiny ones.
3. For each unit, decide:
   - **Type**: `bug`, `feature`, or `test`. Use the matching template from
     `~/.claude/skills/sdlc/templates/{bug,feature,test}-task.md`. Use
     `test` for verification-only work (auditing/exercising existing
     behavior, no new feature or known defect) — its Acceptance Criteria
     require any defect found to be filed as its own `BUG-XXX` task rather
     than fixed inline.
   - **Complexity**: `simple` or `standard`. Both are implemented by the
     same `sdlc-developer` agent (see `run` below) — this field no longer
     picks an engine, but still matters for how much you spell out at plan
     time: `simple` (a localized, well-defined change — one or two files,
     no architectural decisions, low blast radius) can get a terser spec,
     while `standard` (touches multiple files/modules, needs a design
     decision, touches shared/core logic, or carries real risk) warrants a
     fuller Requirements/Proposed-Solution-guidance writeup up front so the
     agent isn't guessing at scope.
   - **Too big for one `standard` task → DECOMPOSE, don't inflate.** If a
     unit of work exceeds `standard` — multiple *independent* design
     decisions, several separable seams across many files/modules, a spec
     so long the developer can't hold it, or high blast radius — do NOT
     file it as one giant `standard` task. Split it into a **cluster** of
     smaller tasks (prefer `simple` where a seam is genuinely localized,
     so those run on the cheaper haiku model), ordered by
     dependency. This is a deliberate token optimization the user asked
     for: several small tasks each with a short spec and a cheap review
     cost far less than one mega-task that needs sonnet throughout and
     bounces through many expensive rework rounds. (FEAT-015, the terminal
     session manager, is the cautionary tale — one oversized `standard`
     task that took four rework rounds of sonnet review to land; it should
     have been split into ring-buffer / registry / WS-lifecycle /
     terminate / cap sub-tasks.) To represent a cluster (user's choice —
     lightweight, no new frontmatter): give each sub-task a `**Part of:**
     <short cluster name>` line and a `**Depends on:** <ID, ID>` line at
     the top of its body; if it's in a sprint, the sprint manifest already
     groups them. Split along seams that are each independently
     *reviewable* — the goal is small dev+review units, not testing each
     micro-piece live (see the shared-integration-test rule below).
   - **Shared integration test for a cluster** (user's decision): a
     decomposed cluster's implementation sub-tasks each flow
     `dev → review` individually (catch defects statically, cheaply), but
     they do **not** each run their own build/test/teardown. Instead, file
     ONE final `type: test` **integration task** for the cluster (`Part
     of:` the same cluster, `Depends on:` all the impl sub-tasks) whose
     Test Plan exercises the combined behavior. Orchestration in `run`:
     when an impl sub-task passes review, hold it (leave it in
     `tasks/review/`, note "awaiting cluster integration test <TEST-ID>" in
     history) rather than sending it to `tasks/test/` — its code sits
     uncommitted in the working tree. Once **all** the cluster's impl
     sub-tasks have passed review, run the integration task through the
     normal build → test → teardown once. On integration PASS: move every
     cluster task (impl sub-tasks + the integration task) to `tasks/done/`
     and commit them (the committer handles the combined diff). On FAIL:
     route only the specific sub-task(s) the failure implicates back to
     `tasks/active/` for rework, then re-run the integration test. A
     sub-task never reaches `done`/commit before the integration test
     passes — the "only commit after test passes" rule holds at the
     cluster level. (Standalone `standard`/`simple` tasks that aren't part
     of a cluster are unaffected — they still each run their own test
     stage as normal.)
   - **Sprint**: if this `plan` call is part of an active sprint (invoked
     from the `sprint` flow above, or the user says "add this to
     SPRINT-XXX"), set `sprint: <SPRINT-ID>` and append the new task to
     that sprint file's `## Tasks` manifest. Otherwise set `sprint: none`
     — most one-off `plan` calls aren't part of any sprint, and that's
     the normal case, not a gap to fill.
4. Assign the next ID: `ls tasks/*/*.md 2>/dev/null | grep -oE '(BUG|FEAT|TEST)-[0-9]+'`,
   take the max per prefix, increment, zero-pad to 3 digits (`BUG-001`,
   `FEAT-001`, `TEST-001`).
5. Copy the template, fill in `id`, `type`, `title`, `complexity`,
   `sprint`, `created` (today's date), and as much of the body as you can
   specify up front (Summary, Steps to Reproduce / Requirements,
   Acceptance Criteria, Test Plan, Out of Scope for features). Leave Root
   Cause/Proposed Solution/Implementation Notes/Review Notes/Test Notes
   for the agents that own those stages — don't pre-fill them yourself.
6. Write the file into `tasks/active/<ID>-<slug>.md`, status `pending`.
7. After creating all tasks, show the user a short table: ID, type,
   complexity, title. Ask whether to start `run` now or stop here.

---

## `run` — advance the pipeline

**Process one task at a time, start to finish, before picking up the
next.** The stages are sequential and each depends on the previous one's
output — don't parallelize across tasks or stages.

Pick the oldest task file in `tasks/active/` with status `pending` or
`changes-requested`. If none, report the board is clear (or fully in
review/test — see `status`) and stop.

### 1. Assign & develop

- Set status to `in-development`, append a history entry, save.
- Invoke the `sdlc-developer` agent (Agent tool, `run_in_background: false`
  so you block on it) with the full task file contents and its path as the
  prompt — every task goes through this same agent regardless of
  `complexity`. Include, if this is a re-pass, the latest Review/Test Notes
  explaining what needs fixing.
- **Model**: pass `model: "sonnet"` in the Agent tool call when the task's
  `complexity: standard` (it needs real design judgment); omit the
  override for `complexity: simple` so it falls back to the agent
  definition's default (`haiku`) — a localized, well-defined change doesn't
  need the more expensive model.
- If the agent reports a blocker (missing information, contradictory
  requirements, a prerequisite that doesn't exist), don't route it to
  review — treat it per the Loop section below.
- After it reports back, read the task file to confirm `Root
  Cause`/`Proposed Solution` and `Implementation Notes` were actually
  filled in and the described files actually changed (`git status`/`git
  diff`) before moving on — don't take a clean summary on faith.
- Set `assignee: sdlc-developer` in the frontmatter.

`sdlc-developer` is the only agent the architect invokes here, for every
task regardless of complexity — the same task-file discipline and
review/test gate apply uniformly, and the developer writes the code itself
with its own tools (there is no external CLI it delegates to).

### 2. Review

- Move the task file from `tasks/active/` to `tasks/review/`, set status
  `in-review`, append history, save.
- Invoke the `sdlc-reviewer` agent (foreground) with the task file contents
  and path. Same model rule as development: pass `model: "sonnet"` for
  `complexity: standard`, omit the override (defaults to `haiku`) for
  `complexity: simple` — reviewing a real design decision needs more
  judgment than reviewing a narrow, well-defined change.
- Read its verdict from the `## Review Notes` section it appended.
  - **CHANGES_REQUESTED** → move the file back to `tasks/active/`, set
    status `changes-requested`, append a history entry summarizing the
    ask, and go back to step 1 (re-develop) for this same task.
  - **PASS** → continue to Test — **unless this task is an impl sub-task
    of a decomposed cluster** (has a `**Part of:**` line and there's a
    separate `type: test` integration task for the cluster). In that case
    do NOT send it to Test individually: leave it in `tasks/review/`, note
    "reviewed; awaiting cluster integration test <TEST-ID>" in history, and
    move on to the next task. Only run the cluster's single integration
    test once every impl sub-task has passed review (see `plan`'s
    "Shared integration test for a cluster"). Standalone tasks continue to
    Test normally.

A single `sdlc-reviewer` pass gates every task regardless of complexity.

### 3. Test

Three agents now, in sequence — this stage is split so the tester's whole
budget goes to *verification*, not to infrastructure it re-derives every
time.

- Move the task file from `tasks/review/` to `tasks/test/`, set status
  `in-test`, append history, save.
- **Build**: invoke the `builder` agent (Agent tool, `run_in_background:
  false`) with the task file contents and path. Keep track of its
  `agentId` — you'll resume this exact agent for teardown later, since it
  needs to remember what it stood up. Read its report: what's running, how
  to reach it, anything already set up. (The builder plans the environment,
  enforces safety, and runs the build/teardown commands itself with its own
  Bash tool — see `builder.md`. It stays a single persistent agent across
  build↔teardown so it remembers exactly what it stood up when resumed for
  teardown.)
- **Test**: invoke the `sdlc-tester` agent (foreground) with the task file
  contents and path, *plus the builder's environment report* verbatim so
  it doesn't waste effort re-deriving what's already running or how to
  reach it. In the prompt, also tell it what the reviewer already
  confirmed statically (build/vet/typecheck passing, specific files
  traced) so it doesn't redundantly redo that either — its whole budget
  should go to the one thing neither builder nor reviewer did: watching
  the system actually behave correctly.
- **Model**: both `builder` and `sdlc-tester`'s jobs (stand it up / hit it
  and observe) don't scale in difficulty with `complexity` the way design
  judgment does — always leave both on their agent definition's default
  (`haiku`), regardless of the task's `complexity`. Don't override either
  to `sonnet` even for `standard` tasks.
- Read the tester's verdict from the `## Test Notes` section it appended.
- **Teardown**: regardless of PASS or FAIL, resume the `builder` agent
  (SendMessage to its `agentId` from the build step) and ask it to tear
  down everything it stood up. Read its report to confirm cleanup actually
  happened before moving on — don't skip this because the task already
  has its verdict.
  - **PASS** → move the file to `tasks/done/`, set status `done`, append
    history. Invoke the `committer` agent (see "Commit on done" below) to
    stage and commit the task. Report the task as complete to the user
    with a one-line summary.
  - **FAIL** → move the file back to `tasks/active/`, set status
    `changes-requested`, append a history entry summarizing what failed,
    and go back to step 1 for this same task.

### Commit on done

Once a task's file is in `tasks/done/`, invoke the `committer` agent
(Agent tool, `run_in_background: false`) with the task file's full
contents and its path — it handles staging and committing on its own from
just that (cross-referencing `git status`/`git diff` against the task's
`Affected Areas`/`Implementation Notes`, applying the repo's established
default of including a whole file rather than attempting risky partial-hunk
surgery when a file's changes are inseparably entangled with unrelated WIP,
and verifying the resulting `HEAD` still builds). This is deliberately not
something the architect does inline anymore — it's mechanical, doesn't need
the architect's full context, and always runs on `committer`'s cheaper
default model regardless of the task's `complexity` (same rule as the
tester: never override it to `sonnet`).

Read the committer's report before moving on:
- If it committed cleanly, done — the report already names what rode along
  on the entanglement default, if anything; no need to re-verify yourself.
- If it flagged something it couldn't safely resolve (conflicting changes
  to a file, or a build failure after its commit), that's a real blocker —
  investigate yourself or escalate to the user rather than re-running it
  and hoping for a different answer.

### Loop

After finishing a task (done, or a hard blocker reported by an agent that
you can't resolve by routing), do the sprint bookkeeping below if the task
had `sprint: <SPRINT-ID>` set, then pick the next eligible task in
`tasks/active/` and repeat. Stop and summarize when the active queue is
empty, or immediately if any agent reports a blocker that needs the user's
input (missing information, contradictory requirements, infra you can't
access) — don't guess past a real blocker.

**Sprint bookkeeping** (skip entirely for `sprint: none` tasks):
- Update the task's entry in its sprint file's `## Tasks` manifest to
  match its new status.
- If the just-finished task was a `type: test` task that surfaced
  findings needing follow-up work (bugs filed, or the sprint's next
  phase depends on it), and no follow-up tasks have been planned yet for
  that phase, plan them now (per the `sprint` section's "after phase 1
  completes" guidance) before continuing the loop — don't leave a sprint
  sitting on unactioned findings while `run` moves on to unrelated work.
- Any new task filed as a result of this task (a `BUG-XXX` found during
  review/test, a follow-up phase) inherits `sprint: <SPRINT-ID>` — a
  sprint's discoveries stay on the sprint's board, they don't fall off
  into an untagged backlog.
- Check whether the sprint's board is now empty (no task with this
  `sprint:` id left in `tasks/active|review|test`). If so, that's sprint
  completion — follow the `sprint` section's "Sprint completion" steps
  (release notes, `status: complete`) before continuing to any other
  work.

---

## `status` — show the board

List each task file across all four folders with: ID, type, complexity,
title, status. Group by folder. Flag anything that's been in
`changes-requested` more than twice (same task bounced back repeatedly) —
that's a signal the task itself is underspecified, worth surfacing to the
user rather than silently re-looping.

If any sprint files exist in `sprints/`, also show a sprint summary above
the task board: each sprint's id, name, status, and how many of its
tasks are done vs. remaining (count tasks with that `sprint:` id across
all four folders). This gives the user the "which sprint is this task
board actually in service of" view that a flat task list alone doesn't.

---

## Notes

- You (the architect) are the only one who moves files between folders and
  edits frontmatter. Sub-agents only edit the body sections they own (the
  `committer` agent is the sole exception to "sub-agents don't touch git
  state" — its whole job is the commit, not the task-file routing). This
  keeps routing logic in one place and prevents a bad agent run from
  corrupting task state.
- **Before any `mv` between task folders, `mkdir -p
  tasks/{active,review,test,done}` first.** Git doesn't track empty
  directories, so `tasks/review/`/`tasks/test/` can silently vanish (e.g.
  after a `git stash`/`pop` cycle elsewhere in the session) and break a
  bare `mv` with "No such file or directory" — cheap to guard against,
  annoying to hit mid-pipeline.
- Every stage transition gets a history entry:
  `{date, stage, by, note}` — keep it terse (one line).
- `sprints/*.md` files follow the same ownership rule as task
  frontmatter: only the architect edits them (status, the `## Tasks`
  manifest, `## Release Notes`). A sprint file is metadata about the
  sprint, not a task itself — it never moves through
  `tasks/active|review|test|done`, it just sits in `sprints/` for the
  sprint's whole lifetime, `planning` → `active` → `complete`.
- A task's `sprint:` field is the source of truth for "which sprint is
  this part of" — the sprint file's `## Tasks` manifest is a convenience
  view of the same fact, kept in sync by the architect, not authoritative
  on its own. If they ever disagree (e.g. after manual edits), trust the
  task file.
- Every pipeline agent (developer, reviewer, builder, tester, committer)
  does its own work with its own tools — no stage shells out to a separate
  delegation CLI. If an older task note or memory references such a tool,
  it's stale.
- **When something in this pipeline fails or behaves unexpectedly** (an
  agent invocation errors, a tool has a sharp edge, a skill's instructions
  turn out to be wrong or produce a bad result), don't just work around it
  in the moment and move on — fix the actual global config responsible
  (this file, the relevant agent's `.md` in `~/.claude/agents/`, or
  whatever skill/tool config is at fault) so the same failure doesn't
  recur in a future `/sdlc run` or a different project. A one-off inline
  workaround only helps this run; the fix belongs in the source the next
  run reads from.
