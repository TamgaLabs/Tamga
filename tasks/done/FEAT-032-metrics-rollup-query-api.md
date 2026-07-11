---
id: FEAT-032
type: feature
title: Metrics rollup/retention sweep + query API (panels)
status: done
complexity: standard
assignee: sdlc-developer
sprint: SPRINT-004
created: 2026-07-11
history:
  - {date: 2026-07-11, stage: created, by: architect, note: "SPRINT-004 C3 cluster"}
  - {date: 2026-07-11, stage: development, by: architect, note: "assigned (C3 rollup+query; FEAT-030/031 reviewed+holding)"}
  - {date: 2026-07-11, stage: review, by: architect, note: "rollup+retention + percentile helper (numeric le) + query API/handler wired; C3 HOLD pending TEST-015"}
  - {date: 2026-07-11, stage: rework, by: architect, note: "review CHANGES_REQUESTED: rollup fixed-window lookback prunes rows older than the window WITHOUT aggregating them (data loss after a long process gap). Derive oldest source bucket from MIN(bucket_start), not a fixed window."}
  - {date: 2026-07-11, stage: review, by: architect, note: "rework: oldest-row-driven rollup (OldestBucketStart+DistinctDstBucketStarts) + deep-gap test; delta review"}
  - {date: 2026-07-11, stage: hold, by: architect, note: "delta review PASS (deep-gap fix empirically confirmed); holding — running TEST-015"}
  - {date: 2026-07-11, stage: done, by: architect, note: "TEST-015 PASS (rollup+query verified live); cluster C3 committed"}
---

**Part of:** C3-metrics
**Depends on:** FEAT-030, FEAT-031

## Summary
The read side + retention for analytics: a background rollup that aggregates
minute samples → hourly → daily and prunes old fine-grained data (the
minute→hour→day policy), plus the HTTP query API that returns the four
panels' data per project and globally. C4's UI renders this.

