"use client";

import { createContext, useCallback, useContext, useEffect, useState } from "react";
import { usePathname, useRouter } from "next/navigation";
import { listProjects, type Project } from "@/lib/api";
import { useAuth } from "@/lib/auth";

/** Virtual project ID for system containers (tamga-backend, traefik, etc.). */
export const TAMGA_SYSTEM_ID = -1;

const TAMGA_SYSTEM_PROJECT: Project = {
  id: TAMGA_SYSTEM_ID,
  name: "Tamga System",
  source_type: "local",
  repo_url: "",
  branch: "",
  domain: "",
  status: "running",
  created_at: "",
  updated_at: "",
};

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
  if (pathname === "/dashboard/system") return TAMGA_SYSTEM_ID;
  const projectMatch = pathname.match(/^\/projects\/(-?\d+)/);
  if (projectMatch) return Number(projectMatch[1]);
  const codeMatch = pathname.match(/^\/code\/(-?\d+)/);
  if (codeMatch) return Number(codeMatch[1]);
  return "all";
}

function navigateForView(view: WorkspaceView, router: ReturnType<typeof useRouter>) {
  if (view === "all") {
    router.push("/dashboard");
  } else if (view === "non-project") {
    router.push("/dashboard/non-project");
  } else if (view === TAMGA_SYSTEM_ID) {
    router.push("/dashboard/system");
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

  const allProjects = [TAMGA_SYSTEM_PROJECT, ...projects];

  const selectedProject =
    typeof view === "number"
      ? view === TAMGA_SYSTEM_ID
        ? TAMGA_SYSTEM_PROJECT
        : projects.find((p) => p.id === view) ?? null
      : null;

  return (
    <WorkspaceContext.Provider value={{ view, setView, projects: allProjects, loading, selectedProject, refetchProjects }}>
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
