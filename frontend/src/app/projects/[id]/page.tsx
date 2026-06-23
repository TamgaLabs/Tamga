"use client";

import { useEffect, useState, useCallback } from "react";
import { useParams, useRouter } from "next/navigation";
import {
  getProject,
  deleteProject,
  restartProject,
  getProjectLogs,
  updateProject,
  listDeployments,
  listEnvVars,
  createEnvVar,
  deleteEnvVar,
  type Project,
  type Deployment,
  type EnvVar,
} from "@/lib/api";
import { useAuth } from "@/lib/auth";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";

const statusVariant: Record<string, "success" | "warning" | "error" | "info" | "default"> = {
  running: "success",
  building: "warning",
  cloning: "info",
  created: "info",
  error: "error",
};

export default function ProjectDetailPage() {
  const params = useParams();
  const router = useRouter();
  const [project, setProject] = useState<Project | null>(null);
  const [tab, setTab] = useState<"overview" | "settings" | "agent">("overview");
  const { user, loading: authLoading } = useAuth();

  const fetchProject = useCallback(() => {
    if (user && params.id) {
      getProject(Number(params.id)).then(setProject).catch(console.error);
    }
  }, [user, params.id]);

  useEffect(fetchProject, [fetchProject]);

  useEffect(() => {
    if (!authLoading && !user) router.replace("/login");
  }, [user, authLoading, router]);

  const handleRestart = async () => {
    if (!project) return;
    await restartProject(project.id);
    fetchProject();
  };

  const handleRedeploy = async () => {
    if (!project) return;
    await restartProject(project.id);
    fetchProject();
  };

  const handleDelete = async () => {
    if (!project || !confirm("Delete this project?")) return;
    await deleteProject(project.id);
    router.push("/dashboard");
  };

  if (authLoading || !user || !project) return null;

  const tabs = [
    { id: "overview" as const, label: "Overview" },
    { id: "settings" as const, label: "Settings" },
    { id: "agent" as const, label: "Agent" },
  ];

  return (
    <div className="min-h-screen p-6 max-w-5xl mx-auto">
      <Button variant="ghost" onClick={() => router.push("/dashboard")} className="mb-4">
        &larr; Dashboard
      </Button>

      <div className="flex items-center justify-between mb-6">
        <div>
          <h1 className="text-2xl font-bold">{project.name}</h1>
          <p className="text-sm text-neutral-400 mt-1">{project.repo_url}</p>
        </div>
        <Badge variant={statusVariant[project.status] || "default"}>{project.status}</Badge>
      </div>

      <div className="flex gap-1 mb-6 border-b border-neutral-800">
        {tabs.map((t) => (
          <button
            key={t.id}
            onClick={() => setTab(t.id)}
            className={`px-4 py-2 text-sm font-medium transition-colors ${
              tab === t.id
                ? "border-b-2 border-white text-white"
                : "text-neutral-500 hover:text-neutral-300"
            }`}
          >
            {t.label}
          </button>
        ))}
      </div>

      {tab === "overview" && <OverviewTab project={project} onUpdate={fetchProject} />}
      {tab === "settings" && <SettingsTab project={project} onUpdate={fetchProject} />}
      {tab === "agent" && <AgentTab project={project} />}
    </div>
  );
}

