"use client";

import React from "react";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { AlertCircle } from "lucide-react";
import type { StatusClassPoint, MetricResolution } from "@/lib/metrics-types";
import { formatAxisTime, getMaxValue, clamp, formatNumber, formatPercent, parseTimestamp } from "./utils";

interface StatusErrorPanelProps {
  data: StatusClassPoint[];
  resolution: MetricResolution;
  isLoading?: boolean;
}

export function StatusErrorPanel({
  data,
  resolution,
  isLoading = false,
}: StatusErrorPanelProps) {
  // Dimensions for SVG chart
  const width = 400;
  const height = 200;
  const padding = { top: 16, right: 16, bottom: 32, left: 40 };
  const chartWidth = width - padding.left - padding.right;
  const chartHeight = height - padding.top - padding.bottom;

  // Calculate totals for each bucket
  const bucketTotals = data.map((p) => p.count_2xx + p.count_3xx + p.count_4xx + p.count_5xx);
  const maxTotal = getMaxValue(bucketTotals);

  // Calculate scale functions
  const getX = (index: number) => {
    if (data.length <= 1) return padding.left + chartWidth / 2;
    return padding.left + (index / (data.length - 1)) * chartWidth;
  };

  const getY = (value: number) => {
    return padding.top + chartHeight - (clamp(value, 0, maxTotal) / maxTotal) * chartHeight;
  };

  // Generate stacked area paths
  const getStackedPath = (
    countSelector: (p: StatusClassPoint) => number,
    previousStack: number[] = []
  ): { path: string; stack: number[] } => {
    const newStack = [];
    let pathD = "";

    for (let i = 0; i < data.length; i++) {
      const currentValue = countSelector(data[i]);
      const prevValue = previousStack[i] || 0;
      const stackedValue = prevValue + currentValue;
      newStack.push(stackedValue);

      const x = getX(i);
      if (i === 0) {
        pathD = `M ${x} ${getY(stackedValue)}`;
      } else {
        pathD += ` L ${x} ${getY(stackedValue)}`;
      }
    }

    // Close path on return
    for (let i = data.length - 1; i >= 0; i--) {
      const prevValue = previousStack[i] || 0;
      pathD += ` L ${getX(i)} ${getY(prevValue)}`;
    }
    pathD += " Z";

    return { path: pathD, stack: newStack };
  };

  // Build stacked paths in order: 2xx (green), 3xx (blue), 4xx (yellow), 5xx (red)
  let stack: number[] = [];
  const paths2xx = getStackedPath((p) => p.count_2xx, stack);
  stack = paths2xx.stack;
  const paths3xx = getStackedPath((p) => p.count_3xx, stack);
  stack = paths3xx.stack;
  const paths4xx = getStackedPath((p) => p.count_4xx, stack);
  stack = paths4xx.stack;
  const paths5xx = getStackedPath((p) => p.count_5xx, stack);

  // Generate grid lines
  const gridLines = [];
  for (let i = 0; i <= 4; i++) {
    const y = padding.top + (i / 4) * chartHeight;
    const value = maxTotal - (i / 4) * maxTotal;
    gridLines.push({ y, value });
  }

  // Sample x-axis labels
  const labelStep = Math.max(1, Math.floor(data.length / 5));
  const xLabels = [];
  for (let i = 0; i < data.length; i += labelStep) {
    xLabels.push({
      index: i,
      x: getX(i),
      label: formatAxisTime(parseTimestamp(data[i].bucket_start), resolution),
    });
  }

  // Calculate overall stats
  const isEmpty = data.length === 0;
  const totalRequests = data.reduce(
    (sum, p) => sum + p.count_2xx + p.count_3xx + p.count_4xx + p.count_5xx,
    0
  );
  const totalErrors = data.reduce((sum, p) => sum + p.count_4xx + p.count_5xx, 0);
  const overallErrorRate = totalRequests > 0 ? totalErrors / totalRequests : 0;

  // Average error rate (excluding buckets with no data)
  const nonEmptyBuckets = data.filter((p) => p.count_2xx + p.count_3xx + p.count_4xx + p.count_5xx > 0);
  const avgErrorRate = nonEmptyBuckets.length > 0
    ? nonEmptyBuckets.reduce((sum, p) => sum + p.error_rate, 0) / nonEmptyBuckets.length
    : 0;

  const errorColor = overallErrorRate >= 0.1 ? "text-red-600" : overallErrorRate >= 0.05 ? "text-yellow-600" : "text-green-600";

  return (
    <Card className="w-full">
      <CardHeader>
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-2">
            <AlertCircle className="h-4 w-4 text-red-500" />
            <CardTitle>Status / Error Rate</CardTitle>
          </div>
        </div>
      </CardHeader>
      <CardContent className="space-y-4">
        {isEmpty ? (
          <div className="flex h-48 items-center justify-center text-sm text-muted-foreground">
            No data available
          </div>
        ) : (
          <>
            <svg width={width} height={height} className="w-full" viewBox={`0 0 ${width} ${height}`}>
              {/* Grid lines and labels */}
              {gridLines.map((line, i) => (
                <g key={`grid-${i}`}>
                  <line
                    x1={padding.left}
                    y1={line.y}
                    x2={width - padding.right}
                    y2={line.y}
                    stroke="currentColor"
                    strokeOpacity="0.1"
                    strokeWidth="1"
                  />
                  <text
                    x={padding.left - 8}
                    y={line.y + 4}
                    textAnchor="end"
                    fontSize="12"
                    fill="currentColor"
                    opacity="0.7"
                  >
                    {line.value}
                  </text>
                </g>
              ))}

              {/* Stacked areas */}
              <path d={paths2xx.path} fill="#10b981" opacity="0.6" />
              <path d={paths3xx.path} fill="#3b82f6" opacity="0.6" />
              <path d={paths4xx.path} fill="#f59e0b" opacity="0.6" />
              <path d={paths5xx.path} fill="#ef4444" opacity="0.6" />

              {/* X-axis */}
              <line
                x1={padding.left}
                y1={height - padding.bottom}
                x2={width - padding.right}
                y2={height - padding.bottom}
                stroke="currentColor"
                strokeOpacity="0.2"
                strokeWidth="1"
              />

              {/* X-axis labels */}
              {xLabels.map((label, i) => (
                <text
                  key={`label-${i}`}
                  x={label.x}
                  y={height - padding.bottom + 20}
                  textAnchor="middle"
                  fontSize="12"
                  fill="currentColor"
                  opacity="0.7"
                >
                  {label.label}
                </text>
              ))}
            </svg>

            {/* Legend and stats */}
            <div className="space-y-3">
              <div className="flex flex-wrap gap-4 text-sm">
                <div className="flex items-center gap-2">
                  <div className="h-3 w-3 rounded-sm bg-green-500"></div>
                  <span>2xx: {formatNumber(data.reduce((sum, p) => sum + p.count_2xx, 0))}</span>
                </div>
                <div className="flex items-center gap-2">
                  <div className="h-3 w-3 rounded-sm bg-blue-500"></div>
                  <span>3xx: {formatNumber(data.reduce((sum, p) => sum + p.count_3xx, 0))}</span>
                </div>
                <div className="flex items-center gap-2">
                  <div className="h-3 w-3 rounded-sm bg-yellow-500"></div>
                  <span>4xx: {formatNumber(data.reduce((sum, p) => sum + p.count_4xx, 0))}</span>
                </div>
                <div className="flex items-center gap-2">
                  <div className="h-3 w-3 rounded-sm bg-red-500"></div>
                  <span>5xx: {formatNumber(data.reduce((sum, p) => sum + p.count_5xx, 0))}</span>
                </div>
              </div>

              <div className="grid grid-cols-2 gap-2 border-t pt-3 text-sm">
                <div>
                  <div className="text-muted-foreground">Overall Error Rate</div>
                  <div className={`font-semibold ${errorColor}`}>{formatPercent(overallErrorRate)}</div>
                </div>
                <div>
                  <div className="text-muted-foreground">Avg Error Rate</div>
                  <div className={`font-semibold ${errorColor}`}>{formatPercent(avgErrorRate)}</div>
                </div>
              </div>
            </div>
          </>
        )}
        {isLoading && (
          <div className="flex h-48 items-center justify-center text-sm text-muted-foreground">
            Loading...
          </div>
        )}
      </CardContent>
    </Card>
  );
}
