"use client";

import { useEffect, useState, useCallback } from "react";
import { useParams, useRouter } from "next/navigation";
import dynamic from "next/dynamic";
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
  getFileTree,
  readFile,
  writeFile,
  type Project,
  type Deployment,
  type EnvVar,
  type FileEntry,
} from "@/lib/api";
import {
  listAgentProviders,
  type AgentProvider,
} from "@/lib/api";
import { useAuth } from "@/lib/auth";
import { useTheme } from "@/lib/theme";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";

const MonacoEditor = dynamic(() => import("@monaco-editor/react"), { ssr: false });

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
  const [tab, setTab] = useState<"overview" | "settings" | "environment">("overview");
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
    { id: "environment" as const, label: "Environment" },
  ];

  return (
    <div className="min-h-screen p-6 max-w-5xl mx-auto">
      <Button variant="ghost" onClick={() => router.push("/dashboard")} className="mb-4">
        &larr; Dashboard
      </Button>

      <div className="flex items-center justify-between mb-6">
        <div>
          <h1 className="text-2xl font-bold">{project.name}</h1>
          <p className="text-sm text-muted-foreground mt-1">{project.repo_url}</p>
        </div>
        <Badge variant={statusVariant[project.status] || "default"}>{project.status}</Badge>
      </div>

      <div className="flex gap-1 mb-6 border-b border-border items-center">
        {tabs.map((t) => (
          <button
            key={t.id}
            onClick={() => setTab(t.id)}
            className={`px-4 py-2 text-sm font-medium transition-colors ${
              tab === t.id
                ? "border-b-2 border-foreground text-foreground"
                : "text-muted-foreground hover:text-card-foreground"
            }`}
          >
            {t.label}
          </button>
        ))}
        <div className="ml-auto">
          <Button
            size="sm"
            className="bg-accent text-accent-foreground hover:bg-accent/90"
            onClick={() => router.push(`/code/${project.id}`)}
          >
            Code
          </Button>
        </div>
      </div>

      {tab === "overview" && <OverviewTab project={project} onUpdate={fetchProject} />}
      {tab === "settings" && <ProjectSettingsTab project={project} onUpdate={fetchProject} />}
      {tab === "environment" && <EnvironmentTab projectId={project.id} />}
    </div>
  );
}

