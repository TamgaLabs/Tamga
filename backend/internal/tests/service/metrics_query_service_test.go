package service_test

import (
	"testing"
	"time"

	"github.com/TamgaLabs/Tamga/backend/internal/domain"
	"github.com/TamgaLabs/Tamga/backend/internal/service"
)

// TestMetricsQueryServiceGetPanelsShapesData seeds minute samples + latency
// buckets for one project, then asserts GetPanels shapes them into the
// four panel arrays correctly - request count/rate, status-class breakdown
// + error rate, latency percentiles (via PercentilesFromLatencyBuckets),
// and bandwidth - all aligned to the same bucket_start.
func TestMetricsQueryServiceGetPanelsShapesData(t *testing.T) {
	db := openRollupTestDB(t, "test_metrics_query_shapes_data.db")

	base := time.Date(2026, 7, 1, 10, 0, 0, 0, time.UTC)

	if err := db.InsertMetricSamples([]*domain.MetricSample{
		{ProjectID: 1, Resolution: domain.MetricResolutionMinute, BucketStart: base, Count2xx: 8, Count3xx: 0, Count4xx: 1, Count5xx: 1, BytesIn: 1000, BytesOut: 2000},
	}); err != nil {
		t.Fatalf("seed samples: %v", err)
	}
	if err := db.InsertMetricLatencyBuckets([]*domain.MetricLatencyBucket{
		{ProjectID: 1, Resolution: domain.MetricResolutionMinute, BucketStart: base, Le: "0.1", Count: 5},
		{ProjectID: 1, Resolution: domain.MetricResolutionMinute, BucketStart: base, Le: "1", Count: 9},
		{ProjectID: 1, Resolution: domain.MetricResolutionMinute, BucketStart: base, Le: "+Inf", Count: 10},
	}); err != nil {
		t.Fatalf("seed latency buckets: %v", err)
	}

	svc := service.NewMetricsQueryService(db)

	panels, err := svc.GetPanels(1, base, base.Add(time.Minute), domain.MetricResolutionMinute)
	if err != nil {
		t.Fatalf("get panels: %v", err)
	}

	if panels.ProjectID != 1 || panels.Resolution != domain.MetricResolutionMinute {
		t.Fatalf("unexpected panel header: %+v", panels)
	}

	if len(panels.RequestRate) != 1 {
		t.Fatalf("expected 1 request_rate point, got %d", len(panels.RequestRate))
	}
	rr := panels.RequestRate[0]
	if rr.Count != 10 {
		t.Errorf("request_rate count = %d, want 10 (8+0+1+1)", rr.Count)
	}
	if rr.RatePerSec != 10.0/60.0 {
		t.Errorf("request_rate rate_per_sec = %v, want %v", rr.RatePerSec, 10.0/60.0)
	}
	if !rr.BucketStart.Equal(base) {
		t.Errorf("request_rate bucket_start = %v, want %v", rr.BucketStart, base)
	}

	if len(panels.StatusClass) != 1 {
		t.Fatalf("expected 1 status_class point, got %d", len(panels.StatusClass))
	}
	sc := panels.StatusClass[0]
	if sc.Count2xx != 8 || sc.Count4xx != 1 || sc.Count5xx != 1 {
		t.Errorf("unexpected status_class counts: %+v", sc)
	}
	wantErrorRate := 2.0 / 10.0
	if sc.ErrorRate != wantErrorRate {
		t.Errorf("error_rate = %v, want %v", sc.ErrorRate, wantErrorRate)
	}

	if len(panels.Latency) != 1 {
		t.Fatalf("expected 1 latency point, got %d", len(panels.Latency))
	}
	// Cross-check against the pure helper directly, rather than a second
	// hardcoded expectation, to keep this test about shaping/wiring (does
	// GetPanels group by bucket_start and call the helper correctly), not
	// re-deriving the interpolation math already covered by
	// metrics_percentile_test.go.
	wantP50, wantP95, wantP99 := service.PercentilesFromLatencyBuckets([]*domain.MetricLatencyBucket{
		{Le: "0.1", Count: 5}, {Le: "1", Count: 9}, {Le: "+Inf", Count: 10},
	})
	lp := panels.Latency[0]
	if lp.P50 != wantP50 || lp.P95 != wantP95 || lp.P99 != wantP99 {
		t.Errorf("latency point = %+v, want p50=%v p95=%v p99=%v", lp, wantP50, wantP95, wantP99)
	}

	if len(panels.Bandwidth) != 1 {
		t.Fatalf("expected 1 bandwidth point, got %d", len(panels.Bandwidth))
	}
	bw := panels.Bandwidth[0]
	if bw.BytesIn != 1000 || bw.BytesOut != 2000 {
		t.Errorf("unexpected bandwidth: %+v", bw)
	}
}

// TestMetricsQueryServiceGetPanelsEmptySeries covers a project/scope with
// no data in range: every panel must be an empty (non-nil) array, and
// GetPanels must not return an error.
func TestMetricsQueryServiceGetPanelsEmptySeries(t *testing.T) {
	db := openRollupTestDB(t, "test_metrics_query_empty_series.db")

	svc := service.NewMetricsQueryService(db)

	from := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	to := from.Add(time.Hour)

	panels, err := svc.GetPanels(999, from, to, domain.MetricResolutionMinute)
	if err != nil {
		t.Fatalf("expected no error for a project with no data, got %v", err)
	}
	if panels == nil {
		t.Fatal("expected a non-nil panels struct")
	}
	if panels.RequestRate == nil || len(panels.RequestRate) != 0 {
		t.Errorf("expected empty (non-nil) request_rate, got %#v", panels.RequestRate)
	}
	if panels.StatusClass == nil || len(panels.StatusClass) != 0 {
		t.Errorf("expected empty (non-nil) status_class, got %#v", panels.StatusClass)
	}
	if panels.Latency == nil || len(panels.Latency) != 0 {
		t.Errorf("expected empty (non-nil) latency, got %#v", panels.Latency)
	}
	if panels.Bandwidth == nil || len(panels.Bandwidth) != 0 {
		t.Errorf("expected empty (non-nil) bandwidth, got %#v", panels.Bandwidth)
	}
}

// TestMetricsQueryServiceAutoResolution covers the auto-pick thresholds
// (<48h -> minute, <30d -> hour, else day) when resolution is omitted.
func TestMetricsQueryServiceAutoResolution(t *testing.T) {
	base := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)

	cases := []struct {
		name string
		span time.Duration
		want domain.MetricResolution
	}{
		{"1 hour", time.Hour, domain.MetricResolutionMinute},
		{"exactly 48h", 48 * time.Hour, domain.MetricResolutionMinute},
		{"49h", 49 * time.Hour, domain.MetricResolutionHour},
		{"exactly 30d", 30 * 24 * time.Hour, domain.MetricResolutionHour},
		{"31d", 31 * 24 * time.Hour, domain.MetricResolutionDay},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := service.AutoResolution(base, base.Add(c.span))
			if got != c.want {
				t.Errorf("AutoResolution(span=%v) = %v, want %v", c.span, got, c.want)
			}
		})
	}
}