## Requirements
- **Rollup + retention sweep** (a background goroutine or a tick folded into
  FEAT-031's — coordinate): periodically aggregate minute samples older than
  a threshold into hour buckets, hour into day, and prune the superseded
  finer-grained rows. Concrete policy (per the user's minute→hour→day
  decision): keep minute resolution for ~48h, hourly for ~30d, daily beyond
  — tune to sane defaults, document them. Aggregation = sum the increment
  counts / bucket counts / bytes into the coarser bucket (percentiles stay
  computable because we kept histogram bucket counts, not pre-computed
  percentiles). Idempotent + safe to run repeatedly.
- **Query API** (authenticated, mirror existing route style):
  - `GET /api/projects/{id}/metrics?from=&to=&resolution=` → the project's
    panels over the range; and a global variant
    `GET /api/system/metrics?from=&to=&resolution=` (scope 0/core+aggregate).
  - Response shape: time-series arrays for request rate (per-interval count
    → rate), status-class breakdown + error rate, latency p50/p95/p99
    (computed from the stored histogram bucket counts over the range), and
    bandwidth (bytes). Auto-pick resolution from the range if `resolution`
    omitted (e.g. <48h→minute, <30d→hour, else day). Propose the JSON shape
    C4 will consume — keep it panel-oriented.
  - Handle a project with no data (empty series, not an error).
- Percentile computation from histogram buckets: standard linear
  interpolation within the bucket containing the target quantile — a small
  pure helper, unit-tested (a known bucket distribution → known p50/p95/p99).
- Wire the rollup start + handler into cmd/api/main.go + router.go.
- Tests: pure percentile-from-buckets; rollup aggregation (minute rows →
  correct hour bucket sums, prune drops the right rows) — black-box where
  practical (seed FEAT-030's store, run rollup, assert); the query endpoint
  shaping.

## Out of Scope
- The scraper (FEAT-031), storage schema (FEAT-030), the analytics UI (C4),
  the map overlay (C5 — though it reuses this query API's per-project data).

## Proposed Solution / Approach
Two new services plus one handler, wired the same way FEAT-031's scraper and
FEAT-022's idle-sweep already are (goroutine started at construction, no
graceful-shutdown plumbing needed):

- `MetricsRollupService` (`backend/internal/service/metrics_rollup_service.go`):
  `NewMetricsRollupService(db, period)` starts a ticker (same
  period<=0-disables convention as the scraper) that calls the exported
  `Rollup(now time.Time) error` every tick. `Rollup` does, in order:
  minute→hour `AggregateMetrics` over a lookback window, prune minute rows
  older than `MetricsMinuteRetention` (48h), hour→day `AggregateMetrics`
  over its own lookback, prune hour rows older than `MetricsHourRetention`
  (30d). Day resolution is never pruned (kept indefinitely - daily rows are
  compact). Retention/rollup happen in the same call so `Rollup` alone is
  idempotent and safe to call directly from a test without a live ticker
  (`period=0` disables the goroutine).

  Key correctness point: the aggregation lookback for each resolution pair
  is retention+one extra bucket window (49h for minute→hour, 31d for
  hour→day), one window WIDER than the matching prune cutoff. This
  guarantees any bucket still gets aggregated at least once in the same
  `Rollup()` call that could otherwise prune it - required for the method to
  be safe as a single one-shot call (tests, or a restart after a long gap),
  not just under continuous frequent ticking where the previous tick would
  already have covered it. `AggregateMetrics` is cheap (an idempotent
  `INSERT...SELECT SUM...GROUP BY` per bucket) so re-aggregating buckets
  that already have no source rows left (post-prune) is a harmless no-op -
  `GROUP BY` over zero source rows produces zero output rows to upsert.

- Percentile math is a small pure helper,
  `PercentilesFromLatencyBuckets(buckets []*domain.MetricLatencyBucket) (p50, p95, p99 float64)`
  in `backend/internal/service/metrics_percentile.go`, used per bucket_start
  (one call per point in the latency panel, not once over the whole range -
  keeps every panel point independently computed/aligned to its own
  bucket_start like the other three panels). Implements the same
  linear-interpolation-within-bucket method PromQL's `histogram_quantile`
  uses. Per FEAT-030's review carry-forward: `le` is TEXT with lexical SQL
  order, so this function re-sorts its input NUMERICALLY (parses `le` to
  float64, `+Inf` to `math.Inf(1)`) rather than trusting caller order -
  callers never need to pre-sort.

- `MetricsQueryService` (`backend/internal/service/metrics_query_service.go`):
  `GetPanels(projectID, from, to, resolution)` - if resolution is "",
  auto-picks via `AutoResolution` (<48h→minute, <30d→hour, else day); lists
  samples + latency buckets for the range (FEAT-030's `ListMetricSamples`/
  `ListMetricLatencyBuckets`), groups latency buckets by bucket_start, and
  builds four independent point arrays (request rate, status-class+error
  rate, latency p50/p95/p99, bandwidth) - one point per sample row, sparse
  (no data ⇒ empty arrays, never nil/null, never an error). No zero-filled
  gap points: the task's "empty series for no-data" criterion reads as
  literally-empty, and gap-filling isn't needed for a chart panel that just
  plots whatever points exist.

- `MetricsHandler` (`backend/internal/handler/metrics_handler.go`) mirrors
  `ProjectHandler`'s style: `Project` (parses `{id}`) and `System` (fixed
  `domain.GlobalProjectID`) both funnel into one `respond` helper that
  parses `from`/`to` (Unix seconds - the same unit `bucket_start` already
  uses internally, so C4 stays in one timestamp format end to end) and
  `resolution`, defaulting to a 24h recent window when `from`/`to` are
  omitted, then calls `GetPanels` and encodes the result.

Wired into `cmd/api/main.go` (construct `MetricsQueryService` +
`MetricsRollupService(db, service.DefaultMetricsRollupInterval)`, same
fire-and-forget pattern as the scraper) and `router.go` (`GET
/api/projects/{id}/metrics`, `GET /api/system/metrics`, both inside the
authenticated route group).

## Affected Areas
- `backend/internal/service/metrics_rollup_service.go` (new) - rollup +
  retention sweep, `MetricsMinuteRetention`/`MetricsHourRetention`/
  `DefaultMetricsRollupInterval` consts.
- `backend/internal/service/metrics_percentile.go` (new) -
  `PercentilesFromLatencyBuckets` pure helper.
- `backend/internal/service/metrics_query_service.go` (new) -
  `MetricsQueryService`/`GetPanels`/`AutoResolution`/panel point structs.
- `backend/internal/handler/metrics_handler.go` (new) - `MetricsHandler`
  (`Project`/`System` routes + range-param parsing).
- `backend/cmd/api/main.go` - construct `MetricsRollupService` +
  `MetricsQueryService`, wire `MetricsHandler` into `router.New`.
- `backend/internal/router/router.go` - `metricsHandler` param, `GET
  /api/projects/{id}/metrics`, `GET /api/system/metrics` routes.
- `backend/internal/tests/service/metrics_percentile_test.go`,
  `metrics_rollup_service_test.go`, `metrics_query_service_test.go` (new).

## Acceptance Criteria / Definition of Done
- [ ] Rollup aggregates minute→hour→day and prunes superseded rows per a documented retention policy; idempotent
- [ ] `GET /api/projects/{id}/metrics` and `GET /api/system/metrics` return the four panels (rate, status/error, latency p50/p95/p99, bandwidth) over [from,to] at a resolution (auto-picked if omitted); empty series for no-data, not an error
- [ ] Percentiles computed from stored histogram buckets (linear interpolation), unit-tested against a known distribution
- [ ] Rollup + handler wired into main.go/router.go
- [ ] `go build/vet/test` pass; pure percentile + rollup-aggregation logic unit-tested
- [ ] Code follows KISS/YAGNI

## Test Plan
Unit: percentile-from-buckets, rollup aggregation + prune. Live end-to-end
(traffic → scrape → store → rollup → query returns real panel data) is
TEST-015.

## Implementation Notes
Implemented directly (complexity: standard), no opencode delegation.

- **Retention defaults** (documented as consts):
  `MetricsMinuteRetention = 48h`, `MetricsHourRetention = 30*24h` (30d), day
  resolution has no retention const and is never pruned. Rollup sweep ticks
  every `DefaultMetricsRollupInterval = 5m` (wired in main.go); `Rollup(now
  time.Time) error` is exported so tests call it directly and
  deterministically instead of waiting on the ticker (`period=0` disables
  the goroutine, same convention as the scraper's period<=0 check).
- **Aggregate-before-prune correctness**: each resolution pair's
  aggregation lookback is `retention + one dst window` (49h for
  minute→hour, 31d for hour→day) - one window *wider* than the matching
  prune cutoff. This guarantees a bucket is aggregated at least once in the
  same `Rollup()` call that could otherwise prune its source rows, so
  `Rollup` is correct as a single one-shot call (a test, or a restart after
  a long process gap), not just under continuous frequent ticking. Verified
  by `TestMetricsRollupMinuteToHourAndPrune` (rollup happens before the
  source rows built from it get pruned) and
  `TestMetricsRollupIsIdempotent` (re-running doesn't double-count -
  `AggregateMetrics` upserts a fresh SUM each time, never increments).
- **Query JSON shape** (`MetricsPanels`, `GET /api/projects/{id}/metrics`
  and `GET /api/system/metrics`, both authenticated):
  ```json
  {
    "project_id": 1,
    "from": "2026-07-11T00:00:00Z",
    "to": "2026-07-12T00:00:00Z",
    "resolution": "minute",
    "request_rate": [{"bucket_start": "...", "count": 10, "rate_per_sec": 0.166}],
    "status_class": [{"bucket_start": "...", "count_2xx": 8, "count_3xx": 0, "count_4xx": 1, "count_5xx": 1, "error_rate": 0.2}],
    "latency": [{"bucket_start": "...", "p50": 0.9, "p95": 5, "p99": 5}],
    "bandwidth": [{"bucket_start": "...", "bytes_in": 1000, "bytes_out": 2000}]
  }
  ```
  Four independent arrays, one point per bucket_start that has data
  (sparse, not gap-filled) - each array is `[]` (never `null`) when there's
  no data in range, and `GetPanels` never errors on the empty case, only on
  a malformed range/resolution. `resolution` auto-picks when the query
  param is omitted: <48h→minute, <30d→hour, else day (`AutoResolution`).
  `from`/`to` query params are Unix seconds (same unit `bucket_start`
  already uses internally); both default to a 24h recent window when
  omitted.
- **Percentile helper**: `PercentilesFromLatencyBuckets` in
  `metrics_percentile.go`, called once per bucket_start (so the latency
  panel is point-aligned with the other three panels, not one summary value
  over the whole range). Parses every `le` to `float64` (`"+Inf"` →
  `math.Inf(1)`) and re-sorts NUMERICALLY before doing the
  cumulative-count linear interpolation - the FEAT-030 review carry-forward
  about lexical TEXT ordering (`"10" < "5"` as strings). Target falling
  inside the `+Inf` overflow bucket reports the highest finite `le`
  (standard `histogram_quantile` convention, no upper bound to interpolate
  against). `TestPercentilesFromLatencyBucketsNumericLeSortNotLexical`
  feeds buckets in an unsorted order that includes the exact "10" vs "5"
  lexical trap and asserts against the numerically-correct interpolation.
- No config/env additions - rollup interval and retention are consts
  (YAGNI: task didn't ask for them to be tunable, and none of FEAT-030/031's
  existing consts of this kind are either, aside from the scraper's own
  pre-existing config knob).
- `go build ./...`, `go vet ./...`, `go test ./...` all pass; `gofmt -l` on
  every new/touched file returned nothing.
- Did not touch the live stack (TEST-015 covers end-to-end scrape→rollup→
  query verification against real Traefik traffic).

### 2026-07-11 — rework (review C3 CHANGES_REQUESTED: deep-gap data loss)

Fixed the blocking bug: `Rollup`'s fixed-window lookback (retention + one
extra bucket window) only aggregated source rows within
`[now-lookback, now]`, so any minute/hour row **older** than that lookback
that had never previously been aggregated - the exact "restart after a long
process gap" scenario the old docstring claimed to handle - got pruned in
the same `Rollup()` call with no coarser aggregate ever created: silent,
unrecoverable data loss. Replaced the fixed lookback with aggregation
driven from the actual oldest surviving source row, so no source bucket is
ever pruned before it's had a chance to be rolled up, regardless of gap
length:

- **`backend/internal/repository/sqlite/metrics_repo.go`** (additive, FEAT-030's
  other methods untouched): added `OldestBucketStart(resolution)
  (int64, bool, error)` (`MIN(bucket_start)` across both `metric_samples`
  and `metric_latency_buckets` for a resolution, `ok=false` if no rows) and
  `DistinctDstBucketStarts(resolution, dstWindow, from, to)
  ([]time.Time, error)` (the distinct dst-resolution bucket starts - each
  source `bucket_start` truncated down to the containing `dstWindow`
  boundary via integer division in SQL - that actually have source rows in
  `[from, to)`, so a deep gap with no data in it contributes zero buckets to
  iterate rather than one per `dstWindow` step across the whole gap). Also
  exported `resolutionWindow` → `ResolutionWindow` to close the
  non-blocking dup note (`metrics_query_service.go`'s `bucketWindow` was a
  byte-for-byte duplicate); `metrics_query_service.go` now calls
  `sqlite.ResolutionWindow` directly and its local `bucketWindow` is gone.
- **`backend/internal/service/metrics_rollup_service.go`**: `Rollup` now computes
  each resolution's aggregate-cutoff (`now - retention`, the same instant
  `PruneMetrics` prunes at) and passes only that cutoff to
  `rollupResolution` - no more `now` or a fixed `lookback` argument.
  `rollupResolution` calls `OldestBucketStart(src)` to find where to start,
  then `DistinctDstBucketStarts(src, window, oldest, cutoff)` to get exactly
  the dst buckets that need aggregating, and calls the existing
  (unmodified) `AggregateMetrics` once per bucket - unbounded by gap length,
  bounded only by how much actual data sits in `[oldest, cutoff)`.
  Aggregating up to `cutoff` rather than `now` is also a (minor, explicitly
  requested) behavior correction: buckets more recent than `cutoff` haven't
  hit this resolution's retention age yet, so they're left alone at the
  finer resolution instead of being rolled up early - only buckets that are
  actually about to be pruned get rolled up, matching the AC's "prunes
  superseded rows" wording literally.
