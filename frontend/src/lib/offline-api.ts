const OFFLINE_MODE = process.env.NEXT_PUBLIC_OFFLINE_MODE === "true";

const now = new Date().toISOString();

const project = {
  id: 1,
  name: "Sample storefront",
  source_type: "compose",
  repo_url: "github.com/example/storefront",
  branch: "main",
  domain: "storefront.localhost",
  status: "running",
  container_id: "sample-web",
  compose_yaml: "services:\n  web:\n    image: nginx:alpine",
  exposed_service: "web",
  created_at: now,
  updated_at: now,
};

const containers = [
  {
    id: "sample-web",
    name: "project-1-web",
    image: "nginx:alpine",
    status: "Up 2 hours",
    state: "running",
    ports: ["80/tcp"],
    created: now,
    labels: { "tamga.project_id": "1" },
    project_id: 1,
  },
  {
    id: "sample-proxy",
    name: "tamga-traefik-1",
    image: "traefik:v3.7",
    status: "Up 2 hours",
    state: "running",
    ports: ["80/tcp", "443/tcp"],
    created: now,
    labels: {},
    system_type: "proxy",
  },
];

const timestamps = Array.from({ length: 8 }, (_, index) =>
  new Date(Date.now() - (7 - index) * 60 * 60 * 1000).toISOString()
);

const metrics = {
  project_id: 0,
  from: timestamps[0],
  to: timestamps[timestamps.length - 1],
  resolution: "hour" as const,
  request_rate: timestamps.map((bucket_start, index) => ({ bucket_start, count: 120 + index * 18, rate_per_sec: 2 + index / 3 })),
  status_class: timestamps.map((bucket_start, index) => ({ bucket_start, count_2xx: 110 + index * 16, count_3xx: 4, count_4xx: 3 + (index % 2), count_5xx: index === 5 ? 3 : 1, error_rate: index === 5 ? 0.025 : 0.008 })),
  latency: timestamps.map((bucket_start, index) => ({ bucket_start, p50: 0.04 + index / 1000, p95: 0.11 + index / 100, p99: 0.19 + index / 100 })),
  bandwidth: timestamps.map((bucket_start, index) => ({ bucket_start, bytes_in: 120000 + index * 15000, bytes_out: 480000 + index * 28000 })),
};

const topology = {
  nodes: [
    { id: "sample-proxy", name: "tamga-traefik-1", image: "traefik:v3.7", type: "proxy", project_id: 0, system_type: "traefik", state: "running", status: "Up 2 hours", stats_ref: "/system/containers/sample-proxy/stats" },
    { id: "sample-web", name: "project-1-web", image: "nginx:alpine", type: "web", project_id: 1, system_type: "", state: "running", status: "Up 2 hours", stats_ref: "/system/containers/sample-web/stats", traffic_ref: "project-1" },
  ],
  edges: [{ network: "project-net-1", source: "tamga-traefik-1", target: "project-1-web" }],
};

export function isOfflineMode(): boolean {
  return OFFLINE_MODE;
}

export async function offlineApi(path: string, options: RequestInit = {}): Promise<unknown> {
  const method = options.method || "GET";

  if (path === "/auth/status") return { setup: true };
  if (path === "/auth/login") return { token: "offline-preview-token" };
  if (path === "/auth/setup") return { id: 1, created_at: now };
  if (path === "/auth/me") return { user_id: 1 };

  if (path === "/projects") {
    if (method === "GET") return [project];
    return { ...project, ...(options.body ? JSON.parse(String(options.body)) : {}) };
  }
  if (/^\/projects\/\d+$/.test(path)) return project;
  if (/^\/projects\/\d+\/deployments$/.test(path)) return [{ id: 1, project_id: 1, status: "success", commit_sha: "preview", logs: "Offline preview deployment", created_at: now, updated_at: now }];
  if (/^\/projects\/\d+\/logs$/.test(path)) return { logs: "Offline preview: no live container logs." };
  if (/^\/projects\/\d+\/env-vars$/.test(path)) return [{ id: 1, project_id: 1, key: "NODE_ENV", value: "production", created_at: now, updated_at: now }];
  if (/^\/projects\/\d+\/agent\/sessions$/.test(path)) return [];
  if (/^\/projects\/\d+\/metrics/.test(path)) return { ...metrics, project_id: 1 };
  if (/^\/projects\/\d+\/topology$/.test(path)) return topology;

  if (path === "/system/containers") return containers;
  if (/^\/system\/containers\/[^/]+\/logs/.test(path)) return { logs: "Offline preview: no live container logs." };
  if (/^\/system\/containers\/[^/]+\/stats/.test(path)) return { cpu: { percent: 7.2, usage: 10000000, system: 40000000 }, mem: { usage: 73400320, limit: 536870912, percent: 13.7 }, net: { rx_bytes: 225000, tx_bytes: 815000, rx_packets: 320, tx_packets: 540 } };
  if (/^\/system\/containers\/[^/]+$/.test(path)) return containers[0];
  if (path === "/system/info") return { version: "offline-preview", os: "Linux", architecture: "amd64", containers: 2, running: 2, paused: 0, stopped: 0, images: 2, name: "Tamga preview", kernel: "offline", driver: "overlay2", memory: 8589934592, cpus: 8 };
  if (/^\/system\/metrics/.test(path)) return metrics;
  if (path === "/system/topology") return topology;
  if (path === "/system/resource-limits") return { memory_bytes: 536870912, nano_cpus: 1000000000 };
  if (path === "/system/session-idle-timeout") return { timeout_seconds: 0 };
  if (path === "/system/git-credential") return { provider: "github", username: "preview", has_token: false, created_at: now, updated_at: now };
  if (path === "/system/egress/mode") return { mode: "whitelist" };
  if (path === "/system/egress-whitelist") return [{ id: 1, domain: "github.com", created_at: now }];
  if (path === "/system/egress-blacklist") return [];

  if (path === "/code/projects") return [{ id: 1, name: "Sample storefront", type: "project", path: "/workspace/storefront", project_id: 1 }];
  if (/^\/code\/\d+\/tree$/.test(path)) return [{ name: "app", path: "app", type: "dir" }, { name: "page.tsx", path: "app/page.tsx", type: "file", size: 284 }];
  if (/^\/code\/\d+\/file/.test(path)) return { content: "export default function Page() {\n  return <main>Offline preview</main>;\n}\n" };

  if (method !== "GET") return undefined;
  return {};
}
