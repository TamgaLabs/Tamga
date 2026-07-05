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
  listAgentProviders,
  type AgentProvider,
} from "@/lib/api";
import { useAuth } from "@/lib/auth";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog";
import { FileCode2 } from "lucide-react";

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
  const [tab, setTab] = useState("overview");
  const [deleteDialogOpen, setDeleteDialogOpen] = useState(false);
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

  const handleDelete = async () => {
    if (!project) return;
    await deleteProject(project.id);
    router.push("/dashboard");
  };

  if (authLoading || !user || !project) return null;

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

      <Tabs value={tab} onValueChange={setTab}>
        <div className="flex items-center gap-1 mb-6">
          <TabsList>
            <TabsTrigger value="overview">Overview</TabsTrigger>
            <TabsTrigger value="settings">Settings</TabsTrigger>
            <TabsTrigger value="environment">Environment</TabsTrigger>
          </TabsList>
          <div className="ml-auto">
            <Button
              size="sm"
              onClick={() => router.push(`/code/${project.id}`)}
            >
              Code
            </Button>
          </div>
        </div>

        <TabsContent value="overview">
          <OverviewTab
            project={project}
            onRestart={handleRestart}
            onDelete={() => setDeleteDialogOpen(true)}
          />
        </TabsContent>
        <TabsContent value="settings">
          <ProjectSettingsTab project={project} onUpdate={fetchProject} />
        </TabsContent>
        <TabsContent value="environment">
          <EnvironmentTab projectId={project.id} />
        </TabsContent>
      </Tabs>

      <AlertDialog open={deleteDialogOpen} onOpenChange={setDeleteDialogOpen}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Delete project?</AlertDialogTitle>
            <AlertDialogDescription>
              This will permanently delete &quot;{project.name}&quot; and its
              associated container. This action cannot be undone.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>Cancel</AlertDialogCancel>
            <AlertDialogAction onClick={handleDelete}>Delete</AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  );
}

function OverviewTab({ project, onRestart, onDelete }: { project: Project; onRestart: () => void; onDelete: () => void }) {
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
            <FileCode2 className="h-4 w-4 mr-1" />
            Open in Code IDE
          </Button>
          <Button variant="outline" size="sm" className="w-full" onClick={onRestart}>
            Restart
          </Button>
          <Button variant="outline" size="sm" className="w-full" onClick={loadLogs}>
            View Logs
          </Button>
          <Button variant="destructive" size="sm" className="w-full" onClick={onDelete}>
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
          <div className="space-y-1">
            <Label className="text-xs">Name</Label>
            <Input value={editName} onChange={(e) => setEditName(e.target.value)} />
          </div>
          <div className="space-y-1">
            <Label className="text-xs">Domain</Label>
            <Input value={editDomain} onChange={(e) => setEditDomain(e.target.value)} />
          </div>
          <div className="space-y-1">
            <Label className="text-xs">Branch</Label>
            <Input value={editBranch} onChange={(e) => setEditBranch(e.target.value)} />
          </div>
          <div className="space-y-1">
            <Label className="text-xs">Agent Provider</Label>
            <Select value={editProviderId} onValueChange={setEditProviderId}>
              <SelectTrigger>
                <SelectValue placeholder="Default (builtin-opencode)" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="">Default (builtin-opencode)</SelectItem>
                {providers.map((p) => (
                  <SelectItem key={p.id} value={p.id}>{p.name}</SelectItem>
                ))}
              </SelectContent>
            </Select>
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
