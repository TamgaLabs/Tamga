package sqlite_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/TamgaLabs/Tamga/backend/internal/domain"
	"github.com/TamgaLabs/Tamga/backend/internal/repository/sqlite"
)

// TestMetricsMigrationAppliesOnFreshDB covers FEAT-030's migration 000017:
// a brand new DB must migrate cleanly and end up with metric_samples/
// metric_latency_buckets usable (query returns no error, empty result).
func TestMetricsMigrationAppliesOnFreshDB(t *testing.T) {
	db := openTestDB(t)

	from := time.Unix(0, 0)
	to := time.Now().Add(24 * time.Hour)

	samples, err := db.ListMetricSamples(domain.GlobalProjectID, domain.MetricResolutionMinute, from, to)
	if err != nil {
		t.Fatalf("query metric_samples on fresh db: %v", err)
	}
	if len(samples) != 0 {
		t.Fatalf("expected 0 samples on fresh db, got %d", len(samples))
	}

	buckets, err := db.ListMetricLatencyBuckets(domain.GlobalProjectID, domain.MetricResolutionMinute, from, to)
	if err != nil {
		t.Fatalf("query metric_latency_buckets on fresh db: %v", err)
	}
	if len(buckets) != 0 {
		t.Fatalf("expected 0 latency buckets on fresh db, got %d", len(buckets))
	}
}

