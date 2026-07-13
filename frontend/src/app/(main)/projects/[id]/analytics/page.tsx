"use client";

import React, { useEffect, useState } from "react";
import { useProjectContext } from "../project-context";
import { useProjectMetrics } from "@/hooks/useMetrics";
import type { MetricsQueryParams, MetricResolution } from "@/lib/api";
import { PageHeader, PageHeaderDescription, PageHeaderTitle } from "@/components/page-header";
import { Badge } from "@/components/ui/badge";
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
import { Card, CardContent } from "@/components/ui/card";
import { Empty, EmptyDescription, EmptyHeader, EmptyMedia, EmptyTitle } from "@/components/ui/empty";
import { Skeleton } from "@/components/ui/skeleton";
import { AlertCircle, BarChart3, Clock3 } from "lucide-react";

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
    <div className="mx-auto max-w-7xl p-4 sm:p-6">
      <PageHeader className="mb-6">
        <div><PageHeaderTitle>Analytics</PageHeaderTitle><PageHeaderDescription>Traffic and service performance for this project.</PageHeaderDescription></div>
        <Badge variant="secondary" className="gap-1.5"><Clock3 className="size-3" />Refreshes every minute</Badge>
      </PageHeader>
      <Card className="mb-6">

        <CardContent className="grid gap-5 p-4 sm:grid-cols-[minmax(0,1fr)_minmax(0,1fr)] sm:p-5">
          <div className="space-y-2">
            <p className="text-sm font-medium">Time range</p>
            <TimeRangeSelector value={timeRange} onChange={handleTimeRangeChange} />
          </div>
          <div className="space-y-2">
            <p className="text-sm font-medium">Resolution</p>
            <ResolutionSelector value={resolution} onChange={handleResolutionChange} />
          </div>
        </CardContent>
      </Card>

      {/* Error state */}
      {error && (
        <div role="alert" className="mb-6 flex gap-3 rounded-xl border border-destructive/30 bg-destructive/10 p-4 text-sm text-destructive">
          <AlertCircle className="mt-0.5 size-4 shrink-0" /><div><p className="font-medium">Metrics are unavailable</p><p className="mt-1 text-destructive/80">{error.message}</p></div>
        </div>
      )}

      {/* Loading state */}
      {loading && !metrics && (
        <div className="grid gap-6 lg:grid-cols-2" aria-label="Loading analytics">{[0, 1, 2, 3].map((item) => <Skeleton key={item} className="h-[25rem] w-full" />)}</div>
      )}

      {/* Empty state */}
      {!loading && metrics && metrics.request_rate.length === 0 && (
        <Empty className="min-h-72"><EmptyHeader><EmptyMedia><BarChart3 className="size-5" /></EmptyMedia><EmptyTitle>No metrics available yet</EmptyTitle><EmptyDescription>Check back once this project has processed some traffic.</EmptyDescription></EmptyHeader></Empty>
      )}

      {/* Panels grid */}
      {metrics && (
        <div className="grid grid-cols-1 gap-6 lg:grid-cols-2">
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
