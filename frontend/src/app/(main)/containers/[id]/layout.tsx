"use client";

import { useCallback, useEffect, useState } from "react";
import Link from "next/link";
import { usePathname, useParams, useRouter } from "next/navigation";
import {
  getContainer,
  listProjects,
  startContainer,
  stopContainer,
  restartContainer,
  removeContainer,
  type Project,
} from "@/lib/api";
import { useAuth } from "@/lib/auth";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { AlertDialog, AlertDialogCancel, AlertDialogContent, AlertDialogDescription, AlertDialogFooter, AlertDialogHeader, AlertDialogTitle } from "@/components/ui/alert-dialog";
import { PageHeader, PageHeaderActions, PageHeaderDescription, PageHeaderTitle } from "@/components/page-header";
import { Skeleton } from "@/components/ui/skeleton";
import { Play, RotateCw, Square, Trash2 } from "lucide-react";
import { toast } from "sonner";
import { ContainerContextProvider } from "./container-context";

const statusVariant: Record<string, "success" | "warning" | "error" | "info" | "default"> = {
  running: "success",
  paused: "warning",
  exited: "error",
  created: "info",
};

// Derives the owning project id from the Docker container name using the
// same project-<id> / agent-<id> convention the backend's list-endpoint
// derivation uses (client.go's Sscanf pattern-match, see TEST-008 §4).
// The detail/Inspect endpoint returns raw Docker inspect data with no
// project_id field, so it's re-derived client-side here rather than adding
// a backend field.
function deriveProjectId(rawName?: string): number | null {
  if (!rawName) return null;
  const name = rawName.replace(/^\//, "");
  const match = /^(?:project|agent)-(\d+)/.exec(name);
  return match ? Number(match[1]) : null;
}

export default function ContainerDetailLayout({ children }: { children: React.ReactNode }) {
  const params = useParams();
  const pathname = usePathname();
  const router = useRouter();
  const { user, loading: authLoading } = useAuth();
  const [container, setContainer] = useState<any>(null);
  const [loading, setLoading] = useState(true);
  const [projects, setProjects] = useState<Project[]>([]);
  const [loadError, setLoadError] = useState("");
  const [actionError, setActionError] = useState("");
  const [pendingAction, setPendingAction] = useState<"start" | "stop" | "restart" | "remove" | null>(null);
  const [removeOpen, setRemoveOpen] = useState(false);
  const id = params.id as string;

  const fetchContainer = useCallback(() => {
    if (!user || !id) return;
    setLoading(true);
    setLoadError("");
    getContainer(id)
      .then(setContainer)
      .catch((error) => {
        console.error(error);
        setLoadError(error instanceof Error ? error.message : "Failed to load container.");
      })
      .finally(() => setLoading(false));
  }, [user, id]);

  useEffect(fetchContainer, [fetchContainer]);

  useEffect(() => {
    if (!user) return;
    listProjects().then(setProjects).catch(console.error);
  }, [user]);

  useEffect(() => {
    if (!authLoading && !user) router.replace("/login");
  }, [user, authLoading, router]);

  const handleAction = async (action: "start" | "stop" | "restart" | "remove") => {
    if (pendingAction) return;
    if (action === "remove") { setActionError(""); setRemoveOpen(true); return; }
    const copy = action === "start" ? { pending: "Starting", success: "started" } : action === "stop" ? { pending: "Stopping", success: "stopped" } : { pending: "Restarting", success: "restarted" };
    const toastId = toast.loading(`${copy.pending} container...`);
    setActionError("");
    setPendingAction(action);
    try {
      if (action === "start") await startContainer(id);
      else if (action === "stop") await stopContainer(id);
      else await restartContainer(id);
      fetchContainer();
      toast.success(`Container ${copy.success}.`, { id: toastId });
    } catch (error) {
      console.error(error);
      const message = error instanceof Error ? error.message : `Failed to ${action} container.`;
      setActionError(message);
      toast.error(message, { id: toastId });
    } finally { setPendingAction(null); }
  };

  const confirmRemove = async () => {
    if (pendingAction) return;
    const toastId = toast.loading("Removing container...");
    setActionError("");
    setPendingAction("remove");
    try {
      await removeContainer(id);
      toast.success("Container removed.", { id: toastId });
      setRemoveOpen(false);
      router.push("/containers");
    } catch (error) {
      console.error(error);
      const message = error instanceof Error ? error.message : "Failed to remove container.";
      setActionError(message);
      toast.error(message, { id: toastId });
    } finally { setPendingAction(null); }
  };

  if (authLoading || !user || loading) {
    return (
      <div className="mx-auto max-w-6xl space-y-4 p-4 sm:p-6">
        <Skeleton className="h-8 w-52" /><Skeleton className="h-10 w-full" /><Skeleton className="h-72 w-full" />
      </div>
    );
  }

  if (!container) {
    return (
      <div className="p-6 max-w-6xl mx-auto">
        <Button variant="ghost" onClick={() => router.push("/containers")} className="mb-4">
          &larr; Containers
        </Button>
        <p role="alert" className="text-muted-foreground">{loadError || "Container not found."}</p>
      </div>
    );
  }

  const name = container.Name?.replace(/^\//, "") || id.slice(0, 12);
  const status = container.State?.Status;
  const projectId = deriveProjectId(container.Name);
  const project = projectId ? projects.find((p) => p.id === projectId) : undefined;

  const sections = [
    { href: `/containers/${id}`, label: "Inspect" },
    { href: `/containers/${id}/logs`, label: "Logs" },
    { href: `/containers/${id}/stats`, label: "Stats" },
    { href: `/containers/${id}/resources`, label: "Resources" },
  ];

  return (
    <div className="mx-auto max-w-6xl space-y-6 p-4 sm:p-6">
      <PageHeader>
        <div className="space-y-2">
          <Button variant="ghost" size="sm" onClick={() => router.push("/containers")} className="-ml-2">
          &larr; Containers
        </Button>
          <div>
          <PageHeaderTitle className="font-mono text-xl break-all" title={name}>
            {name}
          </PageHeaderTitle>
          {status && (
            <Badge variant={statusVariant[status] || "default"} className="mt-2">
              {status}
            </Badge>
          )}
          {project ? (
            <Link
              href={`/projects/${project.id}`}
              className="block mt-2 text-xs text-muted-foreground hover:text-foreground transition-colors"
            >
              Project: {project.name}
            </Link>
          ) : <PageHeaderDescription>Container details and live operational controls.</PageHeaderDescription>}
          </div>
        </div>
        <PageHeaderActions>
        <div className="flex flex-wrap gap-2">
          {status === "running" ? (
            <Button variant="outline" size="sm" disabled={!!pendingAction} onClick={() => void handleAction("stop")}>
              <Square className="size-3.5" />{pendingAction === "stop" ? "Stopping..." : "Stop"}
            </Button>
          ) : (
            <Button variant="outline" size="sm" disabled={!!pendingAction} onClick={() => void handleAction("start")}>
              <Play className="size-3.5" />{pendingAction === "start" ? "Starting..." : "Start"}
            </Button>
          )}
          <Button variant="outline" size="sm" disabled={!!pendingAction} onClick={() => void handleAction("restart")}>
            <RotateCw className="size-3.5" />{pendingAction === "restart" ? "Restarting..." : "Restart"}
          </Button>
          <Button variant="destructive" size="sm" disabled={!!pendingAction} onClick={() => void handleAction("remove")}>
            <Trash2 className="size-3.5" />Remove
          </Button>
        </div>
        </PageHeaderActions>
      </PageHeader>
      {actionError && !removeOpen && <p role="alert" className="rounded-md border border-destructive/30 bg-destructive/10 p-3 text-sm text-destructive">{actionError}</p>}
      <nav aria-label="Container sections" className="flex gap-1 overflow-x-auto rounded-lg border bg-muted/30 p-1">
          {sections.map((s) => {
            const active = pathname === s.href;
            return (
              <Link
                key={s.href}
                href={s.href}
                className={`shrink-0 rounded-md px-3 py-2 text-sm transition-colors ${
                  active
                    ? "bg-muted text-foreground"
                    : "text-muted-foreground hover:text-foreground hover:bg-muted"
                }`}
              >
                {s.label}
              </Link>
            );
          })}
      </nav>
      <div className="min-w-0">
        <ContainerContextProvider value={{ id, container, refetch: fetchContainer }}>
          {children}
        </ContainerContextProvider>
      </div>
      <AlertDialog open={removeOpen} onOpenChange={(open) => { if (!pendingAction) setRemoveOpen(open); }}>
        <AlertDialogContent><AlertDialogHeader><AlertDialogTitle>Remove container?</AlertDialogTitle><AlertDialogDescription>This permanently removes {name}. This action cannot be undone.</AlertDialogDescription></AlertDialogHeader>{actionError && <p role="alert" className="rounded-md border border-destructive/30 bg-destructive/10 p-3 text-sm text-destructive">{actionError}</p>}<AlertDialogFooter><AlertDialogCancel disabled={!!pendingAction}>Cancel</AlertDialogCancel><Button variant="destructive" onClick={() => void confirmRemove()} disabled={!!pendingAction}>{pendingAction === "remove" ? "Removing..." : "Remove"}</Button></AlertDialogFooter></AlertDialogContent>
      </AlertDialog>
    </div>
  );
}
