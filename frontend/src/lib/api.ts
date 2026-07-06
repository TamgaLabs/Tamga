const API_BASE = (() => {
  if (typeof window !== "undefined") {
    return "/api";
  }
  return process.env.NEXT_PUBLIC_API_URL || "http://tamga-backend:8080/api";
})();

function getToken(): string | null {
  if (typeof window === "undefined") return null;
  return localStorage.getItem("token");
}

export async function api<T>(
  path: string,
  options: RequestInit = {}
): Promise<T> {
  const token = getToken();
  const headers: Record<string, string> = {
    "Content-Type": "application/json",
    ...(options.headers as Record<string, string>),
  };
  if (token) headers["Authorization"] = `Bearer ${token}`;

  const res = await fetch(`${API_BASE}${path}`, { ...options, headers });
  if (!res.ok) {
    const text = await res.text();
    throw new Error(text || `HTTP ${res.status}`);
  }
  return res.json();
}

export type User = {
  id: number;
  created_at: string;
};

export type AgentProvider = {
  id: string;
  name: string;
  type: "docker";
  image?: string;
  is_default: boolean;
  created_at: string;
  updated_at: string;
};

export type Project = {
  id: number;
  name: string;
  source_type: "local" | "remote";
  repo_url: string;
  branch: string;
  domain: string;
  status: string;
  container_id?: string;
  agent_provider_id?: string;
  created_at: string;
  updated_at: string;
};

export type Deployment = {
  id: number;
  project_id: number;
  status: string;
  commit_sha?: string;
  logs?: string;
  created_at: string;
  updated_at: string;
};

export type EnvVar = {
  id: number;
  project_id: number;
  key: string;
  value: string;
  created_at: string;
  updated_at: string;
};

// Auth
export const checkSetup = () => api<{ setup: boolean }>("/auth/status");
export const setup = (password: string) =>
  api<User>("/auth/setup", {
    method: "POST",
    body: JSON.stringify({ password }),
  });
export const login = (password: string) =>
  api<{ token: string }>("/auth/login", {
    method: "POST",
    body: JSON.stringify({ password }),
  });
export const me = () => api<{ user_id: number }>("/auth/me");

// Projects
export const listProjects = () => api<Project[]>("/projects");
export const getProject = (id: number) => api<Project>(`/projects/${id}`);
export const createProject = (data: { name: string; source_type: string; repo_url: string; domain: string }) =>
  api<Project>("/projects", {
    method: "POST",
    body: JSON.stringify(data),
  });
export const deleteProject = (id: number) =>
  api<void>(`/projects/${id}`, { method: "DELETE" });
export const restartProject = (id: number) =>
  api<void>(`/projects/${id}/restart`, { method: "POST" });
export const getProjectLogs = (id: number) =>
  api<{ logs: string }>(`/projects/${id}/logs`);
export const updateProject = (id: number, data: Partial<Project>) =>
  api<Project>(`/projects/${id}`, { method: "PUT", body: JSON.stringify(data) });

// Deployments
export const listDeployments = (projectId: number) =>
  api<Deployment[]>(`/projects/${projectId}/deployments`);

// Env Vars
export const listEnvVars = (projectId: number) =>
  api<EnvVar[]>(`/projects/${projectId}/env-vars`);
export const createEnvVar = (projectId: number, key: string, value: string) =>
  api<EnvVar>(`/projects/${projectId}/env-vars`, {
    method: "POST",
    body: JSON.stringify({ key, value }),
  });
export const deleteEnvVar = (projectId: number, envVarId: number) =>
  api<void>(`/projects/${projectId}/env-vars/${envVarId}`, { method: "DELETE" });

// Containers
export type ContainerInfo = {
  id: string;
  name: string;
  image: string;
  status: string;
  state: string;
  ports: string[];
  created: string;
  labels: Record<string, string>;
  project_id?: number;
  system_type?: string;
};