- **Test**: added `TestMetricsRollupDeepGapDoesNotLoseData` in
  `metrics_rollup_service_test.go` - seeds minute rows, then calls `Rollup`
  with `now` two full `MetricsMinuteRetention+1h` margins past them (a gap
  deep enough that the *old* fixed-lookback design's aggregation window
  would never reach back to the seeded rows at all, while `PruneMetrics`'
  cutoff would still be past them and delete them) and asserts the hour
  aggregate exists with the correct summed counts/bytes/latency-bucket
  count, and that the source minute rows are pruned. This reproduces
  exactly the bug the reviewer described and would fail against the
  pre-fix code.
- Updated the now-stale `TestMetricsRollupIsIdempotent`: its old `now`
  (5 minutes past the seeded row) relied on the *old* design's "aggregate
  up to `now`, prune only past retention" behavior to get an initial hour
  row to check idempotency against. Under the corrected "aggregate up to
  `cutoff`, not `now`" semantics, a bucket still within retention is
  correctly left at minute resolution and never touches the hour table -
  so the test now uses a `now` past retention (consistent with the other
  two rollup tests) and asserts the *second* `Rollup` call, which finds no
  surviving minute rows left (`OldestBucketStart` returns `ok=false`), is a
  pure no-op that leaves the already-rolled-up hour sum untouched rather
  than double-counting or zeroing it.
