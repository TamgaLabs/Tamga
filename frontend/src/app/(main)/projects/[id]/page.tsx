"use client";

import { useCallback, useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import {
  listDeployments,
  listContainers,
  startContainer,
  stopContainer,
  restartContainer,
  restartProject,
  getProjectLogs,
  deleteProject,
  type Deployment,
  type ContainerInfo,
} from "@/lib/api";
import { useProjectMetrics } from "@/hooks/useMetrics";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Empty, EmptyDescription, EmptyHeader, EmptyTitle } from "@/components/ui/empty";
import { PageHeader, PageHeaderActions, PageHeaderDescription, PageHeaderTitle } from "@/components/page-header";
import { Skeleton } from "@/components/ui/skeleton";
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
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { useProjectContext } from "./project-context";
import { ContainerRow } from "./container-row";
import { CardDescription } from "@/components/ui/card";
import { toast } from "sonner";
import { MoreVertical, Play, RotateCw, Square, Trash2, Activity } from "lucide-react";

const statusVariant: Record<string, "success" | "warning" | "error" | "info" | "default"> = {
  running: "success",
  building: "warning",
  cloning: "info",
  created: "info",
  error: "error",
};

function MetricsSummary({ projectId }: { projectId: number }) {
  const now = Math.floor(Date.now() / 1000);
  const { data: metrics, loading } = useProjectMetrics(projectId, { from: now - 86400, to: now }, { refetchInterval: 60000 });

  if (loading && !metrics) {
    return (
      <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
        {[0, 1, 2, 3].map((i) => <Skeleton key={i} className="h-24" />)}
      </div>
    );
  }

  if (!metrics) return null;

  const latestRequestRate = metrics.request_rate.length > 0 ? metrics.request_rate[metrics.request_rate.length - 1] : null;
  const latestBandwidth = metrics.bandwidth.length > 0 ? metrics.bandwidth[metrics.bandwidth.length - 1] : null;
  const totalErrors = metrics.status_class.reduce((sum, p) => sum + (p.count_4xx || 0) + (p.count_5xx || 0), 0);
  const latestLatency = metrics.latency.length > 0 ? metrics.latency[metrics.latency.length - 1] : null;

  return (
    <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
      <Card>
        <CardHeader className="space-y-1 pb-2">
          <CardDescription>Request Rate</CardDescription>
          <CardTitle className="text-2xl tabular-nums">{latestRequestRate ? latestRequestRate.rate_per_sec.toFixed(1) : "0"}</CardTitle>
        </CardHeader>
        <CardContent className="text-xs text-muted-foreground">req/s (last 24h)</CardContent>
      </Card>
      <Card>
        <CardHeader className="space-y-1 pb-2">
          <CardDescription>Status Errors</CardDescription>
          <CardTitle className="text-2xl tabular-nums">{totalErrors}</CardTitle>
        </CardHeader>
        <CardContent className="text-xs text-muted-foreground">4xx + 5xx (last 24h)</CardContent>
      </Card>
      <Card>
        <CardHeader className="space-y-1 pb-2">
          <CardDescription>Avg Latency</CardDescription>
          <CardTitle className="text-2xl tabular-nums">{latestLatency ? (latestLatency.p50 * 1000).toFixed(0) : "0"}ms</CardTitle>
        </CardHeader>
        <CardContent className="text-xs text-muted-foreground">p50 (last 24h)</CardContent>
      </Card>
      <Card>
        <CardHeader className="space-y-1 pb-2">
          <CardDescription>Bandwidth</CardDescription>
          <CardTitle className="text-2xl tabular-nums">{latestBandwidth ? formatBytes(latestBandwidth.bytes_in + latestBandwidth.bytes_out) : "0 B"}</CardTitle>
        </CardHeader>
        <CardContent className="text-xs text-muted-foreground">in + out (last 24h)</CardContent>
      </Card>
    </div>
  );
}

