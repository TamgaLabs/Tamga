package domain

import "time"

// MetricResolution is the rollup granularity a metric row was recorded or
// aggregated at (migration 000017's minute->hour->day rollup dimension).
type MetricResolution string

const (
	MetricResolutionMinute MetricResolution = "minute"
	MetricResolutionHour   MetricResolution = "hour"
	MetricResolutionDay    MetricResolution = "day"
)

// GlobalProjectID is the project_id used for Tamga's own core/global
// traffic scope (not a real project row) rather than one specific project.
const GlobalProjectID int64 = 0

// MetricSample is one (project_id, resolution, bucket_start) row of Traefik
// traffic increments: request counts by status class and request/response
// byte counts. Every counter field is the DELTA observed during
// [BucketStart, BucketStart+resolution's interval) - not a cumulative
// total. See migration 000017's header for why (rate/rollup become plain
// sums), and metrics_repo.go for the exact write/read seam FEAT-031/032
// share.
type MetricSample struct {
	ID          int64
	ProjectID   int64
	Resolution  MetricResolution
	BucketStart time.Time
	Count2xx    int64
	Count3xx    int64
	Count4xx    int64
	Count5xx    int64
	BytesIn     int64
	BytesOut    int64
}

// MetricLatencyBucket is one Traefik histogram bucket's per-interval
// increment for one (project_id, resolution, bucket_start): how many
// additional requests fell at-or-under Le seconds during this interval.
// Cumulative across Le within one BucketStart, mirroring Prometheus's own
// histogram_bucket semantics, so percentiles are computed at query time
// the same way PromQL's histogram_quantile does.
type MetricLatencyBucket struct {
	ID          int64
	ProjectID   int64
	Resolution  MetricResolution
	BucketStart time.Time
	Le          string // Traefik/Prometheus `le` label verbatim: "0.1", "0.3", "1.2", "5", "+Inf"
	Count       int64
}