function OverviewTab({ project, onUpdate }: { project: Project; onUpdate: () => void }) {
  const [deployments, setDeployments] = useState<Deployment[]>([]);
  const [logs, setLogs] = useState<string>("");
  const [showLogs, setShowLogs] = useState(false);

  useEffect(() => {
    listDeployments(project.id).then(setDeployments).catch(console.error);
  }, [project.id]);

  const loadLogs = async () => {
    try {
      const result = await getProjectLogs(project.id);
      setLogs(result.logs);
      setShowLogs(true);
    } catch (e) {
      console.error(e);
    }
  };

  return (
    <div className="grid gap-4 md:grid-cols-2">
      <Card>
        <CardHeader>
          <CardTitle className="text-sm">Details</CardTitle>
        </CardHeader>
        <CardContent className="text-sm space-y-2 text-neutral-400">
          <div className="flex justify-between">
            <span>Domain</span>
            <span className="text-blue-400">{project.domain || "-"}</span>
          </div>
          <div className="flex justify-between">
            <span>Branch</span>
            <span>{project.branch}</span>
          </div>
          <div className="flex justify-between">
            <span>Container</span>
            <span className="font-mono text-xs">{project.container_id?.slice(0, 12) || "-"}</span>
          </div>
          <div className="flex justify-between">
            <span>Created</span>
            <span>{new Date(project.created_at).toLocaleDateString()}</span>
          </div>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle className="text-sm">Actions</CardTitle>
        </CardHeader>
        <CardContent className="space-y-2">
          <Button variant="outline" size="sm" className="w-full" onClick={handleRestart}>
            Restart
          </Button>
          <Button variant="outline" size="sm" className="w-full" onClick={loadLogs}>
            View Logs
          </Button>
          <Button variant="destructive" size="sm" className="w-full" onClick={handleDelete}>
            Delete
          </Button>
        </CardContent>
      </Card>

      {showLogs && (
        <Card className="md:col-span-2">
          <CardHeader className="flex flex-row items-center justify-between">
            <CardTitle className="text-sm">Container Logs</CardTitle>
            <Button variant="ghost" size="sm" onClick={() => setShowLogs(false)}>Close</Button>
          </CardHeader>
          <CardContent>
            <pre className="bg-black rounded p-4 text-xs text-green-400 overflow-auto max-h-80 font-mono whitespace-pre-wrap">
              {logs || "(no output)"}
            </pre>
          </CardContent>
        </Card>
      )}

      <Card className="md:col-span-2">
        <CardHeader>
          <CardTitle className="text-sm">Deployments</CardTitle>
        </CardHeader>
        <CardContent>
          {deployments.length === 0 ? (
            <p className="text-sm text-neutral-500">No deployments yet.</p>
          ) : (
            <div className="text-sm space-y-2">
              {deployments.map((d) => (
                <div key={d.id} className="flex items-center justify-between py-1 border-b border-neutral-800 last:border-0">
                  <div className="flex items-center gap-2">
                    <Badge variant={d.status === "success" ? "success" : d.status === "failed" ? "error" : "warning"}>
                      {d.status}
                    </Badge>
                    <span className="text-neutral-400">
                      {new Date(d.created_at).toLocaleString()}
                    </span>
                  </div>
                  {d.commit_sha && (
                    <span className="font-mono text-xs text-neutral-500">{d.commit_sha.slice(0, 7)}</span>
                  )}
                </div>
              ))}
            </div>
          )}
        </CardContent>
      </Card>
    </div>
  );

  function handleRestart() {
    restartProject(project.id).then(onUpdate).catch(console.error);
  }

  function handleDelete() {
    if (!confirm("Delete this project?")) return;
    deleteProject(project.id).then(() => window.location.href = "/dashboard");
  }
}

