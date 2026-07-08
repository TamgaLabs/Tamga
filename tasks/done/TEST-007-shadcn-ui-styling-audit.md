---
id: TEST-007
type: test
title: shadcn/component usage & styling audit
status: done
complexity: standard
assignee: sdlc-developer
created: 2026-07-07
history:
  - {date: 2026-07-07, stage: created, by: architect, note: "task created — Phase 3 (frontend UI), starts only after Phase 1+2 confirm the backend and its contract are sound; this is an audit only, the visual refactor itself is planned as follow-up FEAT tasks once this lands"}
  - {date: 2026-07-07, stage: in-development, by: architect, note: "assigned to sdlc-developer; Phase 1 and Phase 2 both fully done/committed (TEST-001..006, BUG-010..021, FEAT-013)"}
  - {date: 2026-07-07, stage: in-review, by: architect, note: "dev complete: full findings document across 7 buckets (color tokens, buttons, modals/dialogs, layout shell, cards, data display, missing primitives) plus a clean components/ui/ primitive-layer verdict; architect independently spot-checked the highest-priority finding (text-accent contrast bug, all 4 file:line usages confirmed exact); no code touched"}
  - {date: 2026-07-07, stage: done, by: architect, note: "sdlc-reviewer PASS: independently recomputed WCAG math, spot-checked 10 findings across all 7 buckets plus the primitive-layer verdict, all matched exactly. Builder/tester ceremony skipped per TEST-005 precedent (pure documentation audit, no runtime surface). Reviewer flagged pre-existing uncommitted frontend WIP (partial badge/card/input rewrite, new radio-group/switch primitives, frontend-refactor.md) not owned by any task in this sprint - noted for the user, not this task's concern. This closes the planned sprint: Phase 1-3 (TEST-001..007) all done, plus every bug/feature found along the way."}
---

## Summary
Audit the entire frontend (`frontend/src/app/**`, `frontend/src/components/**`)
for how consistently it actually uses shadcn/ui: are components built from
shadcn primitives with proper variants, or hand-rolled with ad-hoc
Tailwind classes that duplicate what a shadcn primitive already does?
Where is styling inconsistent (spacing scale, color tokens, dark-mode
handling)? This audit's findings become the backlog for the actual visual
refactor (target bar: a cloud-console product like AWS/Huawei Cloud/
Vercel/Dokploy — functional and information-dense, but polished) — this
task does not change any UI code itself.

## Scope
- Every file in `frontend/src/components/ui/` — confirm each is an
  unmodified-or-deliberately-extended shadcn primitive, not something that
  drifted from the shadcn pattern without reason
- Every page under `frontend/src/app/(main)/**` and `frontend/src/app/(auth)/**`
  — for each, note: which shadcn components it uses, any custom
  one-off component that duplicates an existing shadcn primitive
  (e.g. a hand-built modal instead of `alert-dialog`/`dialog`), any
  inconsistent variant usage (e.g. three different "danger button" stylings
  instead of one `variant="destructive"`), any raw hex/Tailwind color
  literal instead of a theme token from `globals.css`/`tailwind.config.ts`
- `frontend/src/components/sidebar.tsx` and any other shared shell/layout
  component — the overall navigation/shell structure, since that's the
  first thing that reads as "cloud console" or not