// TestMetricsMigrationAppliesOnCopiedLiveDB re-runs Migrate() against a
// throwaway copy of the actual on-disk dev database (never the live file
// itself). Confirms migration 000017 applies cleanly on top of whatever
// migrations that DB already has, and re-running Migrate() again is a
// no-op. Skips if the live DB isn't present in this environment.
func TestMetricsMigrationAppliesOnCopiedLiveDB(t *testing.T) {
	liveDBPath := filepath.Join("..", "..", "..", "..", "data", "tamga.db")
	if _, err := os.Stat(liveDBPath); err != nil {
		t.Skipf("live dev db not present at %s, skipping: %v", liveDBPath, err)
	}

	copyPath := filepath.Join(t.TempDir(), "tamga_copy.db")
	if err := copyFile(liveDBPath, copyPath); err != nil {
		t.Fatalf("copy live db: %v", err)
	}

	db, err := sqlite.Open(copyPath)
	if err != nil {
		t.Fatalf("open copied db: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	if err := db.Migrate(); err != nil {
		t.Fatalf("migrate copied live db: %v", err)
	}

	if _, err := db.ListMetricSamples(domain.GlobalProjectID, domain.MetricResolutionMinute, time.Unix(0, 0), time.Now()); err != nil {
		t.Fatalf("query metric_samples on copied live db after migrate: %v", err)
	}

	// Re-running Migrate() again must be a no-op (idempotent), same as
	// every other migration in this codebase.
	if err := db.Migrate(); err != nil {
		t.Fatalf("re-run migrate on already-migrated copied db: %v", err)
	}
}

// TestMetricSamplesInsertAndRangeQuery covers the batch insert + range
// query round-trip for metric_samples: per-project and global scopes,
// resolution isolation, and [from, to) boundary semantics.
func TestMetricSamplesInsertAndRangeQuery(t *testing.T) {
	db := openTestDB(t)

	base := time.Date(2026, 7, 11, 10, 0, 0, 0, time.UTC)

	samples := []*domain.MetricSample{
		{ProjectID: 1, Resolution: domain.MetricResolutionMinute, BucketStart: base, Count2xx: 10, Count3xx: 1, Count4xx: 2, Count5xx: 0, BytesIn: 1000, BytesOut: 5000},
		{ProjectID: 1, Resolution: domain.MetricResolutionMinute, BucketStart: base.Add(time.Minute), Count2xx: 20, Count3xx: 0, Count4xx: 1, Count5xx: 1, BytesIn: 2000, BytesOut: 6000},
		{ProjectID: 1, Resolution: domain.MetricResolutionMinute, BucketStart: base.Add(2 * time.Minute), Count2xx: 5, Count3xx: 0, Count4xx: 0, Count5xx: 0, BytesIn: 500, BytesOut: 1000},
		// Different project - must not leak into project 1's query.
		{ProjectID: 2, Resolution: domain.MetricResolutionMinute, BucketStart: base, Count2xx: 99, Count3xx: 0, Count4xx: 0, Count5xx: 0, BytesIn: 0, BytesOut: 0},
		// Global scope (project_id 0).
		{ProjectID: domain.GlobalProjectID, Resolution: domain.MetricResolutionMinute, BucketStart: base, Count2xx: 7, Count3xx: 0, Count4xx: 0, Count5xx: 0, BytesIn: 100, BytesOut: 200},
		// Different resolution at the same bucket_start - must not leak into a minute-resolution query.
		{ProjectID: 1, Resolution: domain.MetricResolutionHour, BucketStart: base, Count2xx: 1000, Count3xx: 0, Count4xx: 0, Count5xx: 0, BytesIn: 0, BytesOut: 0},
	}
	if err := db.InsertMetricSamples(samples); err != nil {
		t.Fatalf("insert metric samples: %v", err)
	}

	// Range covering only the first two minute buckets for project 1 (to
	// exclusive at base+2min excludes the third).
	got, err := db.ListMetricSamples(1, domain.MetricResolutionMinute, base, base.Add(2*time.Minute))
	if err != nil {
		t.Fatalf("range query: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 samples in range, got %d: %+v", len(got), got)
	}
	if got[0].BucketStart.Unix() != base.Unix() || got[1].BucketStart.Unix() != base.Add(time.Minute).Unix() {
		t.Fatalf("unexpected bucket_start ordering: %+v", got)
	}
	if got[0].Count2xx != 10 || got[0].Count3xx != 1 || got[0].Count4xx != 2 || got[0].Count5xx != 0 {
		t.Errorf("unexpected status counts for first bucket: %+v", got[0])
	}
	if got[0].BytesIn != 1000 || got[0].BytesOut != 5000 {
		t.Errorf("unexpected byte counts for first bucket: %+v", got[0])
	}
	if got[0].ProjectID != 1 {
		t.Errorf("expected ProjectID 1, got %d", got[0].ProjectID)
	}

	// Full range for project 1 at minute resolution returns exactly the 3
	// minute rows, never the hour-resolution row at the same bucket_start.
	full, err := db.ListMetricSamples(1, domain.MetricResolutionMinute, base, base.Add(10*time.Minute))
	if err != nil {
		t.Fatalf("full range query: %v", err)
	}
	if len(full) != 3 {
		t.Fatalf("expected 3 minute-resolution samples for project 1, got %d", len(full))
	}

	// Global scope isolated from project-scoped rows.
	global, err := db.ListMetricSamples(domain.GlobalProjectID, domain.MetricResolutionMinute, base, base.Add(time.Minute))
	if err != nil {
		t.Fatalf("global range query: %v", err)
	}
	if len(global) != 1 || global[0].Count2xx != 7 {
		t.Fatalf("unexpected global scope result: %+v", global)
	}

	// Upsert accumulates: re-inserting the same (project_id, resolution,
	// bucket_start) key SUMS into the existing row rather than overwriting or
	// duplicating. This is how several sub-minute scrapes' disjoint
	// per-interval increments fold into one minute bucket (see TEST-015).
	firstCount2xx := afterInsertBaseline(t, db, base)
	if err := db.InsertMetricSamples([]*domain.MetricSample{
		{ProjectID: 1, Resolution: domain.MetricResolutionMinute, BucketStart: base, Count2xx: 5, Count3xx: 1, Count4xx: 2, Count5xx: 0, BytesIn: 1000, BytesOut: 5000},
	}); err != nil {
		t.Fatalf("re-insert (upsert) metric sample: %v", err)
	}
	afterUpsert, err := db.ListMetricSamples(1, domain.MetricResolutionMinute, base, base.Add(10*time.Minute))
	if err != nil {
		t.Fatalf("range query after upsert: %v", err)
	}
	if len(afterUpsert) != 3 {
		t.Fatalf("expected upsert to accumulate not duplicate, got %d rows", len(afterUpsert))
	}
	if afterUpsert[0].Count2xx != firstCount2xx+5 {
		t.Fatalf("expected accumulated Count2xx %d, got %d", firstCount2xx+5, afterUpsert[0].Count2xx)
	}
}

// afterInsertBaseline returns the current Count2xx of project 1's row at the
// given bucket, so the accumulation assertion is independent of the exact
// seed value the surrounding test inserted.
func afterInsertBaseline(t *testing.T, db *sqlite.DB, base time.Time) int64 {
	t.Helper()
	rows, err := db.ListMetricSamples(1, domain.MetricResolutionMinute, base, base.Add(time.Minute))
	if err != nil {
		t.Fatalf("baseline query: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected exactly 1 baseline row at bucket, got %d", len(rows))
	}
	return rows[0].Count2xx
}

// TestMetricSamplesSubMinuteScrapesAccumulate is the direct TEST-015
// regression: when the scrape interval is finer than the minute bucket,
// each scrape emits the per-interval increment (a disjoint slice of the
// minute's traffic) against the SAME bucket_start. The stored bucket must be
// the SUM of those increments, not just the last one - overwriting kept only
// the final (often 0, when traffic went idle) delta and lost everything
// before it.
func TestMetricSamplesSubMinuteScrapesAccumulate(t *testing.T) {
	db := openTestDB(t)
	base := time.Date(2026, 7, 11, 10, 0, 0, 0, time.UTC)

	// Six 10s scrapes within one minute: a burst of 200s/404s early, then
	// idle (0-delta) scrapes - exactly the shape that produced all-zero
	// project rows in the live test.
	scrapeDeltas := []struct{ c2xx, c4xx, bin, bout int64 }{
		{40, 15, 400, 2000}, // burst
		{25, 10, 250, 1250}, // burst continues
		{0, 0, 0, 0},        // idle
		{0, 0, 0, 0},        // idle
		{0, 0, 0, 0},        // idle
		{0, 0, 0, 0},        // idle
	}
	var wantC2xx, wantC4xx, wantBin, wantBout int64
	for _, d := range scrapeDeltas {
		if err := db.InsertMetricSamples([]*domain.MetricSample{
			{ProjectID: 40, Resolution: domain.MetricResolutionMinute, BucketStart: base, Count2xx: d.c2xx, Count4xx: d.c4xx, BytesIn: d.bin, BytesOut: d.bout},
		}); err != nil {
			t.Fatalf("scrape insert: %v", err)
		}
		if err := db.InsertMetricLatencyBuckets([]*domain.MetricLatencyBucket{
			{ProjectID: 40, Resolution: domain.MetricResolutionMinute, BucketStart: base, Le: "0.1", Count: d.c2xx},
		}); err != nil {
			t.Fatalf("scrape latency insert: %v", err)
		}
		wantC2xx += d.c2xx
		wantC4xx += d.c4xx
		wantBin += d.bin
		wantBout += d.bout
	}

	got, err := db.ListMetricSamples(40, domain.MetricResolutionMinute, base, base.Add(time.Minute))
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected exactly 1 accumulated minute row, got %d", len(got))
	}
	if got[0].Count2xx != wantC2xx || got[0].Count4xx != wantC4xx || got[0].BytesIn != wantBin || got[0].BytesOut != wantBout {
		t.Fatalf("sub-minute scrapes did not accumulate: got 2xx=%d 4xx=%d in=%d out=%d, want 2xx=%d 4xx=%d in=%d out=%d",
			got[0].Count2xx, got[0].Count4xx, got[0].BytesIn, got[0].BytesOut, wantC2xx, wantC4xx, wantBin, wantBout)
	}

	lb, err := db.ListMetricLatencyBuckets(40, domain.MetricResolutionMinute, base, base.Add(time.Minute))
	if err != nil {
		t.Fatalf("latency query: %v", err)
	}
	if len(lb) != 1 || lb[0].Count != wantC2xx {
		t.Fatalf("latency buckets did not accumulate: got %+v, want single le=0.1 count=%d", lb, wantC2xx)
	}
}

