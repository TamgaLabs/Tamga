package service

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"math"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/TamgaLabs/Tamga/backend/internal/domain"
	"github.com/TamgaLabs/Tamga/backend/internal/repository/sqlite"
)

// MetricsScraperService periodically scrapes Traefik's Prometheus metrics
// endpoint, computes per-interval increments from Traefik's cumulative
// counters, and writes minute-resolution samples via FEAT-030's
// metrics_repo.go (domain.MetricSample / domain.MetricLatencyBucket - see
// that task's Implementation Notes for the exact row shape this writes).
//
// It follows the same "goroutine started at construction, runs for the
// process lifetime" pattern as AgentService.startIdleSweep (FEAT-022) -
// there is no graceful-shutdown plumbing for services in cmd/api/main.go,
// so a plain unstoppable goroutine is the KISS choice here too.
type MetricsScraperService struct {
	db     *sqlite.DB
	client *http.Client
	url    string
	period time.Duration

	mu   sync.Mutex
	prev *scrapeState // nil until the first successful scrape (baseline)
}

// NewMetricsScraperService constructs the scraper and starts its background
// scrape loop. url/period come from config.Config.TraefikMetricsURL/Period
// (TRAEFIK_METRICS_URL / TRAEFIK_METRICS_INTERVAL_SECONDS).
func NewMetricsScraperService(db *sqlite.DB, url string, period time.Duration) *MetricsScraperService {
	s := &MetricsScraperService{
		db:     db,
		client: &http.Client{Timeout: 10 * time.Second},
		url:    url,
		period: period,
	}
	s.startScrapeLoop()
	return s
}

// startScrapeLoop launches the long-lived background goroutine that scrapes
// Traefik's metrics endpoint every s.period. A non-positive period disables
// the loop entirely (e.g. a misconfigured/zero-value Config in a test)
// rather than busy-looping.
func (s *MetricsScraperService) startScrapeLoop() {
	if s.period <= 0 {
		return
	}
	go func() {
		ticker := time.NewTicker(s.period)
		defer ticker.Stop()
		for range ticker.C {
			s.tick(time.Now())
		}
	}()
}

// tick performs one scrape: fetch, parse, diff against the previous scrape,
// and write the resulting increments. Any failure (fetch or parse) is
// logged and the tick is skipped entirely - no partial writes, no crash,
// and no corruption of s.prev (a failed tick never updates the baseline,
// so the next successful tick's increment still spans the full elapsed
// interval since the last successful scrape).
func (s *MetricsScraperService) tick(now time.Time) {
	body, err := s.scrape(context.Background())
	if err != nil {
		slog.Warn("metrics scrape: fetch failed", "url", s.url, "error", err)
		return
	}

	cur := parseTraefikMetrics(body)
	bucketStart := now.UTC().Truncate(time.Minute)

	samples, buckets := s.ingest(cur, bucketStart)
	if samples == nil && buckets == nil {
		// First tick after (re)start - no previous scrape to diff against,
		// so this tick only establishes the baseline. Nothing to store.
		return
	}

	if err := s.db.InsertMetricSamples(samples); err != nil {
		slog.Warn("metrics scrape: insert samples failed", "error", err)
	}
	if err := s.db.InsertMetricLatencyBuckets(buckets); err != nil {
		slog.Warn("metrics scrape: insert latency buckets failed", "error", err)
	}
}

// ingest swaps in the newly-parsed scrape state as the new baseline and
// returns the increments since the previous one (nil, nil on the first
// call - see tick). Kept separate from tick's HTTP fetch so the
// baseline/diff bookkeeping is a pure, directly-testable operation on a
// bare &MetricsScraperService{} with no HTTP or DB involved.
func (s *MetricsScraperService) ingest(cur *scrapeState, bucketStart time.Time) ([]*domain.MetricSample, []*domain.MetricLatencyBucket) {
	s.mu.Lock()
	prev := s.prev
	s.prev = cur
	s.mu.Unlock()

	if prev == nil {
		return nil, nil
	}
	return computeIncrements(prev, cur, bucketStart)
}

// scrape fetches the raw Prometheus exposition text from s.url.
func (s *MetricsScraperService) scrape(ctx context.Context) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.url, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}
	return body, nil
}

// --- Pure parsing + increment computation (unit-tested without HTTP/DB) ---

// countKey identifies one status-class series, already folded/aggregated
// to the granularity metric_samples stores at (project + class) - see
// resolveProjectID and statusClass.
type countKey struct {
	projectID int64
	class     string // "2xx", "3xx", "4xx", "5xx"
}

// latencyKey identifies one histogram bucket series, aggregated to the
// granularity metric_latency_buckets stores at (project + le).
type latencyKey struct {
	projectID int64
	le        string
}

// scrapeState is one scrape's parsed, per-project-aggregated CUMULATIVE
// values - Traefik's raw counters summed across method/code (bytes) or
// summed by status class (counts), keyed at the exact granularity
// FEAT-030's tables store. Diffing two scrapeStates (computeIncrements)
// yields the per-interval increment each table row needs.
type scrapeState struct {
	counts   map[countKey]float64
	latency  map[latencyKey]float64
	bytesIn  map[int64]float64
	bytesOut map[int64]float64
}

