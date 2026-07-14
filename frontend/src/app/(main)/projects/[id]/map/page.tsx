"use client";

import React from "react";
import { useRouter } from "next/navigation";
import { useProjectContext } from "../project-context";
import { useProjectTopology } from "@/hooks/useTopology";
import { useProjectTrafficOverlay } from "@/components/topology/useTrafficOverlay";
import type { TopologyNode } from "@/lib/api";
import { TopologyGraph } from "@/components/topology";
import { PageHeader, PageHeaderActions, PageHeaderDescription, PageHeaderTitle } from "@/components/page-header";
import { Badge } from "@/components/ui/badge";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { AlertCircle, Info, RefreshCw } from "lucide-react";

export default function ProjectMapPage() {
  const { project } = useProjectContext();
  const projectId = project.id;
  const router = useRouter();

  const { data: topology, loading, error } = useProjectTopology(projectId, {
    refetchInterval: 8000,
  });

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
          <Badge variant="secondary" className="gap-1.5">
            <RefreshCw className="size-3" />
            Refreshes every 8s
          </Badge>
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
