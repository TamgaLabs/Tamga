"use client";

import React, { useMemo } from "react";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import type { TopologyNode } from "@/lib/api";
import type { Topology } from "@/lib/topology-types";
import {
  calculateNodePositions,
  getNodePosition,
  getEdgePath,
  getNodeTypeStyle,
  getStatusColor,
  calculateCanvasDimensions,
} from "./utils";

interface TopologyNodeDecorations {
  accentColor?: string;
}

interface TopologyEdgeDecorations {
  thickness?: number;
}

interface TopologyNodeStats {
  reqRate: number; // requests per second
  errorPct: number; // error percentage (0-100)
  p95Ms?: number; // p95 latency in milliseconds
}

interface TopologyGraphProps {
  topology: Topology;
  onNodeClick?: (node: TopologyNode) => void;
  loading?: boolean;
  // Overlay seam for FEAT-039: per-node and per-edge decorations
  nodeDecorations?: Record<string, TopologyNodeDecorations>;
  edgeDecorations?: Record<string, TopologyEdgeDecorations>;
  // Overlay seam for FEAT-039: per-node traffic mini-stats for hover
  nodeStats?: Record<string, TopologyNodeStats>;
}

const NODE_RADIUS = 30;
const ICON_SIZE = 18;

/**
 * TopologyGraph renders an infrastructure topology as an inline SVG.
 * Nodes are grouped by project_id and laid out deterministically.
 * Edges are drawn between nodes resolved by name.
 *
 * This is a presentational component: it does not fetch data or navigate.
 * The page/hook provides the topology data and handles navigation from onNodeClick.
 *
 * Overlay decorations (FEAT-039) can customize node colors and edge thickness.
 */
export function TopologyGraph({
  topology,
  onNodeClick,
  loading = false,
  nodeDecorations,
  edgeDecorations,
  nodeStats,
}: TopologyGraphProps) {
  const { nodes, edges } = topology;

  // Calculate node positions using deterministic layout
  const positions = useMemo(() => calculateNodePositions(nodes), [nodes]);

  // Calculate canvas dimensions
  const { width, height } = useMemo(
    () => calculateCanvasDimensions(positions),
    [positions]
  );

  // Build a map of node name -> position for edge resolution
  const nodePositionMap = useMemo(() => {
    const map: Record<string, { x: number; y: number }> = {};
    for (const pos of positions) {
      map[pos.node.name] = { x: pos.x, y: pos.y };
    }
    return map;
  }, [positions]);

  // Empty state
  if (nodes.length === 0) {
    return (
      <Card>
        <CardHeader>
          <CardTitle>Infrastructure Topology</CardTitle>
        </CardHeader>
        <CardContent className="text-center text-muted-foreground py-12">
          {loading ? "Loading topology..." : "No containers"}
        </CardContent>
      </Card>
    );
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle>Infrastructure Topology</CardTitle>
      </CardHeader>
      <CardContent className="p-6">
        <div className="w-full overflow-auto border border-border rounded-lg">
          <svg
            viewBox={`0 0 ${width} ${height}`}
            className="w-full min-w-full h-auto"
            style={{ minHeight: "400px" }}
          >
            {/* Grid background (optional, subtle) */}
            <defs>
              <pattern
                id="grid"
                width="50"
                height="50"
                patternUnits="userSpaceOnUse"
              >
                <path
                  d="M 50 0 L 0 0 0 50"
                  fill="none"
                  stroke="hsl(var(--border))"
                  strokeWidth="0.5"
                  opacity="0.2"
                />
              </pattern>
            </defs>
            <rect width={width} height={height} fill="hsl(var(--card))" />
            <rect
              width={width}
              height={height}
              fill="url(#grid)"
              opacity="0.3"
            />

            {/* Edges (drawn first so they appear behind nodes) */}
            <g className="topology-edges">
              {edges.map((edge, idx) => {
                const sourcePos = nodePositionMap[edge.source];
                const targetPos = nodePositionMap[edge.target];

                if (!sourcePos || !targetPos) return null;

                const edgeKey = `${edge.source}:${edge.target}:${edge.network}`;
                const decorations = edgeDecorations?.[edgeKey];
                const pathD = getEdgePath(sourcePos, targetPos, NODE_RADIUS);

                return (
                  <path
                    key={idx}
                    d={pathD}
                    fill="none"
                    stroke="hsl(var(--border))"
                    strokeWidth={decorations?.thickness ?? 2}
                    opacity="0.6"
                    className="cursor-default"
                  />
                );
              })}
            </g>

            {/* Nodes */}
            <g className="topology-nodes">
              {positions.map(({ node, x, y }) => {
                const typeStyle = getNodeTypeStyle(node.type);
                const statusColor = getStatusColor(node.state);
                const decorations = nodeDecorations?.[node.id];
                const stats = nodeStats?.[node.id];

                // Build hover title with stats if available
                let tooltipText = `${node.name} (${node.type})\nProject: ${node.project_id}\nState: ${node.state}`;
                if (stats) {
                  tooltipText += `\n\nTraffic Stats:\nReq/s: ${stats.reqRate.toFixed(2)}\nError: ${stats.errorPct.toFixed(1)}%`;
                  if (stats.p95Ms !== undefined) {
                    tooltipText += `\nP95 Latency: ${stats.p95Ms.toFixed(0)}ms`;
                  }
                }

                return (
                  <g
                    key={node.id}
                    onClick={() => onNodeClick?.(node)}
                    className={onNodeClick ? "cursor-pointer" : ""}
                  >
                    {/* Node circle background */}
                    <circle
                      cx={x}
                      cy={y}
                      r={NODE_RADIUS}
                      fill={decorations?.accentColor || typeStyle.bgColor}
                      stroke={typeStyle.color}
                      strokeWidth="2"
                      className="hover:opacity-80 transition-opacity"
                    />

                    {/* Status dot (running/exited indicator) */}
                    <circle
                      cx={x + NODE_RADIUS - 6}
                      cy={y - NODE_RADIUS + 6}
                      r="5"
                      fill={statusColor}
                      stroke="hsl(var(--card))"
                      strokeWidth="2"
                    />

                    {/* Node label */}
                    <text
                      x={x}
                      y={y + NODE_RADIUS + 20}
                      textAnchor="middle"
                      className="text-xs font-medium fill-foreground"
                      style={{
                        pointerEvents: "none",
                        userSelect: "none",
                      }}
                    >
                      {node.name.length > 20
                        ? node.name.substring(0, 17) + "..."
                        : node.name}
                    </text>

                    {/* Hover tooltip info */}
                    <title>{tooltipText}</title>
                  </g>
                );
              })}
            </g>
          </svg>
        </div>

        {/* Legend */}
        <div className="mt-4 text-xs text-muted-foreground flex flex-wrap gap-4">
          <div className="flex items-center gap-2">
            <div
              className="w-3 h-3 rounded-full"
              style={{ backgroundColor: "hsl(var(--success))" }}
            />
            <span>Running</span>
          </div>
          <div className="flex items-center gap-2">
            <div
              className="w-3 h-3 rounded-full"
              style={{ backgroundColor: "hsl(var(--muted-foreground))" }}
            />
            <span>Stopped</span>
          </div>
        </div>
      </CardContent>
    </Card>
  );
}
