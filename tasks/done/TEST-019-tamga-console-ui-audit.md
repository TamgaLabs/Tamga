---
id: TEST-019
type: test
title: Tamga Console UI inventory, visual baseline, and WIP compatibility audit
status: done
complexity: standard
assignee: unassigned
sprint: SPRINT-005
created: 2026-07-12
history:
  - {date: 2026-07-12, stage: created, by: architect, note: "task created"}
  - {date: 2026-07-13, stage: development, by: architect, note: "assigned to developer_standard"}
  - {date: 2026-07-13, stage: review, by: architect, note: "developer source-audit complete; assigned to reviewer_standard"}
  - {date: 2026-07-13, stage: changes-requested, by: architect, note: "reviewer found missing frontend/AGENTS.md WIP entry; user set Tailwind CSS 4.3 target"}
  - {date: 2026-07-13, stage: review, by: architect, note: "developer rework complete; assigned to reviewer_standard"}
  - {date: 2026-07-13, stage: runtime-test, by: architect, note: "review passed; assigned to builder for deterministic environment preparation"}
  - {date: 2026-07-13, stage: changes-requested, by: architect, note: "runtime audit found BUG-035; responsive overflow remains within FEAT-043 scope"}
  - {date: 2026-07-13, stage: integration-owner, by: architect, note: "reclassified as C0 integration test for BUG-035 and BUG-036"}
  - {date: 2026-07-13, stage: runtime-test, by: architect, note: "C0 parts reviewed and held; assigned to builder for single combined verification"}
  - {date: 2026-07-13, stage: changes-requested, by: architect, note: "C0 builder could not build BUG-036 image; BUG-036 returned for rework and BUG-035 remains held"}
  - {date: 2026-07-13, stage: runtime-test, by: architect, note: "BUG-036 rework review passed; assigned to builder for the single C0 rerun"}
  - {date: 2026-07-13, stage: done, by: architect, note: "C0 integration passed; BUG-035 and BUG-036 ready for cluster commit"}
---

## Summary
Establish the concrete visual and functional baseline before the console-wide
shadcn migration. This protects the existing operations workflows and the
ambient frontend styling WIP already present in the worktree.

**Cluster:** C0 Frontend runtime baseline

**Verifies:** BUG-035, BUG-036

## Scope
- Inventory all 27 route pages, nested layouts, and shared components under
  `frontend/src/app` and `frontend/src/components`.
- Capture desktop and narrow-width evidence for auth, dashboard/new project,
  project workspace, container detail, settings, code/terminal, analytics,
  and infrastructure, including loading, empty, error, and destructive flows
  where the current API permits them.
- Record local uncommitted frontend files and identify overlap hazards for
  `globals.css`, `layout.tsx`, `tailwind.config.ts`, Card, Badge, Input, and
  login/code surfaces.
- Produce a route-to-pattern migration map and an exact shortlist of shadcn
  blocks/components needed by FEAT-042 through FEAT-049.
- Audit the current Tailwind 3.4 to Tailwind 4.3 migration seam for this
  Next.js application, including the official PostCSS integration and every
  responsibility currently held by `tailwind.config.ts`.

## Out of Scope
- Changing UI production code or resolving defects in place.
- Changing API fixtures, database data, or the shared compose stack.

## Test Approach
1. Create an isolated lifecycle manifest and run the shared-stack smoke check;
   no shared-compose resources will be recorded as task-owned.
2. Enumerate every app route/layout and shared UI seam, then inspect the source
   branches that produce loading, empty, error, confirmation, and destructive
   states. The previously attempted frontend build failed, but its interrupted
   invocation retained no diagnostic output; this stage does not rerun it.
3. Defer running frontend/API, desktop, narrow-width, authenticated, and
   browser-state evidence to the tester. That evidence must use existing local
   data only and record any authentication/fixture limitation rather than
   changing data or bypassing a flow.
