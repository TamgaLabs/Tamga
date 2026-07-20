package service_test

import (
	"os"
	"testing"
	"time"

	"github.com/TamgaLabs/Tamga/backend/internal/domain"
	"github.com/TamgaLabs/Tamga/backend/internal/repository/sqlite"
	"github.com/TamgaLabs/Tamga/backend/internal/service"
)

func openRollupTestDB(t *testing.T, name string) *sqlite.DB {
	t.Helper()
	dbPath := "/tmp/" + name
	t.Cleanup(func() {
		os.Remove(dbPath)
		os.Remove(dbPath + "-wal")
		os.Remove(dbPath + "-shm")
	})

	db, err := sqlite.Open(dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	if err := db.Migrate(); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO seals (id, name) VALUES (1, 'metrics-test');
		INSERT INTO projects (id, seal_id, name) VALUES (1, 1, 'metrics-test');`); err != nil {
		t.Fatalf("seed metric project ownership: %v", err)
	}
	return db
}

// TestMetricsRollupMinuteToHourAndPrune covers the minute->hour rollup +
// retention sweep end to end: seeded minute samples/latency buckets sum
// correctly into an hour bucket, and once "now" is far enough past
// MetricsMinuteRetention, the source minute rows are pruned in the same
// Rollup() call that produced the hour rollup (never before it).
func TestMetricsRollupMinuteToHourAndPrune(t *testing.T) {
	db := openRollupTestDB(t, "test_metrics_rollup_minute_to_hour.db")

	hourStart := time.Date(2026, 7, 1, 10, 0, 0, 0, time.UTC)

	if err := db.InsertMetricSamples([]*domain.MetricSample{
		{ProjectID: 1, Resolution: domain.MetricResolutionMinute, BucketStart: hourStart, Count2xx: 10, Count5xx: 1, BytesIn: 100, BytesOut: 200},
		{ProjectID: 1, Resolution: domain.MetricResolutionMinute, BucketStart: hourStart.Add(30 * time.Minute), Count2xx: 20, BytesIn: 300, BytesOut: 400},
		{ProjectID: 1, Resolution: domain.MetricResolutionMinute, BucketStart: hourStart.Add(59 * time.Minute), Count2xx: 5, BytesIn: 50, BytesOut: 60},
	}); err != nil {
		t.Fatalf("seed minute samples: %v", err)
	}
	if err := db.InsertMetricLatencyBuckets([]*domain.MetricLatencyBucket{
		{ProjectID: 1, Resolution: domain.MetricResolutionMinute, BucketStart: hourStart, Le: "0.1", Count: 3},
		{ProjectID: 1, Resolution: domain.MetricResolutionMinute, BucketStart: hourStart.Add(30 * time.Minute), Le: "0.1", Count: 4},
	}); err != nil {
		t.Fatalf("seed minute latency buckets: %v", err)
	}

	// period=0 disables the background ticker - Rollup is called directly
	// and deterministically instead.
	svc := service.NewMetricsRollupService(db, 0)

	// "now" is far enough past hourStart that MetricsMinuteRetention (48h)
	// has fully elapsed for this bucket - the aggregate-before-prune margin
	// (rollupResolution's doc) guarantees the hour rollup still happens in
	// this same call, before the minute rows it was built from are pruned.
	now := hourStart.Add(MetricsMinuteRetentionPlus(t))
	if err := svc.Rollup(now); err != nil {
		t.Fatalf("rollup: %v", err)
	}

	hourSamples, err := db.ListMetricSamples(1, domain.MetricResolutionHour, hourStart, hourStart.Add(time.Hour))
	if err != nil {
		t.Fatalf("list hour samples: %v", err)
	}
	if len(hourSamples) != 1 {
		t.Fatalf("expected exactly 1 rolled-up hour row, got %d: %+v", len(hourSamples), hourSamples)
	}
	hs := hourSamples[0]
	if hs.Count2xx != 35 || hs.Count5xx != 1 {
		t.Errorf("unexpected summed status counts: %+v", hs)
	}
	if hs.BytesIn != 450 || hs.BytesOut != 660 {
		t.Errorf("unexpected summed byte counts: %+v", hs)
	}

	hourBuckets, err := db.ListMetricLatencyBuckets(1, domain.MetricResolutionHour, hourStart, hourStart.Add(time.Hour))
	if err != nil {
		t.Fatalf("list hour latency buckets: %v", err)
	}
	if len(hourBuckets) != 1 || hourBuckets[0].Count != 7 {
		t.Fatalf("expected summed le=0.1 count of 7, got %+v", hourBuckets)
	}

	minuteSamples, err := db.ListMetricSamples(1, domain.MetricResolutionMinute, hourStart, hourStart.Add(time.Hour))
	if err != nil {
		t.Fatalf("list minute samples after rollup: %v", err)
	}
	if len(minuteSamples) != 0 {
		t.Fatalf("expected source minute samples pruned once past retention, got %d: %+v", len(minuteSamples), minuteSamples)
	}

	minuteBuckets, err := db.ListMetricLatencyBuckets(1, domain.MetricResolutionMinute, hourStart, hourStart.Add(time.Hour))
	if err != nil {
		t.Fatalf("list minute latency buckets after rollup: %v", err)
	}
	if len(minuteBuckets) != 0 {
		t.Fatalf("expected source minute latency buckets pruned once past retention, got %d: %+v", len(minuteBuckets), minuteBuckets)
	}
}

// TestMetricsRollupIsIdempotent covers re-running Rollup over the same data
// twice: the rolled-up hour sum must not double-count.
func TestMetricsRollupIsIdempotent(t *testing.T) {
	db := openRollupTestDB(t, "test_metrics_rollup_idempotent.db")

	hourStart := time.Date(2026, 7, 1, 10, 0, 0, 0, time.UTC)
	if err := db.InsertMetricSamples([]*domain.MetricSample{
		{ProjectID: 1, Resolution: domain.MetricResolutionMinute, BucketStart: hourStart, Count2xx: 10},
	}); err != nil {
		t.Fatalf("seed minute samples: %v", err)
	}

	svc := service.NewMetricsRollupService(db, 0)

	// "now" is far enough past hourStart that the minute row is past
	// MetricsMinuteRetention - the first Rollup call both aggregates it into
	// the hour bucket and prunes the source minute row; the second call has
	// no surviving minute rows left to re-aggregate (OldestBucketStart finds
	// none), so it must be a pure no-op that leaves the hour sum untouched
	// rather than double-counting or zeroing it out.
	now := hourStart.Add(MetricsMinuteRetentionPlus(t))
	if err := svc.Rollup(now); err != nil {
		t.Fatalf("first rollup: %v", err)
	}
	if err := svc.Rollup(now); err != nil {
		t.Fatalf("second rollup: %v", err)
	}

	hourSamples, err := db.ListMetricSamples(1, domain.MetricResolutionHour, hourStart, hourStart.Add(time.Hour))
	if err != nil {
		t.Fatalf("list hour samples: %v", err)
	}
	if len(hourSamples) != 1 || hourSamples[0].Count2xx != 10 {
		t.Fatalf("expected idempotent re-run to leave Count2xx=10 (not double-counted), got %+v", hourSamples)
	}
}

// TestMetricsRollupDeepGapDoesNotLoseData is the C3 review regression test:
// a minute row that's already OLDER than the old fixed-lookback margin
// (MetricsMinuteRetention + one extra hour) by the time the very first
// post-gap Rollup call fires - e.g. the process was down far longer than
// that - must still get rolled up into its hour bucket before it's pruned.
// Under the old fixed-`now.Add(-lookback)` design this source row would
// have been silently pruned with no hour aggregate ever created for it
// (unrecoverable data loss); this must FAIL on that design and PASS once
// rollupResolution derives its start from the oldest surviving row instead.
func TestMetricsRollupDeepGapDoesNotLoseData(t *testing.T) {
	db := openRollupTestDB(t, "test_metrics_rollup_deep_gap.db")

	hourStart := time.Date(2026, 7, 1, 10, 0, 0, 0, time.UTC)
	if err := db.InsertMetricSamples([]*domain.MetricSample{
		{ProjectID: 1, Resolution: domain.MetricResolutionMinute, BucketStart: hourStart, Count2xx: 10, Count5xx: 1, BytesIn: 100, BytesOut: 200},
		{ProjectID: 1, Resolution: domain.MetricResolutionMinute, BucketStart: hourStart.Add(30 * time.Minute), Count2xx: 20, BytesIn: 300, BytesOut: 400},
	}); err != nil {
		t.Fatalf("seed minute samples: %v", err)
	}
	if err := db.InsertMetricLatencyBuckets([]*domain.MetricLatencyBucket{
		{ProjectID: 1, Resolution: domain.MetricResolutionMinute, BucketStart: hourStart, Le: "0.1", Count: 3},
	}); err != nil {
		t.Fatalf("seed minute latency buckets: %v", err)
	}

	svc := service.NewMetricsRollupService(db, 0)

	// "now" is a whole extra retention window past the old margin
	// (MetricsMinuteRetention + 1h) - a gap deep enough that the old
	// fixed-lookback design's aggregation window would never reach back to
	// hourStart at all, but PruneMetrics' cutoff (now-retention) would still
	// be past it and delete it.
	now := hourStart.Add(MetricsMinuteRetentionPlus(t)).Add(MetricsMinuteRetentionPlus(t))
	if err := svc.Rollup(now); err != nil {
		t.Fatalf("rollup: %v", err)
	}

	hourSamples, err := db.ListMetricSamples(1, domain.MetricResolutionHour, hourStart, hourStart.Add(time.Hour))
	if err != nil {
		t.Fatalf("list hour samples: %v", err)
	}
	if len(hourSamples) != 1 {
		t.Fatalf("expected the deep-gap minute data to still be rolled up into 1 hour row, got %d: %+v", len(hourSamples), hourSamples)
	}
	hs := hourSamples[0]
	if hs.Count2xx != 30 || hs.Count5xx != 1 || hs.BytesIn != 400 || hs.BytesOut != 600 {
		t.Errorf("unexpected summed hour sample after deep-gap rollup: %+v", hs)
	}

	hourBuckets, err := db.ListMetricLatencyBuckets(1, domain.MetricResolutionHour, hourStart, hourStart.Add(time.Hour))
	if err != nil {
		t.Fatalf("list hour latency buckets: %v", err)
	}
	if len(hourBuckets) != 1 || hourBuckets[0].Count != 3 {
		t.Fatalf("expected the deep-gap minute latency data rolled up too, got %+v", hourBuckets)
	}

	minuteSamples, err := db.ListMetricSamples(1, domain.MetricResolutionMinute, hourStart, hourStart.Add(time.Hour))
	if err != nil {
		t.Fatalf("list minute samples after deep-gap rollup: %v", err)
	}
	if len(minuteSamples) != 0 {
		t.Fatalf("expected source minute samples pruned once past retention, got %d: %+v", len(minuteSamples), minuteSamples)
	}
}

// MetricsMinuteRetentionPlus returns a duration safely past
// service.MetricsMinuteRetention, kept as a helper so the test doesn't
// hardcode a magic number disconnected from the constant it's meant to
// exceed.
func MetricsMinuteRetentionPlus(t *testing.T) time.Duration {
	t.Helper()
	return service.MetricsMinuteRetention + time.Hour
}