## Out of Scope
- Actually changing any component, page, or style (that's follow-up FEAT
  work planned after this audit's findings land)
- Backend/API concerns (Phases 1-2)

## Test Approach
Static source audit, no code changes:
1. Read every file in `frontend/src/components/ui/` (14 files) end to end and
   diffed each against the canonical upstream shadcn/ui output for that
   primitive (variant shape, `cva` usage, class list) to spot drift vs.
   deliberate/justified extension.
2. Read `frontend/src/app/globals.css` and `frontend/tailwind.config.ts`
   first to establish the actual token system (CSS vars + Tailwind color/
   radius mappings) before judging anything else as "hardcoded" vs.
   "tokenized". Also checked `frontend/components.json` (shadcn CLI config)
   to confirm base theme (`slate`) and that missing primitives can be added
   mechanically via `npx shadcn add <name>`.
3. Read every page under `frontend/src/app/(main)/**` and
   `frontend/src/app/(auth)/**` (13 files) plus `components/sidebar.tsx`,
   `components/agent-terminal.tsx`, `lib/theme.tsx`, `lib/utils.ts`,
   `app/layout.tsx` and `(main)/layout.tsx`, noting per page: which shadcn
   primitives it uses, any hand-rolled component duplicating an existing
   primitive, inconsistent variant usage for the same affordance, and any
   color/spacing value that bypasses the token system.
4. Cross-checked specific patterns with targeted greps across
   `app/` + `components/` (raw Tailwind color literals e.g.
   `bg-red-500`/`bg-black`, bare `rounded` vs. `rounded-{sm,md,lg}`,
   `confirm()`/`alert()` native dialogs, `variant="destructive"` vs.
   `text-destructive` ad-hoc styling, `text-accent`/`text-success` used as
   foreground text, duplicated `statusVariant` maps, `cursor-pointer` card
   hover treatments) to confirm findings weren't isolated to the files read
   first and to get an exact count/file:line list for each.
5. For the single highest-severity finding (`text-accent` used as
   foreground text color on 4 pages), didn't rely on eyeballing — computed
   the actual WCAG contrast ratio of `--accent` against `--background` from
   the HSL values in `globals.css` (light mode: 1.05:1, dark mode: 1.37:1;
   WCAG AA minimum for text is 4.5:1) to confirm it's a genuine
   near-invisible-text defect, not a subjective style opinion. Did not
   start the dev server for a full visual pass — the contrast computation
   plus reading actual rendered class output on every page gave a more
   precise, reproducible result than eyeballing rendered pages would have,
   and every other finding was confirmed by reading the exact JSX/props,
   not by inference.
6. Findings below are grouped into buckets, each independently scoped
   enough to become one follow-up FEAT task.

## Affected Areas
<A findings document only.>

## Acceptance Criteria
- [ ] Every file in `components/ui/` is checked against its upstream
      shadcn source pattern (variants, `cva` usage, prop shape) and any
      drift is noted
- [ ] Every page is checked for hand-rolled components that duplicate an
      existing shadcn primitive, with the specific file:line and which
      primitive should replace it
- [ ] Every instance of a hardcoded color/spacing value that bypasses the
      theme's tokens is noted with file:line
- [ ] Findings are grouped into concrete, independently-actionable buckets
      (e.g. "buttons," "modals/dialogs," "layout shell," "forms") so each
      can become its own follow-up `FEAT` task with a clear scope
- [ ] No UI code is modified as part of this task

## Test Plan
This is a read-and-document task — "testing" here means confirming the
findings are accurate by cross-checking each flagged instance against the
actual rendered component (read the JSX/props, and where feasible start
the frontend dev server and visually confirm what's flagged actually
looks inconsistent).

## Implementation Notes
No UI code was changed — this task's output is the findings document below.
Token system baseline used throughout: `frontend/src/app/globals.css`
defines the shadcn "slate"-based CSS vars (`--background`, `--accent`,
`--success` (custom addition), `--code-block` (custom addition), etc.);
`frontend/tailwind.config.ts` maps them to Tailwind color/radius utilities.
`frontend/components.json` confirms the shadcn CLI is set up (`baseColor:
"slate"`), so any missing primitive noted below can be added with
`npx shadcn add <name>` rather than hand-written.

### `components/ui/` primitive audit result
All 14 files (`alert-dialog`, `badge`, `button`, `card`, `checkbox`,
`dropdown-menu`, `input`, `label`, `radio-group`, `scroll-area`, `select`,
`separator`, `switch`, `tabs`) are essentially unmodified shadcn output —
correct `cva` usage, standard prop/variant shape, `React.forwardRef` +
`displayName` boilerplate all present. Only two points of drift, both
covered in the buckets below: Badge's extended variant set (bucket 1) and
Button's outline-variant border token (bucket 2). This part of the codebase
is in good shape and is not a priority for the follow-up refactor.

### Bucket 1 — Color tokens (highest priority)
- **`text-accent` used as foreground text color — near-invisible text,
  confirmed via WCAG contrast math, not just a style opinion.**
  `--accent` (`globals.css:19,58`) is defined as a *background* tint token
  (very close in lightness to `--background`), not a text color. Computed
  contrast of `text-accent` against `bg-background`: **1.05:1 in light
  mode, 1.37:1 in dark mode** (WCAG AA minimum for normal text is 4.5:1).
  Used as foreground text at:
  - `frontend/src/app/(main)/dashboard/page.tsx:71` (project domain)
  - `frontend/src/app/(main)/projects/[id]/page.tsx:183` (domain) and
    `:362` (env var key)
  - `frontend/src/app/(main)/code/[id]/page.tsx:119` (active file-tree
    label)
  Fix: use `text-primary` (or a new dedicated "highlight/link" token) for
  these instead of repurposing `--accent`.
- **`text-success` repurposed as a generic "terminal green" text color**,
  not an actual status/semantic use: `frontend/src/app/(main)/
  containers/[id]/page.tsx:151,167` and `frontend/src/app/(main)/
  projects/[id]/page.tsx:228` (`bg-code-block ... text-success` for
  `<pre>` log/inspect output). Not a contrast bug (green-on-dark reads
  fine) but a semantic misuse of a status token for an unrelated "terminal
  output" style. Fix: introduce a dedicated token (e.g.
  `--terminal-foreground`) or a `.terminal-output` utility class instead of
  reusing `success`.
