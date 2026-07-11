"use client";

import React, { useMemo } from "react";
import { useSystemMetrics, useProjectMetrics } from "@/hooks/useMetrics";
import { getProjectMetrics } from "@/lib/api";
import type { Topology } from "@/lib/topology-types";
import type { MetricsPanels } from "@/lib/metrics-types";

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

interface TrafficOverlayResult {
  nodeDecorations: Record<string, TopologyNodeDecorations>;
  edgeDecorations: Record<string, TopologyEdgeDecorations>;
  nodeStats: Record<string, TopologyNodeStats>;
}

/**
 * Get error rate color thresholds:
 * - < 1% (0.01): Green (ok)
 * - < 5% (0.05): Yellow (warning)
 * - >= 5%: Red (error)
 */
function getErrorRateColor(errorRate: number): string | undefined {
  if (errorRate < 0.01) {
    return "hsl(142, 71%, 45%)"; // Green
  }
  if (errorRate < 0.05) {
    return "hsl(43, 96%, 56%)"; // Yellow
  }
  return "hsl(0, 84.2%, 60.2%)"; // Red
}

/**
 * Scale request volume to edge thickness.
 * Base thickness is 2, max is 8.
 * Maps request rate (0-100 req/sec typical) to thickness.
 */
function getEdgeThickness(requestRatePerSec: number): number {
  const BASE_THICKNESS = 2;
  const MAX_THICKNESS = 8;
  // Scale: 0 req/sec -> 2px, 50 req/sec -> 4.5px, 100+ req/sec -> 8px
  const scaled = Math.min(MAX_THICKNESS, BASE_THICKNESS + requestRatePerSec / 20);
  return scaled;
}

/**
 * Find the Traefik node in the topology.
 * Identified by type="proxy" or name containing "traefik".
 */
function findTraefikNode(
  nodes: any[]
): { name: string; id: string } | null {
  for (const node of nodes) {
    if (node.type === "proxy" || node.name.toLowerCase().includes("traefik")) {
      return { name: node.name, id: node.id };
    }
  }
  return null;
}

/**
 * Extract mini-stats from metrics for hover display.
 * Returns: req rate (req/s), error % (0-100), and p95 latency (in ms).
 */
function buildNodeStats(metrics: MetricsPanels): TopologyNodeStats | null {
  const latestRequest = metrics.request_rate[metrics.request_rate.length - 1];
  const latestStatus = metrics.status_class[metrics.status_class.length - 1];
  const latestLatency = metrics.latency[metrics.latency.length - 1];

  if (!latestRequest || !latestStatus || !latestLatency) {
    return null;
  }

  return {
    reqRate: latestRequest.rate_per_sec,
    errorPct: latestStatus.error_rate * 100,
    p95Ms: latestLatency.p95 * 1000, // Convert seconds to milliseconds
  };
}

/**
 * Build decorations for project-specific view.
 * Metrics only available for that single project.
 */
function buildProjectOverlay(
  topology: Topology,
  projectMetrics: MetricsPanels | null
): TrafficOverlayResult {
  const nodeDecorations: Record<string, TopologyNodeDecorations> = {};
  const edgeDecorations: Record<string, TopologyEdgeDecorations> = {};
  const nodeStats: Record<string, TopologyNodeStats> = {};

  if (!projectMetrics || !projectMetrics.status_class.length) {
    return { nodeDecorations, edgeDecorations, nodeStats };
  }

  const { nodes, edges } = topology;
  const traefik = findTraefikNode(nodes);

  // Get latest error rate from status_class
  const latestStatus = projectMetrics.status_class[projectMetrics.status_class.length - 1];
  if (!latestStatus) {
    return { nodeDecorations, edgeDecorations, nodeStats };
  }

  const errorRate = latestStatus.error_rate;

  // Build nodeStats for this project and attach to all its nodes
  const stats = buildNodeStats(projectMetrics);

  // Color all project nodes by error rate and attach mini-stats
  for (const node of nodes) {
    if (node.project_id !== 0 && node.project_id === projectMetrics.project_id) {
      const color = getErrorRateColor(errorRate);
      if (color) {
        nodeDecorations[node.id] = { accentColor: color };
      }
      if (stats) {
        nodeStats[node.id] = stats;
      }
    }
  }

  // Thicken ingress edges (Traefik -> project node) by request rate
  if (traefik && projectMetrics.request_rate.length) {
    const latestRequest = projectMetrics.request_rate[projectMetrics.request_rate.length - 1];
    const thickness = getEdgeThickness(latestRequest.rate_per_sec);

    for (const edge of edges) {
      const isIngressEdge =
        (edge.source === traefik.name &&
          nodes.find((n) => n.name === edge.target)?.project_id === projectMetrics.project_id) ||
        (edge.target === traefik.name &&
          nodes.find((n) => n.name === edge.source)?.project_id === projectMetrics.project_id);

      if (isIngressEdge) {
        const edgeKey = `${edge.source}:${edge.target}:${edge.network}`;
        edgeDecorations[edgeKey] = { thickness };
      }
    }
  }

  return { nodeDecorations, edgeDecorations, nodeStats };
}

/**
 * Build decorations for global view.
 * Metrics available per project via the metrics map.
 */