4. Record the exact frontend worktree delta and inspect diffs only to identify
   ownership/merge hazards. Map routes and variants to the smallest compatible
   shadcn block/component set and a non-overwriting preset strategy.
5. Append evidence and cleanup solely through the manifest helper.

## Affected Areas
- `frontend/src/app/**`
- `frontend/src/components/**`
- `frontend/src/app/globals.css`
- `frontend/components.json`
- `frontend/package.json`
- `tasks/active/TEST-019-tamga-console-ui-audit.md`

## Acceptance Criteria
- [ ] Every route has a recorded current state and intended target pattern;
      primary user actions and state variants are enumerated.
- [ ] The report distinguishes observed functional defects from presentation
      defects; no defect is fixed inline.
- [ ] Ambient frontend WIP and its exact overlap risk are recorded.
- [ ] The report names the required shadcn components/blocks and records a
      safe preset-application strategy (no blind overwrite).
- [ ] The report distinguishes current Tailwind 3 facts from the explicit
      Tailwind 4.3 target and gives FEAT-042 a safe, Next.js-specific
      migration checklist; it must not prescribe the Vite plugin.
- [ ] Results are concrete observations.

## Test Plan
1. Use `scripts/sdlc-environment.sh prepare <manifest> <repo-root> --smoke`
   once for C0; retain the manifest for cleanup and record the currently
   running frontend image before replacing it.
2. Build current workspace source with `docker compose build frontend`, then
   run `docker compose up -d --no-deps frontend`; wait for the service to run
   and record its exact image ID. If it cannot become healthy, restore only the
   recorded prior frontend image and report the concrete failure.
3. Run `npm run build` from `frontend` and inspect the fresh standalone image
   for a running container, no `server.js` module error, and a non-502 HTTPS
   frontend response.
4. In fresh browser contexts, verify unauthenticated direct `/dashboard/new`
   redirects to `/login` without rendering the form; then authenticate with
   existing local credentials and verify the local/remote/compose controls
   render. Do not create a project.
5. Record desktop/narrow shell evidence and the worktree delta without altering
   fixtures; file only defects not already owned by C0/FEAT-043. Cleanup only
   manifest-recorded resources and leave the working frontend image running if
   C0 passes.

## Implementation Notes
### Developer source-audit findings (2026-07-13)

**Evidence boundary.** This is a static source and worktree audit only. The
earlier isolated shared-stack `prepare --smoke` completed successfully; its
frontend build attempt failed without retained diagnostic detail. No build,
browser, Docker, or product-code action was run in this developer stage.
Both exact task manifests were cleaned with `scripts/sdlc-environment.sh
cleanup`: `/tmp/tamga-sdlc-test019-20260713.manifest` and
`/tmp/tamga-sdlc-test019-20260713-run2.manifest` each returned
`TASK_RESOURCES_CLEANED=true`. Their recorded `note`/exact temporary-log
entries were the only task resources considered; the shared compose stack was
not touched. Runtime and browser evidence is deferred to the tester.

#### Route map — current surface, target shadcn pattern

The 27 `page.tsx` route patterns below are complete (route groups are omitted
from URLs). The root and setup routes are redirect-only routes; layouts are
listed separately after the map.

