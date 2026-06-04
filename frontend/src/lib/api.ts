const API = process.env.NEXT_PUBLIC_API_URL || '';

function base(): string {
  return API || (typeof window !== 'undefined' ? window.location.origin : '');
}

function token(): string | null {
  if (typeof window === 'undefined') return null;
  return localStorage.getItem('token');
}

export function setToken(t: string) {
  localStorage.setItem('token', t);
}

export function removeToken() {
  localStorage.removeItem('token');
}

export function isAuthenticated(): boolean {
  return !!token();
}

async function request<T>(method: string, path: string, body?: unknown): Promise<T> {
  const headers: Record<string, string> = { 'Content-Type': 'application/json' };
  const t = token();
  if (t) headers['Authorization'] = `Bearer ${t}`;

  const res = await fetch(`${base()}${path}`, {
    method,
    headers,
    body: body ? JSON.stringify(body) : undefined,
  });

  if (!res.ok) {
    const err = await res.json().catch(() => ({ error: res.statusText }));
    throw new Error(err.error || 'request failed');
  }

  if (res.status === 204) return undefined as T;
  return res.json();
}

export const api = {
  get: <T>(path: string) => request<T>('GET', path),
  post: <T>(path: string, body?: unknown) => request<T>('POST', path, body),
  put: <T>(path: string, body?: unknown) => request<T>('PUT', path, body),
  del: <T>(path: string) => request<T>('DELETE', path),
};

export const auth = {
  register: (name: string, email: string, password: string) =>
    api.post<{ token: string; user: { id: string; name: string; email: string } }>('/api/auth/register', { name, email, password }),
  login: (email: string, password: string) =>
    api.post<{ token: string; user: { id: string; name: string; email: string } }>('/api/auth/login', { email, password }),
  me: () => api.get<{ id: string; name: string; email: string }>('/api/auth/me'),
};

export const projects = {
  list: () => api.get<Project[]>('/api/projects'),
  get: (id: string) => api.get<Project>(`/api/projects/${id}`),
  create: (name: string, description?: string) =>
    api.post<Project>('/api/projects', { name, description }),
  update: (id: string, name: string, description?: string) =>
    api.put<Project>(`/api/projects/${id}`, { name, description }),
  del: (id: string) => api.del<void>(`/api/projects/${id}`),
};

export const domains = {
  list: (projectId: string) => api.get<Domain[]>(`/api/projects/${projectId}/domains`),
  create: (projectId: string, domain: string) =>
    api.post<Domain>(`/api/projects/${projectId}/domains`, { domain }),
  del: (id: string) => api.del<void>(`/api/domains/${id}`),
};

export const envVars = {
  list: (projectId: string) => api.get<EnvVar[]>(`/api/projects/${projectId}/env-vars`),
  create: (projectId: string, key: string, value: string) =>
    api.post<EnvVar>(`/api/projects/${projectId}/env-vars`, { key, value }),
  update: (id: string, key: string, value: string) =>
    api.put<EnvVar>(`/api/env-vars/${id}`, { key, value }),
  del: (id: string) => api.del<void>(`/api/env-vars/${id}`),
};

export const git = {
  get: (projectId: string) => api.get<GitRepo>(`/api/projects/${projectId}/git`),
  connect: (projectId: string, url: string, branch?: string) =>
    api.post<GitRepo>(`/api/projects/${projectId}/git`, { url, branch: branch || 'main' }),
  disconnect: (id: string) => api.del<void>(`/api/git/${id}`),
};

export const deployments = {
  list: (projectId: string) => api.get<Deployment[]>(`/api/projects/${projectId}/deployments`),
  create: (projectId: string) =>
    api.post<Deployment>(`/api/projects/${projectId}/deployments`),
  get: (id: string) => api.get<Deployment>(`/api/deployments/${id}`),
  restart: (id: string) => api.post<{ message: string }>(`/api/deployments/${id}/restart`),
  logs: (id: string) => api.get<DeploymentLog[]>(`/api/deployments/${id}/logs`),
  logStream: (id: string): WebSocket => {
    const b = base().replace(/^http/, 'ws');
    const t = token();
    return new WebSocket(`${b}/api/deployments/${id}/logs/stream`);
  },
};

export interface Project {
  id: string;
  name: string;
  description: string;
  user_id: string;
  created_at: string;
  updated_at: string;
}

export interface Domain {
  id: string;
  project_id: string;
  domain: string;
  verified: boolean;
  created_at: string;
  updated_at: string;
}

export interface EnvVar {
  id: string;
  project_id: string;
  key: string;
  value: string;
  created_at: string;
  updated_at: string;
}

export interface GitRepo {
  id: string;
  project_id: string;
  url: string;
  branch: string;
  created_at: string;
  updated_at: string;
}

export interface Deployment {
  id: string;
  project_id: string;
  status: string;
  commit_sha: string;
  commit_message: string;
  image_tag: string;
  container_id: string;
  domain: string;
  created_at: string;
  updated_at: string;
}

export interface DeploymentLog {
  id: string;
  deployment_id: string;
  stream: string;
  message: string;
  created_at: string;
}
