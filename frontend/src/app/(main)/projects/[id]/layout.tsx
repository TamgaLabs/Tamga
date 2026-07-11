"use client";

import { useCallback, useEffect, useState } from "react";
import Link from "next/link";
import { usePathname, useParams, useRouter } from "next/navigation";
import { getProject, type Project } from "@/lib/api";
import { useAuth } from "@/lib/auth";
import { ProjectContextProvider } from "./project-context";
import { ProjectSwitcher } from "./project-switcher";

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
      <div className="min-h-screen p-6 max-w-5xl mx-auto">
        <p className="text-muted-foreground">Loading...</p>
      </div>
    );
  }

  if (!project) {
    return (
      <div className="min-h-screen p-6 max-w-5xl mx-auto">
        <p className="text-muted-foreground">Project not found.</p>
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
  ];

  return (
    <div className="flex min-h-screen">
      <aside className="w-56 shrink-0 border-r border-border p-4 space-y-4">
        <ProjectSwitcher current={project} />
        <nav className="space-y-1">
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
          <Link
            href={`/code/${project.id}`}
            className="block px-3 py-2 rounded-md text-sm transition-colors text-muted-foreground hover:text-foreground hover:bg-muted"
          >
            Code
          </Link>
        </nav>
      </aside>
      <div className="flex-1">
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