function OverviewTab({ project, onUpdate }: { project: Project; onUpdate: () => void }) {
  const router = useRouter();
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
        <CardContent className="text-sm space-y-2 text-muted-foreground">
          <div className="flex justify-between">
            <span>Domain</span>
            <span className="text-accent">{project.domain || "-"}</span>
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
          <Button variant="outline" size="sm" className="w-full" onClick={() => router.push(`/code/${project.id}`)}>
            Open in Code IDE
          </Button>
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
            <pre className="bg-code-block rounded p-4 text-xs text-success overflow-auto max-h-80 font-mono whitespace-pre-wrap">
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
            <p className="text-sm text-muted-foreground">No deployments yet.</p>
          ) : (
            <div className="text-sm space-y-2">
              {deployments.map((d) => (
                <div key={d.id} className="flex items-center justify-between py-1 border-b border-border last:border-0">
                  <div className="flex items-center gap-2">
                    <Badge variant={d.status === "success" ? "success" : d.status === "failed" ? "error" : "warning"}>
                      {d.status}
                    </Badge>
                    <span className="text-muted-foreground">
                      {new Date(d.created_at).toLocaleString()}
                    </span>
                  </div>
                  {d.commit_sha && (
                    <span className="font-mono text-xs text-muted-foreground">{d.commit_sha.slice(0, 7)}</span>
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

function ProjectSettingsTab({ project, onUpdate }: { project: Project; onUpdate: () => void }) {
  const [editName, setEditName] = useState(project.name);
  const [editDomain, setEditDomain] = useState(project.domain);
  const [editBranch, setEditBranch] = useState(project.branch);
  const [providers, setProviders] = useState<AgentProvider[]>([]);
  const [editProviderId, setEditProviderId] = useState(project.agent_provider_id || "");

  useEffect(() => {
    listAgentProviders().then(setProviders).catch(console.error);
  }, []);

  const handleSaveProject = async () => {
    await updateProject(project.id, {
      name: editName,
      domain: editDomain,
      branch: editBranch,
      agent_provider_id: editProviderId || null,
    } as any);
    onUpdate();
  };

  return (
    <div className="max-w-xl">
      <Card>
        <CardHeader>
          <CardTitle className="text-sm">Project Settings</CardTitle>
        </CardHeader>
        <CardContent className="space-y-3">
          <div>
            <label className="text-xs text-muted-foreground block mb-1">Name</label>
            <Input value={editName} onChange={(e) => setEditName(e.target.value)} />
          </div>
          <div>
            <label className="text-xs text-muted-foreground block mb-1">Domain</label>
            <Input value={editDomain} onChange={(e) => setEditDomain(e.target.value)} />
          </div>
          <div>
            <label className="text-xs text-muted-foreground block mb-1">Branch</label>
            <Input value={editBranch} onChange={(e) => setEditBranch(e.target.value)} />
          </div>
          <div>
            <label className="text-xs text-muted-foreground block mb-1">Agent Provider</label>
            <select
              className="w-full h-9 rounded-md border border-border bg-card px-3 text-sm"
              value={editProviderId}
              onChange={(e) => setEditProviderId(e.target.value)}
            >
              <option value="">Default (builtin-opencode)</option>
              {providers.map((p) => (
                <option key={p.id} value={p.id}>{p.name} ({p.provider_type})</option>
              ))}
            </select>
          </div>
          <Button size="sm" onClick={handleSaveProject}>Save</Button>
        </CardContent>
      </Card>
    </div>
  );
}

function EnvironmentTab({ projectId }: { projectId: number }) {
  const [envVars, setEnvVars] = useState<EnvVar[]>([]);
  const [newKey, setNewKey] = useState("");
  const [newValue, setNewValue] = useState("");

  useEffect(() => {
    listEnvVars(projectId).then(setEnvVars).catch(console.error);
  }, [projectId]);

  const handleAddEnvVar = async () => {
    if (!newKey) return;
    await createEnvVar(projectId, newKey, newValue);
    setNewKey("");
    setNewValue("");
    listEnvVars(projectId).then(setEnvVars).catch(console.error);
  };

  const handleDeleteEnvVar = async (id: number) => {
    await deleteEnvVar(projectId, id);
    listEnvVars(projectId).then(setEnvVars).catch(console.error);
  };

  return (
    <div className="max-w-xl">
      <Card>
        <CardHeader>
          <CardTitle className="text-sm">Environment Variables</CardTitle>
        </CardHeader>
        <CardContent className="space-y-3">
          {envVars.length === 0 && (
            <p className="text-sm text-muted-foreground">No environment variables configured.</p>
          )}
          {envVars.map((ev) => (
            <div key={ev.id} className="flex items-center gap-2 text-sm">
              <span className="font-mono text-accent min-w-24">{ev.key}</span>
              <span className="text-muted-foreground">=</span>
              <span className="font-mono text-card-foreground flex-1 truncate">{ev.value}</span>
              <Button variant="ghost" size="sm" className="text-destructive" onClick={() => handleDeleteEnvVar(ev.id)}>
                &times;
              </Button>
            </div>
          ))}
          <div className="flex gap-2 pt-2 border-t border-border">
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

function CodeTab({ projectId }: { projectId: number }) {
  const { theme } = useTheme();
  const [files, setFiles] = useState<FileEntry[]>([]);
  const [currentPath, setCurrentPath] = useState("");
  const [content, setContent] = useState("");
  const [originalContent, setOriginalContent] = useState("");
  const [dirty, setDirty] = useState(false);
  const [expandedDirs, setExpandedDirs] = useState<Set<string>>(new Set());

  useEffect(() => {
    getFileTree(projectId).then(setFiles).catch(console.error);
  }, [projectId]);

  const openFile = async (path: string) => {
    if (dirty && !confirm("Discard unsaved changes?")) return;
    try {
      const res = await readFile(projectId, path);
      setCurrentPath(path);
      setContent(res.content);
      setOriginalContent(res.content);
      setDirty(false);
    } catch (e) {
      console.error(e);
    }
  };

  const handleSave = async () => {
    if (!currentPath) return;
    try {
      await writeFile(projectId, currentPath, content);
      setOriginalContent(content);
      setDirty(false);
    } catch (e) {
      console.error(e);
    }
  };

  const toggleDir = (path: string) => {
    setExpandedDirs((prev) => {
      const next = new Set(prev);
      if (next.has(path)) next.delete(path);
      else next.add(path);
      return next;
    });
  };

  const buildTree = (entries: FileEntry[]) => {
    const root: Record<string, any> = {};
    for (const e of entries) {
      const parts = e.path.split("/");
      let current = root;
      for (let i = 0; i < parts.length; i++) {
        const part = parts[i];
        if (!current[part]) current[part] = {};
        current = current[part];
      }
      current._entry = e;
    }
    const render = (obj: Record<string, any>, depth = 0): React.ReactNode[] => {
      return Object.keys(obj)
        .filter((k) => !k.startsWith("_"))
        .sort((a, b) => {
          const ae = obj[a]._entry;
          const be = obj[b]._entry;
          if (ae?.type !== be?.type) return ae?.type === "dir" ? -1 : 1;
          return a.localeCompare(b);
        })
        .map((key) => {
          const entry = obj[key]._entry;
          if (!entry) return null;
          const isExpanded = expandedDirs.has(entry.path);
          return (
            <div key={entry.path}>
              <div
                className={`flex items-center gap-1 px-2 py-0.5 text-xs cursor-pointer rounded hover:bg-muted ${
                  currentPath === entry.path ? "bg-muted text-accent" : "text-muted-foreground"
                }`}
                style={{ paddingLeft: `${12 + depth * 12}px` }}
                onClick={() => {
                  if (entry.type === "dir") toggleDir(entry.path);
                  else openFile(entry.path);
                }}
              >
                <span className="w-4 text-center">
                  {entry.type === "dir" ? (isExpanded ? "▼" : "▶") : "📄"}
                </span>
                <span className="truncate">{key}</span>
              </div>
              {entry.type === "dir" && isExpanded && render(obj[key], depth + 1)}
            </div>
          );
        });
    };
    return render(root);
  };

  const detectLanguage = (path: string): string => {
    const ext = path.split(".").pop()?.toLowerCase() || "";
    const map: Record<string, string> = {
      ts: "typescript", tsx: "typescript", js: "javascript", jsx: "javascript",
      go: "go", py: "python", rs: "rust", rb: "ruby", java: "java",
      json: "json", yaml: "yaml", yml: "yaml", md: "markdown",
      css: "css", scss: "scss", html: "html", xml: "xml",
      sql: "sql", sh: "shell", bash: "shell", dockerfile: "dockerfile",
    };
    return map[ext] || "plaintext";
  };

  return (
    <div className="flex gap-4 h-[70vh]">
      <div className="w-56 bg-card border border-border rounded overflow-auto flex-shrink-0">
        <div className="p-2 text-xs font-semibold text-muted-foreground uppercase border-b border-border">Files</div>
        <div className="py-1">{buildTree(files)}</div>
      </div>
      <div className="flex-1 flex flex-col border border-border rounded overflow-hidden">
        {currentPath ? (
          <>
            <div className="flex items-center justify-between px-3 py-1.5 border-b border-border bg-card">
              <span className="text-xs text-card-foreground font-mono">{currentPath}</span>
              {dirty && <Button size="sm" variant="outline" onClick={handleSave}>Save</Button>}
            </div>
            <div className="flex-1">
              <MonacoEditor
                language={detectLanguage(currentPath)}
                theme={theme === "dark" ? "vs-dark" : "vs"}
                value={content}
                onChange={(v) => {
                  setContent(v || "");
                  setDirty(v !== originalContent);
                }}
                options={{ minimap: { enabled: false }, fontSize: 13, lineNumbers: "on", scrollBeyondLastLine: false, automaticLayout: true }}
              />
            </div>
          </>
        ) : (
          <div className="flex-1 flex items-center justify-center text-sm text-muted-foreground">Select a file to edit</div>
        )}
      </div>
    </div>
  );
}

