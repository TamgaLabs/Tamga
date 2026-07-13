"use client";

import { useEffect, useState, useCallback } from "react";
import { useRouter } from "next/navigation";
import Link from "next/link";
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
import { useAuth } from "@/lib/auth";
import { getShowSystem } from "@/lib/settings";
import { Input } from "@/components/ui/input";
import {
  AlertDialog,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog";
import { Empty, EmptyDescription, EmptyHeader, EmptyMedia, EmptyTitle } from "@/components/ui/empty";
import { PageHeader, PageHeaderActions, PageHeaderDescription, PageHeaderTitle } from "@/components/page-header";
import { Skeleton } from "@/components/ui/skeleton";
import { Button } from "@/components/ui/button";
import { Search, Container as ContainerIcon } from "lucide-react";
import { toast } from "sonner";
import { ContainerRow } from "../projects/[id]/container-row";

// Grouped view: the containers list API already carries a numeric
// project_id per container (derived by the backend from the
// project-<id>/agent-<id> name convention - see TEST-008 §4), but not the
// project's name, so groups are labeled via a client-side join against
// listProjects() by id.
type Group = { projectId: number; name: string; containers: ContainerInfo[] };

export default function ContainersPage() {
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
  const nonProject: ContainerInfo[] = [];
  for (const c of filtered) {
    if (c.project_id) {
      const list = groupsById.get(c.project_id) || [];
      list.push(c);
      groupsById.set(c.project_id, list);
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
        <div className="space-y-1"><PageHeaderTitle>Containers</PageHeaderTitle><PageHeaderDescription>Inspect and operate every container available to Tamga Console.</PageHeaderDescription></div>
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
        <div className="space-y-3"><Skeleton className="h-7 w-40" /><Skeleton className="h-20 w-full" /><Skeleton className="h-20 w-full" /></div>
      ) : error ? (
        <Empty className="min-h-56 border-destructive/30"><EmptyHeader><EmptyMedia className="bg-destructive/10 text-destructive"><ContainerIcon className="size-5" /></EmptyMedia><EmptyTitle>Containers could not be loaded</EmptyTitle><EmptyDescription>{error}</EmptyDescription></EmptyHeader><Button variant="outline" onClick={fetchAll}>Try again</Button></Empty>
      ) : filtered.length === 0 ? (
        <Empty className="min-h-56"><EmptyHeader><EmptyMedia><ContainerIcon className="size-5" /></EmptyMedia><EmptyTitle>No containers found</EmptyTitle><EmptyDescription>{search ? "Try a different container name." : "Containers will appear here when they are available."}</EmptyDescription></EmptyHeader></Empty>
      ) : (
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
