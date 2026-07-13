"use client";

import React from "react";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Network } from "lucide-react";
import type { BandwidthPoint, MetricResolution } from "@/lib/metrics-types";
import { formatAxisTime, getMaxValue, clamp, formatBytes, parseTimestamp } from "./utils";

interface BandwidthPanelProps {
  data: BandwidthPoint[];
  resolution: MetricResolution;
  isLoading?: boolean;
}

export function BandwidthPanel({
  data,
  resolution,
  isLoading = false,
}: BandwidthPanelProps) {
  // Dimensions for SVG chart
  const width = 400;
  const height = 200;
  const padding = { top: 16, right: 16, bottom: 32, left: 40 };
  const chartWidth = width - padding.left - padding.right;
  const chartHeight = height - padding.top - padding.bottom;

  // Extract bandwidth values for scaling
  const inValues = data.map((p) => p.bytes_in);
  const outValues = data.map((p) => p.bytes_out);
  const maxBandwidth = getMaxValue([...inValues, ...outValues]);

  // Calculate scale functions
  const getX = (index: number) => {
    if (data.length <= 1) return padding.left + chartWidth / 2;
    return padding.left + (index / (data.length - 1)) * chartWidth;
  };

  const getY = (bytes: number) => {
    return padding.top + chartHeight - (clamp(bytes, 0, maxBandwidth) / maxBandwidth) * chartHeight;
  };

  // Generate grid lines with byte labels
  const gridLines = [];
  for (let i = 0; i <= 4; i++) {
    const y = padding.top + (i / 4) * chartHeight;
    const value = maxBandwidth - (i / 4) * maxBandwidth;
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
  const totalBytesIn = data.reduce((sum, p) => sum + p.bytes_in, 0);
  const totalBytesOut = data.reduce((sum, p) => sum + p.bytes_out, 0);
  const totalBytes = totalBytesIn + totalBytesOut;

  const avgBytesIn = data.length > 0 ? totalBytesIn / data.length : 0;
  const avgBytesOut = data.length > 0 ? totalBytesOut / data.length : 0;

  const peakBytesIn = inValues.length > 0 ? Math.max(...inValues) : 0;
  const peakBytesOut = outValues.length > 0 ? Math.max(...outValues) : 0;

  return (
    <Card className="w-full">
      <CardHeader className="p-4 sm:p-6">
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-2">
            <Network className="h-4 w-4 text-green-500" />
            <CardTitle>Bandwidth</CardTitle>
          </div>
          {isLoading && !isEmpty && <Badge variant="secondary">Refreshing</Badge>}
        </div>
      </CardHeader>
      <CardContent className="space-y-4 p-4 pt-0 sm:p-6 sm:pt-0">
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
                    fontSize="11"
                    fill="currentColor"
                    opacity="0.7"
                  >
                    {formatBytes(line.value)}
                  </text>
                </g>
              ))}

              {/* Bytes In line */}
              <polyline
                points={data.map((p, i) => `${getX(i)},${getY(p.bytes_in)}`).join(" ")}
                fill="none"
                stroke="#3b82f6"
                strokeWidth="2"
              />

              {/* Bytes Out line */}
              <polyline
                points={data.map((p, i) => `${getX(i)},${getY(p.bytes_out)}`).join(" ")}
                fill="none"
                stroke="#ef4444"
                strokeWidth="2"
              />

              {/* Data points */}
              {data.map((p, i) => (
                <g key={`point-${i}`}>
                  {p.bytes_in > 0 && (
                    <circle cx={getX(i)} cy={getY(p.bytes_in)} r="2" fill="#3b82f6" opacity="0.5" />
                  )}
                  {p.bytes_out > 0 && (
                    <circle cx={getX(i)} cy={getY(p.bytes_out)} r="2" fill="#ef4444" opacity="0.5" />
                  )}
                </g>
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
              <div className="flex gap-4 text-sm">
                <div className="flex items-center gap-2">
                  <div className="h-1 w-4 bg-blue-500"></div>
                  <span>Bytes In</span>
                </div>
                <div className="flex items-center gap-2">
                  <div className="h-1 w-4 bg-red-500"></div>
                  <span>Bytes Out</span>
                </div>
              </div>

              <div className="grid grid-cols-3 gap-2 border-t pt-3 text-sm">
                <div>
                  <div className="text-muted-foreground">Total In</div>
                  <div className="font-semibold text-blue-600">{formatBytes(totalBytesIn)}</div>
                </div>
                <div>
                  <div className="text-muted-foreground">Total Out</div>
                  <div className="font-semibold text-red-600">{formatBytes(totalBytesOut)}</div>
                </div>
                <div>
                  <div className="text-muted-foreground">Total</div>
                  <div className="font-semibold text-gray-600">{formatBytes(totalBytes)}</div>
                </div>
              </div>

              <div className="grid grid-cols-4 gap-2 border-t pt-3 text-sm">
                <div>
                  <div className="text-muted-foreground">Avg In</div>
                  <div className="font-semibold text-blue-600">{formatBytes(avgBytesIn)}</div>
                </div>
                <div>
                  <div className="text-muted-foreground">Avg Out</div>
                  <div className="font-semibold text-red-600">{formatBytes(avgBytesOut)}</div>
                </div>
                <div>
                  <div className="text-muted-foreground">Peak In</div>
                  <div className="font-semibold text-blue-600">{formatBytes(peakBytesIn)}</div>
                </div>
                <div>
                  <div className="text-muted-foreground">Peak Out</div>
                  <div className="font-semibold text-red-600">{formatBytes(peakBytesOut)}</div>
                </div>
              </div>
            </div>
          </>
        )}
      </CardContent>
    </Card>
  );
}
