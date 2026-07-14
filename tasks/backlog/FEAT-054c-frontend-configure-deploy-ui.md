---
id: FEAT-054c
type: feature
title: "Draft → Configure → Deploy: frontend (configure page, compose editor, deploy button)"
status: backlog
complexity: large
assignee: unassigned
sprint: SPRINT-006
created: 2026-07-14
history:
  - {date: 2026-07-14, stage: created, by: architect, note: "backlog; design discussed with user"}
---

**Part of:** FEAT-054-draft-configure-deploy
**Depends on:** FEAT-054a, FEAT-054b

## Summary
Frontend for the new Draft → Configure → Deploy flow. New project creation
creates a draft (no deploy). A new `/projects/[id]/configure` page shows
the generated compose YAML (form + raw YAML toggle), env vars, and exposed
service selection. A Deploy button triggers the actual deployment. Project
status badges and detail pages updated for the new lifecycle states.

## Requirements

### Modified: Project Creation (`dashboard/new/page.tsx`)
- Remove "Create & Deploy" button → replace with "Create Project"
- On submit: `POST /api/projects` creates draft, redirect to `/projects/{id}/configure`
- For compose source type: still accepts pasted YAML, but it goes to `configuring` directly
- For remote/local source type: creates draft, background analysis runs

### New: Configure Page (`/projects/[id]/configure/page.tsx`)
The main configuration page after project creation. Layout:

```
┌─────────────────────────────────────────────────────┐
│ Configure: my-project                               │
│ Status: Configuring (analyzing repo...)             │
├─────────────────────────────────────────────────────┤
│                                                     │
│ ┌─ Compose Configuration ─────────────────────────┐ │
│ │ Detected: Node.js (Next.js)                     │ │
│ │ Auto-generated: Yes                             │ │
│ │                                                  │ │
│ │ [Form View]        [Raw YAML View]               │ │
│ │                                                  │ │
│ │ Service: app                                    │ │
│ │   Image: (auto-detected)                        │ │
│ │   Port:  [3000] ← editable                      │ │
│ │   Build: ./  (context)                          │ │
│ │                                                  │ │
│ │ [+ Add Service]                                 │ │
│ └──────────────────────────────────────────────────┘ │
│                                                     │
│ ┌─ Environment Variables ─────────────────────────┐ │
│ │ KEY              VALUE                           │ │
│ │ NODE_ENV         production                      │ │
│ │ DATABASE_URL     [________________]              │ │
│ │ [+ Add]                                         │ │
│ └──────────────────────────────────────────────────┘ │
│                                                     │
│ ┌─ Routing ───────────────────────────────────────┐ │
│ │ Exposed Service: [app ▼]                         │ │
│ │ Domain: my-project.example.com                   │ │
│ └──────────────────────────────────────────────────┘ │
│                                                     │
│              [Save Draft]  [Deploy →]               │
└─────────────────────────────────────────────────────┘
```

**Behavior:**
- On mount: `GET /api/projects/{id}/config` to load config state
- While status is `cloning`: show spinner "Analyzing repository..."
- Once status is `configuring`: show compose editor + env vars + deploy button
- **Form View:** structured fields per service (image, ports, build context). Generated from compose YAML.
- **Raw YAML View:** syntax-highlighted textarea with the full compose YAML. Changes sync back to form view.
- **Env Var section:** CRUD for env vars via existing endpoints (`POST /api/projects/{id}/env-vars`, `DELETE /api/projects/{id}/env-vars/{id}`)
- **Deploy button:** calls `POST /api/projects/{id}/deploy`, then redirect to `/projects/{id}` (dashboard)
- **Save Draft:** calls `PUT /api/projects/{id}/config` to persist compose + env var changes without deploying

### New: Compose Editor Components

**`components/compose-editor.tsx`** — main component with form/raw toggle:
- Tab switcher: "Form" | "Raw YAML"
- Form tab: renders `ServiceForm` for each service
- Raw tab: textarea with monospace font, basic YAML syntax highlighting
- Bidirectional sync: form changes update YAML, YAML changes update form

**`components/service-form.tsx`** — per-service structured form:
- Service name (read-only if auto-generated)
- Image input
- Ports: dynamic list of host:container pairs
- Build context (if build: directive)
- Volumes: dynamic list of source:target pairs
- Environment: (moved to project-level env vars section, not per-service)

**`components/deploy-button.tsx`** — deploy trigger:
- Disabled when status is not `configuring`
- Shows loading state during deploy
- Confirmation dialog before deploying
- On success: redirect to project dashboard

### Modified: Project Detail Pages

**Status badges** (`projects/[id]/page.tsx`, `projects/[id]/layout.tsx`):
- `draft` → gray badge "Draft"
- `configuring` → blue badge "Configuring"
- `deploying` → yellow badge "Deploying"
- Existing statuses unchanged

**Project dashboard** (`projects/[id]/page.tsx`):
- When status is `draft` or `configuring`: show "Configure" CTA button linking to `/projects/{id}/configure`
- When status is `deploying`: show progress indicator
- When status is `running`: show existing dashboard