func newScrapeState() *scrapeState {
	return &scrapeState{
		counts:   make(map[countKey]float64),
		latency:  make(map[latencyKey]float64),
		bytesIn:  make(map[int64]float64),
		bytesOut: make(map[int64]float64),
	}
}

// promLineRe matches one Prometheus text-exposition sample line with
// labels: `metric_name{label="value",...} numeric_value`. Every family
// this scraper cares about (traefik_router_requests_total,
// traefik_router_request_duration_seconds_bucket,
// traefik_router_requests_bytes_total,
// traefik_router_responses_bytes_total) always carries labels, so a bare
// `metric_name value` form (no braces) is deliberately not matched/needed.
var promLineRe = regexp.MustCompile(`^(\w+)\{(.*)\}\s+(\S+)$`)

// promLabelRe extracts one `key="value"` label pair at a time.
var promLabelRe = regexp.MustCompile(`(\w+)="((?:[^"\\]|\\.)*)"`)

// parseTraefikMetrics parses the Prometheus text exposition format,
// extracting only the four traefik_router_* families TEST-010 §4
// confirmed (everything else - go_*, process_*, traefik_entrypoint_*,
// traefik_service_*, HELP/TYPE comments, etc. - is ignored). A hand parser
// rather than github.com/prometheus/common/expfmt: the subset of the
// format actually needed (four counter/histogram families, no exemplars,
// no NaN/staleness markers) is simple enough that expfmt's large
// dependency tree (protobuf, common/model, ...) isn't worth pulling in for
// it - see FEAT-031's Proposed Solution for the full justification.
//
// Router/service names carry Traefik's `@file`/`@docker` provider suffix
// (e.g. "seal-42@file") - stripped via stripProviderSuffix before
// mapping to a project ID via resolveProjectID. Unparseable lines (labels
// that don't match, non-numeric values) are skipped, not fatal - one
// malformed line must not lose the whole scrape.
func parseTraefikMetrics(data []byte) *scrapeState {
	state := newScrapeState()

	scanner := bufio.NewScanner(bytes.NewReader(data))
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if !strings.HasPrefix(line, "traefik_router_") {
			continue
		}

		m := promLineRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		name, labelStr, valStr := m[1], m[2], m[3]

		val, err := strconv.ParseFloat(valStr, 64)
		if err != nil {
			continue
		}

		labels := parsePromLabels(labelStr)
		router := stripProviderSuffix(labels["router"])
		projectID := resolveProjectID(router)

		switch name {
		case "traefik_router_requests_total":
			class, ok := statusClass(labels["code"])
			if !ok {
				continue
			}
			state.counts[countKey{projectID, class}] += val
		case "traefik_router_request_duration_seconds_bucket":
			le := labels["le"]
			if le == "" {
				continue
			}
			state.latency[latencyKey{projectID, le}] += val
		case "traefik_router_requests_bytes_total":
			state.bytesIn[projectID] += val
		case "traefik_router_responses_bytes_total":
			state.bytesOut[projectID] += val
		}
	}

	return state
}

// parsePromLabels parses a Prometheus label-list body (the text between
// `{` and `}`) into a key->value map.
func parsePromLabels(s string) map[string]string {
	labels := make(map[string]string)
	for _, m := range promLabelRe.FindAllStringSubmatch(s, -1) {
		labels[m[1]] = m[2]
	}
	return labels
}

// stripProviderSuffix removes Traefik's file/docker provider suffix
// (e.g. "seal-42@file" -> "seal-42", "tamga-ui@file" -> "tamga-ui")
// per TEST-010 §4.
func stripProviderSuffix(name string) string {
	if idx := strings.IndexByte(name, '@'); idx >= 0 {
		return name[:idx]
	}
	return name
}

// resolveProjectID maps a bare (provider-suffix-stripped) router name to
// the project_id its samples belong to: "seal-<id>" routers map to
// that project (fmt.Sscanf's "%d" verb stops at the first non-digit,
// mirroring backend/internal/repository/docker/client.go's
// containerProjectInfo); every other router - the tamga.yml core UI/API
// routers (tamga-ui, tamga-ui-secure, tamga-api, tamga-api-secure) and any
// future non-project router - is Tamga's own core/global traffic scope
// (domain.GlobalProjectID).
func resolveProjectID(bareRouterName string) int64 {
	if strings.HasPrefix(bareRouterName, "seal-") {
		var id int64
		fmt.Sscanf(bareRouterName, "seal-%d", &id)
		return id
	}
	return domain.GlobalProjectID
}