- `go build ./...`, `go vet ./...`, `go test ./...` all pass (including the
  new deep-gap test); `gofmt -l` clean on every touched file.

## Review Notes
<filled in by reviewer>

### 2026-07-11 — reviewer

**Verdict: CHANGES_REQUESTED**

Scope check: `git status`/`git diff` scoped to backend confirms this task's
footprint matches its Affected Areas exactly — new `metrics_percentile.go`,
`metrics_rollup_service.go`, `metrics_query_service.go`,
`handler/metrics_handler.go`, plus `main.go`/`router.go` wiring and the
three new test files. The other uncommitted changes in the tree
(`.env.example`, `AGENTS.md` deletion, frontend files, `plan.md` deletion,
`backend/internal/config/config.go`, `backend/internal/service/metrics_scraper.go`,
`backend/internal/repository/sqlite/metrics_repo.go` + migration, etc.) are
FEAT-030/031's already-reviewed-and-holding work plus unrelated ambient WIP
— not this task's doing, not scope creep here.

`go build ./...`, `go vet ./...`, `go test ./...` all pass (backend). `gofmt -l`
clean on the touched files.

**1. Percentile helper (`metrics_percentile.go`) — solid, no issues.**
- Numeric `le` sort verified correct: parses to float64, `"+Inf"` →
  `math.Inf(1)`, `sort.Slice` on parsed value (not string). The
  `TestPercentilesFromLatencyBucketsNumericLeSortNotLexical` test's "10" vs
  "5" trap genuinely traps a lexical-sort regression — lexical order would
  be `"+Inf" < "1" < "10" < "5"` (since `'1'<'5'` byte-wise), which would
  scramble the walk; the test's expected values are only reachable via the
  correct numeric order (1, 5, 10, +Inf). Ran it — passes.