| Route family | Route(s) and current source fact | Intended block/components |
|---|---|---|
| Entry/auth | `/` reads the stored token and redirects to `/dashboard` or `/login`; `/setup` redirects to `/login`; `/login` is a password Card form with inline error and redirect on success. | Minimal auth/login block; `Card`, `Field`/`Label`, `Input`, `Button`, `Alert` (submission error), `Skeleton`/pending state. Keep redirect/auth behavior intact. |
| Primary dashboard | `/dashboard` fetches project cards and links to creation/detail; `/dashboard/new` is a three-mode (local/remote/compose) creation form with conditional URL/YAML/service inputs and inline error. | Dashboard/application shell block; page header with primary action; project-card grid plus `Empty`, `Skeleton`, `Alert`; `Card`, `Button`, `DropdownMenu` where needed. New-project form block using `Field`, `Input`, `Textarea`, `RadioGroup`, `Select`, `Button`, `Alert` and `Tooltip` for deployment choices. |
| Project workspace | `/projects/[id]` presents details, containers and deployments; `/projects/[id]/containers` lists/actions containers with confirmed delete; `/projects/[id]/environment` adds/removes environment variables; `/projects/[id]/settings` edits name/domain/branch/exposed service; `/projects/[id]/actions` restarts, reveals logs, and confirms project delete; `/projects/[id]/analytics` is metrics with selectors/panels; `/projects/[id]/map` is the live topology graph with loading/error. | Secondary workspace/sidebar block plus `Breadcrumb`, tabs/nav, page header. Reusable data-card/list or `Table`, `Badge`, `DropdownMenu`, `AlertDialog`, `Empty`, `Skeleton`, `Alert`, `ScrollArea`; forms use `Field`, `Input`, `Select`, `Button`; analytics/map retain existing graph/panels inside `Card`/`Skeleton`/`Alert` frames. |
| Containers | `/containers` searches/groups project and non-project containers, supports start/stop/restart/delete; `/containers/[id]` inspects config; `/containers/[id]/logs` polls and refreshes logs; `/containers/[id]/resources` edits CPU/memory; `/containers/[id]/stats` fetches CPU/memory/network. | Operations list/table block with search `Input`/input-group, `Table` or grouped card list, `Badge`, `DropdownMenu`, `AlertDialog`, `Empty`, `Skeleton`, `Alert`. Detail secondary nav with `Breadcrumb`/tabs; logs in `ScrollArea`/monospace `Card` with `Button`; resource form uses `Field`, `Input`, `Button`; stat cards use `Card`, `Skeleton`, `Alert`. |
| Code/terminal | `/code` fetches selectable system/project codebases; `/code/[id]` contains file tree, Monaco editor, dirty/save/error controls, terminal-tab creation/termination confirmation, and terminal errors. | Workspace/detail block with responsive secondary sidebar/`Sheet`, `Resizable` panels if added only when compatible, `ScrollArea`, `Tabs`, `Tooltip`, `Button`, `AlertDialog`, `Skeleton`, `Empty`, `Alert`, and code `Card` framing. Preserve Monaco/xterm lifecycle and terminal IDs. |
| Global observability | `/analytics` polls system metrics and has explicit initial loading, error, empty metric series and four panel views; `/infrastructure` polls topology and has error/loading plus graph legend/click-through. | Analytics dashboard block: page header, selector toolbar, metric-card grid; existing chart components wrapped by `Card`, `Skeleton`, `Empty`, `Alert`, `Tooltip`. Infrastructure graph remains existing implementation, framed by page header/legend `Card`, `Badge` and async state components. |
| Settings | `/settings` redirects to appearance; `/settings/appearance` controls theme/display; `/settings/git` loads/edits/deletes a credential; `/settings/network` controls egress mode and whitelist/blacklist CRUD with confirmation/errors; `/settings/sandbox` edits idle timeout and resource limits; `/settings/system` presents Docker facts and confirmed prune. | Settings sidebar/nav block with `Breadcrumb`; setting-card/form rows using `Card`, `Field`, `Input`, `Select`, `RadioGroup`, `Switch`, `Button`, `Separator`, `Alert`, `Skeleton`, `Empty`, `AlertDialog`, `Sonner`/toast feedback. Retain destructive confirmation and server calls. |

**Nested layout seams.** `app/layout.tsx` owns metadata, font classes,
`ThemeProvider`, `AuthProvider`, and offline banner; `(main)/layout.tsx` owns
the primary sidebar/main offset; `projects/[id]/layout.tsx` and
`containers/[id]/layout.tsx` own their data/context and secondary navigation;
`settings/layout.tsx` owns settings navigation. FEAT-043 must replace the
fixed primary sidebar with the shadcn Sidebar block/provider and responsive
Sheet behavior while preserving the latter three nested layout contexts and
their URLs.