**Project actions** (`projects/[id]/actions/page.tsx`):
- "Restart" renamed to "Redeploy" (or both available)
- Add "Configure" link for draft/configuring projects

**Project settings** (`projects/[id]/settings/page.tsx`):
- Add compose YAML textarea for editing (for projects with compose)
- Save triggers `PUT /api/projects/{id}/config`

### API Client Updates (`lib/api.ts`)

New types:
```typescript
type ProjectConfig = {
  project: Project
  compose_yaml: string
  detected_language: string
  has_dockerfile: boolean
  auto_generated: boolean
  env_vars: EnvVar[]
  services: string[]
}
```

New functions:
```typescript
getProjectConfig(projectId: string): Promise<ProjectConfig>
updateProjectConfig(projectId: string, data: {
  compose_yaml?: string
  exposed_service?: string
  env_vars?: Array<{key: string, value: string}>
}): Promise<void>
deployProject(projectId: string): Promise<void>
```

Modified:
- `createProject` return type: project is now in `draft` status
- `Project` type: add `status: "draft" | "configuring" | "deploying" | ...`

### Loading state (`/projects/[id]/configure/loading.tsx`)
Skeleton loader for the configure page.

## Out of Scope
- Live compose validation (real-time YAML linting)
- Compose diff view (before/after changes)
- Undo/redo in compose editor
- Multi-service form editing with drag-and-drop reorder
- Deploy progress streaming (SSE/WebSocket)

## Proposed Solution / Approach

### Configure page flow
1. `layout.tsx` fetches project (existing pattern via `ProjectContextProvider`)
2. `configure/page.tsx` fetches config via `getProjectConfig(id)`
3. If status is `cloning`: poll every 2s until status changes to `configuring` or `error`
4. If status is `configuring`: render compose editor + env vars + deploy button
5. If status is `error`: show error message + retry option

### Form ↔ Raw YAML sync
The form view is the source of truth. When user switches to raw YAML:
- Current form state is serialized to YAML and shown in textarea
- When user switches back to form: YAML is parsed and form is re-populated
- If YAML parsing fails: show error, keep raw YAML as-is

This avoids complex bidirectional real-time sync. The toggle is explicit, not live.

### Env var integration
Use existing env var endpoints (`POST /api/projects/{id}/env-vars`, `DELETE /api/projects/{id}/env-vars/{id}`). The configure page's env var section is a wrapper around these endpoints with a nicer UI (inline add, not a separate form).

### Deploy button states
- `draft` → disabled (can't deploy before configure)
- `configuring` → enabled, shows "Deploy"
- `deploying` → disabled, shows "Deploying..."
- `running` → disabled (use Restart/Redeploy instead)
- `error` → enabled, shows "Retry Deploy"

## Affected Areas
- `frontend/src/app/(main)/dashboard/new/page.tsx` — button text change, redirect to configure
- `frontend/src/app/(main)/projects/[id]/configure/page.tsx` (new) — configure page
- `frontend/src/app/(main)/projects/[id]/configure/loading.tsx` (new) — loading skeleton
- `frontend/src/app/(main)/projects/[id]/page.tsx` — status badges, configure CTA
- `frontend/src/app/(main)/projects/[id]/layout.tsx` — status-aware layout
- `frontend/src/app/(main)/projects/[id]/actions/page.tsx` — rename restart, add configure link
- `frontend/src/app/(main)/projects/[id]/settings/page.tsx` — compose YAML editing
- `frontend/src/components/compose-editor.tsx` (new) — form + raw YAML toggle
- `frontend/src/components/service-form.tsx` (new) — per-service form
- `frontend/src/components/deploy-button.tsx` (new) — deploy trigger
- `frontend/src/lib/api.ts` — new types + functions

## Acceptance Criteria / Definition of Done
- [ ] "Create Project" creates draft, redirects to configure page
- [ ] Configure page shows spinner during analysis, then compose editor
- [ ] Form view: structured fields for each service (image, ports, build)
- [ ] Raw YAML view: editable textarea with full compose YAML
- [ ] Form ↔ YAML sync works on tab switch
- [ ] Env var CRUD works on configure page
- [ ] Deploy button triggers deployment, redirects to dashboard
- [ ] Status badges updated for new lifecycle states
- [ ] Project dashboard shows CTA for draft/configuring projects
- [ ] Settings page allows compose YAML editing
- [ ] `npx tsc --noEmit` and `npm run build` pass
- [ ] Consistent with existing shadcn UI patterns

## Test Plan
Browser: create a remote project → see draft status → configure page shows analysis spinner → compose appears → edit form → switch to raw YAML → deploy → see running status.
Create a compose project → paste YAML → configure page shows it → add env vars → deploy.

## Implementation Notes
To be filled during implementation.

## Review Notes
<filled in by reviewer>

## Test Notes
<filled in by tester>
