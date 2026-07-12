---
id: FEAT-041
type: feature
title: Domain-binding UI — pick which service the domain routes to (+ set/clear domain)
status: done
complexity: standard
assignee: sdlc-reviewer
sprint: SPRINT-004
created: 2026-07-11
history:
  - {date: 2026-07-11, stage: created, by: architect, note: "SPRINT-004 C6 cluster (domain-binding edit)"}
  - {date: 2026-07-11, stage: development, by: architect, note: "assigned (domain-binding UI; FEAT-040 held with settled error contract)"}
  - {date: 2026-07-11, stage: review, by: architect, note: "binding control on settings (service picker from compose_yaml, domain edit/unbind, 400/409 error surfacing); build passes; reviewing"}
  - {date: 2026-07-11, stage: hold, by: architect, note: "review PASS (extractServices robust, graceful degrade, unbind sends empty, 400/409 surfaced); holding for TEST-018"}
  - {date: 2026-07-11, stage: done, by: architect, note: "TEST-018 PASS; cluster C6 committed"}
---

**Part of:** C6-domain-binding
**Depends on:** FEAT-040

## Summary
The UI for the one edit action the sprint promised: let the user choose which
of their compose project's services the domain routes to (and set/clear the
domain), calling FEAT-040's `PUT /api/projects/{id}` (`exposed_service` +
`domain`). Effectively "editing the routing through the UI."

## Scope
- A control on the project detail view (Settings tab is the natural home; the
  project already has a settings page) to:
  - Show the current domain + which service is exposed.
  - Pick the exposed service from the project's compose services (a
    dropdown/radio of the service names parsed from `compose_yaml`).
  - Edit the domain; a clear/unbind affordance.
  - Save → `PUT /api/projects/{id}` with `{domain, exposed_service}` via the
    api client (`updateProject`), then reflect the result (success/error toast,
    refresh project state).
- Get the list of services to choose from: from the project's `compose_yaml`
  (parse client-side) or a field the project API already returns; use whatever
  the existing project detail already has. Only offer services that exist.
