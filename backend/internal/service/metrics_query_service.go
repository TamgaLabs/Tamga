package service

import (
	"fmt"
	"time"

	"github.com/TamgaLabs/Tamga/backend/internal/domain"
	"github.com/TamgaLabs/Tamga/backend/internal/repository/sqlite"
)

// autoResolutionMinuteMax/autoResolutionHourMax are AutoResolution's
// thresholds, per the task's Requirements: <48h -> minute, <30d -> hour,
// else day.
const (
	autoResolutionMinuteMax = 48 * time.Hour
	autoResolutionHourMax   = 30 * 24 * time.Hour
)

// AutoResolution picks the bucket resolution FEAT-032's query API uses when
// a request doesn't specify one, based on the requested range's span - a
// coarser resolution keeps a wide range's response small and its buckets
// visually reasonable to plot.
func AutoResolution(from, to time.Time) domain.MetricResolution {
	span := to.Sub(from)
	switch {
	case span <= autoResolutionMinuteMax:
		return domain.MetricResolutionMinute
	case span <= autoResolutionHourMax:
		return domain.MetricResolutionHour
	default:
		return domain.MetricResolutionDay
	}
}

// MetricsQueryService is the read side of FEAT-030/031/032's metrics
// pipeline: it turns stored samples/latency buckets into the panel-oriented
// shape the HTTP handler (and eventually C4's UI) consumes.
type MetricsQueryService struct {
	db *sqlite.DB
}

func NewMetricsQueryService(db *sqlite.DB) *MetricsQueryService {
	return &MetricsQueryService{db: db}
}

// RequestRatePoint is one bucket's request-count panel point.
type RequestRatePoint struct {
	BucketStart time.Time `json:"bucket_start"`
	Count       int64     `json:"count"`
	RatePerSec  float64   `json:"rate_per_sec"`
}

// StatusClassPoint is one bucket's status-class breakdown + error-rate
// panel point. ErrorRate is (4xx+5xx)/total, 0 when the bucket has no
// requests at all.
type StatusClassPoint struct {
	BucketStart time.Time `json:"bucket_start"`
	Count2xx    int64     `json:"count_2xx"`
	Count3xx    int64     `json:"count_3xx"`
	Count4xx    int64     `json:"count_4xx"`
	Count5xx    int64     `json:"count_5xx"`
	ErrorRate   float64   `json:"error_rate"`
}

// LatencyPoint is one bucket's p50/p95/p99 panel point, computed from that
// bucket_start's stored histogram buckets via PercentilesFromLatencyBuckets.
// Zero when the bucket has no latency data at all.
type LatencyPoint struct {
	BucketStart time.Time `json:"bucket_start"`
	P50         float64   `json:"p50"`
	P95         float64   `json:"p95"`
	P99         float64   `json:"p99"`
}

// BandwidthPoint is one bucket's request/response byte-count panel point.
type BandwidthPoint struct {
	BucketStart time.Time `json:"bucket_start"`
	BytesIn     int64     `json:"bytes_in"`
	BytesOut    int64     `json:"bytes_out"`
}

// MetricsPanels is the query API's full response shape: the four panels
// (request rate, status-class+error-rate, latency, bandwidth), each a
// sparse array of one point per bucket_start that actually has data - no
// data in range means an empty (never nil) array in every panel, not an
// error.
type MetricsPanels struct {
	ProjectID   int64                   `json:"project_id"`
	From        time.Time               `json:"from"`
	To          time.Time               `json:"to"`
	Resolution  domain.MetricResolution `json:"resolution"`
	RequestRate []RequestRatePoint      `json:"request_rate"`
	StatusClass []StatusClassPoint      `json:"status_class"`
	Latency     []LatencyPoint          `json:"latency"`
	Bandwidth   []BandwidthPoint        `json:"bandwidth"`
}

// GetPanels builds the four panels for one project (or domain.GlobalProjectID
// for the system-wide scope) over [from, to) at resolution - resolution ""
// auto-picks via AutoResolution. A project/scope with no rows in range
// returns MetricsPanels with empty (not nil) panel arrays, not an error.
func (s *MetricsQueryService) GetPanels(projectID int64, from, to time.Time, resolution domain.MetricResolution) (*MetricsPanels, error) {
	if !to.After(from) {
		return nil, fmt.Errorf("to must be after from")
	}
	if resolution == "" {
		resolution = AutoResolution(from, to)
	}
	window, err := sqlite.ResolutionWindow(resolution)
	if err != nil {
		return nil, err
	}

	samples, err := s.db.ListMetricSamples(projectID, resolution, from, to)
	if err != nil {
		return nil, fmt.Errorf("list metric samples: %w", err)
	}
	latencyBuckets, err := s.db.ListMetricLatencyBuckets(projectID, resolution, from, to)
	if err != nil {
		return nil, fmt.Errorf("list metric latency buckets: %w", err)
	}

	latencyByBucket := make(map[int64][]*domain.MetricLatencyBucket)
	for _, b := range latencyBuckets {
		key := b.BucketStart.Unix()
		latencyByBucket[key] = append(latencyByBucket[key], b)
	}

	panels := &MetricsPanels{
		ProjectID:   projectID,
		From:        from.UTC(),
		To:          to.UTC(),
		Resolution:  resolution,
		RequestRate: []RequestRatePoint{},
		StatusClass: []StatusClassPoint{},
		Latency:     []LatencyPoint{},
		Bandwidth:   []BandwidthPoint{},
	}

	for _, sm := range samples {
		total := sm.Count2xx + sm.Count3xx + sm.Count4xx + sm.Count5xx

		panels.RequestRate = append(panels.RequestRate, RequestRatePoint{
			BucketStart: sm.BucketStart,
			Count:       total,
			RatePerSec:  float64(total) / window.Seconds(),
		})

		var errorRate float64
		if total > 0 {
			errorRate = float64(sm.Count4xx+sm.Count5xx) / float64(total)
		}
		panels.StatusClass = append(panels.StatusClass, StatusClassPoint{
			BucketStart: sm.BucketStart,
			Count2xx:    sm.Count2xx,
			Count3xx:    sm.Count3xx,
			Count4xx:    sm.Count4xx,
			Count5xx:    sm.Count5xx,
			ErrorRate:   errorRate,
		})

		p50, p95, p99 := PercentilesFromLatencyBuckets(latencyByBucket[sm.BucketStart.Unix()])
		panels.Latency = append(panels.Latency, LatencyPoint{
			BucketStart: sm.BucketStart,
			P50:         p50,
			P95:         p95,
			P99:         p99,
		})

		panels.Bandwidth = append(panels.Bandwidth, BandwidthPoint{
			BucketStart: sm.BucketStart,
			BytesIn:     sm.BytesIn,
			BytesOut:    sm.BytesOut,
		})
	}

	return panels, nil
}
