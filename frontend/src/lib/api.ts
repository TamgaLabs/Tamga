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

  // Handle empty-body responses (204 No Content, Content-Length: 0, etc.)
  if (res.status === 204 || res.headers.get("content-length") === "0") {
    return undefined as T;
  }

  // For other status codes, attempt to parse JSON, but fall back to undefined
  // if the body is truly empty (e.g., 200 with no body)
  try {
    return await res.json();
  } catch (e) {
    // If parsing fails due to empty body, return undefined
    if (e instanceof SyntaxError && e.message.includes("JSON")) {
      return undefined as T;
    }
    throw e;
  }
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
//
// With no `sessionId`, the backend creates a brand new session. Passing a
// `sessionId` reattaches to an existing, still-live session instead (the
// backend replays its scrollback before streaming live) - see FEAT-015.
export function agentTerminalUrl(projectId: number, sessionId?: string): string {
  const token = getToken() || "";
  const proto = typeof window !== "undefined" && window.location.protocol === "https:" ? "wss:" : "ws:";
  const host = typeof window !== "undefined" ? window.location.host : "";
  const session = sessionId ? `&session=${encodeURIComponent(sessionId)}` : "";
  return `${proto}//${host}/api/projects/${projectId}/agent/terminal?token=${encodeURIComponent(token)}${session}`;
}

// Agent terminal sessions (see FEAT-015): a project can have multiple
// concurrent, server-persisted terminal sessions (capped at 10). These
// mirror backend/internal/service/terminal_session.go's SessionInfo.
export type AgentSession = {
  id: string;
  created_at: string;
  connected: boolean;
};

export const listAgentSessions = (projectId: number) =>
  api<AgentSession[]>(`/projects/${projectId}/agent/sessions`);
export const terminateAgentSession = (projectId: number, sessionId: string) =>
  api<void>(`/projects/${projectId}/agent/sessions/${sessionId}`, { method: "DELETE" });

// Agent sandbox default resource limit (see FEAT-007)
export type ResourceLimit = {
  memory_bytes: number;
  nano_cpus: number;
};

export const getResourceLimit = () =>
  api<ResourceLimit>("/system/resource-limits");
export const updateResourceLimit = (data: ResourceLimit) =>
  api<ResourceLimit>("/system/resource-limits", {
    method: "PUT",
    body: JSON.stringify(data),
  });

// Global git credential (see FEAT-008): used both by the backend for
// `git clone`/`pull` and injected into every agent sandbox for
// `git commit`/`push`. Single value, not a list - same GET/PUT shape as
// resource limits, plus DELETE to clear it.
export type GitCredential = {
  provider: string;
  username: string;
  has_token: boolean;
  created_at: string;
  updated_at: string;
};

export const getGitCredential = () =>
  api<GitCredential>("/system/git-credential");
export const setGitCredential = (data: { provider: string; username?: string; token: string }) =>
  api<GitCredential>("/system/git-credential", {
    method: "PUT",
    body: JSON.stringify(data),
  });
export const deleteGitCredential = () =>
  api<void>("/system/git-credential", { method: "DELETE" });

// Agent sandbox egress whitelist (see FEAT-006): domains the sandbox
// egress proxy will permit outbound requests to. Global setting, multiple
// entries.
export type WhitelistDomain = {
  id: number;
  domain: string;
  created_at: string;
};

export const listWhitelist = () =>
  api<WhitelistDomain[]>("/system/egress-whitelist");
export const addWhitelistDomain = (domain: string) =>
  api<WhitelistDomain>("/system/egress-whitelist", {
    method: "POST",
    body: JSON.stringify({ domain }),
  });
export const deleteWhitelistDomain = (id: number) =>
  api<void>(`/system/egress-whitelist/${id}`, { method: "DELETE" });

// Agent sandbox egress mode + blacklist (see FEAT-016): the mode setting
// picks which of the whitelist/blacklist lists (if either) the sandbox
// egress proxy enforces. Blacklist is a mirror of the whitelist list
// above, kept as a separate endpoint/type since the two lists are edited
// independently.
export type EgressMode = "open" | "whitelist" | "blacklist";

export type EgressSettings = {
  mode: EgressMode;
};

export const getEgressMode = () =>
  api<EgressSettings>("/system/egress/mode");
export const setEgressMode = (mode: EgressMode) =>
  api<EgressSettings>("/system/egress/mode", {
    method: "PUT",
    body: JSON.stringify({ mode }),
  });

export type BlacklistDomain = {
  id: number;
  domain: string;
  created_at: string;
};

export const listBlacklist = () =>
  api<BlacklistDomain[]>("/system/egress-blacklist");
export const addBlacklistDomain = (domain: string) =>
  api<BlacklistDomain>("/system/egress-blacklist", {
    method: "POST",
    body: JSON.stringify({ domain }),
  });
export const deleteBlacklistDomain = (id: number) =>
  api<void>(`/system/egress-blacklist/${id}`, { method: "DELETE" });

// Detached terminal session idle timeout (see FEAT-022): how long a
// session may sit with no attached WebSocket before the backend
// auto-terminates it. Global setting, single value. 0 means Never (the
// default) - sessions persist until explicitly terminated.
export type IdleTimeoutSettings = {
  timeout_seconds: number;
};

export const getIdleTimeout = () =>
  api<IdleTimeoutSettings>("/system/session-idle-timeout");
export const setIdleTimeout = (timeoutSeconds: number) =>
  api<IdleTimeoutSettings>("/system/session-idle-timeout", {
    method: "PUT",
    body: JSON.stringify({ timeout_seconds: timeoutSeconds }),
  });
