"use client";

import React from "react";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Clock } from "lucide-react";
import type { LatencyPoint, MetricResolution } from "@/lib/metrics-types";
import {
  formatAxisTime,
  getMaxValue,
  clamp,
  secondsToMs,
  parseTimestamp,
} from "./utils";

interface LatencyPanelProps {
  data: LatencyPoint[];
  resolution: MetricResolution;
  isLoading?: boolean;
}

export function LatencyPanel({
  data,
  resolution,
  isLoading = false,
}: LatencyPanelProps) {
  // Dimensions for SVG chart
  const width = 400;
  const height = 200;
  const padding = { top: 16, right: 16, bottom: 32, left: 40 };
  const chartWidth = width - padding.left - padding.right;
  const chartHeight = height - padding.top - padding.bottom;

  // Convert seconds to ms for display
  const dataInMs = data.map((p) => ({
    ...p,
    p50: secondsToMs(p.p50),
    p95: secondsToMs(p.p95),
    p99: secondsToMs(p.p99),
  }));

  // Extract max latency for scaling
  const allLatencies = dataInMs.flatMap((p) => [p.p50, p.p95, p.p99]);
  const maxLatency = getMaxValue(allLatencies);

  // Calculate scale functions
  const getX = (index: number) => {
    if (data.length <= 1) return padding.left + chartWidth / 2;
    return padding.left + (index / (data.length - 1)) * chartWidth;
  };

  const getY = (latency: number) => {
    return padding.top + chartHeight - (clamp(latency, 0, maxLatency) / maxLatency) * chartHeight;
  };

  // Generate path for each percentile
  const generatePath = (selector: (p: { p50: number; p95: number; p99: number }) => number): string => {
    if (data.length === 0) return "";
    let pathD = `M ${getX(0)} ${getY(selector(dataInMs[0]))}`;
    for (let i = 1; i < dataInMs.length; i++) {
      pathD += ` L ${getX(i)} ${getY(selector(dataInMs[i]))}`;
    }
    return pathD;
  };

  const path50 = generatePath((p) => p.p50);
  const path95 = generatePath((p) => p.p95);
  const path99 = generatePath((p) => p.p99);

  // Generate grid lines
  const gridLines = [];
  for (let i = 0; i <= 4; i++) {
    const y = padding.top + (i / 4) * chartHeight;
    const value = maxLatency - (i / 4) * maxLatency;
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

  // Calculate stats
  const isEmpty = data.length === 0;
  const nonZeroData = dataInMs.filter((p) => p.p50 > 0 || p.p95 > 0 || p.p99 > 0);

  const avgP50 = nonZeroData.length > 0
    ? nonZeroData.reduce((sum, p) => sum + p.p50, 0) / nonZeroData.length
    : 0;
  const avgP95 = nonZeroData.length > 0
    ? nonZeroData.reduce((sum, p) => sum + p.p95, 0) / nonZeroData.length
    : 0;
  const avgP99 = nonZeroData.length > 0
    ? nonZeroData.reduce((sum, p) => sum + p.p99, 0) / nonZeroData.length
    : 0;

  const maxP99 = allLatencies.length > 0 ? Math.max(...allLatencies) : 0;

  return (
    <Card className="w-full">
      <CardHeader>
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-2">
            <Clock className="h-4 w-4 text-purple-500" />
            <CardTitle>Latency Percentiles</CardTitle>
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
                    {line.value.toFixed(0)}ms
                  </text>
                </g>
              ))}

              {/* Percentile lines */}
              <polyline points={data.map((_, i) => `${getX(i)},${getY(dataInMs[i].p50)}`).join(" ")} fill="none" stroke="#8b5cf6" strokeWidth="2" />
              <polyline points={data.map((_, i) => `${getX(i)},${getY(dataInMs[i].p95)}`).join(" ")} fill="none" stroke="#ec4899" strokeWidth="2" />
              <polyline points={data.map((_, i) => `${getX(i)},${getY(dataInMs[i].p99)}`).join(" ")} fill="none" stroke="#f97316" strokeWidth="2" />

              {/* Data points for p99 (most important) */}
              {dataInMs.map((p, i) => (
                p.p99 > 0 && (
                  <circle
                    key={`point-${i}`}
                    cx={getX(i)}
                    cy={getY(p.p99)}
                    r="2"
                    fill="#f97316"
                    opacity="0.5"
                  />
                )
              ))}

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
                  <div className="h-1 w-4 bg-violet-500"></div>
                  <span>p50</span>
                </div>
                <div className="flex items-center gap-2">
                  <div className="h-1 w-4 bg-pink-500"></div>
                  <span>p95</span>
                </div>
                <div className="flex items-center gap-2">
                  <div className="h-1 w-4 bg-orange-500"></div>
                  <span>p99</span>
                </div>
              </div>

              <div className="grid grid-cols-4 gap-2 border-t pt-3 text-sm">
                <div>
                  <div className="text-muted-foreground">Avg p50</div>
                  <div className="font-semibold text-violet-600">{avgP50.toFixed(0)}ms</div>
                </div>
                <div>
                  <div className="text-muted-foreground">Avg p95</div>
                  <div className="font-semibold text-pink-600">{avgP95.toFixed(0)}ms</div>
                </div>
                <div>
                  <div className="text-muted-foreground">Avg p99</div>
                  <div className="font-semibold text-orange-600">{avgP99.toFixed(0)}ms</div>
                </div>
                <div>
                  <div className="text-muted-foreground">Max p99</div>
                  <div className="font-semibold text-orange-600">{maxP99.toFixed(0)}ms</div>
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
