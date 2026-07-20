package sqlite

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/TamgaLabs/Tamga/backend/internal/domain"
)

// InsertMetricSamples batch-upserts status-class/byte-count increment rows
// (migration 000017's metric_samples table). The ON CONFLICT DO UPDATE
// ACCUMULATES (count = existing + excluded), it does not overwrite: each
// scrape produces the per-interval increment since the previous scrape, and
// when the scrape interval is finer than the bucket resolution (e.g. a 10s
// scrape into a 1-minute bucket) several disjoint increments land on the
// same (project_id, resolution, bucket_start) key and must SUM into that
// bucket's total. Overwriting would keep only the last interval's delta
// (typically 0 when traffic is idle), discarding the rest - the TEST-015
// data-loss bug. Each interval-increment is written exactly once (a failed
// tick never advances the scraper's baseline, so the next tick spans the
// full elapsed interval rather than re-emitting a written one), so additive
// accumulation never double-counts.
func (db *DB) InsertMetricSamples(samples []*domain.MetricSample) error {
	if len(samples) == 0 {
		return nil
	}

	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("begin insert metric samples: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`
		INSERT INTO metric_samples
			(project_id, resolution, bucket_start, count_2xx, count_3xx, count_4xx, count_5xx, bytes_in, bytes_out)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT (project_id, resolution, bucket_start) DO UPDATE SET
			count_2xx = metric_samples.count_2xx + excluded.count_2xx,
			count_3xx = metric_samples.count_3xx + excluded.count_3xx,
			count_4xx = metric_samples.count_4xx + excluded.count_4xx,
			count_5xx = metric_samples.count_5xx + excluded.count_5xx,
			bytes_in = metric_samples.bytes_in + excluded.bytes_in,
			bytes_out = metric_samples.bytes_out + excluded.bytes_out
	`)
	if err != nil {
		return fmt.Errorf("prepare insert metric samples: %w", err)
	}
	defer stmt.Close()

	for _, s := range samples {
		if _, err := stmt.Exec(
			s.ProjectID, string(s.Resolution), s.BucketStart.UTC().Unix(),
			s.Count2xx, s.Count3xx, s.Count4xx, s.Count5xx, s.BytesIn, s.BytesOut,
		); err != nil {
			return fmt.Errorf("insert metric sample (project %d, %s, %s): %w", s.ProjectID, s.Resolution, s.BucketStart, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit insert metric samples: %w", err)
	}
	return nil
}

// InsertMetricLatencyBuckets batch-upserts histogram bucket increment rows
// (migration 000017's metric_latency_buckets table). Same additive
// accumulation as InsertMetricSamples: multiple sub-bucket-resolution
// scrapes sharing a (project_id, resolution, bucket_start, le) key SUM into
// that bucket rather than overwriting.
func (db *DB) InsertMetricLatencyBuckets(buckets []*domain.MetricLatencyBucket) error {
	if len(buckets) == 0 {
		return nil
	}

	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("begin insert metric latency buckets: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`
		INSERT INTO metric_latency_buckets
			(project_id, resolution, bucket_start, le, count)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT (project_id, resolution, bucket_start, le) DO UPDATE SET
			count = metric_latency_buckets.count + excluded.count
	`)
	if err != nil {
		return fmt.Errorf("prepare insert metric latency buckets: %w", err)
	}
	defer stmt.Close()

	for _, b := range buckets {
		if _, err := stmt.Exec(
			b.ProjectID, string(b.Resolution), b.BucketStart.UTC().Unix(), b.Le, b.Count,
		); err != nil {
			return fmt.Errorf("insert metric latency bucket (project %d, %s, %s, le=%s): %w", b.ProjectID, b.Resolution, b.BucketStart, b.Le, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit insert metric latency buckets: %w", err)
	}
	return nil
}

// ListMetricSamples range-queries one project's (or domain.GlobalProjectID's)
// status-class/byte samples at a given resolution over [from, to) (from
// inclusive, to exclusive), ordered by bucket_start ascending.
func (db *DB) ListMetricSamples(projectID int64, resolution domain.MetricResolution, from, to time.Time) ([]*domain.MetricSample, error) {
	rows, err := db.Query(`
		SELECT id, project_id, resolution, bucket_start, count_2xx, count_3xx, count_4xx, count_5xx, bytes_in, bytes_out
		FROM metric_samples
		WHERE project_id = ? AND resolution = ? AND bucket_start >= ? AND bucket_start < ?
		ORDER BY bucket_start ASC
	`, projectID, string(resolution), from.UTC().Unix(), to.UTC().Unix())
	if err != nil {
		return nil, fmt.Errorf("list metric samples: %w", err)
	}
	defer rows.Close()

	var samples []*domain.MetricSample
	for rows.Next() {
		s := &domain.MetricSample{}
		var res string
		var bucketUnix int64
		if err := rows.Scan(&s.ID, &s.ProjectID, &res, &bucketUnix, &s.Count2xx, &s.Count3xx, &s.Count4xx, &s.Count5xx, &s.BytesIn, &s.BytesOut); err != nil {
			return nil, fmt.Errorf("scan metric sample: %w", err)
		}
		s.Resolution = domain.MetricResolution(res)
		s.BucketStart = time.Unix(bucketUnix, 0).UTC()
		samples = append(samples, s)
	}
	return samples, nil
}

// ListMetricLatencyBuckets range-queries one project's (or
// domain.GlobalProjectID's) histogram bucket increments at a given
// resolution over [from, to), ordered by bucket_start then le for a
// stable, query-side-groupable result (the shape FEAT-032 needs to compute
// percentiles per bucket_start).
func (db *DB) ListMetricLatencyBuckets(projectID int64, resolution domain.MetricResolution, from, to time.Time) ([]*domain.MetricLatencyBucket, error) {
	rows, err := db.Query(`
		SELECT id, project_id, resolution, bucket_start, le, count
		FROM metric_latency_buckets
		WHERE project_id = ? AND resolution = ? AND bucket_start >= ? AND bucket_start < ?
		ORDER BY bucket_start ASC, le ASC
	`, projectID, string(resolution), from.UTC().Unix(), to.UTC().Unix())
	if err != nil {
		return nil, fmt.Errorf("list metric latency buckets: %w", err)
	}
	defer rows.Close()

	var buckets []*domain.MetricLatencyBucket
	for rows.Next() {
		b := &domain.MetricLatencyBucket{}
		var res string
		var bucketUnix int64
		if err := rows.Scan(&b.ID, &b.ProjectID, &res, &bucketUnix, &b.Le, &b.Count); err != nil {
			return nil, fmt.Errorf("scan metric latency bucket: %w", err)
		}
		b.Resolution = domain.MetricResolution(res)
		b.BucketStart = time.Unix(bucketUnix, 0).UTC()
		buckets = append(buckets, b)
	}
	return buckets, nil
}

// PruneMetrics deletes every sample/latency-bucket row at the given
// resolution with bucket_start older than cutoff, across every project.
// FEAT-032's retention sweep calls this once per resolution with that
// resolution's own retention window.
func (db *DB) PruneMetrics(resolution domain.MetricResolution, cutoff time.Time) error {
	cutoffUnix := cutoff.UTC().Unix()

	if _, err := db.Exec("DELETE FROM metric_samples WHERE resolution = ? AND bucket_start < ?", string(resolution), cutoffUnix); err != nil {
		return fmt.Errorf("prune metric samples: %w", err)
	}
	if _, err := db.Exec("DELETE FROM metric_latency_buckets WHERE resolution = ? AND bucket_start < ?", string(resolution), cutoffUnix); err != nil {
		return fmt.Errorf("prune metric latency buckets: %w", err)
	}
	return nil
}

// DeleteMetricsByProject deletes every metric_samples/metric_latency_buckets
// row (every resolution) for a single concrete project_id. Unlike
// PruneMetrics (cutoff-based, across every project), this is
// project-scoped, called from ProjectService.Delete's synchronous DB
// cleanup so a deleted project's metric rows don't outlive it - day
// resolution rows in particular have no other retention path and would
// otherwise leak forever (day rows are exempt from PruneMetrics' rolling
// cutoff sweep). Deliberately takes a concrete projectID rather than
// accepting a cross-project scope: callers must not be able to wipe metrics
// belonging to another project.
func (db *DB) DeleteMetricsByProject(projectID int64) error {
	if _, err := db.Exec("DELETE FROM metric_samples WHERE project_id = ?", projectID); err != nil {
		return fmt.Errorf("delete metric samples by project: %w", err)
	}
	if _, err := db.Exec("DELETE FROM metric_latency_buckets WHERE project_id = ?", projectID); err != nil {
		return fmt.Errorf("delete metric latency buckets by project: %w", err)
	}
	return nil
}

// OldestBucketStart returns the earliest bucket_start still present at the
// given resolution, across both metric_samples and metric_latency_buckets
// (whichever table has the older row), or ok=false if there are no rows at
// that resolution at all. FEAT-032's rollup sweep uses this - instead of a
// fixed lookback window - to find how far back it needs to aggregate so
// that no surviving source row is ever pruned without first having had a
// chance to be rolled up into the next coarser resolution, regardless of
// how long a gap there was since the last successful Rollup call.
func (db *DB) OldestBucketStart(resolution domain.MetricResolution) (int64, bool, error) {
	row := db.QueryRow(`
		SELECT MIN(bucket_start) FROM (
			SELECT bucket_start FROM metric_samples WHERE resolution = ?
			UNION ALL
			SELECT bucket_start FROM metric_latency_buckets WHERE resolution = ?
		)
	`, string(resolution), string(resolution))

	var oldest sql.NullInt64
	if err := row.Scan(&oldest); err != nil {
		return 0, false, fmt.Errorf("oldest bucket start (%s): %w", resolution, err)
	}
	if !oldest.Valid {
		return 0, false, nil
	}
	return oldest.Int64, true, nil
}

// DistinctDstBucketStarts returns the distinct dst-resolution bucket starts
// - each srcResolution bucket_start truncated down to the containing
// dstWindow-sized boundary - that have at least one srcResolution row (in
// either metric_samples or metric_latency_buckets) with bucket_start in
// [from, to). FEAT-032's rollup sweep aggregates exactly these buckets
// rather than sweeping every dstWindow-sized step between from and to, so a
// deep gap in the data (long process downtime) doesn't force iterating
// over spans of time that have no source rows at all.
func (db *DB) DistinctDstBucketStarts(resolution domain.MetricResolution, dstWindow time.Duration, from, to time.Time) ([]time.Time, error) {
	windowSeconds := int64(dstWindow.Seconds())
	fromUnix, toUnix := from.UTC().Unix(), to.UTC().Unix()

	rows, err := db.Query(`
		SELECT DISTINCT (bucket_start / ?) * ? AS dst_bucket_start FROM (
			SELECT bucket_start FROM metric_samples WHERE resolution = ? AND bucket_start >= ? AND bucket_start < ?
			UNION
			SELECT bucket_start FROM metric_latency_buckets WHERE resolution = ? AND bucket_start >= ? AND bucket_start < ?
		)
		ORDER BY dst_bucket_start ASC
	`, windowSeconds, windowSeconds, string(resolution), fromUnix, toUnix, string(resolution), fromUnix, toUnix)
	if err != nil {
		return nil, fmt.Errorf("distinct dst bucket starts (%s): %w", resolution, err)
	}
	defer rows.Close()

	var out []time.Time
	for rows.Next() {
		var bucketUnix int64
		if err := rows.Scan(&bucketUnix); err != nil {
			return nil, fmt.Errorf("scan distinct dst bucket start: %w", err)
		}
		out = append(out, time.Unix(bucketUnix, 0).UTC())
	}
	return out, nil
}

// ResolutionWindow returns the interval a single bucket at this resolution
// spans - e.g. one "hour" bucket covers 1 hour of finer-resolution data.
// Exported so callers outside this package (FEAT-032's query/rollup
// services) share this one mapping instead of each keeping their own
// duplicate switch that could silently drift if a new resolution is added.
func ResolutionWindow(res domain.MetricResolution) (time.Duration, error) {
	switch res {
	case domain.MetricResolutionMinute:
		return time.Minute, nil
	case domain.MetricResolutionHour:
		return time.Hour, nil
	case domain.MetricResolutionDay:
		return 24 * time.Hour, nil
	default:
		return 0, fmt.Errorf("unknown metric resolution %q", res)
	}
}

// AggregateMetrics is the minute->hour / hour->day rollup primitive:
// it sums every srcResolution row (across both tables) whose bucket_start
// falls inside [dstBucketStart, dstBucketStart+dstResolution's window) into
// a single dstResolution row per project_id (and, for latency, per
// project_id + le), upserting the result. Idempotent - safe for FEAT-032's
// rollup sweep to re-run for the same dstBucketStart (e.g. after a retry or
// a late-arriving scrape).
//
// srcResolution must be the one directly finer than dstResolution (minute
// for dst=hour, hour for dst=day) - that pairing is a caller contract, not
// enforced here beyond what ResolutionWindow validates.
func (db *DB) AggregateMetrics(srcResolution, dstResolution domain.MetricResolution, dstBucketStart time.Time) error {
	window, err := ResolutionWindow(dstResolution)
	if err != nil {
		return err
	}
	from := dstBucketStart.UTC().Unix()
	to := dstBucketStart.UTC().Add(window).Unix()

	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("begin aggregate metrics: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`
		INSERT INTO metric_samples (project_id, resolution, bucket_start, count_2xx, count_3xx, count_4xx, count_5xx, bytes_in, bytes_out)
		SELECT project_id, ?, ?, SUM(count_2xx), SUM(count_3xx), SUM(count_4xx), SUM(count_5xx), SUM(bytes_in), SUM(bytes_out)
		FROM metric_samples
		WHERE resolution = ? AND bucket_start >= ? AND bucket_start < ?
		GROUP BY project_id
		ON CONFLICT (project_id, resolution, bucket_start) DO UPDATE SET
			count_2xx = excluded.count_2xx,
			count_3xx = excluded.count_3xx,
			count_4xx = excluded.count_4xx,
			count_5xx = excluded.count_5xx,
			bytes_in = excluded.bytes_in,
			bytes_out = excluded.bytes_out
	`, string(dstResolution), from, string(srcResolution), from, to); err != nil {
		return fmt.Errorf("aggregate metric samples: %w", err)
	}

	if _, err := tx.Exec(`
		INSERT INTO metric_latency_buckets (project_id, resolution, bucket_start, le, count)
		SELECT project_id, ?, ?, le, SUM(count)
		FROM metric_latency_buckets
		WHERE resolution = ? AND bucket_start >= ? AND bucket_start < ?
		GROUP BY project_id, le
		ON CONFLICT (project_id, resolution, bucket_start, le) DO UPDATE SET
			count = excluded.count
	`, string(dstResolution), from, string(srcResolution), from, to); err != nil {
		return fmt.Errorf("aggregate metric latency buckets: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit aggregate metrics: %w", err)
	}
	return nil
}
