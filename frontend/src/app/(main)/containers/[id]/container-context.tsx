"use client";

import { createContext, useContext } from "react";

export type ContainerContextValue = {
  id: string;
  container: any;
  refetch: () => void;
};

const ContainerContext = createContext<ContainerContextValue | null>(null);

export const ContainerContextProvider = ContainerContext.Provider;

// Sub-routes under /containers/[id] share the container fetched once by the
// layout instead of each re-fetching it (mirrors ProjectContext from
// projects/[id]/project-context.tsx).
export function useContainerContext(): ContainerContextValue {
  const ctx = useContext(ContainerContext);
  if (!ctx) {
    throw new Error("useContainerContext must be used within the container detail layout");
  }
  return ctx;
}
