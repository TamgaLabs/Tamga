"use client";

import { useCallback, useEffect, useState } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import {
  listContainers,
  listProjects,
  startContainer,
  stopContainer,
  restartContainer,
  removeContainer,
  type ContainerInfo,
  type Project,
} from "@/lib/api";
import { useSystemMetrics } from "@/hooks/useMetrics";
import { useAuth } from "@/lib/auth";
import { getShowSystem } from "@/lib/settings";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Empty, EmptyDescription, EmptyHeader, EmptyMedia, EmptyTitle } from "@/components/ui/empty";
import { PageHeader, PageHeaderActions, PageHeaderDescription, PageHeaderTitle } from "@/components/page-header";
import { Skeleton } from "@/components/ui/skeleton";
import { ContainerRow } from "../projects/[id]/container-row";
import {
  AlertDialog,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog";
import { Search, Container as ContainerIcon, Plus, FolderKanban, AlertCircle, Settings } from "lucide-react";
import { toast } from "sonner";

type Group = { projectId: number; name: string; containers: ContainerInfo[] };

const statusVariant: Record<string, "success" | "warning" | "error" | "info" | "default"> = {
  running: "success",
  building: "warning",
  cloning: "info",
  created: "info",
  error: "error",
  clone_failed: "error",
  build_failed: "error",
  ready_to_deploy: "info",
  configuring: "warning",
  archived: "default",
};

function GlobalMetricsSummary() {
  const now = Math.floor(Date.now() / 1000);
  const { data: metrics, loading } = useSystemMetrics({ from: now - 86400, to: now }, { refetchInterval: 60000 });

  if (loading && !metrics) {
    return (
      <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
        {[0, 1, 2, 3].map((i) => <Skeleton key={i} className="h-24" />)}
      </div>
    );
  }

  if (!metrics) return null;

  const latestRequestRate = metrics.request_rate.length > 0 ? metrics.request_rate[metrics.request_rate.length - 1] : null;
  const totalErrors = metrics.status_class.reduce((sum, p) => sum + (p.count_4xx || 0) + (p.count_5xx || 0), 0);
  const latestLatency = metrics.latency.length > 0 ? metrics.latency[metrics.latency.length - 1] : null;
  const latestBandwidth = metrics.bandwidth.length > 0 ? metrics.bandwidth[metrics.bandwidth.length - 1] : null;

  return (
    <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
      <Card>
        <CardHeader className="space-y-1 pb-2">
          <CardTitle className="text-xs text-muted-foreground">Request Rate</CardTitle>
          <CardContent className="p-0 text-2xl tabular-nums">{latestRequestRate ? latestRequestRate.rate_per_sec.toFixed(1) : "0"}<span className="text-xs text-muted-foreground ml-1">req/s</span></CardContent>
        </CardHeader>
      </Card>
      <Card>
        <CardHeader className="space-y-1 pb-2">
          <CardTitle className="text-xs text-muted-foreground">Status Errors</CardTitle>
          <CardContent className="p-0 text-2xl tabular-nums">{totalErrors}<span className="text-xs text-muted-foreground ml-1">total</span></CardContent>
        </CardHeader>
      </Card>
      <Card>
        <CardHeader className="space-y-1 pb-2">
          <CardTitle className="text-xs text-muted-foreground">Avg Latency</CardTitle>
          <CardContent className="p-0 text-2xl tabular-nums">{latestLatency ? (latestLatency.p50 * 1000).toFixed(0) : "0"}<span className="text-xs text-muted-foreground ml-1">ms</span></CardContent>
        </CardHeader>
      </Card>
      <Card>
        <CardHeader className="space-y-1 pb-2">
          <CardTitle className="text-xs text-muted-foreground">Bandwidth</CardTitle>
          <CardContent className="p-0 text-2xl tabular-nums">{latestBandwidth ? formatBytes(latestBandwidth.bytes_in + latestBandwidth.bytes_out) : "0 B"}</CardContent>
        </CardHeader>
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

export default function GlobalDashboardPage() {
  const [containers, setContainers] = useState<ContainerInfo[]>([]);
  const [projects, setProjects] = useState<Project[]>([]);
  const [loading, setLoading] = useState(true);
  const [search, setSearch] = useState("");
  const [deleteTarget, setDeleteTarget] = useState<ContainerInfo | null>(null);
  const [error, setError] = useState("");
  const [actionError, setActionError] = useState("");
  const [pendingContainerId, setPendingContainerId] = useState<string | null>(null);
  const [deleting, setDeleting] = useState(false);
  const [deleteError, setDeleteError] = useState("");
  const { user, loading: authLoading } = useAuth();
  const router = useRouter();

  useEffect(() => {
    if (!authLoading && !user) router.replace("/login");
  }, [user, authLoading, router]);

  const fetchAll = useCallback(() => {
    if (!user) return;
    setLoading(true);
    setError("");
    Promise.all([listContainers(), listProjects()])
      .then(([c, p]) => {
        setContainers(c);
        setProjects(p);
      })
      .catch((requestError) => {
        console.error(requestError);
        setError(requestError instanceof Error ? requestError.message : "Failed to load containers.");
      })
      .finally(() => setLoading(false));
  }, [user]);

  useEffect(fetchAll, [fetchAll]);

  const handleAction = async (id: string, action: "start" | "stop" | "restart") => {
    if (pendingContainerId) return;
    const actionCopy = action === "restart" ? { pending: "Restarting", success: "restarted" } : action === "start" ? { pending: "Starting", success: "started" } : { pending: "Stopping", success: "stopped" };
    const toastId = toast.loading(`${actionCopy.pending} container...`);
    setActionError("");
    setPendingContainerId(id);
    try {
      if (action === "start") await startContainer(id);
      else if (action === "stop") await stopContainer(id);
      else await restartContainer(id);
      fetchAll();
      toast.success(`Container ${actionCopy.success}.`, { id: toastId });
    } catch (requestError) {
      console.error(requestError);
      const message = requestError instanceof Error ? requestError.message : `Failed to ${action} container.`;
      setActionError(message);
      toast.error(message, { id: toastId });
    } finally { setPendingContainerId(null); }
  };

  const confirmDelete = async () => {
    if (!deleteTarget) return;
    if (deleting) return;
    const toastId = toast.loading("Deleting container...");
    setActionError("");
    setDeleteError("");
    setDeleting(true);
    try {
      await removeContainer(deleteTarget.id);
      fetchAll();
      toast.success("Container deleted.", { id: toastId });
      setDeleteTarget(null);
    } catch (requestError) {
      console.error(requestError);
      const message = requestError instanceof Error ? requestError.message : "Failed to delete container.";
      setDeleteError(message);
      toast.error(message, { id: toastId });
    } finally { setDeleting(false); }
  };

  const showSystem = getShowSystem();

  const filtered = (containers || []).filter((c) => {
    const name = c.name || "";
    const isSystem = !!c.system_type;
    if (!showSystem && isSystem) return false;
    if (search && !name.toLowerCase().includes(search.toLowerCase())) return false;
    return true;
  });

  const projectsById = new Map(projects.map((p) => [p.id, p]));
  const groupsById = new Map<number, ContainerInfo[]>();
  const systemContainers: ContainerInfo[] = [];
  const nonProject: ContainerInfo[] = [];
  for (const c of filtered) {
    if (c.project_id) {
      const list = groupsById.get(c.project_id) || [];
      list.push(c);
      groupsById.set(c.project_id, list);
    } else if (c.system_type) {
      systemContainers.push(c);
    } else {
      nonProject.push(c);
    }
  }
  const groups: Group[] = Array.from(groupsById.entries())
    .map(([projectId, list]) => ({
      projectId,
      name: projectsById.get(projectId)?.name || `Project #${projectId}`,
      containers: list,
    }))
    .sort((a, b) => a.name.localeCompare(b.name));

  if (authLoading || !user) return null;

  return (
    <div className="mx-auto max-w-6xl space-y-6 p-4 sm:p-6">
      <PageHeader>
        <div className="space-y-1">
          <PageHeaderTitle>Dashboard</PageHeaderTitle>
          <PageHeaderDescription>Overview of all projects and containers.</PageHeaderDescription>
        </div>
        <PageHeaderActions>
          <Button onClick={() => router.push("/dashboard/new")}>
            <Plus className="size-4" aria-hidden="true" />
            New project
          </Button>
        </PageHeaderActions>
      </PageHeader>

      <GlobalMetricsSummary />

      {actionError && <p role="alert" className="rounded-md border border-destructive/30 bg-destructive/10 p-3 text-sm text-destructive">{actionError}</p>}

      {loading ? (
        <div className="space-y-3">
          <Skeleton className="h-7 w-40" />
          <Skeleton className="h-20 w-full" />
          <Skeleton className="h-20 w-full" />
        </div>
      ) : error ? (
        <Empty className="min-h-56 border-destructive/30">
          <EmptyHeader>
            <EmptyMedia className="bg-destructive/10 text-destructive"><AlertCircle className="size-5" /></EmptyMedia>
            <EmptyTitle>Containers could not be loaded</EmptyTitle>
            <EmptyDescription>{error}</EmptyDescription>
          </EmptyHeader>
          <Button variant="outline" onClick={fetchAll}>Try again</Button>
        </Empty>
      ) : (
        <>
          {/* Project cards grid */}
          {projects.length > 0 && (
            <section>
              <h2 className="text-sm font-semibold text-foreground mb-3">Projects</h2>
              <div className="grid gap-4 sm:grid-cols-2 md:grid-cols-3 lg:grid-cols-5">
                {projects.map((p) => (
                  <Card key={p.id} className="relative group">
                    <CardHeader className="pb-2">
                      <div className="flex items-start justify-between gap-2">
                        <CardTitle className="text-sm font-medium truncate">{p.name}</CardTitle>
                        <Button
                          variant="ghost"
                          size="sm"
                          className="size-7 p-0 shrink-0 opacity-0 group-hover:opacity-100 transition-opacity relative z-10"
                          onClick={(e) => { e.preventDefault(); e.stopPropagation(); router.push(`/projects/${p.id}/settings`); }}
                          aria-label={`Edit ${p.name}`}
                        >
                          <Settings className="size-3.5" aria-hidden="true" />
                        </Button>
                      </div>
                    </CardHeader>
                    <CardContent className="pb-3">
                      <div className="flex items-center justify-between gap-2">
                        <Badge variant={statusVariant[p.status] || "default"} className="text-xs">{p.status}</Badge>
                        <span className="text-xs text-muted-foreground truncate">{p.source_type}</span>
                      </div>
                    </CardContent>
                    <Link
                      href={`/projects/${p.id}`}
                      className="absolute inset-0 rounded-xl"
                      tabIndex={-1}
                      aria-hidden="true"
                    />
                  </Card>
                ))}
              </div>
            </section>
          )}

          {/* Container groups */}
          {groups.length > 0 && (
            <div className="space-y-8">
              {groups.map((g) => (
                <section key={g.projectId}>
                  <Link
                    href={`/projects/${g.projectId}`}
                    className="inline-block text-sm font-semibold text-foreground hover:text-accent transition-colors mb-3"
                  >
                    {g.name}
                  </Link>
                  <div className="space-y-2">
                    {g.containers.map((c) => (
                      <ContainerRow key={c.id} container={c} onAction={handleAction} onDelete={(container) => { setDeleteError(""); setDeleteTarget(container); }} actionPending={pendingContainerId === c.id || (deleting && deleteTarget?.id === c.id)} />
                    ))}
                  </div>
                </section>
              ))}
              {nonProject.length > 0 && (
                <section>
                  <h2 className="text-sm font-semibold text-foreground mb-3">Non-project</h2>
                  <div className="space-y-2">
                    {nonProject.map((c) => (
                      <ContainerRow key={c.id} container={c} onAction={handleAction} onDelete={(container) => { setDeleteError(""); setDeleteTarget(container); }} actionPending={pendingContainerId === c.id || (deleting && deleteTarget?.id === c.id)} />
                    ))}
                  </div>
                </section>
              )}
              {systemContainers.length > 0 && (
                <section>
                  <Link
                    href="/dashboard/system"
                    className="inline-block text-sm font-semibold text-foreground hover:text-accent transition-colors mb-3"
                  >
                    Tamga System
                  </Link>
                  <div className="space-y-2">
                    {systemContainers.map((c) => (
                      <ContainerRow key={c.id} container={c} onAction={handleAction} onDelete={(container) => { setDeleteError(""); setDeleteTarget(container); }} actionPending={pendingContainerId === c.id || (deleting && deleteTarget?.id === c.id)} />
                    ))}
                  </div>
                </section>
              )}
            </div>
          )}

          {projects.length === 0 && filtered.length === 0 && (
            <Empty className="min-h-56">
              <EmptyHeader>
                <EmptyMedia><ContainerIcon className="size-5" /></EmptyMedia>
                <EmptyTitle>No projects or containers found</EmptyTitle>
                <EmptyDescription>{search ? "Try a different container name." : "Create a project to get started."}</EmptyDescription>
              </EmptyHeader>
            </Empty>
          )}
        </>
      )}

      <AlertDialog open={!!deleteTarget} onOpenChange={(open) => { if (!open && !deleting) setDeleteTarget(null); }}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Delete container?</AlertDialogTitle>
            <AlertDialogDescription>
              This will permanently delete &quot;
              {deleteTarget?.name || deleteTarget?.id.slice(0, 12)}&quot;. This action
              cannot be undone.
            </AlertDialogDescription>
          </AlertDialogHeader>
          {deleteError && <p role="alert" className="rounded-md border border-destructive/30 bg-destructive/10 p-3 text-sm text-destructive">{deleteError}</p>}
          <AlertDialogFooter>
            <AlertDialogCancel disabled={deleting}>Cancel</AlertDialogCancel>
            <Button type="button" variant="destructive" onClick={() => void confirmDelete()} disabled={deleting}>{deleting ? "Deleting..." : "Delete"}</Button>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  );
}
