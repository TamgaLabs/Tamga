"use client";

import React, { useMemo } from "react";
import {
  ReactFlow,
  Background,
  MiniMap,
  Controls,
} from "@xyflow/react";
import "@xyflow/react/dist/style.css";

import type { TopologyNode } from "@/lib/api";
import type { Topology } from "@/lib/topology-types";
import { TopologyNodeComponent, type TopologyFlowNode } from "./TopologyNode";
import { ProjectClusterComponent, type ProjectClusterNode } from "./ProjectCluster";
import { topologyToFlow } from "./utils";

interface TopologyNodeDecorations {
  accentColor?: string;
}

interface TopologyEdgeDecorations {
  thickness?: number;
}

interface TopologyNodeStats {
  reqRate: number;
  errorPct: number;
  p95Ms?: number;
}

interface TopologyGraphProps {
  topology: Topology;
  onNodeClick?: (node: TopologyNode) => void;
  loading?: boolean;
  nodeDecorations?: Record<string, TopologyNodeDecorations>;
  edgeDecorations?: Record<string, TopologyEdgeDecorations>;
  nodeStats?: Record<string, TopologyNodeStats>;
}

const nodeTypes = {
  topologyNode: TopologyNodeComponent,
  projectCluster: ProjectClusterComponent,
};

export function TopologyGraph({
  topology,
  onNodeClick,
  loading = false,
  nodeDecorations,
  edgeDecorations,
  nodeStats,
}: TopologyGraphProps) {
  const { nodes: rfNodes, edges: rfEdges } = useMemo(
    () =>
      topologyToFlow(topology, {
        nodeDecorations,
        edgeDecorations,
        nodeStats,
        onNodeClick,
      }),
    [topology, nodeDecorations, edgeDecorations, nodeStats, onNodeClick],
  );

  if (topology.nodes.length === 0) {
    return (
      <div className="rounded-lg border border-border bg-card p-12 text-center text-sm text-muted-foreground">
        {loading ? "Loading topology..." : "No containers"}
      </div>
    );
  }

  return (
    <div className="rounded-lg border border-border bg-card overflow-hidden" style={{ height: "32rem" }}>
      <ReactFlow
        nodes={rfNodes as (TopologyFlowNode | ProjectClusterNode)[]}
        edges={rfEdges}
        nodeTypes={nodeTypes}
        fitView
        fitViewOptions={{ padding: 0.3 }}
        minZoom={0.2}
        maxZoom={2}
        proOptions={{ hideAttribution: true }}
        style={{ background: "hsl(var(--background))" }}
        defaultEdgeOptions={{
          style: { stroke: "hsl(var(--border))", strokeWidth: 1.5, opacity: 0.6 },
        }}
      >
        <Background gap={24} size={1} color="hsl(var(--border))" />
        <Controls showInteractive={false} />
        <MiniMap
          nodeColor={(n) => {
            if (n.type === "projectCluster") return "hsl(var(--muted) / 0.5)";
            const d = n.data as { type?: string };
            switch (d.type) {
              case "proxy":
                return "hsl(217, 91%, 60%)";
              case "database":
                return "hsl(262, 83%, 58%)";
              case "cache":
                return "hsl(38, 92%, 50%)";
              case "web":
                return "hsl(142, 71%, 45%)";
              case "queue":
                return "hsl(330, 81%, 60%)";
              default:
                return "hsl(220, 9%, 46%)";
            }
          }}
          maskColor="hsl(var(--background) / 0.8)"
          style={{ backgroundColor: "hsl(var(--card))" }}
        />
      </ReactFlow>
    </div>
  );
}