- Hand-checked `TestPercentilesFromLatencyBucketsKnownDistribution`'s
  distribution independently: le→cumulative-count 0.1:10, 0.3:30, 1.2:60,
  5:90, +Inf:100. p50 target=50 falls between le=0.3(30) and le=1.2(60):
  `0.3 + (50-30)/(60-30)*(1.2-0.3) = 0.3 + 0.667*0.9 = 0.9` ✓. p95/p99
  targets (95,99) both exceed le=5's count (90) and land in `+Inf` →
  correctly report the highest finite le (5) per the documented
  `histogram_quantile` convention ✓.
- "Cumulative before interpolating" question: confirmed by reading
  `domain.MetricLatencyBucket`'s doc (`metric.go:39-44`) — `Count` is
  already "cumulative across Le within one BucketStart" (a per-interval
  delta of Traefik's own cumulative-per-le counter, so the cumulative-
  across-le property survives the diff). The helper correctly uses
  `b.Count` directly as the cumulative value without re-summing — that
  matches the stored data's actual semantics; re-summing would have been
  the bug here, not the omission.
- Edge cases (zero total, single `+Inf`-only bucket, all-zero counts) all
  degrade to 0/0/0 or a defensible `prevLe` fallback, no panic/NaN/divide-
  by-zero. Verified by `TestPercentilesFromLatencyBucketsEmpty` and by
  hand-tracing a single-`+Inf`-bucket case.

**2. Rollup service (`metrics_rollup_service.go`) — real correctness gap,
this is the blocking issue.**

