---
id: BUG-036
type: bug
title: Frontend standalone Docker image starts server from the wrong path
status: done
complexity: simple
assignee: unassigned
sprint: SPRINT-005
created: 2026-07-13
history:
  - {date: 2026-07-13, stage: created, by: architect, note: "filed from BUG-035 current-source runtime preparation"}
  - {date: 2026-07-13, stage: development, by: architect, note: "selected as C0 blocker; assigned to developer_simple"}
  - {date: 2026-07-13, stage: review, by: architect, note: "implementation verified; assigned to reviewer_simple"}
  - {date: 2026-07-13, stage: integration-hold, by: architect, note: "review passed; held with BUG-035 for TEST-019 C0 integration verification"}
  - {date: 2026-07-13, stage: changes-requested, by: architect, note: "C0 builder found nested standalone path absent in Docker build; assigned to developer_simple"}
  - {date: 2026-07-13, stage: development, by: architect, note: "rework assigned to developer_simple using Docker-build artifact evidence"}
  - {date: 2026-07-13, stage: review, by: architect, note: "Docker-build path rework complete; assigned to reviewer_simple"}
  - {date: 2026-07-13, stage: integration-hold, by: architect, note: "rework review passed; held with BUG-035 for TEST-019 C0 integration verification"}
  - {date: 2026-07-13, stage: done, by: architect, note: "TEST-019 C0 integration passed; ready for cluster commit"}
---

## Summary
A freshly built frontend image enters a restart loop because its Docker command
looks for `/app/server.js`, while Next standalone output is nested at
`/app/.next/standalone/frontend/server.js` before the Dockerfile copy step.
This makes the frontend unavailable behind the shared proxy (HTTPS 502).

**Part of:** C0 Frontend runtime baseline

**Cluster Test:** TEST-019

**Depends on:** none

## Steps to Reproduce
1. From the repository root, run `docker compose build frontend`.
2. Run `docker compose up -d --no-deps frontend`.
3. Inspect frontend container state/logs and request `https://localhost`.

## Expected Behavior
The current frontend source builds into an image whose frontend service remains
running and serves the application through the proxy.

## Actual Behavior
`tamga-frontend-1` restarts and logs `Error: Cannot find module
'/app/server.js'`; the proxy returns HTTPS 502.

## Environment / Context
Observed 2026-07-13 in builder runtime preparation. The new image was
`sha256:972f28e…`; the previously running frontend image predated the current
source change. Locally, `next.config.ts` produces standalone output under
`frontend/.next/standalone/frontend/server.js`; the Docker build stage instead
copies only `frontend/` into `/app` before running the same build.

## Root Cause
`outputFileTracingRoot` makes Next preserve the application path relative to
the tracing root in the standalone artifact. On the host, the app is the
repository's `frontend/` child, which yields `standalone/frontend`. In the
Docker builder, `WORKDIR /app` and `COPY frontend/ .` make the app `/app`,
while the configured tracing root is its parent `/`; Next therefore emits
`/app/.next/standalone/app/server.js`. The first fix incorrectly used the
host-only `frontend` segment, so its runner-stage COPY source did not exist.

## Proposed Solution
Copy the Docker builder's `app` subdirectory of the standalone artifact into
the runner working directory. This makes `/app/server.js` (and its relative
`.next` and `public` paths) agree with the existing command and the separately
copied static assets, without changing the tracing configuration.

## Affected Areas
- `deploy/Dockerfile.frontend`
- `frontend/next.config.ts`
- `docker-compose.yml` frontend service (read for command/image context)

## Acceptance Criteria
- [ ] A fresh `docker compose build frontend` followed by frontend-only start
      leaves the frontend service running, without module-not-found restarts.
- [ ] `https://localhost` returns the frontend application rather than 502.
- [ ] The image continues to run the intended Next standalone server and does
      not depend on host source mounts or the old image.

## Test Plan
1. Inspect the actual standalone build directory and Dockerfile copy/CMD path.
2. Build only the frontend image and start only the frontend service through
   the project helper-prepared shared stack.
3. Verify container state/logs and HTTPS response; cleanup only task-owned
   resources and never compose-down the shared stack.

## Implementation Notes
- Initial implementation used the local host artifact segment (`frontend`),
  which was invalid in the Docker builder and was rejected by the C0 builder.
