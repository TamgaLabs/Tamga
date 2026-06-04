"use client";

import { use, useEffect, useState, useRef, useCallback } from "react";
import { useRouter } from "next/navigation";
import Link from "next/link";
import {
  projects, git as gitApi, domains as domainsApi, envVars as envVarsApi,
  deployments as deploymentsApi,
  removeToken, isAuthenticated,
  type Project, type GitRepo, type Domain, type EnvVar, type Deployment,
} from "@/lib/api";

type Tab = "overview" | "domains" | "env" | "deployments";

export default function ProjectPage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = use(params);
  const router = useRouter();
  const [tab, setTab] = useState<Tab>("overview");
  const [project, setProject] = useState<Project | null>(null);
  const [gitRepo, setGitRepo] = useState<GitRepo | null>(null);
  const [domainList, setDomainList] = useState<Domain[]>([]);
  const [envList, setEnvList] = useState<EnvVar[]>([]);
  const [deployList, setDeployList] = useState<Deployment[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    if (!isAuthenticated()) { router.replace("/login"); return; }
    Promise.all([
      projects.get(id).then(setProject),
      gitApi.get(id).then(setGitRepo).catch(() => {}),
      domainsApi.list(id).then(setDomainList).catch(() => {}),
      envVarsApi.list(id).then(setEnvList).catch(() => {}),
      deploymentsApi.list(id).then(setDeployList).catch(() => {}),
    ]).catch(() => router.replace("/login")).finally(() => setLoading(false));
  }, [id, router]);

  const refresh = useCallback(() => {
    domainsApi.list(id).then(setDomainList).catch(() => {});
    envVarsApi.list(id).then(setEnvList).catch(() => {});
    deploymentsApi.list(id).then(setDeployList).catch(() => {});
  }, [id]);

  if (loading) return <div className="p-8 text-zinc-400">Loading...</div>;
  if (!project) return <div className="p-8 text-zinc-400">Project not found</div>;

  const tabs: { key: Tab; label: string }[] = [
    { key: "overview", label: "Overview" },
    { key: "domains", label: "Domains" },
    { key: "env", label: "Env Vars" },
    { key: "deployments", label: "Deployments" },
  ];

  return (
    <div className="mx-auto max-w-4xl p-8">
      <div className="mb-6 flex items-center justify-between">
        <div>
          <Link href="/dashboard" className="text-sm text-zinc-500 hover:text-zinc-300">&larr; Projects</Link>
          <h1 className="mt-1 text-2xl font-bold">{project.name}</h1>
          {project.description && <p className="mt-1 text-sm text-zinc-400">{project.description}</p>}
        </div>
      </div>

      <div className="mb-6 flex gap-1 border-b border-zinc-800">
        {tabs.map((t) => (
          <button
            key={t.key}
            onClick={() => setTab(t.key)}
            className={`px-4 py-2 text-sm font-medium border-b-2 transition-colors ${
              tab === t.key ? "border-blue-500 text-blue-400" : "border-transparent text-zinc-500 hover:text-zinc-300"
            }`}
          >
            {t.label}
          </button>
        ))}
      </div>

      {tab === "overview" && <OverviewTab projectId={id} gitRepo={gitRepo} onGitChange={setGitRepo} />}
      {tab === "domains" && <DomainsTab projectId={id} domains={domainList} onRefresh={refresh} />}
      {tab === "env" && <EnvTab projectId={id} vars={envList} onRefresh={refresh} />}
      {tab === "deployments" && <DeploymentsTab projectId={id} deployments={deployList} onRefresh={refresh} />}
    </div>
  );
}

function OverviewTab({ projectId, gitRepo, onGitChange }: {
  projectId: string;
  gitRepo: GitRepo | null;
  onGitChange: (r: GitRepo | null) => void;
}) {
  const [url, setUrl] = useState("");
  const [branch, setBranch] = useState("main");
  const [error, setError] = useState("");

  async function handleConnect(e: React.FormEvent) {
    e.preventDefault();
    setError("");
    try {
      const r = await gitApi.connect(projectId, url, branch);
      onGitChange(r);
      setUrl("");
      setBranch("main");
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : "failed to connect");
    }
  }

  async function handleDisconnect() {
    if (!gitRepo) return;
    await gitApi.disconnect(gitRepo.id);
    onGitChange(null);
  }

  return (
    <div className="space-y-6">
      <div className="rounded-xl border border-zinc-800 bg-zinc-900 p-6">
        <h2 className="mb-4 text-lg font-semibold">Git Repository</h2>
        {gitRepo ? (
          <div className="space-y-2">
            <p className="text-sm"><span className="text-zinc-500">URL:</span> {gitRepo.url}</p>
            <p className="text-sm"><span className="text-zinc-500">Branch:</span> {gitRepo.branch}</p>
            <button onClick={handleDisconnect} className="mt-3 rounded-lg border border-red-800 px-3 py-1 text-sm text-red-400 hover:bg-red-900/30">
              Disconnect
            </button>
          </div>
        ) : (
          <form onSubmit={handleConnect} className="space-y-3">
            {error && <p className="text-sm text-red-400">{error}</p>}
            <input
              placeholder="Git repository URL (e.g. https://github.com/user/repo.git)"
              value={url}
              onChange={(e) => setUrl(e.target.value)}
              required
              className="w-full rounded-lg border border-zinc-700 bg-zinc-800 px-4 py-2 text-sm focus:border-blue-500 focus:outline-none"
            />
            <input
              placeholder="Branch (default: main)"
              value={branch}
              onChange={(e) => setBranch(e.target.value)}
              className="w-full rounded-lg border border-zinc-700 bg-zinc-800 px-4 py-2 text-sm focus:border-blue-500 focus:outline-none"
            />
            <button type="submit" className="rounded-lg bg-blue-600 px-4 py-2 text-sm font-semibold hover:bg-blue-700">
              Connect
            </button>
          </form>
        )}
      </div>
    </div>
  );
}

