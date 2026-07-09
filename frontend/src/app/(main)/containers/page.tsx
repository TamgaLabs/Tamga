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
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog";
import { Search } from "lucide-react";
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
  const { user, loading: authLoading } = useAuth();
  const router = useRouter();

  useEffect(() => {
    if (!authLoading && !user) router.replace("/login");
  }, [user, authLoading, router]);

  const fetchAll = useCallback(() => {
    if (!user) return;
    setLoading(true);
    Promise.all([listContainers(), listProjects()])
      .then(([c, p]) => {
        setContainers(c);
        setProjects(p);
      })
      .catch(console.error)
      .finally(() => setLoading(false));
  }, [user]);

  useEffect(fetchAll, [fetchAll]);

  const handleAction = async (id: string, action: "start" | "stop" | "restart") => {
    try {
      if (action === "start") await startContainer(id);
      else if (action === "stop") await stopContainer(id);
      else await restartContainer(id);
      fetchAll();
    } catch (e) {
      console.error(e);
    }
  };

  const confirmDelete = async () => {
    if (!deleteTarget) return;
    try {
      await removeContainer(deleteTarget.id);
      fetchAll();
    } catch (e) {
      console.error(e);
    } finally {
      setDeleteTarget(null);
    }
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
    <div className="p-6 max-w-6xl mx-auto">
      <div className="flex items-center justify-between mb-6">
        <h1 className="text-2xl font-bold">Containers</h1>
        <div className="relative max-w-xs">
          <Search className="absolute left-2.5 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground" />
          <Input
            type="text"
            placeholder="Search by name..."
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            className="pl-8"
          />
        </div>
      </div>

      {loading ? (
        <p className="text-muted-foreground">Loading...</p>
      ) : filtered.length === 0 ? (
        <p className="text-muted-foreground">No containers found.</p>
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
                  <ContainerRow key={c.id} container={c} onAction={handleAction} onDelete={setDeleteTarget} />
                ))}
              </div>
            </section>
          ))}
          {nonProject.length > 0 && (
            <section>
              <h2 className="text-sm font-semibold text-foreground mb-3">Non-project</h2>
              <div className="space-y-2">
                {nonProject.map((c) => (
                  <ContainerRow key={c.id} container={c} onAction={handleAction} onDelete={setDeleteTarget} />
                ))}
              </div>
            </section>
          )}
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