- **Badge `warning` variant hardcodes raw Tailwind color literals** instead
  of a theme token: `frontend/src/components/ui/badge.tsx:21` —
  `"bg-yellow-500/20 text-yellow-600 dark:text-yellow-400"`. Every other
  Badge variant (`success`, `destructive`, `error`, `info`) is built from a
  CSS var; `warning` is the only one bypassing the token system, and there
  is no `--warning` var in `globals.css` to point it at. Fix: add
  `--warning`/`--warning-foreground` CSS vars and use them here.
- **Badge `destructive` vs. `error` variants are redundant/confusingly
  named**: both are semantically "red/danger" (`badge.tsx:15-16` solid
  `bg-destructive`, `badge.tsx:22-23` soft `bg-destructive/20`), but
  `destructive` is never actually used anywhere in the app (grep across
  `app/`) — every call site uses `error` for failure states
  (`dashboard/page.tsx:11-17`, `projects/[id]/page.tsx:47-53,247`,
  `containers/page.tsx:38-43`). Fix: rename to something that captures
  "solid" vs. "soft" intent (e.g. keep `destructive` as the solid one, rename
  `error` to `destructive-soft`), or drop the unused one.
- Low severity: `frontend/src/components/agent-terminal.tsx:68` hardcodes
  `bg-black` for the terminal container background. Arguably intentional
  (xterm always renders on black regardless of app theme) but still a
  literal, not a token — worth a one-line confirmation in the follow-up
  task rather than silently carrying it forward.

### Bucket 2 — Buttons
- `frontend/src/components/ui/button.tsx:17` outline variant uses
  `border border-border`, while `Input` (`input.tsx:11`) and `Select`
  (`select.tsx:22`) both correctly use `border-input` for the same "form
  control border" semantic. `--border` and `--input` currently resolve to
  the same value so there's no visible difference today, but the variant
  has drifted from the canonical shadcn pattern and from its sibling
  primitives. Fix: change to `border-input` for consistency.
