"use client";

import { useCallback, useEffect, useState } from "react";
import {
  listContainers,
  startContainer,
  stopContainer,
  restartContainer,
  removeContainer,
  type ContainerInfo,
} from "@/lib/api";
import {
  AlertDialog,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog";
import { useProjectContext } from "../project-context";
import { ContainerRow } from "../container-row";
import { Empty, EmptyDescription, EmptyHeader, EmptyTitle } from "@/components/ui/empty";
import { PageHeader, PageHeaderDescription, PageHeaderTitle } from "@/components/page-header";
import { Skeleton } from "@/components/ui/skeleton";
import { Button } from "@/components/ui/button";
import { toast } from "sonner";

export default function ProjectContainersPage() {
  const { project } = useProjectContext();
  const [containers, setContainers] = useState<ContainerInfo[]>([]);
  const [loading, setLoading] = useState(true);
  const [deleteTarget, setDeleteTarget] = useState<ContainerInfo | null>(null);
  const [error, setError] = useState("");
  const [actionError, setActionError] = useState("");
  const [pendingContainerId, setPendingContainerId] = useState<string | null>(null);
  const [deleting, setDeleting] = useState(false);
  const [deleteError, setDeleteError] = useState("");

  const fetchContainers = useCallback(() => {
    setLoading(true);
    setError("");
    listContainers()
      .then((all) => setContainers(all.filter((c) => c.project_id === project.id)))
      .catch((requestError) => {
        console.error(requestError);
        setError(requestError instanceof Error ? requestError.message : "Failed to load containers.");
      })
      .finally(() => setLoading(false));
  }, [project.id]);

  useEffect(fetchContainers, [fetchContainers]);

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
      fetchContainers();
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
      fetchContainers();
      toast.success("Container deleted.", { id: toastId });
      setDeleteTarget(null);
    } catch (requestError) {
      console.error(requestError);
      const message = requestError instanceof Error ? requestError.message : "Failed to delete container.";
      setDeleteError(message);
      toast.error(message, { id: toastId });
    } finally { setDeleting(false); }
  };

  return (
    <div className="mx-auto max-w-6xl space-y-6 p-4 sm:p-6">
      <PageHeader><div><PageHeaderTitle>Containers</PageHeaderTitle><PageHeaderDescription>Manage the containers attached to this project.</PageHeaderDescription></div></PageHeader>
      {actionError && <p role="alert" className="rounded-md border border-destructive/30 bg-destructive/10 p-3 text-sm text-destructive">{actionError}</p>}

      {loading ? (
        <div className="space-y-2"><Skeleton className="h-16 w-full" /><Skeleton className="h-16 w-full" /></div>
      ) : error ? (
        <p role="alert" className="rounded-md border border-destructive/30 bg-destructive/10 p-3 text-sm text-destructive">{error}</p>
      ) : containers.length === 0 ? (
        <Empty><EmptyHeader><EmptyTitle>No containers yet</EmptyTitle><EmptyDescription>Deploy the project to create its first container.</EmptyDescription></EmptyHeader></Empty>
      ) : (
        <div className="space-y-2">
          {containers.map((c) => (
            <ContainerRow key={c.id} container={c} onAction={handleAction} onDelete={(container) => { setDeleteError(""); setDeleteTarget(container); }} actionPending={pendingContainerId === c.id || (deleting && deleteTarget?.id === c.id)} />
          ))}
        </div>
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