#### Existing primitives and ambient-WIP overlap

There are exactly 16 local UI primitives: `alert-dialog`, `badge`, `button`,
`card`, `checkbox`, `dropdown-menu`, `input`, `label`, `popover`,
`radio-group`, `scroll-area`, `select`, `separator`, `switch`, `tabs`, and
`textarea`. This is a partial `new-york` shadcn set; it lacks the sprint's
required Sidebar, Breadcrumb, Tooltip, Skeleton, Sheet, Table, Empty,
Sonner/toast, and Field/input-group support.

Ambient frontend WIP is present in exactly these paths:

- `frontend/src/app/globals.css` — expanded light/dark semantic theme tokens,
  including sidebar/chart/input/ring/radius values.
- `frontend/tailwind.config.ts` — matching Tailwind 3 color/radius/accordion
  mappings.
- `frontend/src/app/layout.tsx` — imports `Geist` from `next/font/google`,
  creates `--font-sans`, and applies it on `<html>`.
- `frontend/src/components/ui/card.tsx`, `badge.tsx`, and `input.tsx` —
  in-progress `new-york`-style implementations (forward refs; Card footer;
  CVA badge variants; compact input).
- `frontend/src/lib/utils.ts` — style-only `cn` formatting change.
- `frontend/src/app/(auth)/login/page.tsx` — local `Label`/`htmlFor` a11y
  improvement.
- `frontend/src/app/(main)/code/page.tsx` — card-title spacing adjustment.

Exact overlap risk: FEAT-042 directly overlaps every item except the code-list
spacing adjustment; FEAT-043 directly overlaps `layout.tsx`, `globals.css`,
Tailwind tokens and the `Card`/`Badge`/`Input` contracts; FEAT-044 overlaps
the login change; FEAT-048 overlaps the code-list change. No generated shadcn
file may replace `Card`, `Badge`, `Input`, `globals.css`, `layout.tsx`, or
`tailwind.config.ts` wholesale. Preserve the local `success`, `warning`,
`info`, `code-block`, and sidebar semantics when merging registry output.

#### Functional facts versus presentation findings

**Source-established functional facts:** protected pages consistently redirect
after auth resolution; all listed action forms call existing API helpers;
container/project/credential/network/system destructive operations use
`AlertDialog`; analytics/topology poll and expose explicit initial loading and
error branches; analytics also has an empty-series branch; code preserves
pending terminal tabs across session fetches, exposes save/terminal errors,
and confirms terminal termination. The dashboard, containers, code list, and
project pages keep local loading flags. These are behavior to preserve, not
proof of runtime success.

**Source-established presentation limitations:** the primary `Sidebar` is a
custom fixed `w-56` desktop-only aside, brands itself "Tamga", and lacks a
mobile Sheet/collapse/tooltip/current-page semantics; pages repeatedly use
ad-hoc `text-2xl` headers, cards and inline text errors; data lists are
card/loop treatments rather than a shared table/list/empty system; no shared
skeleton, toast, breadcrumb, field/input-group, or reusable async-state
pattern exists; nested secondary layouts need responsive composition with the
new primary shell. Most settings fetch failures are only `console.error`, so
users have no established visible async failure state. CSS is token-capable
but only the first WIP pass—contrast, focus, responsive layout and font
loading still require browser verification.

**Known limitations:** no desktop/narrow viewport, keyboard, theme contrast,
authenticated fixture, action-result, loading/error/empty rendering, or
network-error behavior was observed in a browser. The prior `npm run build`
failure has no retained error text and is not attributed to a source defect.
Tester must reproduce the build with diagnostic capture and collect those
runtime observations before any bug is filed.

#### Safe Nova strategy for this codebase

