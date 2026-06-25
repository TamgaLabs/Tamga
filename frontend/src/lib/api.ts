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

export type Project = {
  id: number;
  name: string;
  source_type: "local" | "remote";
  repo_url: string;
  branch: string;
  domain: string;
  status: string;
  container_id?: string;
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

export type AgentTask = {
  id: string;
  project_id: number;
  message: string;
  status: "pending" | "processing" | "completed" | "failed";
  response?: string;
  diff?: string;
  created_at: string;
  completed_at?: string;
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

// Agent
export const chatWithAgent = (projectId: number, message: string) =>
  api<{ task_id: string }>(`/projects/${projectId}/agent/chat`, {
    method: "POST",
    body: JSON.stringify({ message }),
  });
export const getTask = (projectId: number, taskId: string) =>
  api<AgentTask>(`/projects/${projectId}/agent/tasks/${taskId}`);

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
export const chatWithCodeAgent = (projectId: number, message: string) =>
  api<{ task_id: string }>(`/code/${projectId}/agent/chat`, {
    method: "POST",
    body: JSON.stringify({ message }),
  });
export const getCodeTask = (projectId: number, taskId: string) =>
  api<AgentTask>(`/code/${projectId}/agent/tasks/${taskId}`);
export const listCodeTasks = (projectId: number) =>
  api<AgentTask[]>(`/code/${projectId}/agent/tasks`);
export const getCodeAgentStatus = (projectId: number) =>
  api<{ running: boolean }>(`/code/${projectId}/agent/status`);
export const startCodeAgent = (projectId: number) =>
  api<void>(`/code/${projectId}/agent/start`, { method: "POST" });
export const stopCodeAgent = (projectId: number) =>
  api<void>(`/code/${projectId}/agent/stop`, { method: "POST" });

// Agent tasks for project
export const listAgentTasks = (projectId: number) =>
  api<AgentTask[]>(`/projects/${projectId}/agent/tasks`);
