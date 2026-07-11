"use client";

import React from "react";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Activity } from "lucide-react";
import type { RequestRatePoint, MetricResolution } from "@/lib/metrics-types";
import { formatAxisTime, getMaxValue, clamp, formatNumber, parseTimestamp } from "./utils";

interface RequestRatePanelProps {
  data: RequestRatePoint[];
  resolution: MetricResolution;
  isLoading?: boolean;
}

export function RequestRatePanel({
  data,
  resolution,
  isLoading = false,
}: RequestRatePanelProps) {
  // Dimensions for SVG chart
  const width = 400;
  const height = 200;
  const padding = { top: 16, right: 16, bottom: 32, left: 40 };
  const chartWidth = width - padding.left - padding.right;
  const chartHeight = height - padding.top - padding.bottom;

  // Extract rate values for scaling
  const rateValues = data.map((p) => p.rate_per_sec);
  const maxRate = getMaxValue(rateValues);

  // Calculate scale functions
  const getX = (index: number) => {
    if (data.length <= 1) return padding.left + chartWidth / 2;
    return padding.left + (index / (data.length - 1)) * chartWidth;
  };

  const getY = (rate: number) => {
    return padding.top + chartHeight - (clamp(rate, 0, maxRate) / maxRate) * chartHeight;
  };

  // Generate SVG path for area chart
  let pathD = "";
  if (data.length > 0) {
    // Start at first point on bottom
    pathD = `M ${getX(0)} ${height - padding.bottom}`;
    // Line to first point
    pathD += ` L ${getX(0)} ${getY(data[0].rate_per_sec)}`;
    // Draw line through all points
    for (let i = 1; i < data.length; i++) {
      pathD += ` L ${getX(i)} ${getY(data[i].rate_per_sec)}`;
    }
    // Close the path
    pathD += ` L ${getX(data.length - 1)} ${height - padding.bottom} Z`;
  }

  // Generate grid lines
  const gridLines = [];
  for (let i = 0; i <= 4; i++) {
    const y = padding.top + (i / 4) * chartHeight;
    const value = maxRate - (i / 4) * maxRate;
    gridLines.push({ y, value });
  }

  // Sample x-axis labels (show every nth point to avoid crowding)
  const labelStep = Math.max(1, Math.floor(data.length / 5));
  const xLabels = [];
  for (let i = 0; i < data.length; i += labelStep) {
    xLabels.push({
      index: i,
      x: getX(i),
      label: formatAxisTime(parseTimestamp(data[i].bucket_start), resolution),
    });
  }

  const isEmpty = data.length === 0;
  const avgRate = data.length > 0 ? data.reduce((sum, p) => sum + p.rate_per_sec, 0) / data.length : 0;
  const peakRate = rateValues.length > 0 ? Math.max(...rateValues) : 0;

  return (
    <Card className="w-full">
      <CardHeader>
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-2">
            <Activity className="h-4 w-4 text-blue-500" />
            <CardTitle>Request Rate</CardTitle>
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
              <defs>
                <linearGradient id="rateGradient" x1="0%" y1="0%" x2="0%" y2="100%">
                  <stop offset="0%" style={{ stopColor: "#3b82f6", stopOpacity: 0.3 }} />
                  <stop offset="100%" style={{ stopColor: "#3b82f6", stopOpacity: 0 }} />
                </linearGradient>
              </defs>

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
                    {line.value.toFixed(1)}
                  </text>
                </g>
              ))}

              {/* Area chart */}
              <path d={pathD} fill="url(#rateGradient)" />

              {/* Line chart */}
              <polyline
                points={data.map((p, i) => `${getX(i)},${getY(p.rate_per_sec)}`).join(" ")}
                fill="none"
                stroke="#3b82f6"
                strokeWidth="2"
              />

              {/* Data points */}
              {data.map((p, i) => (
                <circle
                  key={`point-${i}`}
                  cx={getX(i)}
                  cy={getY(p.rate_per_sec)}
                  r="3"
                  fill="#3b82f6"
                  opacity="0.5"
                />
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

            {/* Stats */}
            <div className="grid grid-cols-3 gap-2 text-sm">
              <div>
                <div className="text-muted-foreground">Avg Rate</div>
                <div className="font-semibold text-blue-600">{avgRate.toFixed(2)} req/s</div>
              </div>
              <div>
                <div className="text-muted-foreground">Peak Rate</div>
                <div className="font-semibold text-blue-600">{peakRate.toFixed(2)} req/s</div>
              </div>
              <div>
                <div className="text-muted-foreground">Total Requests</div>
                <div className="font-semibold text-blue-600">
                  {formatNumber(data.reduce((sum, p) => sum + p.count, 0))}
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
