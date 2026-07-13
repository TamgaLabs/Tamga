---
id: SPRINT-005
name: Tamga Console UI Polish
status: planning
created: 2026-07-12
completed:
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
- FEAT-042 — [C1] Tamga Console shadcn design foundation — pending
- FEAT-043 — [C1] Responsive Tamga Console application shell — pending
- FEAT-044 — [C1] Dashboard and auth UI refresh — pending
- FEAT-045 — [C1] Project workspace UI refresh — pending
- FEAT-046 — [C1] Container operations UI refresh — pending
- FEAT-047 — [C1] Settings and action-form UI refresh — pending
- FEAT-048 — [C1] Code and terminal workspace UI refresh — pending
- FEAT-049 — [C1] Analytics and infrastructure UI refresh — pending
- TEST-020 — [C1] Tamga Console integrated UI verification — pending
- BUG-037 — [C2] Terminating an agent session does not remove its Code-page terminal tab — pending
- FEAT-050 — [C2] Colored, history-aware and completable agent shell — pending
- TEST-021 — [C2] Terminal interaction reliability integration — pending
- TEST-022 — [C3-preflight] Test automation readiness audit — done
- FEAT-051 — [C3] Deterministic frontend and backend test command foundation — done
- FEAT-052 — [C3] Frontend unit-test foundation and critical behavior coverage — done
- FEAT-053 — [C3] Reproducible Playwright browser E2E foundation — done
- TEST-023 — [C3] Frontend and backend test automation integration — done

## Release Notes
### Added
### Changed
### Fixed