export type ContainerStats = {
  cpu: { percent: number; usage: number; system: number; percpu?: number[] };
  mem: { usage: number; limit: number; percent: number };
  net: { rx_bytes: number; tx_bytes: number; rx_packets: number; tx_packets: number };
};

export type DockerInfo = {
  version: string;
  os: string;
  architecture: string;
  containers: number;
  running: number;
  paused: number;
  stopped: number;
  images: number;
  name: string;
  kernel: string;
  driver: string;
  memory: number;
  cpus: number;
};

export const listContainers = () => api<ContainerInfo[]>("/system/containers");
export const getContainer = (id: string) => api<any>(`/system/containers/${id}`);
export const startContainer = (id: string) =>
  api<void>(`/system/containers/${id}/start`, { method: "POST" });
export const stopContainer = (id: string) =>
  api<void>(`/system/containers/${id}/stop`, { method: "POST" });
export const restartContainer = (id: string) =>
  api<void>(`/system/containers/${id}/restart`, { method: "POST" });
export const removeContainer = (id: string) =>
  api<void>(`/system/containers/${id}`, { method: "DELETE" });
export const getContainerLogs = (id: string, tail?: number) =>
  api<{ logs: string }>(`/system/containers/${id}/logs?tail=${tail || 100}`);
export const getContainerStats = (id: string) =>
  api<ContainerStats>(`/system/containers/${id}/stats`);
export const updateContainerResources = (id: string, data: { memory?: number; nano_cpus?: number }) =>
  api<void>(`/system/containers/${id}/resources`, {
    method: "PUT",
    body: JSON.stringify(data),
  });
export const systemPrune = (data?: { containers?: boolean; images?: boolean; volumes?: boolean; networks?: boolean; all?: boolean }) =>
  api<{ status: string }>("/system/prune", {
    method: "POST",
    body: JSON.stringify(data || { all: true }),
  });
export const systemInfo = () => api<DockerInfo>("/system/info");

// Code
export type Codebase = {
  id: number;
  name: string;
  type: "project" | "system";
  path: string;
  project_id?: number;
};

export type FileEntry = {
  name: string;
  path: string;
  type: "file" | "dir";
  size?: number;
};

export const listCodebases = () =>
  api<Codebase[]>("/code/projects");
export const getFileTree = (projectId: number) =>
  api<FileEntry[]>(`/code/${projectId}/tree`);
export const readFile = (projectId: number, path: string) =>
  api<{ content: string }>(`/code/${projectId}/file?path=${encodeURIComponent(path)}`);
export const writeFile = (projectId: number, path: string, content: string) =>
  api<void>(`/code/${projectId}/file?path=${encodeURIComponent(path)}`, {
    method: "PUT",
    body: JSON.stringify({ content }),
  });

// Agent terminal: WebSocket into an on-demand sandbox container. Built as a
// plain URL (not via the `api()` json helper) since the browser WebSocket API
// can't set an Authorization header - the token travels as a query param.
export function agentTerminalUrl(projectId: number): string {
  const token = getToken() || "";
  const proto = typeof window !== "undefined" && window.location.protocol === "https:" ? "wss:" : "ws:";
  const host = typeof window !== "undefined" ? window.location.host : "";
  return `${proto}//${host}/api/projects/${projectId}/agent/terminal?token=${encodeURIComponent(token)}`;
}

// Agent Providers
export const listAgentProviders = () =>
  api<AgentProvider[]>("/agent-providers");
export const getAgentProvider = (id: string) =>
  api<AgentProvider>(`/agent-providers/${id}`);
export const createAgentProvider = (data: {
  name: string;
  image?: string;
  type?: string;
}) =>
  api<AgentProvider>("/agent-providers", {
    method: "POST",
    body: JSON.stringify(data),
  });
export const updateAgentProvider = (id: string, data: Partial<AgentProvider>) =>
  api<AgentProvider>(`/agent-providers/${id}`, {
    method: "PUT",
    body: JSON.stringify(data),
  });
export const deleteAgentProvider = (id: string) =>
  api<void>(`/agent-providers/${id}`, { method: "DELETE" });