function formatBytes(bytes: number): string {
  if (bytes === 0) return "0 B";
  const k = 1024;
  const sizes = ["B", "KB", "MB", "GB", "TB"];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + " " + sizes[i];
}

export default function ProjectDashboardPage() {
  const { project, refetch } = useProjectContext();
  const router = useRouter();
  const [deployments, setDeployments] = useState<Deployment[]>([]);
  const [containers, setContainers] = useState<ContainerInfo[]>([]);
  const [containersLoading, setContainersLoading] = useState(true);
  const [containersError, setContainersError] = useState("");
  const [deploymentsError, setDeploymentsError] = useState("");
  const [actionError, setActionError] = useState("");
  const [pendingContainerId, setPendingContainerId] = useState<string | null>(null);
  const [restarting, setRestarting] = useState(false);
  const [showActionsMenu, setShowActionsMenu] = useState(false);
  const [logs, setLogs] = useState("");
  const [showLogs, setShowLogs] = useState(false);
  const [deleteDialogOpen, setDeleteDialogOpen] = useState(false);
  const [deleting, setDeleting] = useState(false);

  useEffect(() => {
    listDeployments(project.id).then(setDeployments).catch((error) => {
      console.error(error);
      setDeploymentsError(error instanceof Error ? error.message : "Failed to load deployments.");
    });
  }, [project.id]);

  const fetchContainers = useCallback(() => {
    setContainersLoading(true);
    setContainersError("");
    listContainers()
      .then((all) => setContainers(all.filter((c) => c.project_id === project.id)))
      .catch((error) => {
        console.error(error);
        setContainersError(error instanceof Error ? error.message : "Failed to load containers.");
      })
      .finally(() => setContainersLoading(false));
  }, [project.id]);

  useEffect(fetchContainers, [fetchContainers]);

  const handleContainerAction = async (id: string, action: "start" | "stop" | "restart") => {
    if (pendingContainerId) return;
    const actionCopy = action === "restart" ? { pending: "Restarting", success: "restarted" } : action === "start" ? { pending: "Starting", success: "started" } : { pending: "Stopping", success: "stopped" };
    const toastId = toast.loading(`${actionCopy.pending} container...`);
    setActionError("");
    setPendingContainerId(id);
    try {
      if (action === "start") await startContainer(id);
      else if (action === "stop") await stopContainer(id);
      else await restartContainer(id);
      fetchContainers();
      toast.success(`Container ${actionCopy.success}.`, { id: toastId });
    } catch (requestError) {
      console.error(requestError);
      const message = requestError instanceof Error ? requestError.message : `Failed to ${action} container.`;
      setActionError(message);
      toast.error(message, { id: toastId });
    } finally { setPendingContainerId(null); }
  };

  const handleRestart = async () => {
    setShowActionsMenu(false);
    const toastId = toast.loading("Restarting project...");
    try {
      await restartProject(project.id);
      refetch();
      fetchContainers();
      toast.success("Project restarted.", { id: toastId });
    } catch (err: unknown) {
      toast.error(err instanceof Error ? err.message : "Failed to restart project", { id: toastId });
    }
  };

  const loadLogs = async () => {
    setShowActionsMenu(false);
    try {
      const result = await getProjectLogs(project.id);
      setLogs(result.logs);
      setShowLogs(true);
    } catch (err: unknown) {
      toast.error(err instanceof Error ? err.message : "Failed to load logs");
    }
  };

  const handleDelete = async () => {
    setDeleting(true);
    try {
      await deleteProject(project.id);
      toast.success("Project deleted. Redirecting...");
      setTimeout(() => router.push("/dashboard"), 1500);
    } catch (err: unknown) {
      toast.error(err instanceof Error ? err.message : "Failed to delete project");
      setDeleting(false);
    }
  };

  return (
    <div className="mx-auto max-w-6xl space-y-6 p-4 sm:p-6">
      <PageHeader>
        <div>
          <PageHeaderTitle>{project.name}</PageHeaderTitle>
          <PageHeaderDescription>{project.repo_url || "Local project workspace"}</PageHeaderDescription>
        </div>
        <PageHeaderActions>
          <Badge variant={statusVariant[project.status] || "default"}>{project.status}</Badge>
          <DropdownMenu open={showActionsMenu} onOpenChange={setShowActionsMenu}>
            <DropdownMenuTrigger asChild>
              <Button variant="outline" size="sm">
                <MoreVertical className="size-4" aria-hidden="true" />
                Actions
              </Button>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end">
              <DropdownMenuItem onClick={handleRestart} disabled={restarting}>
                <RotateCw className="size-4" aria-hidden="true" />
                {restarting ? "Restarting..." : "Restart"}
              </DropdownMenuItem>
              <DropdownMenuItem onClick={loadLogs}>
                <Activity className="size-4" aria-hidden="true" />
                View Logs
              </DropdownMenuItem>
              <DropdownMenuItem className="text-destructive" onClick={() => { setShowActionsMenu(false); setDeleteDialogOpen(true); }}>
                <Trash2 className="size-4" aria-hidden="true" />
                Delete
              </DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>
        </PageHeaderActions>
      </PageHeader>

      {actionError && <p role="alert" className="rounded-md border border-destructive/30 bg-destructive/10 p-3 text-sm text-destructive">{actionError}</p>}

      <MetricsSummary projectId={project.id} />

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
            {project.compose_yaml ? (
              <div className="flex justify-between">
                <span>Exposed Service</span>
                <span className="font-mono text-xs">{project.exposed_service || "-"}</span>
              </div>
            ) : (
              <div className="flex justify-between">
                <span>Branch</span>
                <span>{project.branch}</span>
              </div>
            )}
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
            <CardTitle className="text-sm">Containers</CardTitle>
          </CardHeader>
          <CardContent>
            {containersLoading ? (
              <div className="space-y-2"><Skeleton className="h-14 w-full" /><Skeleton className="h-14 w-full" /></div>
            ) : containersError ? (
              <p role="alert" className="text-sm text-destructive">{containersError}</p>
            ) : containers.length === 0 ? (
              <Empty className="min-h-32 p-4"><EmptyHeader><EmptyTitle>No containers yet</EmptyTitle><EmptyDescription>Containers appear here after a deployment.</EmptyDescription></EmptyHeader></Empty>
            ) : (
              <div className="space-y-2">
                {containers.map((c) => (
                  <ContainerRow key={c.id} container={c} onAction={handleContainerAction} actionPending={pendingContainerId === c.id} />
                ))}
              </div>
            )}
          </CardContent>
        </Card>

        <Card className="md:col-span-2">
          <CardHeader>
            <CardTitle className="text-sm">Deployments</CardTitle>
          </CardHeader>
          <CardContent>
            {deploymentsError ? (
              <p role="alert" className="text-sm text-destructive">{deploymentsError}</p>
            ) : deployments.length === 0 ? (
              <Empty className="min-h-32 p-4"><EmptyHeader><EmptyTitle>No deployments yet</EmptyTitle><EmptyDescription>Deployment history will appear here.</EmptyDescription></EmptyHeader></Empty>
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

      {showLogs && (
        <Card className="mt-4">
          <CardHeader className="flex flex-row items-center justify-between">
            <CardTitle className="text-sm">Container Logs</CardTitle>
            <Button variant="ghost" size="sm" onClick={() => setShowLogs(false)}>Close</Button>
          </CardHeader>
          <CardContent>
            <pre className="max-h-80 overflow-auto rounded-md bg-code-block p-4 font-mono text-xs text-success whitespace-pre-wrap">
              {logs || "(no output)"}
            </pre>
          </CardContent>
        </Card>
      )}

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
            <AlertDialogAction onClick={handleDelete} disabled={deleting}>
              {deleting ? "Deleting..." : "Delete"}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  );
}