// TestLatencyBucketsInsertAndRangeQuery covers the batch insert + range
// query round-trip for metric_latency_buckets, including multiple `le`
// boundaries per bucket_start.
func TestLatencyBucketsInsertAndRangeQuery(t *testing.T) {
	db := openTestDB(t)

	base := time.Date(2026, 7, 11, 10, 0, 0, 0, time.UTC)

	buckets := []*domain.MetricLatencyBucket{
		{ProjectID: 1, Resolution: domain.MetricResolutionMinute, BucketStart: base, Le: "0.1", Count: 8},
		{ProjectID: 1, Resolution: domain.MetricResolutionMinute, BucketStart: base, Le: "0.3", Count: 10},
		{ProjectID: 1, Resolution: domain.MetricResolutionMinute, BucketStart: base, Le: "1.2", Count: 12},
		{ProjectID: 1, Resolution: domain.MetricResolutionMinute, BucketStart: base, Le: "5", Count: 13},
		{ProjectID: 1, Resolution: domain.MetricResolutionMinute, BucketStart: base, Le: "+Inf", Count: 13},
	}
	if err := db.InsertMetricLatencyBuckets(buckets); err != nil {
		t.Fatalf("insert latency buckets: %v", err)
	}

	got, err := db.ListMetricLatencyBuckets(1, domain.MetricResolutionMinute, base, base.Add(time.Minute))
	if err != nil {
		t.Fatalf("range query: %v", err)
	}
	if len(got) != 5 {
		t.Fatalf("expected 5 le buckets, got %d: %+v", len(got), got)
	}
	// Ordered by le ASC (lexical: "+Inf" sorts before digits in ASCII, so
	// just assert the full set of (le, count) pairs is present rather than
	// depending on a specific ordering across the "+Inf" boundary).
	wantByLe := map[string]int64{"0.1": 8, "0.3": 10, "1.2": 12, "5": 13, "+Inf": 13}
	gotByLe := make(map[string]int64, len(got))
	for _, b := range got {
		gotByLe[b.Le] = b.Count
		if b.ProjectID != 1 {
			t.Errorf("unexpected ProjectID on latency bucket: %+v", b)
		}
		if b.BucketStart.Unix() != base.Unix() {
			t.Errorf("unexpected BucketStart on latency bucket: %+v", b)
		}
	}
	for le, want := range wantByLe {
		if gotByLe[le] != want {
			t.Errorf("le=%q: got count %d, want %d", le, gotByLe[le], want)
		}
	}
}

