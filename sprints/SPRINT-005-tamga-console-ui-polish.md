---
id: SPRINT-005
name: Tamga Console UI Polish
status: complete
created: 2026-07-12
completed: 2026-07-13
---

## Goal
Evolve Tamga into **Tamga Console**: a cohesive, responsive operations
console built primarily from current shadcn/ui components and blocks. The
product must retain every existing workflow while replacing the inconsistent
page-by-page presentation with an accessible, polished system. "Tamga
Console" appears in the application header using Geist Pixel as a display
accent (never as dense body or code text).

## Scope
- Upgrade the Next.js frontend from Tailwind CSS 3.4 to **Tailwind CSS 4.3**
  as the design-system foundation, then use shadcn's **Nova** preset. Use the
  official Next.js/PostCSS integration—not the Vite plugin—and adopt v4's
  CSS-first tokens and import model.
- Refresh and, where needed, install current shadcn primitives/blocks for the
  app shell, auth, dashboard, projects, containers, code/terminal, settings,
  analytics, and infrastructure routes.
- Replace repeated loading, empty, error, card, form, table/list, destructive
  action, and navigation treatments with reusable shadcn patterns.
- Add responsive navigation, keyboard-visible focus states, and accessible
  labelling while preserving existing routes, API contracts, authorization,
  terminal behavior, and deploy actions.
- Use the official `geist` package for Geist Pixel; use a readable Geist Sans
  body face and Geist Mono for technical/code content.

## Out of Scope
- Backend/API or database changes, new deployment capabilities, or changes to
  the terminal/session model, except C2's shell ergonomics and terminate-tab
  correctness.
- Replacing the topology graph implementation, chart data model, or Monaco
  editor; this sprint changes their surrounding UX and visual treatment.
- A wholesale shadcn CLI overwrite of locally modified primitives. Changes to
  existing components must be diff-reviewed and merged deliberately.
- A new logo/marketing site or a light-mode-only redesign.

## Phases
1. C0 — Frontend runtime baseline: complete the source/UI audit, then hold the
   auth-guard repair (BUG-035) and standalone-image repair (BUG-036) for one
   combined TEST-019 runtime verification.
2. Migrate Tailwind to 4.3, establish the preset, type hierarchy, semantic
   tokens and shared UI primitives (FEAT-042), then migrate the responsive console shell
   (FEAT-043).
3. Migrate operational surfaces as independently reviewed cluster parts:
   dashboard/auth (FEAT-044), project workspace (FEAT-045), containers
   (FEAT-046), settings (FEAT-047), code/terminal (FEAT-048), and
   analytics/infrastructure (FEAT-049).
4. Hold all cluster implementation parts in review, run the combined browser
   verification, then commit the cluster together (TEST-020).
5. C2 — Terminal interaction reliability: hold the interactive-shell feature
   and terminate-tab repair for one real-terminal TEST-021 run before C1's
   terminal visual polish depends on it.
6. C3 — Test automation: audit current command/fixture boundaries (TEST-022),
   then hold deterministic command, frontend unit, Playwright E2E, and CI
   changes for one complete TEST-023 pipeline verification and one commit.

## Tasks
- TEST-019 — [C0] Frontend runtime baseline integration verification — done
- BUG-035 — [C0] Dashboard new-project route renders without authentication — done
- BUG-036 — [C0] Frontend standalone Docker image starts server from the wrong path — done
- FEAT-042 — [C1] Tamga Console shadcn design foundation — done
- FEAT-043 — [C1] Responsive Tamga Console application shell — done
- FEAT-044 — [C1] Dashboard and auth UI refresh — done
- FEAT-045 — [C1] Project workspace UI refresh — done
- FEAT-046 — [C1] Container operations UI refresh — done
- FEAT-047 — [C1] Settings and action-form UI refresh — done
- FEAT-048 — [C1] Code and terminal workspace UI refresh — done
- FEAT-049 — [C1] Analytics and infrastructure UI refresh — done
- TEST-020 — [C1] Tamga Console integrated UI verification — done
- BUG-037 — [C2] Terminating an agent session does not remove its Code-page terminal tab — done
- FEAT-050 — [C2] Colored, history-aware and completable agent shell — done
- TEST-021 — [C2] Terminal interaction reliability integration — done
- TEST-022 — [C3-preflight] Test automation readiness audit — done
- FEAT-051 — [C3] Deterministic frontend and backend test command foundation — done
- FEAT-052 — [C3] Frontend unit-test foundation and critical behavior coverage — done
- FEAT-053 — [C3] Reproducible Playwright browser E2E foundation — done
- TEST-023 — [C3] Frontend and backend test automation integration — done

## Release Notes
### Added
- Tamga Console branding with Geist Pixel Square display typography
- Tailwind CSS 4.3 with CSS-first theme tokens and shadcn Nova preset
- Responsive sidebar with mobile Sheet drawer, icon collapse, Ctrl/Cmd+B toggle
- Shared shadcn primitives: PageHeader, Card, Table, Empty, Skeleton, Field, InputGroup, AlertDialog, Sheet, Breadcrumb, Tooltip, Sonner
- Light/dark theme with 25+ CSS custom properties and system preference tracking
- Bash completion and git completion in agent sandbox terminals
- Cross-tab command history sync in agent terminal sessions
- Terminal tab helper utilities for snapshot merging and deterministic active-tab fallback

### Changed
- Replaced Tailwind CSS 3 configuration with Tailwind 4.3 CSS-first `@theme inline` mapping
- Login page rebuilt as Tamga Console split login block with shadcn Field/InputGroup
- Dashboard shows loading skeletons, empty states, and error feedback with project status summaries
- Project workspace uses shared PageHeader, Card, Table, Empty patterns with per-row container lifecycle feedback
- Container inventory and detail pages use shadcn Card/Badge/AlertDialog with controlled destructive confirmations
- Settings routes rebuilt on Field/Switch/RadioGroup/Select/Table with inactive form state handling
- Code/terminal workspace uses Tabs, ScrollArea, Tooltip, Empty, and Badge for polished editor and terminal UX
- Analytics and infrastructure pages use shared Card/Badge/Skeleton with responsive overflow for graph controls
- Agent sandbox Dockerfile installs bash-completion and git-bash-completion packages

### Fixed
- Terminating an agent session no longer leaves a stale tab in the Code workspace
- Dashboard new-project route properly requires authentication
- Standalone Docker image starts from the correct server path
