// Types matching backend/internal/service/metrics_query_service.go

export type MetricResolution = "minute" | "hour" | "day";

export interface RequestRatePoint {
  bucket_start: string; // ISO 8601 timestamp
  count: number;
  rate_per_sec: number;
}

export interface StatusClassPoint {
  bucket_start: string; // ISO 8601 timestamp
  count_2xx: number;
  count_3xx: number;
  count_4xx: number;
  count_5xx: number;
  error_rate: number; // 0-1
}

export interface LatencyPoint {
  bucket_start: string; // ISO 8601 timestamp
  p50: number; // seconds
  p95: number; // seconds
  p99: number; // seconds
}

export interface BandwidthPoint {
  bucket_start: string; // ISO 8601 timestamp
  bytes_in: number;
  bytes_out: number;
}

export interface MetricsPanels {
  project_id: number;
  from: string; // ISO 8601 timestamp
  to: string; // ISO 8601 timestamp
  resolution: MetricResolution;
  request_rate: RequestRatePoint[];
  status_class: StatusClassPoint[];
  latency: LatencyPoint[];
  bandwidth: BandwidthPoint[];
}

export interface MetricsQueryParams {
  from?: number; // Unix timestamp (seconds)
  to?: number; // Unix timestamp (seconds)
  resolution?: MetricResolution;
}
