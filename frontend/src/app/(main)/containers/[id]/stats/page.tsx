"use client";

import { useCallback, useEffect, useState } from "react";
import { getContainerStats, type ContainerStats } from "@/lib/api";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { useContainerContext } from "../container-context";

export default function ContainerStatsPage() {
  const { id } = useContainerContext();
  const [stats, setStats] = useState<ContainerStats | null>(null);

  const fetchStats = useCallback(async () => {
    try {
      const s = await getContainerStats(id);
      setStats(s);
    } catch (e) {
      console.error(e);
    }
  }, [id]);

  // Auto-load once on entering this route (matches the original
  // tab === "stats" auto-fetch-on-entry behavior); each card below also
  // keeps its own independent "Load Stats" fallback button, shown
  // whenever stats is still null (e.g. the auto-fetch above failed).
  useEffect(() => {
    fetchStats();
  }, [fetchStats]);

  return (
    <div className="p-6 max-w-6xl mx-auto">
      <div className="grid gap-4 md:grid-cols-3">
        <Card>
          <CardHeader>
            <CardTitle className="text-sm">CPU</CardTitle>
          </CardHeader>
          <CardContent>
            {stats ? (
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
            {stats ? (
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
            {stats ? (
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
