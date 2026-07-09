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
  const id = params.id as string;

  const fetchContainer = useCallback(() => {
    if (!user || !id) return;
    setLoading(true);
    getContainer(id)
      .then(setContainer)
      .catch(console.error)
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
    try {
      if (action === "start") await startContainer(id);
      else if (action === "stop") await stopContainer(id);
      else if (action === "restart") await restartContainer(id);
      else {
        await removeContainer(id);
        router.push("/containers");
        return;
      }
      fetchContainer();
    } catch (e) {
      console.error(e);
    }
  };

  if (authLoading || !user || loading) {
    return (
      <div className="p-6 max-w-6xl mx-auto">
        <p className="text-muted-foreground">Loading...</p>
      </div>
    );
  }

  if (!container) {
    return (
      <div className="p-6 max-w-6xl mx-auto">
        <Button variant="ghost" onClick={() => router.push("/containers")} className="mb-4">
          &larr; Containers
        </Button>
        <p className="text-muted-foreground">Container not found.</p>
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
    <div className="flex min-h-screen">
      <aside className="w-56 shrink-0 border-r border-border p-4 space-y-4">
        <Button variant="ghost" size="sm" onClick={() => router.push("/containers")} className="-ml-2">
          &larr; Containers
        </Button>

        <div>
          <p className="font-mono text-sm font-semibold break-all" title={name}>
            {name}
          </p>
          {status && (
            <Badge variant={statusVariant[status] || "default"} className="mt-2">
              {status}
            </Badge>
          )}
          {project && (
            <Link
              href={`/projects/${project.id}`}
              className="block mt-2 text-xs text-muted-foreground hover:text-foreground transition-colors"
            >
              Project: {project.name}
            </Link>
          )}
        </div>

        <div className="flex flex-col gap-2">
          {status === "running" ? (
            <Button variant="outline" size="sm" onClick={() => handleAction("stop")}>
              Stop
            </Button>
          ) : (
            <Button variant="outline" size="sm" onClick={() => handleAction("start")}>
              Start
            </Button>
          )}
          <Button variant="outline" size="sm" onClick={() => handleAction("restart")}>
            Restart
          </Button>
          <Button variant="destructive" size="sm" onClick={() => handleAction("remove")}>
            Remove
          </Button>
        </div>

        <nav className="space-y-1 pt-2 border-t border-border">
          {sections.map((s) => {
            const active = pathname === s.href;
            return (
              <Link
                key={s.href}
                href={s.href}
                className={`block px-3 py-2 rounded-md text-sm transition-colors ${
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
      </aside>
      <div className="flex-1">
        <ContainerContextProvider value={{ id, container, refetch: fetchContainer }}>
          {children}
        </ContainerContextProvider>
      </div>
    </div>
  );
}
