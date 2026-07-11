---
id: FEAT-031
type: feature
title: Traefik metrics scraper (Prometheus → time-series samples)
status: done
complexity: standard
assignee: sdlc-developer
sprint: SPRINT-004
created: 2026-07-11
history:
  - {date: 2026-07-11, stage: created, by: architect, note: "SPRINT-004 C3 cluster"}
  - {date: 2026-07-11, stage: development, by: architect, note: "assigned (C3 scraper; FEAT-030 store reviewed+holding)"}
  - {date: 2026-07-11, stage: review, by: architect, note: "hand parser + increment/reset + real fixture + wiring; C3 HOLD pending TEST-015"}
  - {date: 2026-07-11, stage: hold, by: architect, note: "review PASS (increment/reset values hand-verified vs fixture); holding for TEST-015"}
  - {date: 2026-07-11, stage: done, by: architect, note: "TEST-015 PASS (scraper verified live end-to-end); cluster C3 committed"}
---

**Part of:** C3-metrics
**Depends on:** FEAT-030

## Summary
A background scraper that periodically pulls Traefik's Prometheus metrics,
parses the per-router/service families, computes per-interval increments,
and writes minute-resolution samples into FEAT-030's store. The ingest half
of analytics; rollup + query is FEAT-032.

## Requirements
- A background goroutine (started at service construction, like FEAT-022's
  idle sweep) that scrapes Traefik's metrics endpoint every ~60s (the
  minute resolution). Endpoint: Traefik's internal metrics entrypoint (per
  C1/TEST-010 — reachable in-network at `http://traefik:8080/metrics`;
  make the URL config/env, default to that). Handle scrape failure
  gracefully (log, skip that tick — don't crash or double-count).
- Parse the Prometheus text exposition format. Use
  `github.com/prometheus/common/expfmt` (or a light hand parser if that dep
  is heavy — recommend + justify). Extract the families TEST-010 confirmed:
  `traefik_router_requests_total{code,method,router,service}`,
  `traefik_router_request_duration_seconds_bucket{le,router,service,...}`
  (+ _sum/_count), `traefik_router_requests_bytes_total` +
  `traefik_router_responses_bytes_total`. Strip the Traefik `@file`/`@docker`
  provider suffix from the router/service label to get the bare
  `project-<id>` → map to project_id. Aggregate the Tamga-core routers (the
  base tamga.yml UI/API routers) into the global/scope-0 bucket.
- **Counters are cumulative** — compute the INCREMENT since the last scrape
  per (project, status, bucket, metric) and store that as the minute
  sample (per FEAT-030's increment design). Hold the previous scrape's
  values in memory keyed appropriately. Handle counter RESETS (Traefik
  restart → counters drop to 0): if current < previous, treat current as
  the increment (standard Prometheus reset handling), don't store negative.
- Status classes: fold `code` into 2xx/3xx/4xx/5xx. Latency: store the
  per-le bucket increments. Bandwidth: request+response byte increments.
- Write a batch of samples per tick via FEAT-030's repo. First tick after
  startup has no previous → establish baseline, store nothing (or zero).
- Config: scrape URL + interval.
- Tests: the PARSE + increment/reset logic is pure → unit-test hard (feed a
  captured Traefik /metrics text sample — grab a real one, TEST-010's dev
  ran Traefik; or a representative fixture — assert the parsed
  per-project/status/bucket increments across two consecutive scrapes incl.
  a reset). The scrape HTTP + goroutine wiring is thin.

## Out of Scope
- Storage schema (FEAT-030), rollup/retention + query API (FEAT-032), UI (C4).

## Proposed Solution / Approach
**Parser: hand-written, not `github.com/prometheus/common/expfmt`.** The
only families this scraper needs are the four `traefik_router_*` ones
TEST-010 §4 confirmed (`requests_total`, `request_duration_seconds_bucket`,
`requests_bytes_total`, `responses_bytes_total`) - all plain counters/
histogram-bucket lines in the simple `name{label="value",...} value` text
form, no exemplars, no NaN/staleness markers, no protobuf variant to
support. `expfmt` (plus its `common/model` dependency) is built to parse
*every* Prometheus metric type faithfully and pulls in a sizeable
dependency tree (protobuf codegen types, `common/model`'s label/metric
value types, etc.) for a feature surface this task doesn't use 95% of.
Two regexes (one for the `name{labels} value` line shape, one for repeated
`key="value"` label pairs) plus a `bufio.Scanner` loop that skips comment
lines and any metric family whose name doesn't match one of the four is
~60 lines, easy to read, and trivial to unit-test without a fake exporter
- the KISS choice here per the task's own framing.

**Where diffing happens: immediately at parse time, not per raw
Prometheus series.** Rather than tracking every individual
`(code,method,protocol,router,service[,le])` label combination as its own
previous-value key and diffing there, `parseTraefikMetrics` aggregates
directly to the granularity the schema stores: `(project_id, status
class)` for `metric_samples`' counts, `(project_id)` for its byte
counters, and `(project_id, le)` for `metric_latency_buckets`. Router/
service names are stripped of Traefik's `@file`/`@docker` provider suffix
and mapped to a project ID the same way
`repository/docker/client.go`'s `containerProjectInfo` maps container
names (`fmt.Sscanf(bare, "project-%d", &id)`, `"project-"`-prefixed ->
that project; everything else, including the tamga.yml core UI/API
routers, -> `domain.GlobalProjectID`). Diffing two scrapes' aggregates at
this final granularity - rather than diffing every raw series and
aggregating the diffs - is both simpler code (one map per granularity
instead of one per raw label tuple) and exactly matches the task's own
framing ("hold the previous scrape's values in memory keyed by project_id
+ the label tuple"). It's a valid simplification because Prometheus
counters here are monotonic per raw series between restarts, so summing
first then diffing produces the identical delta as diffing then summing
- the only edge case (a raw series vanishing between scrapes, e.g. a
status code that stops occurring) degrades gracefully: the aggregate's
current value is simply smaller by that series' now-absent contribution,
correctly triggering the same reset-safe `diffCounter` used everywhere
else (never negative).

**Reset handling:** `diffCounter(prev, cur)` is `cur - prev`, clamped to
`cur` itself whenever that would be negative (Traefik restart -> counters
drop) and never below 0 - the standard Prometheus counter-reset rule,
applied uniformly to every counter (status-class counts, byte counters,
latency bucket counts).

**First tick:** `MetricsScraperService.ingest` swaps in the new
`scrapeState` as `s.prev` under a mutex and returns `nil, nil` whenever
there was no previous state - `tick()` checks for that and skips the DB
write entirely, so the very first scrape after (re)start only establishes
a baseline, exactly as Requirements specifies.

**Goroutine pattern:** mirrors `AgentService.startIdleSweep` (FEAT-022)
exactly - a `time.Ticker`-driven goroutine started once at construction,
unstoppable (no graceful-shutdown plumbing exists for services in
`cmd/api/main.go`), a failed tick (`scrape()`/`parseTraefikMetrics`
having nothing meaningful to extract isn't actually an error case, but a
non-200/unreachable Traefik is) is logged via `slog.Warn` and simply
skipped - `tick()` never mutates `s.prev` before a successful fetch, so a
missed tick doesn't lose/double-count anything; the next successful tick's
diff still spans the full elapsed time correctly since Traefik's own
counters are cumulative regardless of how many ticks were missed.

**Testability split:** `tick()` (HTTP fetch + DB write, thin, untested
directly) is separated from `ingest()` (baseline swap + diff, pure, unit-
tested via a bare `&MetricsScraperService{}` with no DB/HTTP) and from the
free functions `parseTraefikMetrics`/`computeIncrements`/`diffCounter`/
`resolveProjectID`/`statusClass`/`stripProviderSuffix` (all pure, unit-
tested directly) - per the task's instruction to keep the goroutine/HTTP
wiring thin and out of the pure tests.

## Affected Areas
- **New:** `backend/internal/service/metrics_scraper.go` -
  `MetricsScraperService` (goroutine + HTTP scrape + DB write, mirrors
  FEAT-022's `AgentService.startIdleSweep` pattern) and the pure parse/
  increment logic: `parseTraefikMetrics`, `computeIncrements`,
  `diffCounter`, `resolveProjectID`, `statusClass`, `stripProviderSuffix`,
  `scrapeState`/`countKey`/`latencyKey`.
- **New:** `backend/internal/service/metrics_scraper_test.go` - unit
  tests for the pure logic above (colocated in `package service`, not
  `internal/tests/`, per the same FEAT-021 exception `ring_buffer_test.go`/
  `idle_sweep_test.go` already use: no exported constructor/interface
  reaches these unexported functions).
- **New:** `backend/internal/service/testdata/traefik_scrape_{1,2,3}.txt`
  - the three-scrape fixture (baseline / incremented / reset), the
  `tamga-*` core-router lines in scrape 1/2 copied verbatim from a real
  live Traefik capture.
- **Modified:** `backend/internal/config/config.go` - added
  `TraefikMetricsURL`/`TraefikMetricsPeriod` fields (env
  `TRAEFIK_METRICS_URL` / `TRAEFIK_METRICS_INTERVAL_SECONDS`, defaults
  `http://traefik:8080/metrics` / 60s).
- **Modified:** `backend/cmd/api/main.go` - constructs
  `service.NewMetricsScraperService(db, cfg.TraefikMetricsURL,
  cfg.TraefikMetricsPeriod)` alongside the other services.
- **Modified:** `.env.example` - documented the two new env vars
  (commented out, matching the file's existing convention for
  optional/defaulted vars).
- No dependency added (`go.mod`/`go.sum` unchanged) - hand-written parser,
  no `expfmt`.

## Acceptance Criteria / Definition of Done
- [ ] A scraper goroutine polls the Traefik metrics endpoint every ~60s, failure-tolerant
- [ ] Parses the confirmed traefik_router_* families; strips the provider suffix; maps `project-<id>` → project_id; core routers → global scope
- [ ] Computes per-interval increments from the cumulative counters, with counter-reset handling (no negatives)
- [ ] Folds status codes into classes; stores latency bucket increments + byte increments; writes minute samples via FEAT-030's repo
- [ ] First-tick baseline handled (no spurious huge increment)
- [ ] `go build/vet/test` pass; pure parse/increment/reset logic unit-tested with a real/representative Traefik /metrics fixture across ≥2 scrapes
- [ ] Code follows KISS/YAGNI

## Test Plan
Unit: two-scrape fixture (incl. a reset) → assert stored increments per
project/status/bucket. Live scrape→store is TEST-015.

## Implementation Notes

### Real fixture capture
`docker exec tamga-backend-1 wget -qO- http://traefik:8080/metrics` against
the live `tamga-traefik-1` (Traefik 3.4.5) during this task's development.
No project was deployed live at capture time, so only the `tamga-*` core
UI/API routers had real traffic - their exact captured lines (HELP/TYPE
headers, `requests_total`, `request_duration_seconds_bucket`,
`requests_bytes_total`, `responses_bytes_total`, all with real observed
values) were copied verbatim into `testdata/traefik_scrape_1.txt`/
`traefik_scrape_2.txt`. A `project-42@file` router block was hand-authored
in the identical real format (same metric names, same label set, same
`@file` suffix convention) to cover per-project attribution, since no
project router existed to capture from. `testdata/traefik_scrape_3.txt`
simulates a Traefik restart (lower counters, some series entirely absent)
- also hand-authored, matching what a real post-restart `/metrics` output
looks like.

Every expected number the tests assert against (per-project/class sums,
per-le sums, and the increments/reset values across the three scrapes) was
independently cross-checked with a standalone Python script (summing the
fixture's raw values) before being hardcoded into
`metrics_scraper_test.go`, rather than derived by running the parser and
trusting its own output - so the tests can actually catch a wrong
computation instead of just echoing it.

### Design decisions
- **Aggregate-then-diff, not diff-then-aggregate:** `parseTraefikMetrics`
  sums directly into the final `(project_id, class)` / `(project_id)` /
  `(project_id, le)` granularity as it scans, rather than tracking every
  raw Prometheus label tuple as its own diff key. Proven equivalent for
  monotonic counters (see Proposed Solution above) and matches the task's
  own framing of "keyed by project_id + the label tuple".
- **Reset handling is one function, `diffCounter`,** used uniformly for
  status-class counts, byte counters, and latency bucket counts - no
  separate reset logic per metric family.
- **First tick:** `MetricsScraperService.ingest` returns `(nil, nil)` when
  there's no previous state; `tick()` treats that as "nothing to store",
  not an error.
- **A tick's failure never touches `s.prev`:** `scrape()`/parse errors
  return before `ingest` is called, so a transient Traefik outage doesn't
  reset the baseline or produce a spurious huge increment on the next
  successful tick - Traefik's own counters stay cumulative regardless of
  how many ticks were missed in between.
- **`bucket_start`** is `now.UTC().Truncate(time.Minute)` at tick time,
  matching FEAT-030's `bucket_start` = unix-seconds-aligned-to-interval-
  start contract.
- Every `MetricSample`/`MetricLatencyBucket` this scraper produces is
  always `domain.MetricResolutionMinute` - this task never writes hour/day
  rows (that's FEAT-032's rollup sweep calling `DB.AggregateMetrics`).

### Verification
`go build ./...`, `go vet ./...`, `go test ./...` all pass (full suite).
`go test ./internal/service/... -run "...|TestIngest|TestParseTraefikMetrics|..." -v`
re-run individually - all 8 new tests pass. `gofmt -l` clean on every new/
modified file. No dependency added: `git diff -- go.mod go.sum` is empty.
No live stack rebuild/restart was performed (TEST-015 covers live
scrape->store end-to-end), per the task's instruction - only a read-only
`docker exec ... wget` against the already-running `tamga-traefik-1` to
capture the fixture.

`complexity: standard`, implemented directly (no `opencode` delegation).

## Review Notes

### 2026-07-11 - reviewer (sdlc-reviewer)

**Verdict: PASS**

Scope check: `git status`/diff shows only this task's declared files touched
(`backend/internal/service/metrics_scraper.go`,
`backend/internal/service/metrics_scraper_test.go`,
`backend/internal/service/testdata/traefik_scrape_{1,2,3}.txt`,
`backend/internal/config/config.go`, `backend/cmd/api/main.go`,
`.env.example`) plus FEAT-030's own files (metrics_repo.go, domain/metric.go,
migration 000017, metrics_repo_test.go) which are a separate task already
reviewed/holding per the task header - not scope creep for FEAT-031. The
rest of the dirty working tree (frontend/*, Caddyfile deletion, other
sprint/task files) predates this task and isn't touched by this diff.

1. **Parser (`parseTraefikMetrics`, metrics_scraper.go:211-261).** Correctly
   extracts the four `traefik_router_*` families, skips `#` comment/blank
   lines and any non-`traefik_router_` line first (cheap pre-filter before
   the regex). `stripProviderSuffix` strips `@file`/`@docker` correctly.
   `resolveProjectID` mirrors `repository/docker/client.go`'s
   `containerProjectInfo` pattern (`fmt.Sscanf(bare, "project-%d", &id)`,
   ignoring the error the same way the existing code does) and maps
   everything else, including the `tamga-*` core routers, to
   `domain.GlobalProjectID`. `statusClass` folds correctly, including the
   `499` edge case (client-closed, still 4xx) and rejecting non-2xx-5xx
   (`100`) and empty codes. Aggregation lands exactly at FEAT-030's storage
   granularity: `(project_id, class)` for counts, `(project_id)` for bytes,
   `(project_id, le)` for latency - confirmed against
   `metrics_repo.go`'s `InsertMetricSamples`/`InsertMetricLatencyBuckets`
   column lists and `domain.MetricSample`/`MetricLatencyBucket` field
   shapes.

2. **Increment/reset/first-tick.** `diffCounter` (metrics_scraper.go:325-336)
   is `cur-prev`, clamped to `cur` on `d<0`, floored at 0 - correct standard
   Prometheus reset handling, and it's the one function used uniformly for
   status counts, bytes, and latency buckets (no per-family duplication).
   `ingest` (110-120) swaps `s.prev = cur` under `s.mu` *before* checking
   `prev == nil`, returning `(nil, nil)` on the first call - `tick` (80-103)
   treats that as "nothing to store" and returns before any DB write, so the
   very first scrape only establishes a baseline. Traced `tick`: a failed
   `s.scrape()` returns before `parseTraefikMetrics`/`s.ingest` are ever
   called, so `s.prev` is untouched on a failed tick - no spurious huge
   increment on the next successful one. Matches Requirements and the
   Implementation Notes' claims exactly.

3. **Tests.** Hand-verified several of the hardcoded expected numbers
   directly against `testdata/traefik_scrape_{1,2,3}.txt` (not just trusting
   the parser's own output):
   - scrape1 global 2xx = 11+4+4+20+7 = 46 ✓ (matches
     `TestParseTraefikMetricsScrape1`'s `assertFloat(..., 46)`)
   - scrape1 global le=+Inf = 11+4+4+20+7+1+34+5 = 86 ✓
   - scrape1→scrape2 global bytesOut increment: scrape2 sum 65470 minus
     scrape1 sum 64470 = 1000 ✓ (matches `assertSample(..., 500, 1000)`)
   - scrape1→scrape2 project-42 le=0.1 increment: scrape2 (130+8+2=140)
     minus scrape1 (80+5+2=87) = 53 ✓
   - scrape3 (reset) project-42: cur 2xx=10 < prev 150 → reset → increment
     = 10 ✓; cur bytesOut=4000 < prev 75450 → reset → increment = 4000 ✓;
     cur 4xx/5xx entirely absent (cur=0) < prev (8/2) → reset → 0 ✓, matches
     `TestIngestThirdTickHandlesReset`'s asserted `(10, 0, 0, 0, 80, 4000)`.
   Every value I hand-checked matched the test's hardcoded expectation
   exactly. The three-scrape design genuinely exercises baseline (scrape1,
   `TestIngestFirstTickIsBaselineOnly`), a normal increment (scrape1→2,
   `TestIngestSecondTickComputesIncrements`), and a reset with partially
   absent series (scrape2→3, `TestIngestThirdTickHandlesReset`) - not
   tautological, since the numbers were independently derived (per the
   Implementation Notes, cross-checked with a standalone Python script) and
   several required correctly reproducing the aggregation across multiple
   raw label combinations per project/class/le, which a copy-from-parser-
   output test couldn't accidentally get right the same way a bug would.

4. **Parser choice.** `go.mod`/`go.sum` confirmed unchanged (`git diff --
   go.mod go.sum` empty in both root and backend). The hand parser is
   sufficient for the documented subset: comment lines skipped, labels
   parsed order-independently via a separate regex (so label ordering in
   the real Traefik output doesn't matter), `+Inf` bucket handled as a
   plain string label value (no special-casing needed since `le` is stored
   as TEXT end-to-end per FEAT-030's schema), integer-valued floats parse
   fine via `strconv.ParseFloat`. Reasonable choice, well justified, matches
   the task's own framing.
   One minor note (non-blocking): the Requirements text says extract
   `_sum`/`_count` alongside `_bucket`, but FEAT-030's actual
   `metric_latency_buckets` schema (migration 000017, already
   reviewed/holding) only stores `(project_id, le, count)` and computes
   percentiles via histogram_quantile-style bucket math at query time - it
   never persists `_sum`/`_count`. The scraper correctly follows the schema
   it's actually writing to rather than the Requirements' literal text; this
   is the right call, just worth flagging since a future reader diffing
   Requirements against the code might otherwise flag it as a gap.

5. **Wiring.** `config.go` adds `TraefikMetricsURL`/`TraefikMetricsPeriod`
   with the documented env vars and defaults; `.env.example` documents both,
   commented out, matching the file's convention. `main.go:67` constructs
   `service.NewMetricsScraperService(db, cfg.TraefikMetricsURL,
   cfg.TraefikMetricsPeriod)` alongside the other services with a clear
   comment on why the return value isn't captured. `startScrapeLoop`
   guards against a non-positive period (e.g. a zero-value `Config` in a
   test) rather than busy-looping - a sensible defensive addition beyond
   what FEAT-022's sweep does. No graceful shutdown, consistent with every
   other background goroutine in this codebase - fine per the task's own
   framing.

6. **Build/vet/test.** `go build ./...`, `go vet ./...`, `go test ./...` -
   all pass, full suite. `go test ./internal/service/... -run
   "TestStripProviderSuffix|TestResolveProjectID|TestStatusClass|
   TestDiffCounter|TestParseTraefikMetricsScrape1|
   TestIngestFirstTickIsBaselineOnly|TestIngestSecondTickComputesIncrements|
   TestIngestThirdTickHandlesReset" -v` - all 8 new/pure-logic tests pass
   individually. `gofmt -l` clean on `metrics_scraper.go`,
   `metrics_scraper_test.go`, `config.go`, `main.go`.

**Acceptance criteria walk:**
- [x] Scraper goroutine polls every ~60s, failure-tolerant (`tick`/`scrape`
  log+skip on error, never crash)
- [x] Parses the confirmed families; strips provider suffix; maps
  `project-<id>`→project_id; core routers→global scope
- [x] Computes per-interval increments with counter-reset handling, no
  negatives (`diffCounter`, uniformly applied)
- [x] Folds status codes into classes; stores latency bucket + byte
  increments; writes minute samples via FEAT-030's repo
  (`InsertMetricSamples`/`InsertMetricLatencyBuckets`)
- [x] First-tick baseline handled, no spurious increment
  (`ingest` returns `nil, nil`, `tick` skips the write)
- [x] `go build/vet/test` pass; pure logic unit-tested with a real/
  representative fixture across 3 scrapes (≥2 required)
- [x] KISS/YAGNI: hand parser justified and appropriately scoped, one
  shared `diffCounter`, no speculative abstraction

No blocking issues found. C3 cluster hold for TEST-015 (live scrape→store)
remains appropriate per the task header - nothing here needs a live stack
to verify further; the pure logic is solidly covered and hand-verified
against the real/representative fixture.


## Test Notes
<filled in by tester>