function SettingsTab({ project, onUpdate }: { project: Project; onUpdate: () => void }) {
  const [envVars, setEnvVars] = useState<EnvVar[]>([]);
  const [newKey, setNewKey] = useState("");
  const [newValue, setNewValue] = useState("");
  const [editName, setEditName] = useState(project.name);
  const [editDomain, setEditDomain] = useState(project.domain);
  const [editBranch, setEditBranch] = useState(project.branch);

  useEffect(() => {
    listEnvVars(project.id).then(setEnvVars).catch(console.error);
  }, [project.id]);

  const handleSaveProject = async () => {
    await updateProject(project.id, { name: editName, domain: editDomain, branch: editBranch } as Partial<Project>);
    onUpdate();
  };

  const handleAddEnvVar = async () => {
    if (!newKey) return;
    await createEnvVar(project.id, newKey, newValue);
    setNewKey("");
    setNewValue("");
    listEnvVars(project.id).then(setEnvVars).catch(console.error);
  };

  const handleDeleteEnvVar = async (id: number) => {
    await deleteEnvVar(project.id, id);
    listEnvVars(project.id).then(setEnvVars).catch(console.error);
  };

  return (
    <div className="grid gap-4 max-w-xl">
      <Card>
        <CardHeader>
          <CardTitle className="text-sm">Project Settings</CardTitle>
        </CardHeader>
        <CardContent className="space-y-3">
          <div>
            <label className="text-xs text-neutral-500 block mb-1">Name</label>
            <Input value={editName} onChange={(e) => setEditName(e.target.value)} />
          </div>
          <div>
            <label className="text-xs text-neutral-500 block mb-1">Domain</label>
            <Input value={editDomain} onChange={(e) => setEditDomain(e.target.value)} />
          </div>
          <div>
            <label className="text-xs text-neutral-500 block mb-1">Branch</label>
            <Input value={editBranch} onChange={(e) => setEditBranch(e.target.value)} />
          </div>
          <Button size="sm" onClick={handleSaveProject}>Save</Button>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle className="text-sm">Environment Variables</CardTitle>
        </CardHeader>
        <CardContent className="space-y-3">
          {envVars.length === 0 && (
            <p className="text-sm text-neutral-500">No environment variables configured.</p>
          )}
          {envVars.map((ev) => (
            <div key={ev.id} className="flex items-center gap-2 text-sm">
              <span className="font-mono text-blue-400 min-w-24">{ev.key}</span>
              <span className="text-neutral-400">=</span>
              <span className="font-mono text-neutral-300 flex-1 truncate">{ev.value}</span>
              <Button variant="ghost" size="sm" className="text-red-400" onClick={() => handleDeleteEnvVar(ev.id)}>
                &times;
              </Button>
            </div>
          ))}
          <div className="flex gap-2 pt-2 border-t border-neutral-800">
            <Input
              placeholder="KEY"
              className="font-mono text-xs flex-1"
              value={newKey}
              onChange={(e) => setNewKey(e.target.value)}
            />
            <Input
              placeholder="value"
              className="font-mono text-xs flex-1"
              value={newValue}
              onChange={(e) => setNewValue(e.target.value)}
            />
            <Button size="sm" onClick={handleAddEnvVar}>Add</Button>
          </div>
        </CardContent>
      </Card>
    </div>
  );
}

function AgentTab({ project }: { project: Project }) {
  const [messages, setMessages] = useState<{ role: "user" | "agent"; content: string; diff?: string }[]>([]);
  const [input, setInput] = useState("");
  const [loading, setLoading] = useState(false);

  const handleSend = async () => {
    if (!input.trim()) return;
    const msg = input.trim();
    setInput("");
    setMessages((prev) => [...prev, { role: "user", content: msg }]);
    setLoading(true);

    setTimeout(() => {
      setMessages((prev) => [
        ...prev,
        { role: "agent", content: "Agent chat coming in Phase 4.", diff: "" },
      ]);
      setLoading(false);
    }, 500);
  };

  return (
    <div className="flex gap-4 h-[60vh]">
      <Card className="flex-1 flex flex-col">
        <CardHeader>
          <CardTitle className="text-sm">Chat</CardTitle>
        </CardHeader>
        <CardContent className="flex-1 flex flex-col">
          <div className="flex-1 overflow-auto space-y-3 mb-4">
            {messages.length === 0 && (
              <p className="text-sm text-neutral-500">Ask the AI agent to make changes to this project.</p>
            )}
            {messages.map((m, i) => (
              <div key={i} className={`text-sm ${m.role === "user" ? "text-blue-400" : "text-green-400"}`}>
                <span className="font-bold">{m.role === "user" ? "You" : "Agent"}:</span> {m.content}
                {m.diff !== undefined && (
                  <button
                    className="ml-2 text-xs text-neutral-500 hover:text-white"
                    onClick={() => alert("Diff panel coming in Phase 4")}
                  >
                    [diff]
                  </button>
                )}
              </div>
            ))}
            {loading && <p className="text-sm text-neutral-500">Agent is thinking...</p>}
          </div>
          <div className="flex gap-2">
            <Input
              value={input}
              onChange={(e) => setInput(e.target.value)}
              onKeyDown={(e) => e.key === "Enter" && handleSend()}
              placeholder="Type a message..."
              disabled={loading}
            />
            <Button size="sm" onClick={handleSend} disabled={loading}>Send</Button>
          </div>
        </CardContent>
      </Card>

      <Card className="w-80 flex flex-col">
        <CardHeader>
          <CardTitle className="text-sm">Diff</CardTitle>
        </CardHeader>
        <CardContent className="flex-1 text-xs text-neutral-500">
          Select a diff from the chat to view changes.
        </CardContent>
      </Card>
    </div>
  );
}
