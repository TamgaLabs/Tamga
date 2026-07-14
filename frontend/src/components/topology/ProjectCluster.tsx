"use client";

import React from "react";
import type { Node, NodeProps } from "@xyflow/react";

export type ProjectClusterNode = Node<ProjectClusterData, "projectCluster">;

export interface ProjectClusterData extends Record<string, unknown> {
  label: string;
  nodeCount: number;
}

export function ProjectClusterComponent({ data }: NodeProps<ProjectClusterNode>) {
  return (
    <div
      className="rounded-lg border border-dashed pointer-events-none flex flex-col"
      style={{
        width: "100%",
        height: "100%",
        borderColor: "hsl(var(--border))",
        backgroundColor: "hsl(var(--card) / 0.5)",
      }}
    >
      <span className="absolute top-2 left-3 text-[10px] font-medium text-muted-foreground select-none">
        {data.label}
        {data.nodeCount > 1 && <span className="ml-1 opacity-60">({data.nodeCount})</span>}
      </span>
    </div>
  );
}