`Rollup(now)` (lines 77-95) does aggregate-then-prune in the right order
for each resolution pair (minute→hour aggregate, *then* minute prune;
hour→day aggregate, *then* hour prune) — that part of the ordering claim
is correct and verified by `TestMetricsRollupMinuteToHourAndPrune`.

But the "aggregation lookback = retention + one bucket window, so a
bucket always gets aggregated before it can be pruned in the same call"
claim (rollup_service.go:97-112, and Implementation Notes' "load-bearing
correctness fix" paragraph) only holds for buckets within
`[now-lookback, now]` (49h for minute→hour, 31d for hour→day). Any
minute/hour row **older** than that lookback window that has never been
aggregated before — e.g. a fresh restart after >1h of downtime (minute) or
>1d of downtime (hour), with un-rolled-up rows already older than
lookback by the time the first post-restart `Rollup` call fires — gets
pruned by `PruneMetrics` in that same call *without ever having an hour/day
aggregate created for it*. `rollupResolution`'s loop (line 116) starts at
`oldest = now.Add(-lookback).Truncate(window)` and never reaches further
back, while `PruneMetrics` (line 83/90) deletes purely by
`bucket_start < cutoff` with no check that the row was actually rolled up.
This is silent, unrecoverable data loss — once pruned, a later tick can't
fix it because the source rows are gone.

This directly contradicts:
- The AC's own wording: "prunes superseded rows" — a row that was never
  aggregated isn't superseded by anything, it's just discarded.
- The Implementation Notes' explicit claim that the margin makes `Rollup`
  "safe as a single one-shot call... a restart after a long process gap" —
  it's only safe for a gap up to one extra window (1h / 1d), not an
  arbitrarily long gap as stated.

The one-extra-window margin *does* correctly fix what it can actually fix:
truncation/rounding edge cases at the retention boundary under continuous
ticking (`DefaultMetricsRollupInterval=5m` ticks many times over a
bucket's life well before it nears 48h/30d old, so this is the realistic
operating mode and it's fine there). The gap is specifically the "long
process gap" scenario the docstring calls out by name and claims to solve.

Neither test covers this: `TestMetricsRollupMinuteToHourAndPrune` uses
`now = hourStart + MetricsMinuteRetention + 1h`, i.e. exactly at the edge
the margin is designed for, not a gap deeper than the lookback.

Suggested fix (small, stays KISS): instead of a fixed
`now.Add(-lookback)`, derive `oldest` from the actual oldest surviving
src-resolution row (`MIN(bucket_start)` per resolution, once, before the
aggregation loop) when that's older than the fixed lookback — guarantees
every row still present gets a chance to be aggregated before its own
prune step, regardless of how long the gap was, without changing
`AggregateMetrics`'s per-bucket idempotent upsert design. Alternatively,
if the team decides the current bounded-margin behavior is an acceptable
tradeoff, at minimum correct the docstring/Implementation-Notes claim so
it doesn't overstate the guarantee (say "gaps up to one extra window",
not "a long process gap").

**3. Query service + handler — no issues.**
- `GetPanels` shapes all four panels correctly, one point per sample row,
  latency grouped by `bucket_start.Unix()` and passed to the percentile
  helper per point (point-aligned with the other three panels, as
  documented). Cross-checked `TestMetricsQueryServiceGetPanelsShapesData`'s
  math by hand (count=10, rate=10/60, error_rate=2/10) — matches.
- Empty series returns `[]` (non-nil) for every panel, never `null`/error
  — `TestMetricsQueryServiceGetPanelsEmptySeries` covers this directly and
  passes.
- `AutoResolution` thresholds (<48h→minute, <30d→hour, else day) match the
  spec and are tested at the exact boundaries (48h→minute, 49h→hour,
  30d→hour, 31d→day) — correct inclusive/exclusive edges.
- Handler: `from`/`to` parsed as Unix seconds via `strconv.ParseInt`
  (parameterized downstream, no injection surface), `to.After(from)`
  validated in both the handler and `GetPanels` (redundant but harmless),
  malformed `resolution` string surfaces as a 400 via `bucketWindow`'s
  error rather than silently defaulting. Both routes correctly registered
  inside the authenticated route group in `router.go` (`/projects/{id}/metrics`
  line 65, `/system/metrics` line 81), consistent with the existing route
  style.
- Minor/non-blocking: `bucketWindow` in `metrics_query_service.go` (lines
  39-50) is a small duplicate of `resolutionWindow` in
  `metrics_repo.go` (lines 180-191) — three-way switch on
  `domain.MetricResolution`, byte-for-byte the same logic. The doc comment
  acknowledges this (unexported in the sqlite package). Not worth blocking
  over for a 3-line switch, but exporting `resolutionWindow` (or moving the
  mapping to the `domain` package, where `MetricResolution` itself lives)
  would remove the duplication and the risk of the two drifting if a new
  resolution is ever added.

**4. Wiring — correct.** `main.go` constructs `MetricsRollupService` (fire-
and-forget goroutine, same pattern as the scraper/idle-sweep) and
`MetricsQueryService`, wires `MetricsHandler` into `router.New`; both new
routes are in the authenticated group.

**5. Tests — meaningful, not tautological**, with the one coverage gap
noted in #2 (deep-backlog-past-lookback aggregate-before-prune case isn't
exercised).