function buildGlobalOverlay(
  topology: Topology,
  metricsByProjectId: Map<number, MetricsPanels>
): TrafficOverlayResult {
  const nodeDecorations: Record<string, TopologyNodeDecorations> = {};
  const edgeDecorations: Record<string, TopologyEdgeDecorations> = {};
  const nodeStats: Record<string, TopologyNodeStats> = {};

  const { nodes, edges } = topology;
  const traefik = findTraefikNode(nodes);

  // Color project nodes by their error rate and attach mini-stats
  for (const node of nodes) {
    if (node.project_id === 0) {
      // System/core nodes stay neutral
      continue;
    }

    const metrics = metricsByProjectId.get(node.project_id);
    if (!metrics || !metrics.status_class.length) {
      // No traffic data for this project; stay neutral
      continue;
    }

    const latestStatus = metrics.status_class[metrics.status_class.length - 1];
    const errorRate = latestStatus.error_rate;
    const color = getErrorRateColor(errorRate);
    if (color) {
      nodeDecorations[node.id] = { accentColor: color };
    }

    // Attach mini-stats to all nodes with this project_id
    const stats = buildNodeStats(metrics);
    if (stats) {
      nodeStats[node.id] = stats;
    }
  }

  // Thicken ingress edges
  if (traefik) {
    for (const edge of edges) {
      let projectId: number | null = null;

      // Determine if this is an ingress edge
      if (edge.source === traefik.name) {
        projectId = nodes.find((n) => n.name === edge.target)?.project_id ?? null;
      } else if (edge.target === traefik.name) {
        projectId = nodes.find((n) => n.name === edge.source)?.project_id ?? null;
      }

      if (projectId !== null && projectId !== 0) {
        const metrics = metricsByProjectId.get(projectId);
        if (metrics && metrics.request_rate.length) {
          const latestRequest = metrics.request_rate[metrics.request_rate.length - 1];
          const thickness = getEdgeThickness(latestRequest.rate_per_sec);
          const edgeKey = `${edge.source}:${edge.target}:${edge.network}`;
          edgeDecorations[edgeKey] = { thickness };
        }
      }
    }
  }

  return { nodeDecorations, edgeDecorations, nodeStats };
}

/**
 * Hook for project-specific topology overlay.
 * Fetches that project's metrics and builds decorations.
 */
export function useProjectTrafficOverlay(
  topology: Topology | null,
  projectId: number
): TrafficOverlayResult {
  const { data: projectMetrics, loading } = useProjectMetrics(projectId, undefined, {
    refetchInterval: 8000, // Match topology poll interval
  });

  return useMemo(() => {
    if (!topology) {
      return { nodeDecorations: {}, edgeDecorations: {}, nodeStats: {} };
    }
    return buildProjectOverlay(topology, projectMetrics);
  }, [topology, projectMetrics]);
}

/**
 * Hook for global/system topology overlay.
 * Fetches metrics for all projects present in the topology.
 *
 * Note: We fetch per-project metrics for accurate per-project error rates and request volumes.
 * This matches the metric granularity constraint: Traefik emits per-project-ingress metrics.
 */
export function useGlobalTrafficOverlay(
  topology: Topology | null
): TrafficOverlayResult {
  // Extract unique project IDs from topology (excluding 0 for core)
  const projectIds = useMemo(() => {
    if (!topology) return [];
    const ids = new Set<number>();
    for (const node of topology.nodes) {
      if (node.project_id !== 0 && node.project_id > 0) {
        ids.add(node.project_id);
      }
    }
    return Array.from(ids).sort((a, b) => a - b);
  }, [topology]);

  // Fetch metrics for each project in parallel
  // We use useEffect to manage multiple concurrent fetches
  const [metricsByProjectId, setMetricsByProjectId] = React.useState<Map<number, MetricsPanels>>(
    new Map()
  );
  const [metricsLoading, setMetricsLoading] = React.useState(false);

  React.useEffect(() => {
    if (projectIds.length === 0) {
      setMetricsByProjectId(new Map());
      return;
    }

    let isMounted = true;

    const fetchAllMetrics = async () => {
      setMetricsLoading(true);
      try {
        const results = await Promise.allSettled(
          projectIds.map((id) => getProjectMetrics(id))
        );

        if (!isMounted) return;

        const newMap = new Map<number, MetricsPanels>();
        for (let i = 0; i < projectIds.length; i++) {
          const result = results[i];
          if (result.status === "fulfilled") {
            newMap.set(projectIds[i], result.value);
          }
        }
        setMetricsByProjectId(newMap);
      } catch {
        // Silently fail; empty metrics will result in neutral decorations
      } finally {
        if (isMounted) {
          setMetricsLoading(false);
        }
      }
    };

    fetchAllMetrics();

    // Set up polling
    const interval = setInterval(fetchAllMetrics, 8000);

    return () => {
      isMounted = false;
      clearInterval(interval);
    };
  }, [projectIds]);

  return useMemo(() => {
    if (!topology) {
      return { nodeDecorations: {}, edgeDecorations: {}, nodeStats: {} };
    }
    return buildGlobalOverlay(topology, metricsByProjectId);
  }, [topology, metricsByProjectId]);
}
