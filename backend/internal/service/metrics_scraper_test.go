package service

import (
	"os"
	"testing"
	"time"

	"github.com/TamgaLabs/Tamga/backend/internal/domain"
)

// This file is a deliberate exception to FEAT-021's move of tests into
// internal/tests/ (same rationale as ring_buffer_test.go/idle_sweep_test.go):
// parseTraefikMetrics, computeIncrements, scrapeState, resolveProjectID,
// statusClass, stripProviderSuffix and diffCounter are all unexported, with
// no exported constructor/interface that would let a black-box test reach
// them - MetricsScraperService's only exported surface is
// NewMetricsScraperService, which immediately starts a real HTTP scrape
// goroutine against a live Traefik. The goroutine/HTTP wiring itself (tick,
// scrape) is intentionally left thin and untested here, per the task -
// everything below exercises the pure parse+increment+reset+class-folding
// logic directly.
//
// Fixtures: backend/internal/service/testdata/traefik_scrape_{1,2,3}.txt.
// scrape_1/scrape_2's tamga-* core-router lines are copied verbatim from a
// real live Traefik 3.4.5 capture (tamga-traefik-1, via `docker exec
// tamga-backend-1 wget -qO- http://traefik:8080/metrics`) taken during this
// task's development - see scrape_1's header comment. The project-42@file
// router block and scrape_3 (simulating a Traefik restart) are hand-authored
// in the identical real format, since no project was deployed live to
// generate real per-project/reset samples from. Every expected number below
// was cross-checked with a standalone script before being hardcoded (see the
// task's Implementation Notes) rather than derived from the parser itself,
// so these tests actually catch a wrong computation.

func readFixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	return data
}

