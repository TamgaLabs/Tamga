---
id: FEAT-030
type: feature
title: Metrics time-series schema + storage repo
status: done
complexity: standard
assignee: sdlc-developer
sprint: SPRINT-004
created: 2026-07-11
history:
  - {date: 2026-07-11, stage: created, by: architect, note: "SPRINT-004 C3 cluster"}
  - {date: 2026-07-11, stage: development, by: architect, note: "assigned (C3 storage foundation)"}
  - {date: 2026-07-11, stage: review, by: architect, note: "migration 000017 + 2 tables (increments) + repo + documented seam; C3 HOLD pending TEST-015"}
  - {date: 2026-07-11, stage: hold, by: architect, note: "review PASS (rollup SQL traced idempotent); holding for TEST-015. Note: le is TEXT lexical -> FEAT-032 sorts numerically"}
  - {date: 2026-07-11, stage: rework, by: architect, note: "TEST-015 caught: InsertMetricSamples/LatencyBuckets used overwrite upsert (excluded.count), clobbering sub-minute scrapes into 0. Fixed to additive (table.count + excluded.count); repurposed overwrite test + added TestMetricSamplesSubMinuteScrapesAccumulate. Rollup insert stays overwrite (recomputes SUM). Re-verified live via TEST-015."}
  - {date: 2026-07-11, stage: done, by: architect, note: "reworked (additive upsert) + TEST-015 PASS; cluster C3 committed"}
---

**Part of:** C3-metrics
**Depends on:** (none — first of the cluster)

## Summary
The storage foundation for traffic analytics: a SQLite time-series that the
scraper (FEAT-031) writes and the query API (FEAT-032) reads. Stores
per-service (= per-project) traffic metrics derived from Traefik's
Prometheus output (per TEST-010: `traefik_router_requests_total{code,method,
router,service}`, `traefik_router_request_duration_seconds_bucket`,
`traefik_router_requests_bytes_total`/`responses_bytes_total`, router/service
name = `project-<id>`). Retention is a minute→hour→day rollup (decided with
the user).

## Requirements
- Migration `000017` (verify next number): a metrics time-series table. Design
  for the four panels (request rate, status-class distribution + error rate,
  latency p50/p95/p99, bandwidth) at per-project + global granularity, with a
  resolution/rollup dimension. Concretely (adjust as the scraper's data shape
  demands — coordinate the seam with FEAT-031/032):
  - A samples table keyed by (bucket_start timestamp, resolution enum
    minute|hour|day, project_id — 0/NULL for the Tamga core/global scope,
    and enough dimension to reconstruct the panels). For counters store the
    per-interval INCREMENT (delta), not the raw cumulative — so rate/rollup
    are simple sums; document this choice. For status: keep per-status-class
    (2xx/3xx/4xx/5xx) counts. For latency: store the histogram bucket counts
    (le boundaries) per interval so percentiles are computable at query time.
    For bandwidth: request+response bytes increments.
  - Index on (project_id, resolution, bucket_start) for range queries.
- Domain types for a metrics sample / the panel rows.
- Repository (repository/sqlite): insert samples (batch), query a
  project's/global samples over a [from,to] range at a given resolution,
  delete/prune samples older than a cutoff per resolution (for FEAT-032's
  retention), and aggregate minute→hour / hour→day (or expose the primitives
  FEAT-032 needs — coordinate).
- Tests (black-box, backend/internal/tests/): migration applies fresh +
  copied DB; insert + range-query round-trip; prune-by-cutoff.

## Out of Scope
- The scraper (FEAT-031), rollup sweep + query API (FEAT-032), UI (C4).
- Deciding the exact Prometheus parse — that's FEAT-031; here just design the
  storage to hold what those metrics decompose into (increments + histogram
  buckets + status classes + bytes).

