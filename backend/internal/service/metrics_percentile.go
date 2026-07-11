package service

import (
	"math"
	"sort"
	"strconv"

	"github.com/TamgaLabs/Tamga/backend/internal/domain"
)

// lePoint is one parsed (numeric le, cumulative count) pair, used
// internally by PercentilesFromLatencyBuckets.
type lePoint struct {
	le    float64
	isInf bool
	count int64
}

// PercentilesFromLatencyBuckets computes p50/p95/p99 (in the same unit as
// `le` - seconds, for Traefik's request_duration_seconds histogram) from
// one bucket_start's set of cumulative-`le` histogram bucket counts, using
// the same linear-interpolation-within-bucket method PromQL's
// histogram_quantile uses:
//
//  1. Parse every `le` to float64 (and "+Inf" to +Inf) and sort
//     NUMERICALLY. This is required, not cosmetic: FEAT-030's review noted
//     `le` is stored as TEXT with LEXICAL SQL ordering ("10" < "5"
//     lexically despite 10 > 5 numerically, and "+Inf" isn't comparable to
//     the rest as a string at all) - so this function re-sorts its input
//     itself rather than trusting the order buckets arrives in. Callers
//     never need to pre-sort.
//  2. The highest `le` (by Prometheus/Traefik convention, "+Inf") holds the
//     cumulative total count.
//  3. For quantile q, walk buckets in ascending le order to find the first
//     whose cumulative count is >= q*total, then linearly interpolate the
//     target's position between that bucket's le and the previous (lower)
//     bucket's le (0 if there is no previous bucket), weighted by count. If
//     the target falls inside the "+Inf" overflow bucket, there is no upper
//     bound to interpolate against - report the highest finite le instead,
//     the standard histogram_quantile convention for an unbounded top
//     bucket.
//
// buckets must all share one bucket_start (grouping by bucket_start is the
// caller's job - see MetricsQueryService.GetPanels). Empty input or a
// non-positive total returns 0,0,0 rather than dividing by zero.
func PercentilesFromLatencyBuckets(buckets []*domain.MetricLatencyBucket) (p50, p95, p99 float64) {
	points := make([]lePoint, 0, len(buckets))
	for _, b := range buckets {
		if b.Le == "+Inf" {
			points = append(points, lePoint{le: math.Inf(1), isInf: true, count: b.Count})
			continue
		}
		v, err := strconv.ParseFloat(b.Le, 64)
		if err != nil {
			// Unparseable le - defensive, shouldn't happen for
			// Traefik/Prometheus-sourced data. Skip rather than fail the
			// whole panel point over one bad row.
			continue
		}
		points = append(points, lePoint{le: v, count: b.Count})
	}
	if len(points) == 0 {
		return 0, 0, 0
	}
	sort.Slice(points, func(i, j int) bool { return points[i].le < points[j].le })

	total := points[len(points)-1].count
	if total <= 0 {
		return 0, 0, 0
	}

	quantile := func(q float64) float64 {
		target := q * float64(total)
		var prevLe float64
		var prevCount int64
		for _, p := range points {
			if float64(p.count) >= target {
				if p.isInf {
					return prevLe
				}
				if p.count == prevCount {
					return p.le
				}
				frac := (target - float64(prevCount)) / float64(p.count-prevCount)
				return prevLe + frac*(p.le-prevLe)
			}
			prevLe = p.le
			prevCount = p.count
		}
		return prevLe
	}

	return quantile(0.50), quantile(0.95), quantile(0.99)
}
