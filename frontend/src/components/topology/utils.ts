import type { TopologyNode, TopologyEdge } from "@/lib/api";

// Layout algorithm: deterministic grid-based layout grouped by project_id
export interface NodePosition {
  node: TopologyNode;
  x: number;
  y: number;
}

const CLUSTER_SPACING = 120; // vertical space between project clusters
const NODE_SPACING = 100; // horizontal/vertical space between nodes in grid
const CORE_STACK_WIDTH = 600; // width available for core stack nodes
const NODES_PER_ROW = 3; // number of nodes per row in grid layout

/**
 * Calculate positions for all nodes in a deterministic grid layout grouped by project_id.
 * Core stack (project_id=0) is positioned at the top.
 * Other projects are positioned below in order of appearance.
 */
export function calculateNodePositions(nodes: TopologyNode[]): NodePosition[] {
  const positions: NodePosition[] = [];

  // Group nodes by project_id, maintaining order
  const nodesByProject = new Map<number, TopologyNode[]>();
  const projectOrder: number[] = [];

  for (const node of nodes) {
    if (!nodesByProject.has(node.project_id)) {
      projectOrder.push(node.project_id);
      nodesByProject.set(node.project_id, []);
    }
    nodesByProject.get(node.project_id)!.push(node);
  }

  // Sort so project_id 0 (core stack) comes first
  projectOrder.sort((a, b) => {
    if (a === 0) return -1;
    if (b === 0) return 1;
    return a - b;
  });

  let currentY = 50;

  // Position each project's cluster
  for (const projectId of projectOrder) {
    const projectNodes = nodesByProject.get(projectId)!;
    const rowCount = Math.ceil(projectNodes.length / NODES_PER_ROW);

    // Position nodes in this cluster in a grid
    for (let i = 0; i < projectNodes.length; i++) {
      const row = Math.floor(i / NODES_PER_ROW);
      const col = i % NODES_PER_ROW;

      const x = 50 + col * NODE_SPACING;
      const y = currentY + row * NODE_SPACING;

      positions.push({
        node: projectNodes[i],
        x,
        y,
      });
    }

    // Move down for next cluster
    currentY += rowCount * NODE_SPACING + CLUSTER_SPACING;
  }

  return positions;
}

/**
 * Find the position of a node by its name. Used to resolve edge endpoints.
 */
export function getNodePosition(
  nodePositions: NodePosition[],
  nodeName: string
): { x: number; y: number } | null {
  const pos = nodePositions.find((p) => p.node.name === nodeName);
  return pos ? { x: pos.x, y: pos.y } : null;
}

/**
 * Get an SVG path for drawing an edge between two nodes.
 * Uses a quadratic bezier curve for a gentle arc.
 */
export function getEdgePath(
  sourcePos: { x: number; y: number },
  targetPos: { x: number; y: number },
  nodeRadius: number
): string {
  // Add node radius to shorten line to edge of circle
  const dx = targetPos.x - sourcePos.x;
  const dy = targetPos.y - sourcePos.y;
  const distance = Math.sqrt(dx * dx + dy * dy);

  if (distance === 0) return "";

  const angle = Math.atan2(dy, dx);
  const startX = sourcePos.x + nodeRadius * Math.cos(angle);
  const startY = sourcePos.y + nodeRadius * Math.sin(angle);
  const endX = targetPos.x - nodeRadius * Math.cos(angle);
  const endY = targetPos.y - nodeRadius * Math.sin(angle);

  // Control point for bezier curve (midpoint with vertical offset)
  const ctrlX = (startX + endX) / 2;
  const ctrlY = (startY + endY) / 2 + Math.abs(dx) * 0.1;

  return `M ${startX} ${startY} Q ${ctrlX} ${ctrlY} ${endX} ${endY}`;
}

/**
 * Get icon and color for a node based on its type.
 */
export function getNodeTypeStyle(type: string): {
  icon: string;
  color: string;
  bgColor: string;
} {
  switch (type) {
    case "cache":
      return {
        icon: "database",
        color: "hsl(var(--primary))",
        bgColor: "hsl(var(--primary) / 0.1)",
      };
    case "database":
      return {
        icon: "database",
        color: "hsl(var(--accent))",
        bgColor: "hsl(var(--accent) / 0.1)",
      };
    case "proxy":
      return {
        icon: "server",
        color: "hsl(var(--primary))",
        bgColor: "hsl(var(--primary) / 0.1)",
      };
    case "web":
      return {
        icon: "globe",
        color: "hsl(var(--secondary))",
        bgColor: "hsl(var(--secondary) / 0.1)",
      };
    case "queue":
      return {
        icon: "inbox",
        color: "hsl(var(--primary))",
        bgColor: "hsl(var(--primary) / 0.1)",
      };
    case "generic":
    default:
      return {
        icon: "box",
        color: "hsl(var(--muted-foreground))",
        bgColor: "hsl(var(--muted) / 0.3)",
      };
  }
}

/**
 * Get status dot color based on container state.
 */
export function getStatusColor(state: string): string {
  if (state === "running") {
    return "hsl(var(--success))";
  }
  return "hsl(var(--muted-foreground))";
}

/**
 * Calculate SVG dimensions needed to fit all nodes.
 */
export function calculateCanvasDimensions(positions: NodePosition[]): {
  width: number;
  height: number;
} {
  const nodeRadius = 30;
  const padding = 50;

  if (positions.length === 0) {
    return { width: 600, height: 400 };
  }

  let maxX = 0;
  let maxY = 0;

  for (const pos of positions) {
    maxX = Math.max(maxX, pos.x);
    maxY = Math.max(maxY, pos.y);
  }

  return {
    width: maxX + nodeRadius + padding,
    height: maxY + nodeRadius + padding,
  };
}
