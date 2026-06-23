const API_BASE = process.env.NEXT_PUBLIC_API_URL || "/api";

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
  repo_url: string;
  branch: string;
  domain: string;
  status: string;
  container_id?: string;
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
export const createProject = (data: Partial<Project>) =>
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

// Agent
export const chatWithAgent = (projectId: number, message: string) =>
  api<{ task_id: string }>(`/projects/${projectId}/agent/chat`, {
    method: "POST",
    body: JSON.stringify({ message }),
  });
export const getTask = (projectId: number, taskId: string) =>
  api<AgentTask>(`/projects/${projectId}/agent/tasks/${taskId}`);
