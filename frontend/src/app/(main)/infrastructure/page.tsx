"use client";

import React, { useEffect } from "react";
import { useRouter } from "next/navigation";
import { useAuth } from "@/lib/auth";
import { useSystemTopology } from "@/hooks/useTopology";
import { useGlobalTrafficOverlay } from "@/components/topology/useTrafficOverlay";
import { TopologyGraph } from "@/components/topology";
import type { TopologyNode } from "@/lib/api";

export default function InfrastructurePage() {
  const { user, loading: authLoading } = useAuth();
  const router = useRouter();

  // Fetch system topology with auto-refresh
  const { data: topology, loading, error } = useSystemTopology({
    refetchInterval: 8000, // Poll every 8 seconds
  });

  // Compute traffic overlay decorations (FEAT-039)
  const { nodeDecorations, edgeDecorations, nodeStats } = useGlobalTrafficOverlay(topology);

  // Redirect if not authenticated
  useEffect(() => {
    if (!authLoading && !user) {
      router.replace("/login");
    }
  }, [user, authLoading, router]);

  const handleNodeClick = (node: TopologyNode) => {
    router.push(`/containers/${node.id}`);
  };

  if (authLoading || !user) {
    return null;
  }

  return (
    <div className="p-6 max-w-7xl mx-auto">
      {/* Header */}
      <div className="mb-8">
        <h1 className="text-2xl font-bold mb-4">Infrastructure</h1>
      </div>

      {/* Error state */}
      {error && (
        <div className="mb-6 p-4 bg-destructive/10 border border-destructive/30 rounded-lg text-sm text-destructive">
          Failed to load topology: {error.message}
        </div>
      )}

      {/* Topology graph */}
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

          {/* Legend for traffic overlay */}
          <div className="mt-6 p-4 bg-card border border-border rounded-lg">
            <h3 className="text-sm font-semibold mb-3">Traffic Overlay Legend</h3>
            <div className="grid grid-cols-2 gap-4 text-xs">
              <div>
                <div className="font-medium mb-2">Node Color (Error Rate)</div>
                <div className="flex items-center gap-2 mb-1">
                  <div className="w-4 h-4 rounded-full" style={{ backgroundColor: "hsl(142, 71%, 45%)" }} />
                  <span>&lt;1% errors (healthy)</span>
                </div>
                <div className="flex items-center gap-2 mb-1">
                  <div className="w-4 h-4 rounded-full" style={{ backgroundColor: "hsl(43, 96%, 56%)" }} />
                  <span>1–5% errors (warning)</span>
                </div>
                <div className="flex items-center gap-2">
                  <div className="w-4 h-4 rounded-full" style={{ backgroundColor: "hsl(0, 84.2%, 60.2%)" }} />
                  <span>≥5% errors (critical)</span>
                </div>
              </div>
              <div>
                <div className="font-medium mb-2">Edge Thickness (Request Volume)</div>
                <div className="text-muted-foreground">
                  Edges from Traefik to services thicken with request volume. Internal edges (app↔database) remain at base thickness—they have no Traefik metrics.
                </div>
              </div>
            </div>
          </div>
        </>
      )}

      {/* Loading state (shown while initially fetching) */}
      {loading && !topology && (
        <div className="text-center py-20 text-muted-foreground">
          <p className="text-lg">Loading infrastructure...</p>
        </div>
      )}
    </div>
  );
}