## Proposed Solution / Approach
Two purpose-shaped tables rather than one generic "long" samples table,
since the four panels decompose cleanly into two fixed shapes: fixed
per-interval counters (status classes + bytes) map naturally onto columns
on a single row per `(project_id, resolution, bucket_start)`
(`metric_samples`); the latency histogram has a variable number of `le`
boundaries, so it gets its own narrow table additionally keyed by `le`
(`metric_latency_buckets`). Both tables share the same key prefix
`(project_id, resolution, bucket_start)` — that shared key is the seam
FEAT-031 (writer) and FEAT-032 (reader) coordinate on (documented in full
in Implementation Notes below and in the migration file's header comment).

Every counter/count column stores the **per-interval INCREMENT (delta)**
of Traefik's cumulative Prometheus counters, not the raw cumulative value —
diffing happens once, in the scraper (FEAT-031), when it converts a scrape
into a row. This keeps every read-side operation (rate = SUM over a range,
rollup = SUM GROUP BY project_id of the finer-resolution rows in a coarser
bucket's window) a plain SQL SUM with no need to carry/diff a running
cumulative value at query time.

`bucket_start` is stored as an INTEGER unix-second epoch, not a
DATETIME/TEXT column — it's a value the scraper/rollup sweep computes and
range-compares as a plain integer (`bucket_start >= ? AND bucket_start <
?`), avoiding any ambiguity from the driver's TEXT-timestamp formatting
that would otherwise need to match `CURRENT_TIMESTAMP`'s format exactly.

`project_id = 0` (`domain.GlobalProjectID`) is the Tamga
core/global scope, not a real project — no `FOREIGN KEY` on `project_id`
(0 would never satisfy one against `projects.id`).

The `UNIQUE(project_id, resolution, bucket_start[, le])` constraint on
each table doubles as the required range-query index (its leading columns
are exactly `project_id, resolution, bucket_start`), so no separate
`CREATE INDEX` was added — SQLite uses a UNIQUE constraint's implicit
index for range scans on its leading columns.

The minute→hour / hour→day rollup is implemented as one SQL primitive,
`DB.AggregateMetrics(srcResolution, dstResolution, dstBucketStart)`, done
in SQL (an `INSERT ... SELECT ... GROUP BY project_id [, le] ... ON
CONFLICT DO UPDATE`) rather than pulling rows into Go and summing there —
it's a single set-based operation SQLite is well-suited for, keeps the
op atomic within one transaction, and the upsert makes it idempotent
(safe for FEAT-032's rollup sweep to retry). `PruneMetrics` similarly
deletes from both tables in one call per resolution, since FEAT-032's
retention sweep always prunes both tables in lockstep per resolution.

## Affected Areas
- **New:** `backend/internal/repository/sqlite/migrations/000017_create_metrics_timeseries.up.sql` /
  `.down.sql` — the `metric_samples` and `metric_latency_buckets` tables,
  next migration number after 000016 (confirmed: 000016 was last).
- **New:** `backend/internal/domain/metric.go` — `MetricResolution` (+
  `MetricResolutionMinute/Hour/Day` constants), `GlobalProjectID`,
  `MetricSample`, `MetricLatencyBucket`.
- **New:** `backend/internal/repository/sqlite/metrics_repo.go` —
  `InsertMetricSamples`, `InsertMetricLatencyBuckets` (batch upsert),
  `ListMetricSamples`, `ListMetricLatencyBuckets` (range query),
  `PruneMetrics` (cutoff delete, both tables), `AggregateMetrics`
  (minute→hour / hour→day rollup primitive), plus the unexported
  `resolutionWindow` helper.
- **New:** `backend/internal/tests/sqlite/metrics_repo_test.go` —
  black-box tests: migration on fresh DB, migration on a throwaway copy
  of the live `data/tamga.db` (skips if absent, never touches the real
  file), insert+range-query round-trip for both tables (including
  project/global scope isolation and resolution isolation), upsert
  idempotency, prune-by-cutoff (resolution-scoped), and the
  minute→hour aggregation round-trip (including idempotent re-run).
- No existing files were modified — this is additive-only (new
  migration, new domain types, new repo file, new tests).

## Acceptance Criteria / Definition of Done
- [ ] Migration 000017 adds the time-series table(s) with a resolution dimension + project scope (0/global); applies on fresh + copied DB
- [ ] Storage holds what the four panels need: per-interval request counts by status class, latency histogram buckets, request/response byte increments — per project + global
- [ ] Repo: batch insert, range query (project/global, resolution, [from,to]), prune-by-cutoff, and the min→hour/hour→day aggregation primitives FEAT-032 needs
- [ ] Indexed for range queries
- [ ] `go build/vet/test` pass; black-box tests cover migration + insert/query/prune
- [ ] Code follows KISS/YAGNI (don't build a general TSDB — just these panels)

## Test Plan
Black-box: apply migration, insert synthetic samples across resolutions +
projects, range-query them back, prune old ones. (Live scrape→store is
TEST-015.)

## Implementation Notes

### The seam: exact row shape FEAT-031 writes / FEAT-032 reads

**`metric_samples`** — one row per `(project_id, resolution, bucket_start)`:
```
project_id    INTEGER  -- real project's id, or 0 (domain.GlobalProjectID) for core/global
resolution    TEXT     -- 'minute' | 'hour' | 'day'
bucket_start  INTEGER  -- unix seconds, aligned to the start of the interval
count_2xx     INTEGER  -- INCREMENT since the previous scrape (not cumulative)
count_3xx     INTEGER  -- ditto
count_4xx     INTEGER  -- ditto
count_5xx     INTEGER  -- ditto
bytes_in      INTEGER  -- request bytes INCREMENT (traefik_router_requests_bytes_total delta)
bytes_out     INTEGER  -- response bytes INCREMENT (traefik_router_responses_bytes_total delta)
```
FEAT-031, per project (router/service = `project-<id>@file`, per TEST-010
§4), sums `traefik_router_requests_total{code,router=~"project-<id>.*"}` by
first digit of `code` into `count_{2,3,4,5}xx`, and takes the delta of
`traefik_router_requests_bytes_total` / `..._responses_bytes_total` since
its last scrape into `bytes_in`/`bytes_out` — one row per project per
`minute` bucket, `INSERT`ed via `DB.InsertMetricSamples` (batch, one call
per scrape tick covering every project + the `entrypoint`-level global
row written at `project_id = 0`). Request rate = `SUM(count_2xx+count_3xx
+count_4xx+count_5xx)`; error rate = `SUM(count_4xx+count_5xx) /
SUM(all)`, both over `DB.ListMetricSamples(projectID, resolution, from,
to)`'s result.

**`metric_latency_buckets`** — one row per `(project_id, resolution,
bucket_start, le)`:
```
project_id    INTEGER
resolution    TEXT
bucket_start  INTEGER
le            TEXT     -- Traefik's own le label verbatim: "0.1","0.3","1.2","5","+Inf"
count         INTEGER  -- INCREMENT of the CUMULATIVE bucket count (Prometheus histogram_bucket semantics)
```
FEAT-031 takes the per-`le` delta of
`traefik_router_request_duration_seconds_bucket{le,router,...}` since its
last scrape and writes one row per `le` per project per `minute` bucket
via `DB.InsertMetricLatencyBuckets` (batch). Because `count` per `le` is
still cumulative-across-`le` *within* one `bucket_start` (only the
across-time delta was taken), FEAT-032 computes p50/p95/p99 the same way
PromQL's `histogram_quantile` does: walk `le` ascending, find where the
cumulative fraction crosses 0.50/0.95/0.99, linear-interpolate between
that `le` and the previous boundary.

**Rollup contract:** FEAT-032's rollup sweep calls
`DB.AggregateMetrics(domain.MetricResolutionMinute,
domain.MetricResolutionHour, hourBucketStart)` once per completed hour
(and the `Hour`→`Day` equivalent once per completed day) — this sums
every finer-resolution row inside that window, `GROUP BY project_id` (and
`+ le` for the latency table), and upserts one coarser row per project.
Idempotent by construction (`ON CONFLICT DO UPDATE`), so a retried sweep
tick is safe. **Rollup is FEAT-030's stored primitive, not FEAT-030's
scheduler** — nothing in this task runs it periodically; FEAT-032 owns
calling it on a schedule (and owns retention: `DB.PruneMetrics(resolution,
cutoff)` deletes both tables' rows older than `cutoff` for one resolution
in a single call).

### Decisions
- **Increment-vs-cumulative:** every counter column is a per-interval
  DELTA the scraper computes against Traefik's cumulative Prometheus
  counters, never the raw cumulative — documented at the top of the
  migration file itself since this is the single most important
  interpretation rule for both sibling tasks. This makes every read
  (rate, rollup, retention) a plain `SUM`/`DELETE ... WHERE bucket_start <
  cutoff` with no running-value bookkeeping needed at query time.
- **Two purpose-shaped tables, not one generic "long" table:** the four
  panels decompose into exactly two data shapes (fixed per-interval
  counters vs. a variable-`le` histogram), so two tables shaped for each
  keep both FEAT-031's writes and FEAT-032's queries as plain, single-
  purpose SQL rather than a `metric/label/value` long table that would
  need `GROUP BY metric` gymnastics for every panel query.
- **`bucket_start` as INTEGER unix-seconds**, not DATETIME/TEXT — a value
  the application computes and range-compares, not a DB-generated
  timestamp; avoids relying on the sqlite driver's TEXT-timestamp
  round-trip format matching `CURRENT_TIMESTAMP`'s own format.
- **No separate `CREATE INDEX`:** the `UNIQUE(project_id, resolution,
  bucket_start[, le])` constraint's implicit index already has exactly
  the leading columns (`project_id, resolution, bucket_start`) the
  Requirements ask to index for range queries — adding a second identical-
  prefix index would be redundant, not simpler.
- **Aggregation done in SQL** (`INSERT ... SELECT ... GROUP BY ... ON
  CONFLICT DO UPDATE`) inside `DB.AggregateMetrics`, not pulled into Go —
  a single set-based, transactional, idempotent operation; FEAT-032 only
  needs to call it with the right `(srcResolution, dstResolution,
  dstBucketStart)` on its own schedule.

### Files
- `backend/internal/repository/sqlite/migrations/000017_create_metrics_timeseries.{up,down}.sql`
- `backend/internal/domain/metric.go`
- `backend/internal/repository/sqlite/metrics_repo.go`
- `backend/internal/tests/sqlite/metrics_repo_test.go`

### Verification
`go build ./...`, `go vet ./...`, `go test ./...` all pass (full suite,
not just the new package). The copied-live-DB migration test
(`TestMetricsMigrationAppliesOnCopiedLiveDB`) ran against a throwaway
`t.TempDir()` copy of the real `data/tamga.db` present in this
environment and passed; `data/tamga.db`'s own mtime/git-status is
unchanged (untracked, not touched) — confirmed via `ls -la`/`git status`
before and after. No live stack rebuild/restart was performed, per the
task's instruction.

This is additive-only: no existing migration, domain type, or repo file
was modified. `complexity: standard`, implemented directly (no `opencode`
delegation).

## Review Notes

### 2026-07-11 — reviewer (opencode/architect-directed review)

**Verdict: PASS**

Scope check: `git status` shows a large amount of unrelated uncommitted
frontend/infra churn (package.json, layout.tsx, Caddyfile deletion, etc.)
that predates this task and isn't mentioned anywhere in FEAT-030's
Implementation Notes/Affected Areas — treated as ambient WIP per the
repo's normal working-tree state, not scope creep from this task. The
task's own diff is exactly its declared Affected Areas: migration 000017
(up+down), `domain/metric.go`, `repository/sqlite/metrics_repo.go`,
`tests/sqlite/metrics_repo_test.go`. No existing file (db.go, other domain
types, other repos) was touched — confirmed via `git log -- migrations/`
and `git diff --stat` against tracked files. Additive-only claim holds.

1. **Migration 000017.** Confirmed 000016 (`create_project_services`) is
   the prior head, so 000017 is correctly the next number. Both tables
   created with `IF NOT EXISTS`, CHECK-constrained `resolution` enum
   matching `domain.MetricResolution*` constants exactly, `project_id`
   correctly has no FK (0/global documented, matches
   `domain.GlobalProjectID`). Style (AUTOINCREMENT id, `created_at
   TIMESTAMP DEFAULT CURRENT_TIMESTAMP`, UNIQUE constraint) mirrors
   000016/existing migrations. Down drops both tables, no ordering issue
   (no FK between them). UNIQUE(project_id, resolution, bucket_start) on
   `metric_samples` and UNIQUE(project_id, resolution, bucket_start, le)
   on `metric_latency_buckets` — column order matches the range query's
   predicate order exactly (`WHERE project_id = ? AND resolution = ? AND
   bucket_start >= ? AND bucket_start < ?` in `ListMetricSamples`/
   `ListMetricLatencyBuckets`), so SQLite's implicit UNIQUE index serves
   as the range index as claimed — no separate `CREATE INDEX` needed.
   Verified against actual query text, not just the claim.

2. **Increment design.** Columns (`count_2xx/3xx/4xx/5xx`, `bytes_in/out`,
   latency `count`) are plain per-interval deltas, documented in three
   places (migration header comment, domain.go doc comments, task's
   Implementation Notes) with identical framing — unambiguous for
   FEAT-031. Good redundancy for a cross-task seam.

3. **metrics_repo.go.** `InsertMetricSamples`/`InsertMetricLatencyBuckets`
   use `ON CONFLICT DO UPDATE SET <col> = excluded.<col>` (full overwrite,
   not additive) — correct semantics for a re-scrape retry recomputing the
   same interval's increment (DO NOTHING would silently drop a corrected
   recompute; additive would double-count). `ListMetricSamples`/
   `ListMetricLatencyBuckets` range queries match the seam's documented
   `[from, to)` semantics and correctly isolate by project_id AND
   resolution (tested explicitly: a same-bucket_start row at a different
   resolution doesn't leak into a query, per
   `TestMetricSamplesInsertAndRangeQuery`). `PruneMetrics` deletes both
   tables per resolution+cutoff in one call, matching the task's
   documented "both tables in lockstep" design; tested that a same-
   timestamp row at a different resolution survives pruning one
   resolution (`TestPruneMetrics`).

   **AggregateMetrics SQL traced in full:**
   ```sql
   INSERT INTO metric_samples (project_id, resolution, bucket_start, count_2xx, ...)
   SELECT project_id, ?, ?, SUM(count_2xx), ...
   FROM metric_samples
   WHERE resolution = ? AND bucket_start >= ? AND bucket_start < ?
   GROUP BY project_id
   ON CONFLICT (project_id, resolution, bucket_start) DO UPDATE SET ... = excluded...
   ```
   Params bound in placeholder order: `dstResolution, from, srcResolution,
   from, to` — matches the 5 `?`s in appearance order (dst resolution/
   bucket_start literal for the INSERT's constant columns, then the
   src-side WHERE filter). GROUP BY project_id correctly collapses N
   finer-resolution rows into one row per project for the dst bucket;
   latency table's rollup adds `+ le` to the GROUP BY, correctly keeping
   per-le sums separate. Idempotent by construction: the upsert always
   writes the *recomputed* SUM from source rows (via `excluded.*`), it
   never increments an existing value, so a retried/duplicate call
   produces the same result rather than double-counting — this also means
   a late-arriving source row naturally corrects a prior aggregate on
   retry, which is a nice property. Verified against
   `TestAggregateMetricsMinuteToHour`, including its explicit re-run
   idempotency assertion and its "row just outside the window must not be
   summed in" case (`hourStart.Add(time.Hour)` correctly excluded).

4. **The seam (FEAT-031 write / FEAT-032 read).** Row shapes are
   unambiguous and sufficient for all four panels: rate/error-rate from
   summing the status-class columns, bandwidth from summing bytes_in/out,
   latency percentiles from the per-le increment table (cumulative-across-
   le within one bucket_start, matching Prometheus `histogram_bucket`
   semantics, so FEAT-032 can walk `le` ascending and interpolate exactly
   as documented). One thing to flag as **non-blocking**: `le` is stored
   as TEXT and `ListMetricLatencyBuckets`'s `ORDER BY ... le ASC` is a
   lexical sort, not numeric (e.g. "10" would sort before "5" if such a
   boundary ever existed) — the doc comment only promises grouping
   stability per bucket_start, not numeric le order, and Traefik's actual
   le values are conventional histogram boundaries plus `+Inf`, so this
   is very unlikely to bite, but FEAT-032 must parse+sort `le` numerically
   itself rather than trust SQL order. Worth a one-line callout in
   FEAT-032's task file when it's written, not a defect here.

5. **Tests** (`backend/internal/tests/sqlite/metrics_repo_test.go`):
   fresh-DB migration, copied-live-DB migration (re-verified below),
   insert+range round-trip with project/global/resolution isolation,
   upsert-overwrite-not-duplicate, prune-by-cutoff with resolution
   isolation, and minute→hour aggregation with an idempotent re-run and a
   just-outside-window exclusion case. Meaningfully exercises the seam's
   documented guarantees, not just happy-path CRUD. Reuses
   `openTestDB`/`copyFile` helpers already defined in
   `project_service_containers_test.go` (same package) rather than
   duplicating them — good.

6. **Verification re-run (not just trusting Implementation Notes):**
   - `go build ./...` — clean.
   - `go vet ./...` — clean.
   - `go test ./...` — full suite passes.
   - `go test ./internal/tests/sqlite/... -run Metrics -v` — all 5 metrics
     tests pass individually, including `TestMetricsMigrationAppliesOnCopiedLiveDB`
     (confirmed it actually ran against the copied live DB, not skipped —
     log shows "running migration file=000017..." for that subtest).
   - `gofmt -l` on all 4 new files — clean.
   - `data/tamga.db` mtime (1783751329) and md5 checksum
     (6c722459074fc85fcbe3d96b7afa5899) confirmed identical before and
     after the full test run — the live file was not touched, matching
     the developer's Verification claim.
   - Grepped the whole repo for callers of the new exported functions
     outside this task's own files — none yet, as expected (FEAT-031/032
     haven't started), so no duplication/inconsistency to flag there.

**Acceptance criteria walk:**
- [x] Migration 000017 adds both tables with resolution + project scope;
  applies fresh and on copied live DB — verified by test run.
- [x] Storage holds everything the four panels need — traced above.
- [x] Repo: batch insert, range query, prune-by-cutoff, minute→hour/
  hour→day aggregation primitive — all present and correct.
- [x] Indexed for range queries — UNIQUE constraint column order verified
  against actual query predicates.
- [x] `go build/vet/test` pass — re-ran independently, confirmed.
- [x] KISS/YAGNI — two purpose-shaped tables instead of a generic long
  table is the right call for exactly four fixed panels; no speculative
  generality found (no unused config knobs, no premature abstraction over
  resolutions beyond the 3 that exist).

**Minor/non-blocking notes:**
- `metric_samples`/`metric_latency_buckets` have a `created_at` column
  (row-insertion time) that isn't mapped onto `domain.MetricSample`/
  `MetricLatencyBucket` (unlike e.g. `domain.ServiceContainer`, which does
  expose `CreatedAt`). Harmless — nothing in the seam needs it — but
  slightly inconsistent with the sibling `service_container_repo.go`
  convention. Not worth blocking on.
- `AggregateMetrics` doesn't validate `srcResolution` against the known
  enum (only `dstResolution` goes through `resolutionWindow`) — an
  invalid/typo'd `srcResolution` would silently match zero rows rather
  than erroring. Documented as an explicit caller-contract choice in the
  doc comment; acceptable since the only caller is internal (FEAT-032),
  but worth keeping in mind if this function ever gets a wider caller
  surface.
- `le`-numeric-vs-lexical-sort point from §4 above — carry forward to
  FEAT-032, not a FEAT-030 defect.


## Test Notes
<filled in by tester>