- Optional nicety: expose the same action from the map (per the sprint's "the
  one map edit action") — but the Settings control is the required deliverable;
  a map entry point can simply link/scroll to it. Do not build a separate
  editor.
- Handle errors from FEAT-040 (invalid service / no running container) with a
  clear message; don't leave the UI in a stale state.

## Out of Scope
- Backend routing (FEAT-040).
- A full compose-YAML text editor.

## Affected Areas
- `frontend/src/app/(main)/projects/[id]/settings/` (the binding control)
- possibly a small addition to the Map tab linking to it
- `frontend/src/lib/api.ts` — reuse `updateProject` (already exists; extend its
  payload type to include `exposed_service` if not present)

## Acceptance Criteria
- [ ] The project settings show the current domain + exposed service
- [ ] The user can pick a different service (from the compose services) and/or edit the domain and save
- [ ] Save calls `PUT /api/projects/{id}` with domain + exposed_service and reflects success/error
- [ ] Only real services are selectable; invalid/no-container errors surface clearly
- [ ] Unbind (clear domain) works from the UI
- [ ] `npm run build` passes, no type errors, no new dependency; matches existing design system (no restyle)

## Test Plan
In the UI, rebind the domain to a different service, save, confirm the project
reflects it. Verified end-to-end (route actually moves) in TEST-018.

## Implementation Notes
**File changed:** `frontend/src/app/(main)/projects/[id]/settings/page.tsx`

**Service list derivation:** Added `extractServices(composeYaml)` function that safely parses the compose YAML without a dependency. The parser:
- Splits YAML by newlines and tracks indentation levels
- Finds the `services:` key, then calculates service indent level as `services_indent + 2` (standard YAML indentation)
- Extracts service names by matching lines at the service indent level with a colon (e.g., `  web:`)
- Stops when reaching another top-level key (indent ≤ services indent)
- Skips empty lines and comments
- Returns array of service names only

**Error handling:** Backend errors (400 for invalid service, 409 for no running container) are caught in the try-catch, extracted with `err.message`, and displayed via state with `text-destructive` styling, matching the app's existing error pattern.

**UI integration:**
- Added Select component (existing in UI library) for service picker
- Shows "None" option to unbind/clear the service binding
- Only displays service picker if `project.compose_yaml` exists (degrades gracefully for non-compose projects)
- Shows current exposed service as a hint below the picker
- Added error state and visual feedback (disabled Save button while saving)
- Integrated exposed_service into the updateProject call, conditioned on hasCompose

**Build:** `npm run build` passed with no errors or warnings.

## Review Notes (2026-07-12)

**Verdict: PASS**

### 1. extractServices() YAML parsing — primary risk area

Traced the function against real compose YAML patterns:

- **Normal 2-space indented services** (web, db with nested image/ports): ✓ Correctly extracts service names only
- **Top-level siblings** (networks:, volumes: at same indent as services:): ✓ Correctly stops extraction when hitting them
- **Nested keys** (depends_on, environment under services): ✓ Not returned as service names (indent > serviceIndentLevel filters them)
- **Comments and blank lines**: ✓ Skipped
- **Service with comment on same line** (`web: # comment`): ✓ Works (split on `:` takes before-colon text)
- **Inline object syntax** (`web: {image: nginx}`): ✓ Works
- **Empty compose_yaml or no services block**: ✓ Returns []

**Non-blocking edge case — quoted service names:**
If a compose file uses quoted identifiers (non-standard but valid YAML):
```yaml
services:
  "web":
    image: nginx
```
The function extracts `"web"` (with quotes). The backend's ParseComposeYAML (using compose-go) normalizes this to `web` (without quotes). On save, validation fails with HTTP 400 and message `exposed_service ""web"" is not a service defined in compose_yaml`. This is UX-degrading but not data-loss; the error is clear enough and quoted service names violate docker-compose naming conventions (should be bare identifiers). A hand-rolled YAML parser can't fix this without a real YAML library. Not a blocker.

### 2. Graceful degradation

- Non-compose projects (no `compose_yaml`): Service picker and error display hidden; form works normally for name/domain/branch. ✓
- Project with `compose_yaml=""`: `hasCompose = false`, picker hidden. ✓

### 3. Save path and unbind

- Both `domain` and `exposed_service` sent in updateProject call (line 84–89). ✓
- "None" option sets `editExposedService = ""`, which is sent as `exposed_service: ""` when hasCompose. ✓
- Backend test (TestProjectServiceUpdateClearExposedService) confirms empty string unbinds correctly. ✓

### 4. Error handling

- Backend errors (400 for invalid service, 409 for no running container) thrown as Error objects with descriptive text. ✓
- Frontend catches via `err instanceof Error ? err.message : "Failed to save settings"` (line 92). ✓
- Displayed with `text-destructive` styling (line 150), consistent with existing error patterns. ✓
- On error, `refetch()` is not called → state remains consistent (won't show stale success). ✓
- On success, `refetch()` updates project state to reflect new domain/exposed_service. ✓
- Save button disabled during request (`disabled={saving}`), showing "Saving..." (line 153–154). ✓

### 5. Design system adherence

- All components reused: Select/SelectContent/SelectItem/SelectTrigger/SelectValue, Input, Label, Card/CardContent/CardHeader/CardTitle, Button. ✓
- Error styling uses existing `text-destructive` class. ✓
- No new imports in package.json; `extractServices()` is hand-rolled. ✓
- No restyle — follows existing settings page layout and spacing. ✓

### 6. Build

`npm run build` passed with no type errors or warnings. ✓

### Acceptance Criteria met:
- [x] Settings show current domain + exposed service (line 141–145 hint + selector initial value)
- [x] User can pick service and/or edit domain and save (lines 113–115, 127–138, 153)
- [x] Save calls PUT with both fields + reflects success/error (lines 84–90)
- [x] Only real services selectable; invalid/no-container errors surface clearly (backend validation + error display)
- [x] Unbind (clear domain) works (line 132 "None" option → empty string)
- [x] Build passes, no type errors, no new dependency, design system match (confirmed)


## Test Notes
<n/a — held for cluster integration test TEST-018>
