// Types matching backend/internal/service/topology_service.go

export interface TopologyNode {
  id: string; // container ID
  name: string; // container name
  image: string; // container image
  type: string; // classified type: cache, database, proxy, web, queue, generic
  project_id: number; // project ID (0 if system container)
  system_type: string; // system type (e.g., "tamga-backend-1", "agent-system")
  state: string; // container state (running/exited/etc.)
  status: string; // human-readable status (e.g., "Up 3 hours")
  stats_ref: string; // ref to per-container stats endpoint
  traffic_ref?: string; // ref for metrics (project-<id>), only for project nodes
}

export interface TopologyEdge {
  network: string; // network name
  source: string; // source container name
  target: string; // target container name
}

export interface Topology {
  nodes: TopologyNode[];
  edges: TopologyEdge[];
}
