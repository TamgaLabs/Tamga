"use client";

import { useCallback, useEffect, useState } from "react";
import { getContainerStats, type ContainerStats } from "@/lib/api";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { useContainerContext } from "../container-context";
import { Skeleton } from "@/components/ui/skeleton";

export default function ContainerStatsPage() {
  const { id } = useContainerContext();
  const [stats, setStats] = useState<ContainerStats | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");

  const fetchStats = useCallback(async () => {
    try {
      setLoading(true);
      setError("");
      const s = await getContainerStats(id);
      setStats(s);
    } catch (requestError) {
      console.error(requestError);
      setError(requestError instanceof Error ? requestError.message : "Container statistics are unavailable.");
    } finally { setLoading(false); }
  }, [id]);

  // Auto-load once on entering this route (matches the original
  // tab === "stats" auto-fetch-on-entry behavior); each card below also
  // keeps its own independent "Load Stats" fallback button, shown
  // whenever stats is still null (e.g. the auto-fetch above failed).
  useEffect(() => {
    fetchStats();
  }, [fetchStats]);

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between"><div><h2 className="text-lg font-semibold">Live statistics</h2><p className="text-sm text-muted-foreground">Fetch the latest CPU, memory, and network counters.</p></div><Button variant="outline" size="sm" onClick={fetchStats} disabled={loading}>{loading ? "Loading..." : "Refresh"}</Button></div>
      {error && <p role="alert" className="rounded-md border border-destructive/30 bg-destructive/10 p-3 text-sm text-destructive">{error}</p>}
      <div className="grid gap-4 md:grid-cols-3">
        <Card>
          <CardHeader>
            <CardTitle className="text-sm">CPU</CardTitle>
          </CardHeader>
          <CardContent>
            {loading && !stats ? <Skeleton className="h-9 w-24" /> : stats ? (
              <div className="text-2xl font-bold">{stats.cpu.percent.toFixed(1)}%</div>
            ) : (
              <Button variant="outline" size="sm" onClick={fetchStats}>Load Stats</Button>
            )}
          </CardContent>
        </Card>
        <Card>
          <CardHeader>
            <CardTitle className="text-sm">Memory</CardTitle>
          </CardHeader>
          <CardContent>
            {loading && !stats ? <Skeleton className="h-12 w-36" /> : stats ? (
              <div>
                <div className="text-2xl font-bold">{stats.mem.percent.toFixed(1)}%</div>
                <p className="text-xs text-muted-foreground mt-1">
                  {(stats.mem.usage / 1024 / 1024).toFixed(0)}MB / {(stats.mem.limit / 1024 / 1024).toFixed(0)}MB
                </p>
              </div>
            ) : (
              <Button variant="outline" size="sm" onClick={fetchStats}>Load Stats</Button>
            )}
          </CardContent>
        </Card>
        <Card>
          <CardHeader>
            <CardTitle className="text-sm">Network</CardTitle>
          </CardHeader>
          <CardContent>
            {loading && !stats ? <Skeleton className="h-16 w-full" /> : stats ? (
              <div className="text-xs space-y-1 text-muted-foreground">
                <p>RX: {(stats.net.rx_bytes / 1024).toFixed(1)}KB</p>
                <p>TX: {(stats.net.tx_bytes / 1024).toFixed(1)}KB</p>
                <p>RX packets: {stats.net.rx_packets}</p>
                <p>TX packets: {stats.net.tx_packets}</p>
              </div>
            ) : (
              <Button variant="outline" size="sm" onClick={fetchStats}>Load Stats</Button>
            )}
          </CardContent>
        </Card>
      </div>
    </div>
  );
}
