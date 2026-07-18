"use client";

import { createContext, useCallback, useContext, useEffect, useState } from "react";
import { usePathname, useRouter } from "next/navigation";
import { listSeals, type Seal } from "@/lib/api";
import { useAuth } from "@/lib/auth";

/** Virtual Seal ID for system containers (tamga-backend, traefik, etc.). */
export const TAMGA_SYSTEM_ID = -1;

const TAMGA_SYSTEM_SEAL: Seal = {
  id: TAMGA_SYSTEM_ID,
  name: "Tamga System",
  source_type: "local",
  repo_url: "",
  branch: "",
  domain: "",
  status: "running",
  config_authority: "system",
  config_revision: 0,
  build_revision: 0,
  created_at: "",
  updated_at: "",
};

export type WorkspaceView = "all" | "non-seal" | number;

type WorkspaceContextValue = {
  view: WorkspaceView;
  setView: (view: WorkspaceView) => void;
  seals: Seal[];
  loading: boolean;
  selectedSeal: Seal | null;
  addSeal: (seal: Seal) => void;
  refetchSeals: () => void;
};

const WorkspaceContext = createContext<WorkspaceContextValue | null>(null);

function deriveViewFromPath(pathname: string): WorkspaceView {
  if (pathname === "/dashboard") return "all";
  if (pathname === "/dashboard/non-project") return "non-seal";
  if (pathname === "/dashboard/system") return TAMGA_SYSTEM_ID;
  const sealMatch = pathname.match(/^\/seals\/(-?\d+)/);
  if (sealMatch) return Number(sealMatch[1]);
  const codeMatch = pathname.match(/^\/code\/(-?\d+)/);
  if (codeMatch) return Number(codeMatch[1]);
  return "all";
}

function navigateForView(view: WorkspaceView, router: ReturnType<typeof useRouter>) {
  if (view === "all") {
    router.push("/dashboard");
  } else if (view === "non-seal") {
    router.push("/dashboard/non-project");
  } else if (view === TAMGA_SYSTEM_ID) {
    router.push("/dashboard/system");
  } else {
    router.push(`/seals/${view}/configure`);
  }
}

export function WorkspaceProvider({ children }: { children: React.ReactNode }) {
  const { user } = useAuth();
  const pathname = usePathname();
  const router = useRouter();
  const [seals, setSeals] = useState<Seal[]>([]);
  const [loading, setLoading] = useState(true);
  const [view, setViewState] = useState<WorkspaceView>("all");

  const refetchSeals = useCallback(() => {
    if (!user) return;
    setLoading(true);
    listSeals()
      .then((nextSeals) => {
        setSeals((currentSeals) => {
          const sealsByID = new Map(currentSeals.map((seal) => [seal.id, seal]));
          nextSeals.forEach((seal) => sealsByID.set(seal.id, seal));
          return [...sealsByID.values()];
        });
      })
      .catch(() => undefined)
      .finally(() => setLoading(false));
  }, [user]);

  const addSeal = useCallback((seal: Seal) => {
    setSeals((currentSeals) => {
      const existingIndex = currentSeals.findIndex((currentSeal) => currentSeal.id === seal.id);
      if (existingIndex === -1) return [...currentSeals, seal];
      return currentSeals.map((currentSeal) => currentSeal.id === seal.id ? seal : currentSeal);
    });
  }, []);

  useEffect(() => {
    refetchSeals();
  }, [refetchSeals]);

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

  const allSeals = [TAMGA_SYSTEM_SEAL, ...seals];

  const selectedSeal =
    typeof view === "number"
      ? view === TAMGA_SYSTEM_ID
        ? TAMGA_SYSTEM_SEAL
        : seals.find((seal) => seal.id === view) ?? null
      : null;

  return (
    <WorkspaceContext.Provider value={{ view, setView, seals: allSeals, loading, selectedSeal, addSeal, refetchSeals }}>
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
