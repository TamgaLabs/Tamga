package service_test

import (
	"math"
	"testing"

	"github.com/TamgaLabs/Tamga/backend/internal/domain"
	"github.com/TamgaLabs/Tamga/backend/internal/service"
)

// TestPercentilesFromLatencyBucketsKnownDistribution covers the core
// linear-interpolation-within-bucket computation against a hand-worked
// known distribution (le -> cumulative count): 0.1:10, 0.3:30, 1.2:60,
// 5:90, +Inf:100 (total 100 requests).
//
//	p50 target=50 falls between le=0.3 (count 30) and le=1.2 (count 60):
//	    0.3 + (50-30)/(60-30)*(1.2-0.3) = 0.3 + 0.6667*0.9 = 0.9
//	p95/p99 targets (95, 99) both exceed le=5's count (90) and fall inside
//	    the "+Inf" overflow bucket - report the highest finite le (5), the
//	    standard histogram_quantile convention for an unbounded top bucket.
func TestPercentilesFromLatencyBucketsKnownDistribution(t *testing.T) {
	buckets := []*domain.MetricLatencyBucket{
		{Le: "0.1", Count: 10},
		{Le: "0.3", Count: 30},
		{Le: "1.2", Count: 60},
		{Le: "5", Count: 90},
		{Le: "+Inf", Count: 100},
	}

	p50, p95, p99 := service.PercentilesFromLatencyBuckets(buckets)

	if math.Abs(p50-0.9) > 1e-9 {
		t.Errorf("p50 = %v, want 0.9", p50)
	}
	if p95 != 5 {
		t.Errorf("p95 = %v, want 5", p95)
	}
	if p99 != 5 {
		t.Errorf("p99 = %v, want 5", p99)
	}
}

// TestPercentilesFromLatencyBucketsNumericLeSortNotLexical is the carried-
// forward FEAT-030 review note's regression test: `le` is TEXT with lexical
// SQL ordering, where "10" < "5" lexically despite 10 > 5 numerically. Feed
// buckets in an order that is not numerically sorted (and includes that
// exact "10" vs "5" lexical trap) and confirm the result only makes sense
// if the function sorted by parsed float64 value, not string/arrival order.
func TestPercentilesFromLatencyBucketsNumericLeSortNotLexical(t *testing.T) {
	buckets := []*domain.MetricLatencyBucket{
		{Le: "+Inf", Count: 100},
		{Le: "10", Count: 100}, // le=10 already holds every request
		{Le: "5", Count: 40},
		{Le: "1", Count: 10},
	}

	p50, p95, p99 := service.PercentilesFromLatencyBuckets(buckets)

	// Numerically sorted: le=1(10), le=5(40), le=10(100), le=+Inf(100).
	// total=100. All three targets (50, 95, 99) exceed le=5's count (40)
	// and are reached by le=10's count (100) - so every quantile
	// interpolates within (le=5, le=10]. A lexical-sort bug would instead
	// treat "10" as coming before "5" and produce a nonsensical result.
	want := func(q float64) float64 {
		target := q * 100
		return 5 + (target-40)/(100-40)*(10-5)
	}

	if math.Abs(p50-want(0.50)) > 1e-9 {
		t.Errorf("p50 = %v, want %v", p50, want(0.50))
	}
	if math.Abs(p95-want(0.95)) > 1e-9 {
		t.Errorf("p95 = %v, want %v", p95, want(0.95))
	}
	if math.Abs(p99-want(0.99)) > 1e-9 {
		t.Errorf("p99 = %v, want %v", p99, want(0.99))
	}
}

// TestPercentilesFromLatencyBucketsEmpty covers the no-data edge case: no
// buckets (or all-zero counts) must return zeros, never divide by zero /
// NaN / panic.
func TestPercentilesFromLatencyBucketsEmpty(t *testing.T) {
	p50, p95, p99 := service.PercentilesFromLatencyBuckets(nil)
	if p50 != 0 || p95 != 0 || p99 != 0 {
		t.Errorf("expected zeros for nil input, got %v %v %v", p50, p95, p99)
	}

	p50, p95, p99 = service.PercentilesFromLatencyBuckets([]*domain.MetricLatencyBucket{
		{Le: "0.1", Count: 0},
		{Le: "+Inf", Count: 0},
	})
	if p50 != 0 || p95 != 0 || p99 != 0 {
		t.Errorf("expected zeros for all-zero-count input, got %v %v %v", p50, p95, p99)
	}
}
