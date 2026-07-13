"use client";

import { useCallback, useEffect, useState } from "react";
import Link from "next/link";
import { usePathname, useParams, useRouter } from "next/navigation";
import { getProject, type Project } from "@/lib/api";
import { useAuth } from "@/lib/auth";
import { ProjectContextProvider } from "./project-context";
import { ProjectSwitcher } from "./project-switcher";
import { Badge } from "@/components/ui/badge";
import { Skeleton } from "@/components/ui/skeleton";

export default function ProjectDetailLayout({ children }: { children: React.ReactNode }) {
  const params = useParams();
  const pathname = usePathname();
  const router = useRouter();
  const { user, loading: authLoading } = useAuth();
  const [project, setProject] = useState<Project | null>(null);
  const [loading, setLoading] = useState(true);
  const id = Number(params.id);

  const fetchProject = useCallback(() => {
    if (!user || !params.id) return;
    setLoading(true);
    getProject(id)
      .then(setProject)
      .catch(console.error)
      .finally(() => setLoading(false));
  }, [user, id, params.id]);

  useEffect(fetchProject, [fetchProject]);

  useEffect(() => {
    if (!authLoading && !user) router.replace("/login");
  }, [user, authLoading, router]);

  if (authLoading || !user || loading) {
    return (
      <div className="mx-auto w-full max-w-7xl space-y-6 p-4 sm:p-6">
        <Skeleton className="h-10 w-56" />
        <div className="grid gap-4 md:grid-cols-[14rem_1fr]">
          <Skeleton className="h-72" />
          <Skeleton className="h-72" />
        </div>
      </div>
    );
  }

  if (!project) {
    return (
      <div className="mx-auto flex min-h-72 max-w-5xl items-center justify-center p-6">
        <p role="alert" className="text-sm text-muted-foreground">Project not found or no longer accessible.</p>
      </div>
    );
  }

  const sections = [
    { href: `/projects/${project.id}`, label: "Overview" },
    { href: `/projects/${project.id}/containers`, label: "Containers" },
    { href: `/projects/${project.id}/settings`, label: "Settings" },
    { href: `/projects/${project.id}/environment`, label: "Environment" },
    { href: `/projects/${project.id}/actions`, label: "Actions" },
    { href: `/projects/${project.id}/analytics`, label: "Analytics" },
    { href: `/projects/${project.id}/map`, label: "Map" },
  ];

  return (
    <div className="flex min-h-full flex-col bg-muted/20 md:min-h-[calc(100svh-3.5rem)] md:flex-row">
      <aside className="w-full shrink-0 space-y-4 border-b border-border bg-background p-4 md:w-60 md:border-r md:border-b-0">
        <ProjectSwitcher current={project} />
        <div className="flex items-center justify-between px-2">
          <p className="text-xs font-medium uppercase tracking-wider text-muted-foreground">Workspace</p>
          <Badge variant={project.status === "running" ? "success" : "secondary"}>{project.status}</Badge>
        </div>
        <nav aria-label="Project workspace" className="grid grid-cols-2 gap-1 md:grid-cols-1">
          {sections.map((s) => {
            const active = pathname === s.href;
            return (
              <Link
                key={s.href}
                href={s.href}
                aria-current={active ? "page" : undefined}
                className={`rounded-md px-3 py-2 text-sm font-medium transition-colors ${
                  active
                    ? "bg-muted text-foreground"
                    : "text-muted-foreground hover:text-foreground hover:bg-muted"
                }`}
              >
                {s.label}
              </Link>
            );
          })}
          <Link
            href={`/code/${project.id}`}
            className="rounded-md px-3 py-2 text-sm font-medium transition-colors text-muted-foreground hover:bg-muted hover:text-foreground"
          >
            Code
          </Link>
        </nav>
      </aside>
      <div className="min-w-0 flex-1">
        <ProjectContextProvider value={{ project, refetch: fetchProject }}>
          {/* keyed on project.id so switching projects via the switcher
              resets sub-route local state (e.g. Settings form fields)
              instead of carrying over the previous project's values */}
          <div key={project.id}>{children}</div>
        </ProjectContextProvider>
      </div>
    </div>
  );
}
