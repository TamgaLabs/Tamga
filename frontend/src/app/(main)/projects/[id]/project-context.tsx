"use client";

import { createContext, useContext } from "react";
import type { Project } from "@/lib/api";

export type ProjectContextValue = {
  project: Project;
  refetch: () => void;
};

const ProjectContext = createContext<ProjectContextValue | null>(null);

export const ProjectContextProvider = ProjectContext.Provider;

// Sub-routes under /projects/[id] share the project fetched once by the
// layout instead of each re-fetching it (the layout owns loading/not-found
// handling per BUG-024).
export function useProjectContext(): ProjectContextValue {
  const ctx = useContext(ProjectContext);
  if (!ctx) {
    throw new Error("useProjectContext must be used within the project detail layout");
  }
  return ctx;
}
