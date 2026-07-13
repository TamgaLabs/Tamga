"use client";

import { useCallback, useEffect, useState } from "react";
import {
  listDeployments,
  listContainers,
  startContainer,
  stopContainer,
  restartContainer,
  type Deployment,
  type ContainerInfo,
} from "@/lib/api";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Empty, EmptyDescription, EmptyHeader, EmptyTitle } from "@/components/ui/empty";
import { PageHeader, PageHeaderActions, PageHeaderDescription, PageHeaderTitle } from "@/components/page-header";
import { Skeleton } from "@/components/ui/skeleton";
import { useProjectContext } from "./project-context";
import { ContainerRow } from "./container-row";
import { toast } from "sonner";

const statusVariant: Record<string, "success" | "warning" | "error" | "info" | "default"> = {
  running: "success",
  building: "warning",
  cloning: "info",
  created: "info",
  error: "error",
};

export default function ProjectOverviewPage() {
  const { project } = useProjectContext();
  const [deployments, setDeployments] = useState<Deployment[]>([]);
  const [containers, setContainers] = useState<ContainerInfo[]>([]);
  const [containersLoading, setContainersLoading] = useState(true);
  const [containersError, setContainersError] = useState("");
  const [deploymentsError, setDeploymentsError] = useState("");
  const [actionError, setActionError] = useState("");
  const [pendingContainerId, setPendingContainerId] = useState<string | null>(null);

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

  return (
    <div className="mx-auto max-w-6xl space-y-6 p-4 sm:p-6">
      <PageHeader>
        <div>
          <PageHeaderTitle>{project.name}</PageHeaderTitle>
          <PageHeaderDescription>{project.repo_url || "Local project workspace"}</PageHeaderDescription>
        </div>
        <PageHeaderActions><Badge variant={statusVariant[project.status] || "default"}>{project.status}</Badge></PageHeaderActions>
      </PageHeader>
      {actionError && <p role="alert" className="rounded-md border border-destructive/30 bg-destructive/10 p-3 text-sm text-destructive">{actionError}</p>}

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
    </div>
  );
}