func TestStripProviderSuffix(t *testing.T) {
	cases := map[string]string{
		"project-42@file":      "project-42",
		"tamga-ui@file":        "tamga-ui",
		"tamga-ui-secure@file": "tamga-ui-secure",
		"no-suffix":            "no-suffix",
	}
	for in, want := range cases {
		if got := stripProviderSuffix(in); got != want {
			t.Errorf("stripProviderSuffix(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestResolveProjectID(t *testing.T) {
	cases := []struct {
		bare string
		want int64
	}{
		{"project-42", 42},
		{"project-7", 7},
		{"tamga-ui", domain.GlobalProjectID},
		{"tamga-ui-secure", domain.GlobalProjectID},
		{"tamga-api", domain.GlobalProjectID},
		{"tamga-api-secure", domain.GlobalProjectID},
		{"some-future-router", domain.GlobalProjectID},
	}
	for _, c := range cases {
		if got := resolveProjectID(c.bare); got != c.want {
			t.Errorf("resolveProjectID(%q) = %d, want %d", c.bare, got, c.want)
		}
	}
}

func TestStatusClass(t *testing.T) {
	cases := []struct {
		code      string
		wantClass string
		wantOK    bool
	}{
		{"200", "2xx", true},
		{"201", "2xx", true},
		{"301", "3xx", true},
		{"404", "4xx", true},
		{"499", "4xx", true},
		{"500", "5xx", true},
		{"100", "", false},
		{"", "", false},
	}
	for _, c := range cases {
		class, ok := statusClass(c.code)
		if class != c.wantClass || ok != c.wantOK {
			t.Errorf("statusClass(%q) = (%q, %v), want (%q, %v)", c.code, class, ok, c.wantClass, c.wantOK)
		}
	}
}

func TestDiffCounter(t *testing.T) {
	cases := []struct {
		name string
		prev float64
		cur  float64
		want int64
	}{
		{"normal increment", 10, 25, 15},
		{"no change", 10, 10, 0},
		{"reset to zero", 150, 10, 10},
		{"reset with current also zero", 8, 0, 0},
		{"first-ever value (prev zero)", 0, 5, 5},
	}
	for _, c := range cases {
		if got := diffCounter(c.prev, c.cur); got != c.want {
			t.Errorf("%s: diffCounter(%v, %v) = %d, want %d", c.name, c.prev, c.cur, got, c.want)
		}
	}
}

// TestParseTraefikMetricsScrape1 checks the baseline scrape's parsed
// cumulative values against the real captured numbers (for the tamga-*
// core routers, folded into project 0) and the hand-authored project-42
// numbers.
func TestParseTraefikMetricsScrape1(t *testing.T) {
	state := parseTraefikMetrics(readFixture(t, "traefik_scrape_1.txt"))

	// Global (project 0), folded from the real captured tamga-* lines.
	assertFloat(t, "global 2xx", state.counts[countKey{domain.GlobalProjectID, "2xx"}], 46)
	assertFloat(t, "global 3xx", state.counts[countKey{domain.GlobalProjectID, "3xx"}], 0)
	assertFloat(t, "global 4xx", state.counts[countKey{domain.GlobalProjectID, "4xx"}], 40)
	assertFloat(t, "global 5xx", state.counts[countKey{domain.GlobalProjectID, "5xx"}], 0)
	assertFloat(t, "global bytesIn", state.bytesIn[domain.GlobalProjectID], 2007)
	assertFloat(t, "global bytesOut", state.bytesOut[domain.GlobalProjectID], 64470)
	assertFloat(t, "global le=0.1", state.latency[latencyKey{domain.GlobalProjectID, "0.1"}], 81)
	assertFloat(t, "global le=0.3", state.latency[latencyKey{domain.GlobalProjectID, "0.3"}], 83)
	assertFloat(t, "global le=1.2", state.latency[latencyKey{domain.GlobalProjectID, "1.2"}], 86)
	assertFloat(t, "global le=5", state.latency[latencyKey{domain.GlobalProjectID, "5"}], 86)
	assertFloat(t, "global le=+Inf", state.latency[latencyKey{domain.GlobalProjectID, "+Inf"}], 86)

	// Project 42.
	assertFloat(t, "p42 2xx", state.counts[countKey{42, "2xx"}], 100)
	assertFloat(t, "p42 4xx", state.counts[countKey{42, "4xx"}], 5)
	assertFloat(t, "p42 5xx", state.counts[countKey{42, "5xx"}], 2)
	assertFloat(t, "p42 bytesIn", state.bytesIn[42], 1070)
	assertFloat(t, "p42 bytesOut", state.bytesOut[42], 50450)
	assertFloat(t, "p42 le=0.1", state.latency[latencyKey{42, "0.1"}], 87)
	assertFloat(t, "p42 le=+Inf", state.latency[latencyKey{42, "+Inf"}], 114)
}

func assertFloat(t *testing.T, label string, got, want float64) {
	t.Helper()
	if got != want {
		t.Errorf("%s = %v, want %v", label, got, want)
	}
}

// TestIngestFirstTickIsBaselineOnly asserts the first scrape after
// (re)start never produces samples/buckets to store - it only establishes
// the in-memory baseline (Requirements: "First tick after startup has no
// previous -> establish baseline, store nothing").
func TestIngestFirstTickIsBaselineOnly(t *testing.T) {
	s := &MetricsScraperService{}
	state1 := parseTraefikMetrics(readFixture(t, "traefik_scrape_1.txt"))

	samples, buckets := s.ingest(state1, time.Unix(0, 0))
	if samples != nil || buckets != nil {
		t.Fatalf("first tick: expected nil/nil (baseline only), got samples=%v buckets=%v", samples, buckets)
	}
}

// TestIngestSecondTickComputesIncrements exercises the ordinary case: a
// second scrape with higher cumulative counters produces exactly the
// increments since the first.
func TestIngestSecondTickComputesIncrements(t *testing.T) {
	s := &MetricsScraperService{}
	state1 := parseTraefikMetrics(readFixture(t, "traefik_scrape_1.txt"))
	state2 := parseTraefikMetrics(readFixture(t, "traefik_scrape_2.txt"))

	bucketStart := time.Date(2026, 7, 11, 12, 34, 0, 0, time.UTC)
	s.ingest(state1, bucketStart.Add(-time.Minute)) // baseline tick, discarded
	samples, buckets := s.ingest(state2, bucketStart)

	sampleByProject := indexSamples(samples)

	global := sampleByProject[domain.GlobalProjectID]
	if global == nil {
		t.Fatal("expected a global (project 0) sample")
	}
	assertSample(t, "global", global, bucketStart, 10, 0, 10, 0, 500, 1000)

	p42 := sampleByProject[42]
	if p42 == nil {
		t.Fatal("expected a project-42 sample")
	}
	assertSample(t, "project 42", p42, bucketStart, 50, 0, 3, 0, 500, 25000)

	bucketByKey := indexBuckets(buckets)
	assertLatency(t, bucketByKey, domain.GlobalProjectID, "0.1", 20)
	assertLatency(t, bucketByKey, domain.GlobalProjectID, "0.3", 20)
	assertLatency(t, bucketByKey, domain.GlobalProjectID, "1.2", 20)
	assertLatency(t, bucketByKey, domain.GlobalProjectID, "5", 20)
	assertLatency(t, bucketByKey, domain.GlobalProjectID, "+Inf", 20)
	assertLatency(t, bucketByKey, 42, "0.1", 53)
	assertLatency(t, bucketByKey, 42, "0.3", 53)
	assertLatency(t, bucketByKey, 42, "1.2", 53)
	assertLatency(t, bucketByKey, 42, "5", 53)
	assertLatency(t, bucketByKey, 42, "+Inf", 53)

	for _, sample := range samples {
		if sample.Resolution != domain.MetricResolutionMinute {
			t.Errorf("sample project %d: resolution = %q, want minute", sample.ProjectID, sample.Resolution)
		}
		if !sample.BucketStart.Equal(bucketStart) {
			t.Errorf("sample project %d: bucketStart = %v, want %v", sample.ProjectID, sample.BucketStart, bucketStart)
		}
	}
	for _, b := range buckets {
		if b.Resolution != domain.MetricResolutionMinute {
			t.Errorf("bucket project %d le=%s: resolution = %q, want minute", b.ProjectID, b.Le, b.Resolution)
		}
	}
}

// TestIngestThirdTickHandlesReset simulates a Traefik restart between the
// second and third scrapes: scrape_3's cumulative counters are lower than
// scrape_2's (some series absent entirely). Every increment must be
// computed as "current value" (never negative, never the nonsensical
// current-minus-prev that a naive diff would produce).
func TestIngestThirdTickHandlesReset(t *testing.T) {
	s := &MetricsScraperService{}
	state1 := parseTraefikMetrics(readFixture(t, "traefik_scrape_1.txt"))
	state2 := parseTraefikMetrics(readFixture(t, "traefik_scrape_2.txt"))
	state3 := parseTraefikMetrics(readFixture(t, "traefik_scrape_3.txt"))

	bucketStart := time.Date(2026, 7, 11, 12, 36, 0, 0, time.UTC)
	s.ingest(state1, bucketStart.Add(-2*time.Minute))
	s.ingest(state2, bucketStart.Add(-time.Minute))
	samples, buckets := s.ingest(state3, bucketStart)

	sampleByProject := indexSamples(samples)

	global := sampleByProject[domain.GlobalProjectID]
	if global == nil {
		t.Fatal("expected a global (project 0) sample")
	}
	// current 2xx=2 < prev 56 -> reset -> increment = current (2).
	// 4xx/3xx/5xx have no lines in scrape_3 at all (current=0) and prev
	// 4xx=50 -> reset -> increment = current (0). 3xx/5xx were already 0.
	assertSample(t, "global (reset)", global, bucketStart, 2, 0, 0, 0, 0, 500)

	p42 := sampleByProject[42]
	if p42 == nil {
		t.Fatal("expected a project-42 sample")
	}
	// current 2xx=10 < prev 150 -> reset -> increment = current (10).
	// 4xx/5xx absent in scrape_3 (current=0) < prev (8/2) -> reset -> 0.
	assertSample(t, "project 42 (reset)", p42, bucketStart, 10, 0, 0, 0, 80, 4000)

	if p42.Count2xx < 0 || p42.Count4xx < 0 || p42.Count5xx < 0 || p42.BytesIn < 0 || p42.BytesOut < 0 {
		t.Fatalf("project 42 sample has a negative increment: %+v", p42)
	}

	bucketByKey := indexBuckets(buckets)
	assertLatency(t, bucketByKey, domain.GlobalProjectID, "0.1", 2)
	assertLatency(t, bucketByKey, 42, "0.1", 3)
	assertLatency(t, bucketByKey, 42, "0.3", 5)
	assertLatency(t, bucketByKey, 42, "1.2", 8)
	assertLatency(t, bucketByKey, 42, "5", 10)
	assertLatency(t, bucketByKey, 42, "+Inf", 10)

	for _, b := range buckets {
		if b.Count < 0 {
			t.Errorf("bucket project %d le=%s: negative count %d", b.ProjectID, b.Le, b.Count)
		}
	}
}

func indexSamples(samples []*domain.MetricSample) map[int64]*domain.MetricSample {
	out := make(map[int64]*domain.MetricSample, len(samples))
	for _, s := range samples {
		out[s.ProjectID] = s
	}
	return out
}

func indexBuckets(buckets []*domain.MetricLatencyBucket) map[latencyKey]*domain.MetricLatencyBucket {
	out := make(map[latencyKey]*domain.MetricLatencyBucket, len(buckets))
	for _, b := range buckets {
		out[latencyKey{b.ProjectID, b.Le}] = b
	}
	return out
}

func assertSample(t *testing.T, label string, s *domain.MetricSample, bucketStart time.Time, c2, c3, c4, c5, bytesIn, bytesOut int64) {
	t.Helper()
	if s.Count2xx != c2 || s.Count3xx != c3 || s.Count4xx != c4 || s.Count5xx != c5 || s.BytesIn != bytesIn || s.BytesOut != bytesOut {
		t.Errorf("%s: got {2xx:%d 3xx:%d 4xx:%d 5xx:%d in:%d out:%d}, want {2xx:%d 3xx:%d 4xx:%d 5xx:%d in:%d out:%d}",
			label, s.Count2xx, s.Count3xx, s.Count4xx, s.Count5xx, s.BytesIn, s.BytesOut,
			c2, c3, c4, c5, bytesIn, bytesOut)
	}
}

func assertLatency(t *testing.T, buckets map[latencyKey]*domain.MetricLatencyBucket, projectID int64, le string, want int64) {
	t.Helper()
	b, ok := buckets[latencyKey{projectID, le}]
	if !ok {
		t.Errorf("missing latency bucket project=%d le=%s", projectID, le)
		return
	}
	if b.Count != want {
		t.Errorf("latency bucket project=%d le=%s: got %d, want %d", projectID, le, b.Count, want)
	}
}