`components.json` already declares `style: "new-york"`, CSS variables,
Lucide, and `tailwind.config.ts`; `package.json` is Tailwind 3.4 with Next 15.
Treat Nova as a visual token/font/reference preset, not a CLI overwrite or a
registry-style replacement. FEAT-042 should first capture the Nova-generated
diff in a disposable worktree/temporary directory, then manually merge only
Tailwind-3-compatible semantic CSS variables, typography choices, and
component class deltas into the existing `new-york` contract. Retain
`components.json`'s `new-york` style unless an equivalent audited migration
is explicitly approved. Do not copy Tailwind 4-only directives, CSS-first
configuration, or dependency assumptions into this Tailwind 3 app.

Install missing primitives one at a time with the existing aliases and
diff-review each generated file against the 16 local primitives/WIP above.
Use the current shadcn Sidebar/application-sidebar block (the sprint names
`sidebar-07` or equivalent), auth/card/form, dashboard/analytics and settings
patterns as compositional references; preserve API contracts and existing
Monaco/xterm/topology components. Replace the current `next/font/google`
Geist WIP deliberately with the sprint-required official `geist` package
only after its Pixel Square, Sans, and Mono APIs are verified; Pixel Square is
for the "Tamga Console" wordmark/short display text only.

### Developer rework correction — complete WIP and Tailwind 4.3 target (2026-07-13)

**Corrected exact frontend WIP inventory.** The preceding ambient-WIP list is
superseded by this complete `git status --short`/`git diff --name-only --
frontend` inventory: `frontend/AGENTS.md` (**deleted**),
`frontend/src/app/(auth)/login/page.tsx`,
`frontend/src/app/(main)/code/page.tsx`, `frontend/src/app/globals.css`,
`frontend/src/app/layout.tsx`, `frontend/src/components/ui/badge.tsx`,
`frontend/src/components/ui/card.tsx`, `frontend/src/components/ui/input.tsx`,
`frontend/src/lib/utils.ts`, and `frontend/tailwind.config.ts` (all modified).

`frontend/AGENTS.md` is not visual production-code WIP. Its deletion removes
the frontend ownership guidance that requires shadcn primitive reuse, semantic
token use, narrowly justified `"use client"`, and `next/dynamic` with
`ssr: false` for Monaco. Subsequent sprint changes must continue to follow
that guidance; no task may silently treat the deletion as permission to depart
from it. A separately scoped task must restore it or relocate its guidance
before the deletion can be accepted.

**Corrected merge hazards.** In addition to the prior direct code/style
overlaps, every FEAT-042 through FEAT-049 task must preserve the above
guidance while resolving the deletion explicitly. FEAT-042/043 have the
highest config/token/layout overlap; FEAT-044 overlaps the login a11y WIP;
FEAT-048 overlaps the code-list WIP and must preserve Monaco's dynamic,
client-only boundary. Generated shadcn output or a preset must not overwrite
the deleted-guidance decision, `globals.css`, `layout.tsx`,
`tailwind.config.ts`, Card, Badge, Input, or `utils.ts`; reconcile each with
the owning WIP diff first.

#### Safe Tailwind CSS 4.3 migration checklist for this Next.js app

This corrects the earlier Tailwind-3-only recommendation. The current source
facts remain Tailwind 3.4 (`@tailwind base/components/utilities`, a
`tailwindcss` PostCSS plugin, `tailwind.config.ts`, and
`tailwindcss-animate`); the required target is Tailwind CSS 4.3. This is a
static checklist, not evidence that an install, build, or browser verification
has succeeded.

1. Work in an isolated branch/worktree after preserving the listed ambient
   WIP. Confirm the deployed-browser support floor accepts v4's modern-browser
   requirements and review the v3-to-v4 utility changes against all source
   classes before merging.
2. Update the frontend dependencies together: install `tailwindcss@4.3`,
   `@tailwindcss/postcss`, and compatible `postcss`; remove the v3-style
   `tailwindcss` PostCSS plugin. In `postcss.config.mjs` (or convert the
   existing JS config deliberately), use only
   `"@tailwindcss/postcss": {}`. V4 handles imports and vendor prefixing, so
   remove `autoprefixer` only after confirming no non-Tailwind PostCSS consumer
   still needs it. Do **not** install or configure `@tailwindcss/vite`: this is
   a Next.js/PostCSS application, not a Vite application.