**Action needed before this can pass:** address the rollup-service
aggregate-before-prune gap for gaps longer than the lookback margin (fix
the code to be genuinely gap-length-independent, or explicitly scope down
the claim in code comments/Implementation Notes and confirm the team
accepts that bounded guarantee) — everything else in this task is in good
shape.

### 2026-07-11 — reviewer (rework delta)

**Verdict: PASS**

Scope: no committed history exists for these files to `git diff` against (the
whole FEAT-032 feature — `metrics_rollup_service.go`, `metrics_repo.go`,
`metrics_query_service.go`, `metrics_handler.go`, tests — is still untracked
in this working tree, same as at first review), so this pass compares the
current file contents directly against the rework's Implementation Notes
and the prior review's blocking finding. Confirmed via `git status` that no
other files under `backend/internal/service|repository/sqlite` changed
beyond what the rework describes; percentile helper
(`metrics_percentile.go`), handler (`metrics_handler.go`), and their tests
are byte-for-byte what the first review already approved (untouched by this
delta).

**1. The fix — verified correct for any gap length.**
`Rollup` (metrics_rollup_service.go:79-99) computes `minuteCutoff`/
`hourCutoff` once as `now - retention` and passes the *same* value to both
`rollupResolution` (the aggregate step) and `PruneMetrics` (the prune step)
— aggregate-cutoff and prune-cutoff are now provably identical, not two
independently-derived values that could drift. `rollupResolution` (line
127) no longer takes `now`/`lookback` at all — it derives the start of its
range from `DB.OldestBucketStart(src)` (an actual `MIN(bucket_start)`
query), not a fixed window back from `now`. Since `oldest` is by
construction the minimum surviving `bucket_start` at that resolution, every
row with `bucket_start < cutoff` (i.e. every row `PruneMetrics` is about to
delete) is provably inside `[oldest, cutoff)` and gets a chance to be
aggregated via `DistinctDstBucketStarts` + `AggregateMetrics` before the
prune call runs later in the same `Rollup()` invocation — this holds
regardless of how deep the gap is, because there's no fixed lookback bound
left in the code path at all. Both minute→hour and hour→day go through the
same `rollupResolution` function, so the ordering (aggregate before prune,
same cutoff) is structurally guaranteed for both pairs, not just proven for
one and assumed for the other.

