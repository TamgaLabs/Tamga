import dagre from "dagre";
import type { Edge } from "@xyflow/react";
import type { TopologyNode } from "@/lib/api";
import type { Topology } from "@/lib/topology-types";
import type { TopologyFlowNode } from "./TopologyNode";
import type { ProjectClusterNode } from "./ProjectCluster";

// ---------- Dagre layout ----------

const NODE_W = 56;
const NODE_H = 56;
const CLUSTER_PAD = 28;

/**
 * Assign a dagre rank based on node type so the graph flows
 * proxy → web → queue → database/cache → generic.
 */
function typeRank(type: string): number {
  switch (type) {
    case "proxy":
      return 0;
    case "web":
      return 1;
    case "queue":
      return 2;
    case "database":
    case "cache":
      return 3;
    default:
      return 4;
  }
}

/**
 * Convert a Topology to reactflow Nodes + Edges with a dagre layout.
 * Also generates project cluster rectangles behind groups of same-project nodes.
 */
export function topologyToFlow(
  topology: Topology,
  opts?: {
    nodeDecorations?: Record<string, { accentColor?: string }>;
    nodeStats?: Record<string, { reqRate: number; errorPct: number; p95Ms?: number }>;
    edgeDecorations?: Record<string, { thickness?: number }>;
    onNodeClick?: (node: TopologyNode) => void;
  },
): { nodes: (TopologyFlowNode | ProjectClusterNode)[]; edges: Edge[] } {
  const { nodes: topoNodes, edges: topoEdges } = topology;

  if (topoNodes.length === 0) {
    return { nodes: [], edges: [] };
  }

  // Build dagre graph
  const g = new dagre.graphlib.Graph();
  g.setDefaultEdgeLabel(() => ({}));
  g.setGraph({
    rankdir: "TB",
    nodesep: 48,
    ranksep: 64,
    marginx: 24,
    marginy: 24,
  });

  // Sort nodes so proxy gets rank 0, etc.
  const sorted = [...topoNodes].sort(
    (a, b) => typeRank(a.type) - typeRank(b.type) || a.name.localeCompare(b.name),
  );

  for (const n of sorted) {
    g.setNode(n.name, { width: NODE_W, height: NODE_H });
  }

  // De-duplicate edges by source:target pair (keep first)
  const seenEdges = new Set<string>();
  for (const e of topoEdges) {
    const key = `${e.source}→${e.target}`;
    const keyRev = `${e.target}→${e.source}`;
    if (seenEdges.has(key) || seenEdges.has(keyRev)) continue;
    if (!g.hasNode(e.source) || !g.hasNode(e.target)) continue;
    seenEdges.add(key);
    g.setEdge(e.source, e.target);
  }

  dagre.layout(g);

  // Build reactflow nodes with absolute positions
  const nodePositions = new Map<string, { x: number; y: number }>();
  const rfNodes: TopologyFlowNode[] = sorted.map((n) => {
    const pos = g.node(n.name);
    const x = (pos.x ?? 0) - NODE_W / 2;
    const y = (pos.y ?? 0) - NODE_H / 2;
    nodePositions.set(n.name, { x, y });
    return {
      id: n.name,
      type: "topologyNode",
      position: { x, y },
      data: {
        label: n.name,
        type: n.type,
        state: n.state,
        status: n.status,
        image: n.image,
        projectId: n.project_id,
        accentColor: opts?.nodeDecorations?.[n.id]?.accentColor,
        stats: opts?.nodeStats?.[n.id],
        onClick: opts?.onNodeClick ? () => opts.onNodeClick!(n) : undefined,
      },
    };
  });

  // Build cluster nodes for projects with > 1 node
  const projectNodes = new Map<number, string[]>();
  for (const n of sorted) {
    if (n.project_id > 0) {
      const list = projectNodes.get(n.project_id) || [];
      list.push(n.name);
      projectNodes.set(n.project_id, list);
    }
  }

  const clusterNodes: ProjectClusterNode[] = [];
  for (const [projectId, nodeNames] of projectNodes) {
    if (nodeNames.length < 2) continue;

    let minX = Infinity, minY = Infinity, maxX = -Infinity, maxY = -Infinity;
    for (const name of nodeNames) {
      const pos = nodePositions.get(name)!;
      minX = Math.min(minX, pos.x);
      minY = Math.min(minY, pos.y);
      maxX = Math.max(maxX, pos.x + NODE_W);
      maxY = Math.max(maxY, pos.y + NODE_H);
    }

    clusterNodes.push({
      id: `cluster-${projectId}`,
      type: "projectCluster",
      position: { x: minX - CLUSTER_PAD, y: minY - CLUSTER_PAD },
      style: {
        width: maxX - minX + CLUSTER_PAD * 2,
        height: maxY - minY + CLUSTER_PAD * 2,
      },
      data: { label: `Project #${projectId}`, nodeCount: nodeNames.length },
      selectable: false,
      draggable: false,
      zIndex: -1,
    });
  }

  // Build reactflow edges
  const rfEdges: Edge[] = [];
  const edgeSeen = new Set<string>();
  for (const e of topoEdges) {
    const key = `${e.source}→${e.target}`;
    const keyRev = `${e.target}→${e.source}`;
    if (edgeSeen.has(key) || edgeSeen.has(keyRev)) continue;
    if (!g.hasNode(e.source) || !g.hasNode(e.target)) continue;
    edgeSeen.add(key);

    const decKey = `${e.source}:${e.target}:${e.network}`;
    const thickness = opts?.edgeDecorations?.[decKey]?.thickness;

    rfEdges.push({
      id: key,
      source: e.source,
      target: e.target,
      animated: false,
      style: {
        stroke: "hsl(var(--border))",
        strokeWidth: thickness ?? 1.5,
        opacity: 0.6,
      },
      label: e.network,
      labelStyle: {
        fontSize: 8,
        fill: "hsl(var(--muted-foreground))",
        fontWeight: 400,
      },
      labelShowBg: false,
      labelBgPadding: [2, 1] as [number, number],
    });
  }

  return { nodes: [...clusterNodes, ...rfNodes], edges: rfEdges };
}