- Rework changes `deploy/Dockerfile.frontend` so the runner copies
  `/app/.next/standalone/app` to `/app`. The existing `CMD ["node",
  "server.js"]` then resolves the builder's relative standalone entry point;
  static and public copies remain relative to that application root.
- Static checks: inspected Next's standalone writer, which joins the output
  with `path.relative(outputFileTracingRoot, appDir)`; with builder app dir
  `/app` and tracing root `/`, that relative path is `app`. `node` confirmed
  `path.relative("/", "/app") === "app"`; the Dockerfile COPY/CMD/static
  layout is consistent and `git diff --check` is clean. No Docker, compose,
  service, or browser runtime action was run.
- C0 prerequisite: BUG-035 is held in review after its review PASS. Once this
  task receives review PASS, both parts remain held for the single TEST-019
  builder/tester run; that integration test owns fresh-image/container/HTTPS
  verification.
- Rework is still part of C0: TEST-019 alone reruns fresh-image, container,
  and HTTPS runtime QA after review; this task does not run a separate test.

## Review Notes
Reviewer appends.

### 2026-07-13 — PASS (reviewer_simple)

- Confirmed the generated standalone artifact places `server.js`, `package.json`,
  its traced `node_modules`, and `.next/server` beneath
  `frontend/.next/standalone/frontend` because the existing tracing root is the
  repository root.
- The runner now copies exactly that application subdirectory to `/app`, so
  `CMD ["node", "server.js"]` resolves `/app/server.js`; the separately copied
  `.next/static` and `public` directories remain at the relative paths the
  standalone server expects.
- `outputFileTracingRoot` is unchanged, so this does not conceal or alter the
  repository-root tracing behavior. The product diff is limited to the runner
  copy source and `git diff --check` is clean.
- No Docker, Compose, or browser runtime test was run in review. This part is
  **ready for C0 integration**, not runtime-passed; TEST-019 exclusively owns
  fresh-image/container/HTTPS verification with BUG-035.

### 2026-07-13 — PASS after rework (reviewer_simple)

- Rechecked the Docker-builder topology rather than relying on the host artifact:
  the builder application directory is `/app`, while `outputFileTracingRoot` is
  `/`, so Next's standalone writer uses `path.relative("/", "/app") ===
  "app"` and emits the application entry at
  `/app/.next/standalone/app/server.js`.
- `COPY --from=builder /app/.next/standalone/app ./` therefore places the
  standalone `server.js`, its traced dependencies, and `.next/server` under
  the runner workdir `/app`. The generated server sets its directory from
  `__dirname` and changes into it, so the retained `CMD ["node", "server.js"]`
  and separately copied `/app/.next/static` and `/app/public` remain aligned.
- The prior `standalone/frontend` conclusion was host-layout-specific and is
  fully superseded by the C0 Docker-build evidence and this builder-layout
  review. The product diff is limited to the corrected COPY source and
  `git diff --check` is clean.
- No Docker, Compose, service, or browser runtime action was run. BUG-036 is
  **ready for C0 integration** with BUG-035 and remains held for the single
  TEST-019 fresh-image/container/HTTPS verification; it is not runtime-passed.

## Test Notes
Tester appends.

### 2026-07-13 — C0 integration evidence

The single C0 builder cycle ran `docker compose build frontend` after a
successful shared-stack prepare/smoke. The build failed at Dockerfile COPY of
`/app/.next/standalone/frontend`: that source path is absent in the Docker
builder output, so the prior local-output inspection was not representative of
the container build. No new frontend container was started; the previous
frontend image remained serving HTTP 200 after manifest cleanup. Rework must
make the Dockerfile copy path match the artifact produced inside its own build
stage. Runtime/browser QA remains exclusively owned by the TEST-019 rerun.

## Pipeline Telemetry
| date | role | model | effort | result | duration | rework |
|---|---|---|---|---|---|---|
| 2026-07-13 | developer | Luna | medium | implemented; static layout checks passed | — | 0 |
| 2026-07-13 | reviewer | Luna | medium | PASS; ready for C0 integration, not runtime-passed | — | 0 |
| 2026-07-13 | builder | builder | low | C0 FAIL: Docker build lacks nested standalone/frontend source path | n/a | 1 |
| 2026-07-13 | developer | Luna | medium | reworked Docker builder standalone path; ready for review | — | 1 |
| 2026-07-13 | reviewer | Luna | medium | PASS after rework; held for TEST-019 C0 integration, not runtime-passed | — | 1 |
