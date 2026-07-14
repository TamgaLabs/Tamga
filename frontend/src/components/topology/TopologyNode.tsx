"use client";

import React from "react";
import { Handle, Position, type Node, type NodeProps } from "@xyflow/react";

export type TopologyFlowNode = Node<TopologyNodeData, "topologyNode">;

export interface TopologyNodeData extends Record<string, unknown> {
  label: string;
  type: string;
  state: string;
  status: string;
  image: string;
  projectId: number;
  accentColor?: string;
  stats?: {
    reqRate: number;
    errorPct: number;
    p95Ms?: number;
  };
  onClick?: () => void;
}

const typeColors: Record<string, { bg: string; border: string }> = {
  proxy: { bg: "hsl(217, 91%, 60%)", border: "hsl(217, 91%, 50%)" },
  database: { bg: "hsl(262, 83%, 58%)", border: "hsl(262, 83%, 48%)" },
  cache: { bg: "hsl(38, 92%, 50%)", border: "hsl(38, 92%, 40%)" },
  web: { bg: "hsl(142, 71%, 45%)", border: "hsl(142, 71%, 35%)" },
  queue: { bg: "hsl(330, 81%, 60%)", border: "hsl(330, 81%, 50%)" },
  generic: { bg: "hsl(220, 9%, 46%)", border: "hsl(220, 9%, 36%)" },
};

const statusDotColor: Record<string, string> = {
  running: "hsl(142, 71%, 45%)",
  exited: "hsl(0, 84%, 60%)",
  created: "hsl(217, 91%, 60%)",
  paused: "hsl(38, 92%, 50%)",
};

export function TopologyNodeComponent({ data }: NodeProps<TopologyFlowNode>) {
  const colors = typeColors[data.type] || typeColors.generic;
  const dotColor = statusDotColor[data.state] || "hsl(220, 9%, 46%)";
  const borderColor = data.accentColor || colors.border;

  const tooltipLines = [
    `${data.label} (${data.type})`,
    `Image: ${data.image}`,
    `State: ${data.state}`,
    data.projectId ? `Project: ${data.projectId}` : "System",
  ];
  if (data.stats) {
    tooltipLines.push(
      "",
      `${data.stats.reqRate.toFixed(1)} req/s`,
      `${data.stats.errorPct.toFixed(1)}% errors`,
    );
    if (data.stats.p95Ms !== undefined) {
      tooltipLines.push(`p95: ${data.stats.p95Ms.toFixed(0)}ms`);
    }
  }

  return (
    <>
      <Handle type="target" position={Position.Top} className="!bg-transparent !border-0 !w-0 !h-0" />
      <div
        className="group cursor-pointer flex flex-col items-center gap-1"
        onClick={data.onClick}
        title={tooltipLines.join("\n")}
      >
        <div className="relative">
          <div
            className="w-9 h-9 rounded-full flex items-center justify-center transition-transform group-hover:scale-110"
            style={{ backgroundColor: colors.bg, border: `2px solid ${borderColor}` }}
          >
            <span className="text-[10px] font-bold text-white/90 uppercase select-none">
              {data.type.slice(0, 2)}
            </span>
          </div>
          <div
            className="absolute -top-0.5 -right-0.5 w-2.5 h-2.5 rounded-full border-2"
            style={{ backgroundColor: dotColor, borderColor: "hsl(var(--background))" }}
          />
        </div>
        <span className="text-[9px] font-medium text-muted-foreground max-w-[72px] truncate text-center leading-tight">
          {data.label}
        </span>
      </div>
      <Handle type="source" position={Position.Bottom} className="!bg-transparent !border-0 !w-0 !h-0" />
    </>
  );
}
