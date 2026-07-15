"use client";

import { useCallback, useEffect, useState } from "react";
import { useParams, useRouter } from "next/navigation";
import { getProject, type Project } from "@/lib/api";
import { useAuth } from "@/lib/auth";
import { ProjectContextProvider } from "./project-context";
import { Skeleton } from "@/components/ui/skeleton";
import { TAMGA_SYSTEM_ID } from "@/contexts/workspace-context";

export default function ProjectDetailLayout({ children }: { children: React.ReactNode }) {
  const params = useParams();
  const router = useRouter();
  const { user, loading: authLoading } = useAuth();
  const [project, setProject] = useState<Project | null>(null);
  const [loading, setLoading] = useState(true);
  const id = Number(params.id);

  const fetchProject = useCallback(() => {
    if (!user || !params.id) return;
    if (id === TAMGA_SYSTEM_ID) {
      setProject({
        id: TAMGA_SYSTEM_ID,
        name: "Tamga System",
        source_type: "local",
        repo_url: "",
        branch: "",
        domain: "",
        status: "running",
        created_at: "",
        updated_at: "",
      });
      setLoading(false);
      return;
    }
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
        <div className="grid gap-4 md:grid-cols-2">
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

  return (
    <ProjectContextProvider value={{ project, refetch: fetchProject }}>
      <div key={project.id}>{children}</div>
    </ProjectContextProvider>
  );
}