// TestPruneMetrics covers prune-by-cutoff: rows strictly older than cutoff
// (at the given resolution) are removed from both tables, newer rows and
// other resolutions are untouched.
func TestPruneMetrics(t *testing.T) {
	db := openTestDB(t)

	old := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	recent := time.Date(2026, 7, 11, 0, 0, 0, 0, time.UTC)
	cutoff := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)

	if err := db.InsertMetricSamples([]*domain.MetricSample{
		{ProjectID: 1, Resolution: domain.MetricResolutionDay, BucketStart: old, Count2xx: 1},
		{ProjectID: 1, Resolution: domain.MetricResolutionDay, BucketStart: recent, Count2xx: 2},
		// Same old timestamp but a different resolution - must survive
		// pruning the "day" resolution.
		{ProjectID: 1, Resolution: domain.MetricResolutionHour, BucketStart: old, Count2xx: 3},
	}); err != nil {
		t.Fatalf("insert metric samples: %v", err)
	}
	if err := db.InsertMetricLatencyBuckets([]*domain.MetricLatencyBucket{
		{ProjectID: 1, Resolution: domain.MetricResolutionDay, BucketStart: old, Le: "5", Count: 1},
		{ProjectID: 1, Resolution: domain.MetricResolutionDay, BucketStart: recent, Le: "5", Count: 2},
	}); err != nil {
		t.Fatalf("insert latency buckets: %v", err)
	}

	if err := db.PruneMetrics(domain.MetricResolutionDay, cutoff); err != nil {
		t.Fatalf("prune metrics: %v", err)
	}

	daySamples, err := db.ListMetricSamples(1, domain.MetricResolutionDay, time.Unix(0, 0), time.Now().Add(24*time.Hour))
	if err != nil {
		t.Fatalf("list day samples after prune: %v", err)
	}
	if len(daySamples) != 1 || daySamples[0].BucketStart.Unix() != recent.Unix() {
		t.Fatalf("expected only the recent day sample to survive prune, got %+v", daySamples)
	}

	hourSamples, err := db.ListMetricSamples(1, domain.MetricResolutionHour, time.Unix(0, 0), time.Now().Add(24*time.Hour))
	if err != nil {
		t.Fatalf("list hour samples after prune: %v", err)
	}
	if len(hourSamples) != 1 {
		t.Fatalf("expected the hour-resolution row (same old bucket_start) to survive pruning the day resolution, got %d", len(hourSamples))
	}

	dayBuckets, err := db.ListMetricLatencyBuckets(1, domain.MetricResolutionDay, time.Unix(0, 0), time.Now().Add(24*time.Hour))
	if err != nil {
		t.Fatalf("list day latency buckets after prune: %v", err)
	}
	if len(dayBuckets) != 1 || dayBuckets[0].BucketStart.Unix() != recent.Unix() {
		t.Fatalf("expected only the recent day latency bucket to survive prune, got %+v", dayBuckets)
	}
}