3. Replace the three `@tailwind` directives in `src/app/globals.css` with
   `@import "tailwindcss";`. Keep the existing semantic CSS variables as the
   source of truth, but convert the Tailwind-facing color, font, radius, and
   other design mappings into CSS-first `@theme` variables (for example
   `--color-background`, `--color-sidebar`, and `--radius-*`) without losing
   `success`, `warning`, `info`, `code-block`, sidebar, chart, or WIP values.
   Use compatible color syntax deliberately rather than blindly copying a
   shadcn preset.
4. Preserve class-based theme selection used by the existing provider by
   defining the v4 dark variant explicitly (for example
   `@custom-variant dark (&:where(.dark, .dark *));`) and retaining light/dark
   semantic token values. Verify theme switching, system preference behavior,
   token contrast, and the initial non-flashing render in runtime QA.
5. Move custom CSS utility classes out of v3 `@layer utilities/components`
   assumptions to v4 `@utility` where variants are required. Recreate the
   accordion keyframes/animation tokens in CSS (`@keyframes` plus corresponding
   `--animate-*` theme variables), and audit whether `tailwindcss-animate`
   is v4-compatible before retaining it; replace only the animations actually
   consumed by the code if it is not. Audit renamed/removed v3 utilities,
   default border/ring changes, arbitrary-variable syntax, and order-sensitive
   stacked variants.
6. Do not retain `tailwind.config.ts` as the long-term theme authority.
   During a narrowly bounded compatibility step it may be loaded explicitly
   with `@config "../../tailwind.config.ts";` while every referenced extension
   is migrated and shadcn compatibility is checked; v4 does not auto-detect
   that file. Once colors, radii, keyframes/animations, and any plugin use are
   represented in CSS, remove the `@config` bridge and delete the obsolete
   config only in the same reviewed change. Also update `components.json` only
   after confirming its current `new-york`, CSS-variable, alias, and config
   expectations against the v4-compatible shadcn CLI.
7. Before applying Nova or adding any shadcn block/component, use a disposable
   worktree and inspect its v4 output/dependency requirements. Manually merge
   only compatible changes into the WIP-aware new-york primitives; do not let
   registry generation replace existing Card, Badge, Input, token, layout,
   Monaco, or deleted-guidance ownership decisions. Run the build and targeted
   dark/light, responsive, auth, code/terminal, and animation checks only in
   the later runtime stage, and record their actual result there.

## Review Notes

### 2026-07-13 — CHANGES_REQUESTED

The source inventory is otherwise credible: it enumerates all 27 `page.tsx`
files and five layouts, correctly separates source facts from unobserved
runtime behavior, and proposes a Tailwind-3-safe, non-overwriting `new-york`
strategy for the required shadcn components/blocks. The cleanup record is
appropriately limited to exact manifest entries and does not claim ownership
of the shared stack.

**Required correction before runtime test:** the claimed "exact" frontend WIP
inventory omits the uncommitted deletion of `frontend/AGENTS.md`, which appears
in both `git status --short` and `git diff --name-only -- frontend`. Record it
as an ambient worktree change, distinguish it from visual production-code WIP,
and state its merge/ownership consequence: it removes the frontend component,
token, client-component, and Monaco guidance that subsequent sprint work must
continue to follow unless an explicitly scoped task restores or relocates it.
Update the exact-overlap discussion accordingly. This is required by the
scope's instruction to record all local uncommitted frontend files.

### Architect rework scope (2026-07-13)

The user explicitly set **Tailwind CSS 4.3** as the target. The existing
findings correctly describe the current Tailwind 3.4 repository, but the
developer must append a corrected, Next.js-specific v4.3 migration checklist:
package and PostCSS changes; `@import "tailwindcss"`; CSS-first theme/token
migration; dark-mode, custom-utility and animation equivalents; and the safe
removal or retention decision for `tailwind.config.ts`. The linked Vite guide
is not an instruction to add `@tailwindcss/vite` to this Next.js application.