- **Two co-existing "danger button" conventions**, each internally
  consistent but neither codified as a real `cva` variant:
  1. Solid `variant="destructive"` for primary/high-emphasis destructive
     actions — `settings/page.tsx:195` (Prune All), `projects/[id]/
     page.tsx:215` (Delete project), `containers/[id]/page.tsx:132`
     (Remove).
  2. `variant="ghost" className="text-destructive"` repeated verbatim 6x
     for row-level delete triggers — `settings/page.tsx:331,447,664,779`,
     `projects/[id]/page.tsx:365`, plus the equivalent
     `DropdownMenuItem className="text-destructive"` in
     `containers/page.tsx:174`.
  Not a bug (the two-tier convention is reasonable for a cloud console:
  primary destructive action vs. inline row action), but pattern 2 should
  be promoted to a real `buttonVariants` entry (e.g. `variant="ghost-destructive"`)
  so it isn't re-typed by hand at every call site.
- `frontend/src/app/(main)/code/[id]/page.tsx:56` uses the native browser
  `confirm("Discard unsaved changes?")` instead of `AlertDialog` — the one
  spot in the app that doesn't use the shadcn confirm pattern that's used
  consistently everywhere else (8+ other delete/prune/discard flows all use
  `AlertDialog`).

### Bucket 3 — Modals/Dialogs & hand-rolled forms
- Only `AlertDialog` (confirm-style) exists in `components/ui/`; there is
  no generic `Dialog`. As a result, every "Add X" / "Edit X" form in
  `settings.tsx` is a hand-rolled inline-expand block, not a real modal:
  `frontend/src/app/(main)/settings/page.tsx:289` (API key form),
  `:421` (agent provider form), `:622` (git credential form), `:755`
  (whitelist domain form) — four near-identical, independently
  hand-maintained copies of
  `<div className="space-y-2 p-3 border border-border rounded bg-card">`.
  Fix: add `Dialog` via `npx shadcn add dialog` and consolidate these into
  a shared create/edit dialog pattern (or at minimum a shared
  `<InlineFormCard>` wrapper if inline-expand is kept intentionally instead
  of a modal).
- All 4 of those hand-rolled form containers, plus the 3 `<pre>` log/
  inspect blocks (`containers/[id]/page.tsx:151,167`,
  `projects/[id]/page.tsx:228`), use a **bare `rounded`** class (Tailwind's
  default 0.25rem) instead of the `rounded-sm`/`rounded-md`/`rounded-lg`
  scale derived from `--radius` that every other rounded element in the app
  uses (9x `rounded-md`, 6x `rounded-sm`, 3x `rounded-lg` elsewhere). 7
  sites total bypass the radius token scale.

### Bucket 4 — Layout shell
- No shared `PageHeader`/breadcrumb component: every page hand-rolls its
  own `<h1 className="text-2xl font-bold">...</h1>` combo with a different
  structure each time — compare `dashboard/page.tsx:42-45`,
  `containers/page.tsx:108-120`, `code/page.tsx:42`,
  `projects/[id]/page.tsx:90-101`, `settings/page.tsx:120`, and the
  `Button variant="ghost"` + `&larr;` back-link pattern repeated
  differently in `dashboard/new/page.tsx:41-43`, `containers/[id]/
  page.tsx:110-112`, `projects/[id]/page.tsx:91-93`. Good candidate for a
  shared `PageHeader` component (title, optional description/badge,
  optional back-link, optional action button as props).
- Every page picks its own arbitrary content max-width with no shared
  convention: `max-w-6xl` (dashboard, containers), `max-w-2xl`
  (dashboard/new), `max-w-4xl` (code list), `max-w-5xl` (projects/[id]),
  `max-w-3xl` (settings). `code/[id]/page.tsx` (the IDE) correctly opts out
  entirely (`h-full flex flex-col`, no padding/max-width, appropriate for a
  full-bleed editor) — but the other six each invented their own number.
