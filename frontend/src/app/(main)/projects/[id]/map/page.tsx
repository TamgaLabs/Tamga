"use client";

import React, { useMemo } from "react";
import { useRouter } from "next/navigation";
import { useProjectContext } from "../project-context";
import { useProjectTopology, useSystemTopology } from "@/hooks/useTopology";
import { useProjectTrafficOverlay } from "@/components/topology/useTrafficOverlay";
import { getShowSystem } from "@/lib/settings";
import type { TopologyNode } from "@/lib/api";
import type { Topology } from "@/lib/topology-types";
import { TopologyGraph } from "@/components/topology";
import { PageHeader, PageHeaderActions, PageHeaderDescription, PageHeaderTitle } from "@/components/page-header";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { AlertCircle, Info, RefreshCw } from "lucide-react";

function useMergedTopology(projectId: number) {
  const projectTopo = useProjectTopology(projectId);
  const systemTopo = useSystemTopology();

  const merged = useMemo<Topology | null>(() => {
    if (!projectTopo.data) return null;

    const showSystem = getShowSystem();
    const projectNodeNames = new Set(projectTopo.data.nodes.map((n) => n.name));

    const systemNodes = showSystem && systemTopo.data
      ? systemTopo.data.nodes.filter((n) => !projectNodeNames.has(n.name))
      : [];

    const systemEdges = showSystem && systemTopo.data
      ? systemTopo.data.edges.filter(
          (e) => projectNodeNames.has(e.source) || projectNodeNames.has(e.target),
        )
      : [];

    return {
      nodes: [...projectTopo.data.nodes, ...systemNodes],
      edges: [...projectTopo.data.edges, ...systemEdges],
    };
  }, [projectTopo.data, systemTopo.data]);

  return {
    data: merged,
    loading: projectTopo.loading || systemTopo.loading,
    error: projectTopo.error || systemTopo.error,
    refetch: () => { projectTopo.refetch(); systemTopo.refetch(); },
  };
}

export default function ProjectMapPage() {
  const { project } = useProjectContext();
  const projectId = project.id;
  const router = useRouter();

  const { data: topology, loading, error, refetch } = useMergedTopology(projectId);
  const { nodeDecorations, edgeDecorations, nodeStats } = useProjectTrafficOverlay(topology, projectId);

  const handleNodeClick = (node: TopologyNode) => {
    router.push(`/containers/${node.id}`);
  };

  return (
    <div className="mx-auto max-w-7xl p-4 sm:p-6">
      <PageHeader className="mb-6">
        <div>
          <PageHeaderTitle>Topology</PageHeaderTitle>
          <PageHeaderDescription>Live topology and traffic paths for this project.</PageHeaderDescription>
        </div>
        <PageHeaderActions>
          <Button variant="outline" size="sm" onClick={() => refetch()} disabled={loading}>
            <RefreshCw className={`size-3.5 ${loading ? "animate-spin" : ""}`} />
            Refresh
          </Button>
        </PageHeaderActions>
      </PageHeader>

      {error && (
        <div role="alert" className="mb-6 flex gap-3 rounded-xl border border-destructive/30 bg-destructive/10 p-4 text-sm text-destructive">
          <AlertCircle className="mt-0.5 size-4 shrink-0" />
          <div>
            <p className="font-medium">Topology is unavailable</p>
            <p className="mt-1 text-destructive/80">{error.message}</p>
          </div>
        </div>
      )}

      {loading && !topology && (
        <div aria-label="Loading topology"><Skeleton className="h-[32rem] w-full" /></div>
      )}

      {topology && (
        <>
          <TopologyGraph
            topology={topology}
            onNodeClick={handleNodeClick}
            loading={loading}
            nodeDecorations={nodeDecorations}
            nodeStats={nodeStats}
            edgeDecorations={edgeDecorations}
          />

          <Card className="mt-4">
            <CardHeader className="gap-2 p-4 pb-3 sm:p-5 sm:pb-3"><CardTitle className="flex items-center gap-2 text-sm"><Info className="size-4 text-muted-foreground" />Traffic overlay</CardTitle><p className="text-xs text-muted-foreground">Node colour shows errors; edge weight shows traffic entering services.</p></CardHeader>
            <CardContent className="grid gap-5 p-4 pt-0 text-xs sm:grid-cols-2 sm:p-5 sm:pt-0">
              <div>
                <div className="mb-2 font-medium">Node colour · error rate</div>
                <div className="mb-1 flex items-center gap-2">
                  <div className="size-3 rounded-full ring-1 ring-inset ring-black/10 dark:ring-white/15" style={{ backgroundColor: "hsl(142, 71%, 45%)" }} />
                  <span>&lt;1% errors (healthy)</span>
                </div>
                <div className="mb-1 flex items-center gap-2">
                  <div className="size-3 rounded-full ring-1 ring-inset ring-black/10 dark:ring-white/15" style={{ backgroundColor: "hsl(43, 96%, 56%)" }} />
                  <span>1–5% errors (warning)</span>
                </div>
                <div className="flex items-center gap-2">
                  <div className="size-3 rounded-full ring-1 ring-inset ring-black/10 dark:ring-white/15" style={{ backgroundColor: "hsl(0, 84.2%, 60.2%)" }} />
                  <span>≥5% errors (critical)</span>
                </div>
              </div>
              <div>
                <div className="mb-2 font-medium">Edge weight · request volume</div>
                <div className="text-muted-foreground">
                  Edges from Traefik to services thicken with request volume. Internal edges (app↔database) remain at base thickness.
                </div>
              </div>
            </CardContent>
          </Card>
        </>
      )}
    </div>
  );
}