Boundary case checked: a row with `bucket_start == cutoff` is excluded from
aggregation (`DistinctDstBucketStarts`'s `to` bound is exclusive) and also
survives pruning (`PruneMetrics`'s condition is strict `<`) — the two
cutoffs are consistent at the boundary too, no off-by-one gap.

Empirically verified the deep-gap fix is real, not just plausible from
reading: reverted `metrics_rollup_service.go` to the old fixed-lookback
design in a scratch copy and reran the rollup test package —
`TestMetricsRollupDeepGapDoesNotLoseData` **fails** against the old code
(data silently pruned with no hour aggregate created, exactly the
originally-reported bug) and the other two rollup tests still pass against
old code (they were already within the old bounded margin). Restored the
current file (verified byte-identical via diff afterward) and reran the
full suite — passes. This is a genuine regression test, not tautological.

Minor, non-blocking observation: `AggregateMetrics` sums *all* src rows
within a dst bucket's full window (e.g. the whole hour), not just the
portion `< cutoff`. For a dst bucket that straddles the cutoff boundary
(the one hour where `cutoff` falls mid-bucket), this means a small amount
of not-yet-expired src data (still `>= cutoff`, still correctly left
un-pruned) gets pulled into an early aggregate write for that boundary
bucket. It's harmless (no data loss, no double-count — `AggregateMetrics`
recomputes a fresh `SUM` each call, and the boundary bucket gets
re-aggregated correctly on the next tick once fully past cutoff) and is
inherent to window-based aggregation, not something this rework introduced
or worsened. Not worth blocking on.

**2. New repo methods (`metrics_repo.go`) — correct, additive-only.**
- `OldestBucketStart` (line 187): `MIN(bucket_start)` via `UNION ALL` over
  both `metric_samples` and `metric_latency_buckets` for one resolution,
  `sql.NullInt64` scan correctly yields `ok=false` on no rows. Matches spec.
- `DistinctDstBucketStarts` (line 214): `(bucket_start / windowSeconds) *
  windowSeconds` is correct floor-division truncation to the dst bucket
  boundary — SQLite integer division on positive integers floors, and
  every `bucket_start` here is a positive Unix timestamp aligned to
  minute/hour/day boundaries (matches Go's `time.Truncate` semantics for
  these resolutions). `[from, to)` bounds (`>= fromUnix AND < toUnix`)
  match `ListMetricSamples`/`PruneMetrics`'s existing half-open convention
  elsewhere in this file. `UNION` (not `UNION ALL`) plus `DISTINCT` in the
  outer query correctly collapses duplicate dst buckets from either source
  table. Confirmed a gap with zero source rows produces zero output rows
  (query only returns buckets that actually have a source row — no
  per-window iteration over empty spans).
- `ResolutionWindow` (line 247, exported from the former unexported
  `resolutionWindow`) — confirmed FEAT-030's other repo methods
  (`InsertMetricSamples`, `InsertMetricLatencyBuckets`, `ListMetricSamples`,
  `ListMetricLatencyBuckets`, `PruneMetrics`) are byte-for-byte unchanged;
  this file's diff is purely additive (two new methods) plus the
  rename/export of the one existing helper. Confirmed via grep that no
  local `bucketWindow`/`resolutionWindow` duplicate remains anywhere in the
  tree — `metrics_query_service.go:109` now calls `sqlite.ResolutionWindow`
  directly. Dedup complete, no behavior change (same three-case switch,
  same values).

**3. The dedup — confirmed no behavior change**, per above.

**4. Deep-gap test + updated idempotency test — both correct and
meaningful**, per the empirical old-vs-new run in point 1.
`TestMetricsRollupIsIdempotent`'s updated semantics are correct under the
new "aggregate up to cutoff, not now" design: first call aggregates +
prunes the past-retention minute row, second call finds
`OldestBucketStart` returns `ok=false` (nothing left to aggregate) and is a
verified no-op that leaves the hour sum at 10 (not 20, not 0) — this still
meaningfully tests that re-running doesn't double-count or zero out an
already-rolled-up aggregate, just via a different code path (no surviving
src rows) than the old test exercised (aggregation window recomputing the
same sum). Not weakened.

**5. No regression confirmed.** `metrics_percentile.go`,
`metrics_percentile_test.go`, `metrics_handler.go`,
`metrics_query_service_test.go` unchanged from the first (already-approved)
review pass — re-checked their content against what the first review
quoted/described, matches. `GetPanels` (metrics_query_service.go:102) still
lists samples/latency buckets by resolution+range with no dependency on the
rollup change; recent within-retention buckets are left at minute
resolution (never aggregated early, never pruned) per point 1's cutoff
analysis, so they remain queryable at minute resolution exactly as before.

**6. Build/vet/test.** `go build ./...`, `go vet ./...` clean. `go test
./...` (full suite, not just metrics) passes. Targeted re-run of
`TestMetricsRollupMinuteToHourAndPrune`, `TestMetricsRollupIsIdempotent`,
`TestMetricsRollupDeepGapDoesNotLoseData`, the sqlite-layer metrics tests,
and the query/percentile/handler tests all pass individually.
`gofmt -l` clean on every touched/new file.

**Aggregate-before-prune guarantee: confirmed to now hold for any gap
length**, not just gaps up to one extra window as before — this was the
sole blocking issue from the first pass and it's resolved. No new issues
introduced by the rework.


## Test Notes
<filled in by tester>