// TestAggregateMetricsMinuteToHour covers the minute->hour rollup
// primitive: minute rows within an hour window sum into one hour row per
// project, and re-running the aggregation (idempotency) produces the same
// result rather than double-counting.
func TestAggregateMetricsMinuteToHour(t *testing.T) {
	db := openTestDB(t)

	hourStart := time.Date(2026, 7, 11, 10, 0, 0, 0, time.UTC)

	if err := db.InsertMetricSamples([]*domain.MetricSample{
		{ProjectID: 1, Resolution: domain.MetricResolutionMinute, BucketStart: hourStart, Count2xx: 10, Count3xx: 1, Count4xx: 0, Count5xx: 0, BytesIn: 100, BytesOut: 200},
		{ProjectID: 1, Resolution: domain.MetricResolutionMinute, BucketStart: hourStart.Add(30 * time.Minute), Count2xx: 20, Count3xx: 0, Count4xx: 2, Count5xx: 1, BytesIn: 300, BytesOut: 400},
		{ProjectID: 1, Resolution: domain.MetricResolutionMinute, BucketStart: hourStart.Add(59 * time.Minute), Count2xx: 5, Count3xx: 0, Count4xx: 0, Count5xx: 0, BytesIn: 50, BytesOut: 60},
		// Falls in the NEXT hour - must not be summed into this window.
		{ProjectID: 1, Resolution: domain.MetricResolutionMinute, BucketStart: hourStart.Add(time.Hour), Count2xx: 1000, Count3xx: 0, Count4xx: 0, Count5xx: 0, BytesIn: 0, BytesOut: 0},
		// Different project - separate rollup row.
		{ProjectID: 2, Resolution: domain.MetricResolutionMinute, BucketStart: hourStart, Count2xx: 7, Count3xx: 0, Count4xx: 0, Count5xx: 0, BytesIn: 0, BytesOut: 0},
	}); err != nil {
		t.Fatalf("insert minute samples: %v", err)
	}
	if err := db.InsertMetricLatencyBuckets([]*domain.MetricLatencyBucket{
		{ProjectID: 1, Resolution: domain.MetricResolutionMinute, BucketStart: hourStart, Le: "0.1", Count: 3},
		{ProjectID: 1, Resolution: domain.MetricResolutionMinute, BucketStart: hourStart.Add(30 * time.Minute), Le: "0.1", Count: 4},
		{ProjectID: 1, Resolution: domain.MetricResolutionMinute, BucketStart: hourStart.Add(time.Hour), Le: "0.1", Count: 999},
	}); err != nil {
		t.Fatalf("insert minute latency buckets: %v", err)
	}

	if err := db.AggregateMetrics(domain.MetricResolutionMinute, domain.MetricResolutionHour, hourStart); err != nil {
		t.Fatalf("aggregate minute->hour: %v", err)
	}

	hourSamples, err := db.ListMetricSamples(1, domain.MetricResolutionHour, hourStart, hourStart.Add(time.Hour))
	if err != nil {
		t.Fatalf("list hour samples: %v", err)
	}
	if len(hourSamples) != 1 {
		t.Fatalf("expected exactly 1 rolled-up hour row for project 1, got %d", len(hourSamples))
	}
	hs := hourSamples[0]
	if hs.Count2xx != 35 || hs.Count3xx != 1 || hs.Count4xx != 2 || hs.Count5xx != 1 {
		t.Errorf("unexpected summed status counts: %+v", hs)
	}
	if hs.BytesIn != 450 || hs.BytesOut != 660 {
		t.Errorf("unexpected summed byte counts: %+v", hs)
	}

	hourSamplesP2, err := db.ListMetricSamples(2, domain.MetricResolutionHour, hourStart, hourStart.Add(time.Hour))
	if err != nil {
		t.Fatalf("list hour samples for project 2: %v", err)
	}
	if len(hourSamplesP2) != 1 || hourSamplesP2[0].Count2xx != 7 {
		t.Fatalf("unexpected project 2 rollup: %+v", hourSamplesP2)
	}

	hourBuckets, err := db.ListMetricLatencyBuckets(1, domain.MetricResolutionHour, hourStart, hourStart.Add(time.Hour))
	if err != nil {
		t.Fatalf("list hour latency buckets: %v", err)
	}
	if len(hourBuckets) != 1 || hourBuckets[0].Count != 7 {
		t.Fatalf("expected summed le=0.1 count of 7, got %+v", hourBuckets)
	}

	// Idempotency: re-running the same aggregation must not double-count.
	if err := db.AggregateMetrics(domain.MetricResolutionMinute, domain.MetricResolutionHour, hourStart); err != nil {
		t.Fatalf("re-run aggregate minute->hour: %v", err)
	}
	hourSamplesAgain, err := db.ListMetricSamples(1, domain.MetricResolutionHour, hourStart, hourStart.Add(time.Hour))
	if err != nil {
		t.Fatalf("list hour samples after re-run: %v", err)
	}
	if len(hourSamplesAgain) != 1 || hourSamplesAgain[0].Count2xx != 35 {
		t.Fatalf("expected idempotent re-run to leave the same summed row, got %+v", hourSamplesAgain)
	}
}