function DomainsTab({ projectId, domains, onRefresh }: {
  projectId: string;
  domains: Domain[];
  onRefresh: () => void;
}) {
  const [domain, setDomain] = useState("");
  const [error, setError] = useState("");

  async function handleAdd(e: React.FormEvent) {
    e.preventDefault();
    setError("");
    try {
      await domainsApi.create(projectId, domain);
      setDomain("");
      onRefresh();
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : "failed to add domain");
    }
  }

  async function handleRemove(id: string) {
    await domainsApi.del(id);
    onRefresh();
  }

  return (
    <div className="space-y-4">
      <form onSubmit={handleAdd} className="flex gap-3">
        <input
          placeholder="example.com"
          value={domain}
          onChange={(e) => setDomain(e.target.value)}
          required
          className="flex-1 rounded-lg border border-zinc-700 bg-zinc-800 px-4 py-2 text-sm focus:border-blue-500 focus:outline-none"
        />
        <button type="submit" className="rounded-lg bg-blue-600 px-4 py-2 text-sm font-semibold hover:bg-blue-700">Add</button>
      </form>
      {error && <p className="text-sm text-red-400">{error}</p>}
      {domains.length === 0 ? (
        <p className="text-sm text-zinc-500">No domains configured.</p>
      ) : (
        <div className="space-y-2">
          {domains.map((d) => (
            <div key={d.id} className="flex items-center justify-between rounded-lg border border-zinc-800 bg-zinc-900 px-4 py-3">
              <div>
                <span className="text-sm">{d.domain}</span>
                {d.verified ? (
                  <span className="ml-2 rounded bg-green-900/50 px-2 py-0.5 text-xs text-green-400">verified</span>
                ) : (
                  <span className="ml-2 rounded bg-yellow-900/50 px-2 py-0.5 text-xs text-yellow-400">pending</span>
                )}
              </div>
              <button onClick={() => handleRemove(d.id)} className="text-sm text-red-400 hover:text-red-300">Remove</button>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}

function EnvTab({ projectId, vars, onRefresh }: {
  projectId: string;
  vars: EnvVar[];
  onRefresh: () => void;
}) {
  const [key, setKey] = useState("");
  const [value, setValue] = useState("");
  const [editing, setEditing] = useState<string | null>(null);
  const [error, setError] = useState("");

  async function handleAdd(e: React.FormEvent) {
    e.preventDefault();
    setError("");
    try {
      await envVarsApi.create(projectId, key, value);
      setKey("");
      setValue("");
      onRefresh();
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : "failed to add env var");
    }
  }

  async function handleUpdate(id: string) {
    await envVarsApi.update(id, key, value);
    setEditing(null);
    setKey("");
    setValue("");
    onRefresh();
  }

  async function handleRemove(id: string) {
    await envVarsApi.del(id);
    onRefresh();
  }

  function startEdit(v: EnvVar) {
    setEditing(v.id);
    setKey(v.key);
    setValue(v.value);
  }

  return (
    <div className="space-y-4">
      <form onSubmit={editing ? (e) => { e.preventDefault(); handleUpdate(editing); } : handleAdd} className="flex gap-3">
        <input
          placeholder="KEY"
          value={key}
          onChange={(e) => setKey(e.target.value)}
          required
          className="flex-1 rounded-lg border border-zinc-700 bg-zinc-800 px-4 py-2 text-sm font-mono focus:border-blue-500 focus:outline-none"
        />
        <input
          placeholder="value"
          value={value}
          onChange={(e) => setValue(e.target.value)}
          className="flex-1 rounded-lg border border-zinc-700 bg-zinc-800 px-4 py-2 text-sm font-mono focus:border-blue-500 focus:outline-none"
        />
        <button type="submit" className="rounded-lg bg-blue-600 px-4 py-2 text-sm font-semibold hover:bg-blue-700">
          {editing ? "Update" : "Add"}
        </button>
        {editing && (
          <button type="button" onClick={() => { setEditing(null); setKey(""); setValue(""); }} className="text-sm text-zinc-500">
            Cancel
          </button>
        )}
      </form>
      {error && <p className="text-sm text-red-400">{error}</p>}
      {vars.length === 0 ? (
        <p className="text-sm text-zinc-500">No environment variables.</p>
      ) : (
        <div className="space-y-2">
          {vars.map((v) => (
            <div key={v.id} className="flex items-center justify-between rounded-lg border border-zinc-800 bg-zinc-900 px-4 py-3">
              <div className="font-mono text-sm">
                <span className="text-blue-400">{v.key}</span>
                <span className="text-zinc-600">=</span>
                <span className="text-zinc-300">{v.value}</span>
              </div>
              <div className="flex gap-2">
                <button onClick={() => startEdit(v)} className="text-sm text-zinc-500 hover:text-zinc-300">Edit</button>
                <button onClick={() => handleRemove(v.id)} className="text-sm text-red-400 hover:text-red-300">Remove</button>
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}

function DeploymentsTab({ projectId, deployments, onRefresh }: {
  projectId: string;
  deployments: Deployment[];
  onRefresh: () => void;
}) {
  const [selected, setSelected] = useState<Deployment | null>(null);
  const [logLines, setLogLines] = useState<{ stream: string; message: string; ts: string }[]>([]);
  const wsRef = useRef<WebSocket | null>(null);
  const logEndRef = useRef<HTMLDivElement>(null);

  useEffect(() => { logEndRef.current?.scrollIntoView({ behavior: "smooth" }); }, [logLines]);

  async function handleDeploy() {
    await deploymentsApi.create(projectId);
    onRefresh();
  }

  async function handleRedeploy(id: string) {
    await deploymentsApi.restart(id);
    onRefresh();
  }

  function viewLogs(d: Deployment) {
    setSelected(d);
    setLogLines([]);

    deploymentsApi.logs(d.id).then((logs) => {
      setLogLines(logs.map((l) => ({ stream: l.stream, message: l.message, ts: l.created_at })));
    });

    const ws = deploymentsApi.logStream(d.id);
    ws.onmessage = (event) => {
      const msg = JSON.parse(event.data);
      if (msg.type === "log") {
        setLogLines((prev) => [...prev, { stream: msg.stream, message: msg.message, ts: msg.timestamp }]);
      }
    };
    wsRef.current = ws;
  }

  function closeLogs() {
    wsRef.current?.close();
    wsRef.current = null;
    setSelected(null);
    setLogLines([]);
  }

  function statusColor(s: string) {
    switch (s) {
      case "running": return "text-green-400";
      case "failed": return "text-red-400";
      case "building": return "text-yellow-400";
      default: return "text-zinc-400";
    }
  }

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <h2 className="text-lg font-semibold">Deployments</h2>
        <button onClick={handleDeploy} className="rounded-lg bg-green-600 px-4 py-2 text-sm font-semibold hover:bg-green-700">
          Deploy Now
        </button>
      </div>

      {deployments.length === 0 ? (
        <p className="text-sm text-zinc-500">No deployments yet. Connect a git repository and click &quot;Deploy Now&quot;.</p>
      ) : (
        <div className="space-y-2">
          {deployments.map((d) => (
            <div key={d.id} className="rounded-lg border border-zinc-800 bg-zinc-900 px-4 py-3">
              <div className="flex items-center justify-between">
                <div className="space-y-1">
                  <p className={`text-sm font-medium ${statusColor(d.status)}`}>{d.status}</p>
                  {d.commit_message && <p className="text-xs text-zinc-400">{d.commit_message}</p>}
                  <p className="text-xs text-zinc-600">{new Date(d.created_at).toLocaleString()}</p>
                </div>
                <div className="flex gap-2">
                  <button onClick={() => viewLogs(d)} className="text-sm text-blue-400 hover:text-blue-300">Logs</button>
                  <button onClick={() => handleRedeploy(d.id)} className="text-sm text-zinc-400 hover:text-zinc-300">Redeploy</button>
                </div>
              </div>
            </div>
          ))}
        </div>
      )}

      {selected && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/70">
          <div className="mx-4 max-h-[80vh] w-full max-w-2xl overflow-hidden rounded-xl border border-zinc-700 bg-zinc-950">
            <div className="flex items-center justify-between border-b border-zinc-800 px-4 py-3">
              <span className="text-sm font-medium">Deployment Logs</span>
              <button onClick={closeLogs} className="text-sm text-zinc-500 hover:text-white">Close</button>
            </div>
            <div className="h-96 overflow-y-auto p-4 font-mono text-xs leading-relaxed">
              {logLines.map((line, i) => (
                <div key={i} className={`${line.stream === "stderr" ? "text-red-400" : "text-zinc-300"}`}>
                  {line.message}
                </div>
              ))}
              <div ref={logEndRef} />
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