- `min-h-screen` is redundantly re-declared on some page roots
  (`dashboard/page.tsx:41`, `dashboard/new/page.tsx:40`,
  `projects/[id]/page.tsx:90`) even though `(main)/layout.tsx:5` already
  wraps every page in `<div className="flex min-h-screen">` — other pages
  (`containers/page.tsx:107`, `code/page.tsx:41`, `settings/page.tsx:119`)
  correctly omit it. Purely inconsistent, no visual bug, but worth
  cleaning up alongside the PageHeader work.
- Sidebar width (`w-56` / 224px) is a magic number duplicated in two
  files with no shared source of truth: `frontend/src/components/
  sidebar.tsx:31` (`fixed ... w-56`) and `frontend/src/app/(main)/
  layout.tsx:7` (`ml-56`).
- `frontend/src/components/sidebar.tsx` is a single fixed nav column with
  no topbar/breadcrumb strip at all, and no reserved space for an org/
  project switcher or search — the biggest gap vs. the stated cloud-console
  target (AWS/Huawei Cloud/Vercel/Dokploy all pair a slim sidebar with a
  topbar for context + search + account). Logout is a plain `Button
  variant="ghost"` stacked at the bottom of the nav list
  (`sidebar.tsx:72-79`) rather than under a user avatar/profile
  `DropdownMenu`, despite `DropdownMenu` already existing in
  `components/ui/` and being used elsewhere (`containers/page.tsx`).

### Bucket 5 — Cards / clickable rows
- The "clickable card that navigates to a detail page" affordance uses two
  different hover treatments for the identical interaction: `dashboard/
  page.tsx:59` uses `hover:border-muted-foreground`, while `containers/
  page.tsx:134` and `code/page.tsx:53` use `hover:bg-muted/50`. Pick one.

### Bucket 6 — Data display (status badges)
- The `statusVariant` status-string → Badge-variant lookup object is
  duplicated verbatim between `dashboard/page.tsx:11-17` and
  `projects/[id]/page.tsx:47-53` (identical project-status set); `containers/
  page.tsx:38-43` has a third, appropriately different mapping for
  container states. The two identical copies should be extracted to a
  shared constant (e.g. `lib/status.ts`) — not a visual inconsistency today
  since they're identical, but a maintenance hazard (one could drift from
  the other on the next status addition) worth fixing alongside the visual
  refactor.

### Bucket 7 — Missing shadcn primitives (gap vs. cloud-console target)
Not present in `components/ui/` today, and would remove hand-rolled
equivalents noted above once added (`npx shadcn add <name>`, config already
set up correctly in `components.json`):
- `Dialog` — generic modal, needed for bucket 3's inline-form consolidation.
- `Skeleton` — every loading state in the app is currently a literal
  `"Loading..."` text string (`dashboard/page.tsx:48`, `containers/
  page.tsx:123`, `code/page.tsx:45`, `containers/[id]/page.tsx:115`,
  `settings/page.tsx:192`) with no visual placeholder.
