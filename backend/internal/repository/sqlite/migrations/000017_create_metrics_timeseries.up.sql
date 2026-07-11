-- Time-series storage for Traefik traffic analytics (FEAT-030). Two tables,
-- not one generic "long" samples table: the four panels (request rate,
-- status-class/error-rate, latency p50/p95/p99, bandwidth) decompose into
-- two distinct shapes -- fixed per-interval counters (status classes +
-- bytes) fit naturally as columns on one row per (project_id, resolution,
-- bucket_start), while the latency histogram has a variable number of `le`
-- boundaries, so it gets its own narrow table keyed additionally by `le`.
-- Both tables share the same key prefix (project_id, resolution,
-- bucket_start) -- that shared key is the seam FEAT-031 (scraper, writer)
-- and FEAT-032 (query API, reader) coordinate on; see FEAT-030's
-- Implementation Notes for the exact row shape each side produces/consumes.
--
-- IMPORTANT: every counter/count column stores the per-interval INCREMENT
-- (delta) observed since the previous scrape of Traefik's cumulative
-- Prometheus counters -- never the raw cumulative value. This means:
--   - request rate for a range = SUM(count_2xx+count_3xx+count_4xx+count_5xx)
--     over the rows in that range; no diffing against a prior value needed.
--   - bandwidth for a range = SUM(bytes_in), SUM(bytes_out) over the range.
--   - the minute->hour / hour->day rollup is a plain SUM(...) GROUP BY
--     project_id (and, for latency, `le`) of the finer-resolution rows
--     whose bucket_start falls inside the coarser bucket's time window --
--     see DB.AggregateMetrics in repository/sqlite/metrics_repo.go.
--
-- project_id = 0 is not a real project row -- it is the Tamga core/global
-- scope (all-projects-combined / Tamga's own traffic), so there is
-- deliberately no FOREIGN KEY on project_id here (0 would never satisfy one).
--
-- bucket_start is stored as an INTEGER unix-second timestamp (the start of
-- the interval, aligned to the resolution -- e.g. :00 seconds for a minute
-- bucket), not a SQLite DATETIME/TEXT column: it's a value the scraper/
-- rollup sweep computes and compares as a plain integer range
-- (bucket_start >= ? AND bucket_start < ?), so an epoch integer keeps that
-- arithmetic and the writer/reader contract unambiguous.
--
-- Indexing: the UNIQUE constraint on (project_id, resolution, bucket_start)
-- below already is the (project_id, resolution, bucket_start) range index
-- these tables are queried by (SQLite uses a UNIQUE constraint's implicit
-- index for range scans on its leading columns) -- no separate CREATE INDEX
-- is added on top of it.

CREATE TABLE IF NOT EXISTS metric_samples (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    project_id INTEGER NOT NULL DEFAULT 0,
    resolution TEXT NOT NULL CHECK (resolution IN ('minute', 'hour', 'day')),
    bucket_start INTEGER NOT NULL,
    count_2xx INTEGER NOT NULL DEFAULT 0,
    count_3xx INTEGER NOT NULL DEFAULT 0,
    count_4xx INTEGER NOT NULL DEFAULT 0,
    count_5xx INTEGER NOT NULL DEFAULT 0,
    bytes_in INTEGER NOT NULL DEFAULT 0,
    bytes_out INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (project_id, resolution, bucket_start)
);

-- One row per (project_id, resolution, bucket_start, le): the per-interval
-- increment of Traefik's cumulative histogram bucket count for that `le`
-- boundary (Prometheus histogram_bucket semantics -- cumulative across `le`
-- within one bucket_start, so p50/p95/p99 are computed at query time the
-- same way PromQL's histogram_quantile does). `le` is stored as TEXT to
-- preserve Traefik's own label values verbatim (e.g. "0.1", "0.3", "1.2",
-- "5", "+Inf").
CREATE TABLE IF NOT EXISTS metric_latency_buckets (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    project_id INTEGER NOT NULL DEFAULT 0,
    resolution TEXT NOT NULL CHECK (resolution IN ('minute', 'hour', 'day')),
    bucket_start INTEGER NOT NULL,
    le TEXT NOT NULL,
    count INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (project_id, resolution, bucket_start, le)
);