// statusClass folds an HTTP status code into its 2xx/3xx/4xx/5xx class.
// Codes outside 2xx-5xx (e.g. a stray 1xx) return ok=false and are
// ignored - metric_samples has no column for them.
func statusClass(code string) (class string, ok bool) {
	if len(code) == 0 {
		return "", false
	}
	switch code[0] {
	case '2':
		return "2xx", true
	case '3':
		return "3xx", true
	case '4':
		return "4xx", true
	case '5':
		return "5xx", true
	default:
		return "", false
	}
}

// diffCounter computes the per-interval increment of one cumulative
// Prometheus counter, handling a reset (Traefik restart -> counter drops
// back to/near 0): standard Prometheus reset handling is "if current <
// previous, treat current itself as the increment" - never negative.
func diffCounter(prev, cur float64) int64 {
	d := cur - prev
	if d < 0 {
		d = cur
	}
	if d < 0 {
		// cur itself is negative - shouldn't happen for a Prometheus
		// counter, but never store a negative increment.
		d = 0
	}
	return int64(math.Round(d))
}

// computeIncrements diffs two consecutive scrapeStates into the batch of
// domain.MetricSample/domain.MetricLatencyBucket rows for one tick, all at
// domain.MetricResolutionMinute and the given bucketStart (the tick's
// minute, per FEAT-030's seam). Every project_id present in either scrape
// (cur or prev) gets exactly one MetricSample row; every (project_id, le)
// pair present in either scrape gets exactly one MetricLatencyBucket row -
// so a series that disappeared after a restart (see diffCounter) still
// produces a row (with its increment correctly computed against an
// implicit 0 current value).
func computeIncrements(prev, cur *scrapeState, bucketStart time.Time) ([]*domain.MetricSample, []*domain.MetricLatencyBucket) {
	type projectAgg struct {
		count2xx, count3xx, count4xx, count5xx int64
		bytesIn, bytesOut                      int64
	}
	projects := make(map[int64]*projectAgg)
	agg := func(id int64) *projectAgg {
		a, ok := projects[id]
		if !ok {
			a = &projectAgg{}
			projects[id] = a
		}
		return a
	}

	countKeys := make(map[countKey]struct{})
	for k := range cur.counts {
		countKeys[k] = struct{}{}
	}
	for k := range prev.counts {
		countKeys[k] = struct{}{}
	}
	for k := range countKeys {
		delta := diffCounter(prev.counts[k], cur.counts[k])
		a := agg(k.projectID)
		switch k.class {
		case "2xx":
			a.count2xx += delta
		case "3xx":
			a.count3xx += delta
		case "4xx":
			a.count4xx += delta
		case "5xx":
			a.count5xx += delta
		}
	}

	projectIDs := make(map[int64]struct{})
	for id := range cur.bytesIn {
		projectIDs[id] = struct{}{}
	}
	for id := range prev.bytesIn {
		projectIDs[id] = struct{}{}
	}
	for id := range cur.bytesOut {
		projectIDs[id] = struct{}{}
	}
	for id := range prev.bytesOut {
		projectIDs[id] = struct{}{}
	}
	for id := range projectIDs {
		a := agg(id)
		a.bytesIn += diffCounter(prev.bytesIn[id], cur.bytesIn[id])
		a.bytesOut += diffCounter(prev.bytesOut[id], cur.bytesOut[id])
	}

	// Any project that only appears via counts (no byte lines at all,
	// unusual but not impossible) already has an agg entry from the counts
	// loop above; the reverse (bytes but no counts) is handled by the
	// projectIDs loop's agg() call. Every project_id from either source
	// ends up with exactly one row.
	samples := make([]*domain.MetricSample, 0, len(projects))
	for pid, a := range projects {
		samples = append(samples, &domain.MetricSample{
			ProjectID:   pid,
			Resolution:  domain.MetricResolutionMinute,
			BucketStart: bucketStart,
			Count2xx:    a.count2xx,
			Count3xx:    a.count3xx,
			Count4xx:    a.count4xx,
			Count5xx:    a.count5xx,
			BytesIn:     a.bytesIn,
			BytesOut:    a.bytesOut,
		})
	}
	sort.Slice(samples, func(i, j int) bool { return samples[i].ProjectID < samples[j].ProjectID })

	latencyKeys := make(map[latencyKey]struct{})
	for k := range cur.latency {
		latencyKeys[k] = struct{}{}
	}
	for k := range prev.latency {
		latencyKeys[k] = struct{}{}
	}
	buckets := make([]*domain.MetricLatencyBucket, 0, len(latencyKeys))
	for k := range latencyKeys {
		buckets = append(buckets, &domain.MetricLatencyBucket{
			ProjectID:   k.projectID,
			Resolution:  domain.MetricResolutionMinute,
			BucketStart: bucketStart,
			Le:          k.le,
			Count:       diffCounter(prev.latency[k], cur.latency[k]),
		})
	}
	sort.Slice(buckets, func(i, j int) bool {
		if buckets[i].ProjectID != buckets[j].ProjectID {
			return buckets[i].ProjectID < buckets[j].ProjectID
		}
		return buckets[i].Le < buckets[j].Le
	})

	return samples, buckets
}