- `Toast`/`Sonner` — all errors are rendered as inline
  `<p className="text-destructive">` text next to the triggering form
  (consistent, but no ephemeral success/failure feedback for actions like
  Restart/Prune/Save that don't have an inline slot).
- `Tooltip`, `Table`, `Popover`, `Avatar` — not currently needed anywhere
  specific, but standard cloud-console building blocks likely to come up
  once the visual refactor starts (e.g. `Avatar` for the user-menu gap in
  bucket 4, `Table` if container/deployment lists grow denser columns).

### Forms — no significant findings
Every form in the app (`dashboard/new`, `login`, and every settings card)
consistently uses the `Label` + `Input` + `space-y-2` wrapper pattern with
`space-y-4`/`space-y-3` between fields. No hand-rolled inputs, no
inconsistent label styling. The only forms-adjacent issue is the duplicated
inline-form-card wrapper already covered in bucket 3.

## Review Notes

### 2026-07-08 — reviewer pass

**Verdict: PASS**

This is a static-audit (`type: test`) task and no frontend code changes were
expected. Confirmed via `git status`/`git diff` plus `stat` mtimes that the
frontend files currently showing as modified/untracked in the working tree
(`badge.tsx`, `card.tsx`, `input.tsx`, `sidebar.tsx`, `(main)/layout.tsx`,
`globals.css`, `tailwind.config.ts`, `login/page.tsx`, `code/page.tsx`,
`dashboard/new/page.tsx`, `lib/utils.ts`, plus new `radio-group.tsx`,
`switch.tsx`, `components.json`, `frontend-refactor.md`, several
`qa-debug*.js` scratch files) all share one identical mtime
(`2026-07-08 14:49:55`), ~14 minutes before the task file itself was last
touched (`15:03:40`) — this is pre-existing ambient WIP from an earlier,
untracked frontend-refactor effort (see `frontend-refactor.md` at repo
root, and note `tasks/active/` and `tasks/test/` are both empty, so there's
no SDLC task claiming this work). It predates TEST-007 and this task did
not introduce, touch, or claim credit for it. `AGENTS.md`/`Caddyfile`/
`plan.md` diffs are likewise unrelated ambient state. **No genuine scope
creep — outside the task file, the diff is empty.**

One open item worth flagging to the architect (not a defect in this task):
this ambient dirty WIP already contains a partial rewrite of `badge.tsx`,
`card.tsx`, and `input.tsx` to canonical `cva`/`forwardRef` shadcn output,
plus two new primitives (`radio-group.tsx`, `switch.tsx`) not yet in any
tracked task. The audit (correctly, since it's a static read of what's
actually on disk) counted 14 `components/ui/` files and its "in good
shape" verdict is partly graded against this uncommitted state rather than
`HEAD`. Since this WIP isn't owned by any active/test task, it should
either be formally picked up by a task or explicitly reconciled/discarded
before the follow-up FEAT work starts, or the follow-up FEAT tasks may
find the codebase doesn't match this findings doc's file:line citations
once someone commits or reverts it.

**Spot-checks performed (all confirmed accurate against the real source):**

1. **`text-accent` contrast bug (bucket 1, highest priority).** Independently
   recomputed WCAG contrast from the HSL values in `globals.css` (light
   `--accent: 210 40% 96.1%` vs `--background: 210 40% 98%`; dark
   `--accent: 217.2 32.6% 17.5%` vs `--background: 222.2 84% 4.9%`) using
   the standard relative-luminance formula: **light mode 1.046:1, dark mode
   1.369:1** — matches the claimed 1.05:1 / 1.37:1 almost exactly, both
   far below the 4.5:1 AA minimum. "Near-invisible text" conclusion is
   correct, not a style opinion. All 4 usage sites
   (`dashboard/page.tsx:71`, `projects/[id]/page.tsx:183,362`,
   `code/[id]/page.tsx:119`) verified byte-for-byte via grep.
2. **Badge `warning` hardcoded literal** — `badge.tsx:21` is exactly
   `"border-transparent bg-yellow-500/20 text-yellow-600 dark:text-yellow-400"`
   as claimed; `success`/`error`/`info` on the surrounding lines are
   correctly token-based. Confirmed.
3. **Badge `destructive` vs `error` redundancy** — grepped every
   `Badge variant=` call site in `app/`: zero literal
   `variant="destructive"`, all failure-state usages go through
   `statusVariant` maps or literal `"error"`/`"warning"`. Claim confirmed.
   (Note: `Button variant="destructive"` *is* used 3x — `settings/page.tsx:195`,
   `projects/[id]/page.tsx:215`, `containers/[id]/page.tsx:132` — exactly
   as cited in bucket 2; the audit correctly scopes the "unused" claim to
   the Badge variant only, not Button.)
4. **`statusVariant` duplication (bucket 6)** — confirmed
   `dashboard/page.tsx:11-17` and `projects/[id]/page.tsx:47-53` are
   identical (`running/building/cloning/created/error`), while
   `containers/page.tsx:38-43` has a genuinely different set
   (`running/paused/exited/created`). Accurate.
5. **Native `confirm()` (bucket 2)** — `code/[id]/page.tsx:56` is exactly
   `if (dirty && !confirm("Discard unsaved changes?")) return;`. Confirmed
   the only native-dialog usage in the pages read.
6. **Bare `rounded` sites (bucket 3)** — grepped for the bare class across
   the cited files; found exactly 7 matches at exactly the cited lines
   (`settings/page.tsx:289,421,622,755`, `containers/[id]/page.tsx:151,167`,
   `projects/[id]/page.tsx:228`). Exact match, including the "7 sites
   total" count.
7. **Sidebar/layout magic number (bucket 4)** — `sidebar.tsx:31` has
   `w-56`, `(main)/layout.tsx:7` has `ml-56`, `(main)/layout.tsx:5` has the
   `flex min-h-screen` wrapper referenced as the reason
   `min-h-screen`-on-page-root is redundant. All confirmed.
8. **Button outline-variant border token (bucket 2)** — `button.tsx:17` is
   `border border-border`, confirmed as the only form-control-adjacent
   primitive not using `border-input` (`input.tsx` and `select.tsx:22`
   both correctly use `border-input`). Confirmed.
9. **Cards hover treatment (bucket 5)** — confirmed `dashboard/page.tsx:59`
   uses `hover:border-muted-foreground` while `code/page.tsx:53` and
   `containers/page.tsx:134` use `hover:bg-muted/50`. Confirmed.
10. **Inline hand-rolled form blocks (bucket 3)** — all 4 cited sites in
    `settings/page.tsx` (289, 421, 622, 755) use the identical
    `<div className="space-y-2 p-3 border border-border rounded bg-card">`
    wrapper. Confirmed.

**Primitive-layer "in good shape" verdict spot-check.** Read
`select.tsx` and `alert-dialog.tsx` in full: both are byte-for-byte
canonical shadcn/ui registry output (Radix composition, `forwardRef`,
`displayName`, no ad-hoc deviation). This supports the "essentially
unmodified shadcn output" verdict for the primitive layer. Also read
`button.tsx` directly, which confirmed the one specific drift point the
doc claims (outline variant `border-border` vs. sibling primitives'
`border-input`) and nothing else — no undisclosed additional drift in
that file.

**Acceptance Criteria walk:**
- [x] Every `components/ui/` file checked against upstream shadcn pattern,
  drift noted — verified for badge/button/select/alert-dialog directly;
  the two disclosed drift points (badge variant set, button border token)
  both check out and no undisclosed drift was found in the files sampled.
- [x] Hand-rolled duplicates of shadcn primitives noted with file:line —
  `confirm()`, the 4 inline settings forms, all verified exact.
- [x] Hardcoded color/spacing values noted with file:line — `text-accent`,
  `text-success`, badge `warning` yellow literal, `bg-black`, `w-56`/`ml-56`,
  bare `rounded` — all verified exact, including counts.
- [x] Findings grouped into concrete, independently-actionable buckets —
  7 clearly-scoped buckets, each plausibly a standalone follow-up FEAT.
- [x] No UI code modified as part of this task — confirmed; the dirty
  working tree predates this task (see ambient-WIP note above) and the
  task's own diff (task file only) is clean.

No blocking issues found. Every finding spot-checked (10 across all 7
buckets, plus the WCAG math and 2 primitive files) matched the actual
source exactly, including line numbers and counts — this is an unusually
well-verified audit for a document-only task.


## Test Notes
<Filled in by the tester.>
