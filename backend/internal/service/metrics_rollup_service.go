package service

import (
	"fmt"
	"time"

	"github.com/TamgaLabs/Tamga/backend/internal/domain"
	"github.com/TamgaLabs/Tamga/backend/internal/repository/sqlite"
)

// MetricsMinuteRetention/MetricsHourRetention are FEAT-032's documented
// minute->hour->day retention policy (per the user's decision, carried in
// the task's Requirements): minute-resolution rows are kept for ~48h, hour-
// resolution rows for ~30d, both pruned once their data has been rolled up
// into the next coarser resolution. Day-resolution rows are never pruned -
// deliberately no MetricsDayRetention const - daily rows are compact enough
// (one row per project per day) to retain indefinitely; that's the whole
// point of rolling minute/hour data up into them.
const (
	MetricsMinuteRetention = 48 * time.Hour
	MetricsHourRetention   = 30 * 24 * time.Hour
)

// DefaultMetricsRollupInterval is how often cmd/api/main.go's background
// rollup sweep ticks. Frequent relative to the retention windows above (5m
// vs 48h/30d) so that, under normal continuous operation, a bucket is
// re-aggregated many times over its lifetime before it ages out of
// retention - rollupResolution's oldest-row-driven aggregation is what
// makes correctness NOT depend on that frequent-ticking assumption (it's
// correct as a single call after any gap, of any length), but keeping the
// tick frequent still means a missed/late scrape gets rolled up promptly
// rather than sitting around for hours.
const DefaultMetricsRollupInterval = 5 * time.Minute

// MetricsRollupService periodically aggregates FEAT-030/031's stored
// minute-resolution samples up into hour, then day, buckets
// (DB.AggregateMetrics) and prunes the finer-grained rows once superseded
// (DB.PruneMetrics) - see the retention consts above. Follows the same
// "goroutine started at construction, runs for the process lifetime"
// pattern as MetricsScraperService/AgentService.startIdleSweep - there is
// no graceful-shutdown plumbing for services in cmd/api/main.go, so a plain
// unstoppable goroutine is the KISS choice here too.
type MetricsRollupService struct {
	db     *sqlite.DB
	period time.Duration
}

// NewMetricsRollupService constructs the rollup service and starts its
// background sweep loop. A non-positive period disables the loop entirely
// (e.g. tests that want to call Rollup directly, deterministically, rather
// than waiting on a ticker).
func NewMetricsRollupService(db *sqlite.DB, period time.Duration) *MetricsRollupService {
	s := &MetricsRollupService{db: db, period: period}
	s.startRollupLoop()
	return s
}

// startRollupLoop launches the long-lived background goroutine that runs
// Rollup every s.period.
func (s *MetricsRollupService) startRollupLoop() {
	if s.period <= 0 {
		return
	}
	go func() {
		ticker := time.NewTicker(s.period)
		defer ticker.Stop()
		for range ticker.C {
			s.Rollup(time.Now())
		}
	}()
}

// Rollup performs one full rollup+retention sweep as of now: minute->hour
// aggregation then minute prune, hour->day aggregation then hour prune.
// Idempotent and safe to call directly (e.g. from a test, or as the very
// first call after a long process gap, of any length) - see
// rollupResolution's doc for how it derives how far back to aggregate from
// the data actually still present, rather than a fixed lookback window.
func (s *MetricsRollupService) Rollup(now time.Time) error {
	now = now.UTC()

	minuteCutoff := now.Add(-MetricsMinuteRetention)
	if err := s.rollupResolution(domain.MetricResolutionMinute, domain.MetricResolutionHour, time.Hour, minuteCutoff); err != nil {
		return fmt.Errorf("rollup minute->hour: %w", err)
	}
	if err := s.db.PruneMetrics(domain.MetricResolutionMinute, minuteCutoff); err != nil {
		return fmt.Errorf("prune minute metrics: %w", err)
	}

	hourCutoff := now.Add(-MetricsHourRetention)
	if err := s.rollupResolution(domain.MetricResolutionHour, domain.MetricResolutionDay, 24*time.Hour, hourCutoff); err != nil {
		return fmt.Errorf("rollup hour->day: %w", err)
	}
	if err := s.db.PruneMetrics(domain.MetricResolutionHour, hourCutoff); err != nil {
		return fmt.Errorf("prune hour metrics: %w", err)
	}

	return nil
}

// rollupResolution aggregates every dst-resolution bucket that has
// src-resolution source rows anywhere from the OLDEST surviving
// src-resolution row up to cutoff (the exact instant PruneMetrics is about
// to prune src rows older than, in the same Rollup call) into its
// containing dst bucket via DB.AggregateMetrics - idempotent, safe to
// re-run.
//
// Driving the start of the range from DB.OldestBucketStart (an actual MIN
// query), rather than a fixed lookback window back from now, is what makes
// this correct regardless of how long a gap there was since the last
// Rollup call: no matter how old the oldest surviving row is, it's always
// included in [oldest, cutoff) and gets a chance to be aggregated before
// PruneMetrics can delete it in this same call. A fixed lookback window
// (the previous design) only aggregated rows within a bounded distance of
// now - any row older than that, from a gap longer than the window, would
// be pruned with no aggregate ever created for it: silent, unrecoverable
// data loss. This has no such bound.
//
// It aggregates up to cutoff, not up to now: buckets more recent than
// cutoff haven't hit this resolution's retention age yet, so they're left
// alone at the finer resolution rather than rolled up early.
//
// DB.DistinctDstBucketStarts (rather than iterating every dstWindow-sized
// step between oldest and cutoff) keeps this bounded by the amount of
// actual data in that range, not by how long the gap was - a deep gap with
// no data in it contributes no empty buckets to iterate.
func (s *MetricsRollupService) rollupResolution(src, dst domain.MetricResolution, window time.Duration, cutoff time.Time) error {
	oldestUnix, ok, err := s.db.OldestBucketStart(src)
	if err != nil {
		return fmt.Errorf("oldest %s bucket: %w", src, err)
	}
	if !ok {
		return nil // no src rows at all - nothing to aggregate.
	}
	oldest := time.Unix(oldestUnix, 0).UTC()

	dstBuckets, err := s.db.DistinctDstBucketStarts(src, window, oldest, cutoff)
	if err != nil {
		return fmt.Errorf("distinct %s->%s dst buckets: %w", src, dst, err)
	}
	for _, b := range dstBuckets {
		if err := s.db.AggregateMetrics(src, dst, b); err != nil {
			return fmt.Errorf("aggregate %s->%s bucket %s: %w", src, dst, b, err)
		}
	}
	return nil
}
