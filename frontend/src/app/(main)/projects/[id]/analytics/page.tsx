"use client";

import React, { useEffect, useState } from "react";
import { useProjectContext } from "../project-context";
import { useProjectMetrics } from "@/hooks/useMetrics";
import type { MetricsQueryParams, MetricResolution } from "@/lib/api";
import {
  RequestRatePanel,
  StatusErrorPanel,
  LatencyPanel,
  BandwidthPanel,
  TimeRangeSelector,
  ResolutionSelector,
  type TimeRange,
  type ResolutionOption,
} from "@/components/analytics";

export default function ProjectAnalyticsPage() {
  const { project } = useProjectContext();
  const projectId = project.id;

  // Time range state
  const [timeRange, setTimeRange] = useState<TimeRange>("24h");
  const [fromTimestamp, setFromTimestamp] = useState<number>(0);
  const [toTimestamp, setToTimestamp] = useState<number>(0);

  // Resolution state
  const [resolution, setResolution] = useState<ResolutionOption>("auto");

  // Initialize timestamps on mount
  useEffect(() => {
    const now = Math.floor(Date.now() / 1000);
    // Default to last 24 hours
    setFromTimestamp(now - 86400);
    setToTimestamp(now);
  }, []);

  // Build query params for the API
  const queryParams: MetricsQueryParams = {
    from: fromTimestamp,
    to: toTimestamp,
    // Only include resolution if not "auto"
    ...(resolution !== "auto" && { resolution: resolution as MetricResolution }),
  };

  // Fetch metrics with polling
  const { data: metrics, loading, error } = useProjectMetrics(
    projectId,
    queryParams,
    {
      enabled: fromTimestamp > 0,
      refetchInterval: 60000, // Poll every 60 seconds
    }
  );

  const handleTimeRangeChange = (range: TimeRange, from: number, to: number) => {
    setTimeRange(range);
    setFromTimestamp(from);
    setToTimestamp(to);
  };

  const handleResolutionChange = (res: ResolutionOption) => {
    setResolution(res);
  };

  return (
    <div className="p-6 max-w-7xl mx-auto">
      {/* Header */}
      <div className="mb-8">
        <h1 className="text-2xl font-bold mb-4">Analytics</h1>

        {/* Controls */}
        <div className="flex flex-col gap-4">
          <div>
            <p className="text-sm text-muted-foreground mb-2">Time Range</p>
            <TimeRangeSelector value={timeRange} onChange={handleTimeRangeChange} />
          </div>
          <div>
            <p className="text-sm text-muted-foreground mb-2">Resolution</p>
            <ResolutionSelector value={resolution} onChange={handleResolutionChange} />
          </div>
        </div>
      </div>

      {/* Error state */}
      {error && (
        <div className="mb-6 p-4 bg-destructive/10 border border-destructive/30 rounded-lg text-sm text-destructive">
          Failed to load metrics: {error.message}
        </div>
      )}

      {/* Loading state */}
      {loading && !metrics && (
        <div className="text-center py-20 text-muted-foreground">
          <p className="text-lg">Loading analytics...</p>
        </div>
      )}

      {/* Empty state */}
      {!loading && metrics && metrics.request_rate.length === 0 && (
        <div className="text-center py-20 text-muted-foreground">
          <p className="text-lg mb-2">No metrics available</p>
          <p className="text-sm">Check back once this project has processed some traffic</p>
        </div>
      )}

      {/* Panels grid */}
      {metrics && (
        <div className="grid gap-6 grid-cols-1 lg:grid-cols-2">
          <RequestRatePanel
            data={metrics.request_rate}
            resolution={metrics.resolution}
            isLoading={loading}
          />
          <StatusErrorPanel
            data={metrics.status_class}
            resolution={metrics.resolution}
            isLoading={loading}
          />
          <LatencyPanel
            data={metrics.latency}
            resolution={metrics.resolution}
            isLoading={loading}
          />
          <BandwidthPanel
            data={metrics.bandwidth}
            resolution={metrics.resolution}
            isLoading={loading}
          />
        </div>
      )}
    </div>
  );
}
