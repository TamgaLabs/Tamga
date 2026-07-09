---
id: FEAT-017
type: feature
title: Settings secondary sidebar (5 sub-routes) + Light/Dark/System theme moved into Appearance
status: pending
complexity: standard
assignee: unassigned
sprint: SPRINT-003
created: 2026-07-08
history:
  - {date: 2026-07-08, stage: created, by: architect, note: "task created from TEST-008 findings §1/§2/§5; depends on FEAT-014 and FEAT-016"}
---

## Summary
The 805-line single settings page becomes five URL sub-routes with a
shared secondary sidebar, and the theme toggle moves out of the primary
sidebar into Settings > Appearance as a three-way Light/Dark/System
selector. Section mapping was decided with the user and validated by
TEST-008 §5's inventory. Depends on FEAT-014 (provider/API-key cards
already deleted) and FEAT-016 (egress mode API exists for the Network
section).

## Requirements
- Nested layout: `frontend/src/app/(main)/settings/layout.tsx` renders a
  secondary sidebar (visually consistent with the primary one) with five
  entries; `/settings` redirects (or defaults) to `/settings/appearance`.
  Sub-routes and content (from TEST-008 §5's mapping):
  - `/settings/appearance` — the new theme selector + the "Show Tamga
    System" toggle (the ambiguous "Display" section lands here, with
    theme, as user-facing display preferences).
  - `/settings/sandbox` — Sandbox Resource Limits card (+ sandbox image
    info if trivially available; don't build new backend for it).
  - `/settings/network` — egress mode selector (Open/Whitelist/Blacklist,
    from FEAT-016's API) + the whitelist editor (existing card) + a
    blacklist editor (same UX incl. the 409 duplicate handling); make it
    clear which list is active in the current mode (e.g. dim the inactive
    list), and show the existing "applies on next sandbox start" note.
  - `/settings/git` — Git Credential card.
  - `/settings/system` — Docker info + Prune All.
- Theme: widen `theme.tsx` per TEST-008 §2 — `Theme` type gains
  `"system"`, persist `"system"` literally, subscribe to
  `matchMedia("(prefers-color-scheme: dark)")` changes while in system
  mode, resolve to light/dark when applying the `.dark` class. Monaco's
  consumer in `code/[id]/page.tsx` (:34, :251) must keep working —
  expose a resolved light/dark value for it.
- Remove the theme toggle from `sidebar.tsx` (:60-71). Logout stays.
- Move the existing cards without rewriting their logic; this is a
  restructure, not a redesign of each card's behavior.

## Out of Scope
- Project detail and containers secondary sidebars (FEAT-018/FEAT-019).
- Fixing the systemic console.error-only error handling in the moved
  cards (pre-existing pattern, noted in TEST-008).
- New settings content beyond the egress mode/blacklist editors.

## Proposed Solution / Approach
<filled in by developer>

## Affected Areas
<filled in by developer>

## Acceptance Criteria / Definition of Done
- [ ] Each of the five sub-routes renders its section; deep-linking and refresh work on all five; `/settings` lands on appearance
- [ ] The secondary sidebar marks the active section and is present on all five
- [ ] Theme selector offers Light/Dark/System; System tracks a live OS preference change without reload; choice survives reload (localStorage stores "system" literally)
- [ ] Monaco editor theme still follows the resolved theme in both explicit and system modes
- [ ] No theme toggle remains in the primary sidebar
- [ ] Egress mode can be changed from /settings/network and both lists edited there; duplicate add shows the 409 error inline
- [ ] All moved cards keep their previous functionality (spot-check each: resource limits save, git credential save/delete, prune, show-system toggle)
- [ ] `npx tsc --noEmit` and `npm run build` pass
- [ ] Code follows KISS/YAGNI — no speculative abstraction

## Test Plan
Browser walkthrough of all five sub-routes (direct URL + sidebar nav +
refresh); theme matrix (light/dark/system × reload × OS-preference flip
via emulation); egress mode switch verified against the backend (GET mode
reflects the change); regression spot-checks on each moved card's primary
action.

## Implementation Notes
<filled in by developer>

## Review Notes
<filled in by reviewer>

## Test Notes
<filled in by tester>
