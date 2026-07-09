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
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog";
import { useProjectContext } from "../project-context";
import { ContainerRow } from "../container-row";

export default function ProjectContainersPage() {
  const { project } = useProjectContext();
  const [containers, setContainers] = useState<ContainerInfo[]>([]);
  const [loading, setLoading] = useState(true);
  const [deleteTarget, setDeleteTarget] = useState<ContainerInfo | null>(null);

  const fetchContainers = useCallback(() => {
    setLoading(true);
    listContainers()
      .then((all) => setContainers(all.filter((c) => c.project_id === project.id)))
      .catch(console.error)
      .finally(() => setLoading(false));
  }, [project.id]);

  useEffect(fetchContainers, [fetchContainers]);

  const handleAction = async (id: string, action: "start" | "stop" | "restart") => {
    try {
      if (action === "start") await startContainer(id);
      else if (action === "stop") await stopContainer(id);
      else await restartContainer(id);
      fetchContainers();
    } catch (e) {
      console.error(e);
    }
  };

  const confirmDelete = async () => {
    if (!deleteTarget) return;
    try {
      await removeContainer(deleteTarget.id);
      fetchContainers();
    } catch (e) {
      console.error(e);
    } finally {
      setDeleteTarget(null);
    }
  };

  return (
    <div className="p-6 max-w-5xl mx-auto">
      <h1 className="text-2xl font-bold mb-6">Containers</h1>

      {loading ? (
        <p className="text-muted-foreground">Loading...</p>
      ) : containers.length === 0 ? (
        <p className="text-muted-foreground">No containers for this project.</p>
      ) : (
        <div className="space-y-2">
          {containers.map((c) => (
            <ContainerRow key={c.id} container={c} onAction={handleAction} onDelete={setDeleteTarget} />
          ))}
        </div>
      )}

      <AlertDialog open={!!deleteTarget} onOpenChange={(open) => !open && setDeleteTarget(null)}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Delete container?</AlertDialogTitle>
            <AlertDialogDescription>
              This will permanently delete &quot;
              {deleteTarget?.name || deleteTarget?.id.slice(0, 12)}&quot;. This action
              cannot be undone.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>Cancel</AlertDialogCancel>
            <AlertDialogAction onClick={confirmDelete}>Delete</AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  );
}
