---
id: <SPRINT-ID>
name: <SHORT NAME>
status: planning
created: <DATE>
completed:
---

## Goal
<The user's original request, in their own words plus your restatement of
what "done" means for this sprint. This is the thing every task filed
under this sprint traces back to — if a proposed task doesn't serve this
goal, it probably belongs in a different sprint or the general backlog,
not this one.>

## Scope
<What's explicitly in bounds for this sprint.>

## Out of Scope
<What's explicitly NOT part of this sprint, even if related — prevents
scope creep as exploration surfaces tempting side quests.>

## Phases
<How this sprint's work is sequenced, e.g.:
1. Explore/audit (`type: test` tasks) — understand the current state
   before proposing changes.
2. Findings-driven implementation (`type: bug`/`feature` tasks) — planned
   *after* phase 1 lands, using its actual findings, not guessed upfront.
3. ...
Not every sprint needs multiple phases — a small, well-understood sprint
can skip straight to phase 2-style tasks. Use judgment.>

## Tasks
<Running manifest, updated by the architect every time a task is filed or
changes status. One line per task: id, title, status. This is a
convenience view — `sprint: <SPRINT-ID>` in each task file's own
frontmatter is the source of truth if this list and reality ever drift.>
- <ID> — <title> — <status>

## Release Notes
<Filled in by the architect when the sprint's board is fully clear (no
task tagged with this sprint's id remains in active/review/test). Group
by what actually shipped, not by task type:
### Added
### Fixed
### Changed
Keep it user-facing — what changed for someone using the product, not an
internal diff summary. Link back to task IDs for anyone who wants detail,
but the prose itself should read like a changelog entry, not a task
list.>