### 2026-07-13 — PASS (rework review)

The rework resolves the prior blocking omission. The exact frontend WIP
inventory now includes the deleted `frontend/AGENTS.md`, accurately separates
it from visual production-code WIP, preserves its primitive/token/client-boundary
and Monaco `next/dynamic(..., { ssr: false })` ownership guidance, and requires
an explicitly scoped restoration or relocation before accepting that deletion.
The updated merge hazards carry this guidance through FEAT-042 to FEAT-049.

The Tailwind target is now explicitly v4.3 and is correct for this Next.js
application: it uses the official PostCSS path (`@tailwindcss/postcss` and
`@import "tailwindcss"`) and explicitly excludes `@tailwindcss/vite`. The
checklist identifies the required CSS-first token migration, class-based dark
variant preservation, temporary explicit `@config` compatibility bridge and
eventual config removal, custom-utility migration, accordion/animation-plugin
audit, plus v4 browser/utility/default-style risks. It correctly treats all
of that as a migration plan, not build or browser-success evidence. Runtime
verification remains required at the tester stage.

## Test Notes

### 2026-07-13 — FAIL (runtime acceptance)

**Environment.** Used the builder-prepared shared environment at
`https://localhost` with Playwright Chromium in a fresh browser context and
`ignoreHTTPSErrors: true` for the documented local TLS certificate. No Docker
resource, API fixture, project, container, or product source was changed.
Existing projects returned by the authenticated dashboard were observed only;
none were created, edited, restarted, or deleted.

**Build — PASS.** Ran `npm run build` from `frontend`. Next.js 15.5.19
compiled successfully, completed type checking and static generation (18/18),
and emitted all listed application routes. The previously undiagnosed build
failure is therefore not reproducible in this prepared runtime.

**Auth/baseline — PASS.** In a fresh desktop (`1440x900`) context, loaded
`/login`, submitted the already configured local admin credential, and
observed `200` responses for `/api/auth/login`, `/api/auth/me`, and
`/api/projects`; the browser reached `/dashboard`. The page rendered the
primary navigation and six existing project cards. A fresh unauthenticated
context sent `/dashboard` to `/login`, rendering the password form.

**Unauthenticated route protection — FAIL.** In that same type of fresh,
unauthenticated context, direct navigation to `/dashboard/new` returned a
rendered application shell and New Project form, with final URL remaining
`https://localhost/dashboard/new` (rather than redirecting to `/login`). This
is reproducible without fixture changes and is a functional authorization
defect; file a separate bug before UI migration work relies on route guards.

**Direct visual observations — FAIL for narrow responsiveness.** Playwright
screenshots were captured outside the repository at desktop and `390x844`
narrow viewports. Desktop dashboard/new-project pages had document widths of
`1440/1440` and a visible `224px` sidebar. At 390px, the same always-visible
sidebar remained `224px`; dashboard document width was `639px` and New Project
document width was `583px` (both greater than the viewport). The narrow
screenshots visibly crop the main content/action area to the right and provide
no Sheet/collapse affordance. This directly confirms the audit's mobile-shell
presentation finding; responsive shell work remains required.

**Evidence boundary.** Browser tooling was available and used. These checks
observed login, existing-project listing, direct new-project rendering, and
the responsive shell only. Loading/error/destructive actions, project
creation, container operations, code/terminal, analytics, and settings flows
were intentionally not exercised because doing so would mutate or depend on
non-task-owned shared data. TLS certificate handling was limited to the local
test browser context as instructed.

### Architect disposition (2026-07-13)

`BUG-035` records the unauthenticated protected-route defect for independent
repair. The 390px fixed-sidebar overflow is a directly observed presentation
gap but is already an explicit acceptance criterion of the planned FEAT-043
responsive application-shell migration, so it is tracked there rather than
duplicated as a separate implementation task. The audit must be re-reviewed
after BUG-035 is repaired and after its report reflects the Tailwind 4.3
target; do not treat this runtime FAIL as permission to alter product code
inside TEST-019.

