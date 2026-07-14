"use client";

import { createContext, useCallback, useContext, useEffect, useState } from "react";
import { usePathname, useRouter } from "next/navigation";
import { listProjects, type Project } from "@/lib/api";
import { useAuth } from "@/lib/auth";

export type WorkspaceView = "all" | "non-project" | number;

type WorkspaceContextValue = {
  view: WorkspaceView;
  setView: (view: WorkspaceView) => void;
  projects: Project[];
  loading: boolean;
  selectedProject: Project | null;
  refetchProjects: () => void;
};

const WorkspaceContext = createContext<WorkspaceContextValue | null>(null);

function deriveViewFromPath(pathname: string): WorkspaceView {
  if (pathname === "/dashboard") return "all";
  if (pathname === "/dashboard/non-project") return "non-project";
  const projectMatch = pathname.match(/^\/projects\/(\d+)/);
  if (projectMatch) return Number(projectMatch[1]);
  const codeMatch = pathname.match(/^\/code\/(\d+)/);
  if (codeMatch) return Number(codeMatch[1]);
  return "all";
}

function navigateForView(view: WorkspaceView, router: ReturnType<typeof useRouter>) {
  if (view === "all") {
    router.push("/dashboard");
  } else if (view === "non-project") {
    router.push("/dashboard/non-project");
  } else {
    router.push(`/projects/${view}`);
  }
}

export function WorkspaceProvider({ children }: { children: React.ReactNode }) {
  const { user } = useAuth();
  const pathname = usePathname();
  const router = useRouter();
  const [projects, setProjects] = useState<Project[]>([]);
  const [loading, setLoading] = useState(true);
  const [view, setViewState] = useState<WorkspaceView>("all");

  const refetchProjects = useCallback(() => {
    if (!user) return;
    setLoading(true);
    listProjects()
      .then(setProjects)
      .catch(() => setProjects([]))
      .finally(() => setLoading(false));
  }, [user]);

  useEffect(() => {
    refetchProjects();
  }, [refetchProjects]);

  useEffect(() => {
    setViewState(deriveViewFromPath(pathname));
  }, [pathname]);

  const setView = useCallback(
    (newView: WorkspaceView) => {
      setViewState(newView);
      navigateForView(newView, router);
    },
    [router]
  );

  const selectedProject =
    typeof view === "number" ? projects.find((p) => p.id === view) ?? null : null;

  return (
    <WorkspaceContext.Provider value={{ view, setView, projects, loading, selectedProject, refetchProjects }}>
      {children}
    </WorkspaceContext.Provider>
  );
}

export function useWorkspace(): WorkspaceContextValue {
  const ctx = useContext(WorkspaceContext);
  if (!ctx) {
    throw new Error("useWorkspace must be used within a WorkspaceProvider");
  }
  return ctx;
}
