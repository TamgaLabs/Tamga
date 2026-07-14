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
import { useAuth } from "@/lib/auth";
import { getShowSystem } from "@/lib/settings";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Empty, EmptyDescription, EmptyHeader, EmptyMedia, EmptyTitle } from "@/components/ui/empty";
import { PageHeader, PageHeaderActions, PageHeaderDescription, PageHeaderTitle } from "@/components/page-header";
import { Skeleton } from "@/components/ui/skeleton";
import { ContainerRow } from "../../projects/[id]/container-row";
import {
  AlertDialog,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog";
import { Search, Container as ContainerIcon, Globe } from "lucide-react";
import { toast } from "sonner";

export default function NonProjectDashboardPage() {
  const [containers, setContainers] = useState<ContainerInfo[]>([]);
  const [loading, setLoading] = useState(true);
  const [search, setSearch] = useState("");
  const [deleteTarget, setDeleteTarget] = useState<ContainerInfo | null>(null);
  const [error, setError] = useState("");
  const [actionError, setActionError] = useState("");
  const [pendingContainerId, setPendingContainerId] = useState<string | null>(null);
  const [deleting, setDeleting] = useState(false);
  const [deleteError, setDeleteError] = useState("");
  const { user, loading: authLoading } = useAuth();

  const fetchContainers = useCallback(() => {
    if (!user) return;
    setLoading(true);
    setError("");
    listContainers()
      .then((all) => setContainers(all.filter((c) => !c.project_id && !c.system_type)))
      .catch((requestError) => {
        console.error(requestError);
        setError(requestError instanceof Error ? requestError.message : "Failed to load containers.");
      })
      .finally(() => setLoading(false));
  }, [user]);

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

  const showSystem = getShowSystem();

  const filtered = containers.filter((c) => {
    const name = c.name || "";
    const isSystem = !!c.system_type;
    if (!showSystem && isSystem) return false;
    if (search && !name.toLowerCase().includes(search.toLowerCase())) return false;
    return true;
  });

  if (authLoading || !user) return null;

  return (
    <div className="mx-auto max-w-6xl space-y-6 p-4 sm:p-6">
      <PageHeader>
        <div className="space-y-1">
          <PageHeaderTitle>Non-project containers</PageHeaderTitle>
          <PageHeaderDescription>Containers not associated with any project.</PageHeaderDescription>
        </div>
        <PageHeaderActions>
          <div className="relative w-full sm:w-72">
            <Search className="absolute left-2.5 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground" />
            <Input
              type="text"
              placeholder="Search by name..."
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              className="pl-8"
            />
          </div>
        </PageHeaderActions>
      </PageHeader>

      {actionError && <p role="alert" className="rounded-md border border-destructive/30 bg-destructive/10 p-3 text-sm text-destructive">{actionError}</p>}

      {loading ? (
        <div className="space-y-3">
          <Skeleton className="h-20 w-full" />
          <Skeleton className="h-20 w-full" />
        </div>
      ) : error ? (
        <Empty className="min-h-56 border-destructive/30">
          <EmptyHeader>
            <EmptyMedia className="bg-destructive/10 text-destructive"><Globe className="size-5" /></EmptyMedia>
            <EmptyTitle>Containers could not be loaded</EmptyTitle>
            <EmptyDescription>{error}</EmptyDescription>
          </EmptyHeader>
          <Button variant="outline" onClick={fetchContainers}>Try again</Button>
        </Empty>
      ) : filtered.length === 0 ? (
        <Empty className="min-h-56">
          <EmptyHeader>
            <EmptyMedia><ContainerIcon className="size-5" /></EmptyMedia>
            <EmptyTitle>No non-project containers</EmptyTitle>
            <EmptyDescription>{search ? "Try a different container name." : "Containers not associated with any project will appear here."}</EmptyDescription>
          </EmptyHeader>
        </Empty>
      ) : (
        <div className="space-y-2">
          {filtered.map((c) => (
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