### 2026-07-13 — C0 integration builder blocked before tester

The one C0 builder cycle prepared and smoke-tested the shared stack, then
attempted the required current-source frontend image build. Docker failed at
`COPY --from=builder /app/.next/standalone/frontend ./` because that source
path was absent in the actual Docker build output. No new image/container was
started, so browser QA was intentionally not run against the stale prior image.
The builder cleaned its manifest; the prior shared frontend remained running
and `https://localhost/` remained HTTP 200. This is owned by BUG-036, not a
new defect.

### 2026-07-13 — PASS (C0 combined runtime verification)

**Single current-source environment.** Used the builder-prepared C0 manifest
`/tmp/tamga-sdlc-c0-test019-rerun-20260713.manifest` and one fresh Playwright
Chromium session family with local TLS handling (`ignoreHTTPSErrors: true`). No
project was created and no product code, fixture, shared-compose configuration,
or Docker resource was changed by the tester.

**BUG-036 — PASS.** The replacement `tamga-frontend-1` container remained
`running=true`, `restarting=false`, and exit code `0` on image
`sha256:e2c24351559997fde736680e7b767315d0cf10d6cb1f49a77880ea810d949afd`.
Its Next.js 15.5.19 log reached `Ready in 77ms` and contained no
`Cannot find module '/app/server.js'` (or other server.js start-path) error.
`https://localhost/` returned HTTP 200 rather than 502.

**BUG-035 — PASS.** A fresh unauthenticated desktop context directly loaded
`https://localhost/dashboard/new`: the navigation response was HTTP 200, the
final URL was `https://localhost/login`, and exact `New Project` text was not
visible. The observed form was the login form, not the protected project form.
In a separate fresh context, the existing local `admin` credential successfully
reached `/dashboard`; direct navigation to `/dashboard/new` then rendered one
New Project form and all three existing controls, `#local`, `#remote`, and
`#compose`. No submission was made.

**Shell observation.** The authenticated desktop new-project view matched its
1440px document/viewport width. At an authenticated 390x844 viewport, the
route still rendered the existing always-visible 224px `Tamga` sidebar, no
Sheet dialog, and a 583px document width. This is the already-recorded
horizontal-overflow gap owned by FEAT-043; it did not regress the C0 auth or
frontend-start behavior and is not a C0 failure.

**Verdict: PASS.** BUG-035 and BUG-036 pass together in the one C0 current
source image/browser verification.

## Pipeline Telemetry
| date | role | model | effort | result | duration | rework |
|---|---|---|---|---|---|---|
| 2026-07-13 | developer | developer_standard | medium | source audit complete; runtime evidence deferred | n/a | 0 |
| 2026-07-13 | reviewer | reviewer_standard | medium | CHANGES_REQUESTED: frontend WIP inventory omits deleted frontend/AGENTS.md | n/a | 1 |
| 2026-07-13 | developer | developer_standard | medium | rework complete: corrected complete WIP/ownership inventory and Next.js Tailwind 4.3 migration checklist; runtime evidence deferred | n/a | 1 |
| 2026-07-13 | reviewer | reviewer_standard | medium | PASS: rework preserves deleted-guidance ownership and gives a correct static Next.js/PostCSS Tailwind 4.3 migration plan; runtime evidence still deferred | n/a | 1 |
| 2026-07-13 | tester | tester | medium | FAIL: build/login/project-list baseline passed; unauthenticated `/dashboard/new` and narrow viewport horizontal overflow reproduced | n/a | 0 |
| 2026-07-13 | builder | builder | low | C0 blocked: Docker build source path absent; stale frontend intentionally not tested | n/a | 1 |
| 2026-07-13 | tester | tester | medium | PASS: single C0 current-source image verified BUG-035 auth guard and BUG-036 standalone startup; FEAT-043 owns known 390px overflow | n/a | 1 |
