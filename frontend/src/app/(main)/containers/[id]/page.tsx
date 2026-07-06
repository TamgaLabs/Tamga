"use client";

import { useEffect, useState, useCallback } from "react";
import { useParams, useRouter } from "next/navigation";
import {
  getContainer,
  getContainerLogs,
  getContainerStats,
  startContainer,
  stopContainer,
  restartContainer,
  removeContainer,
  updateContainerResources,
  type ContainerStats,
} from "@/lib/api";
import { useAuth } from "@/lib/auth";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { ScrollArea } from "@/components/ui/scroll-area";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";

type Tab = "inspect" | "logs" | "stats" | "resources";

// Log tail polling interval while the Logs tab is active. See Proposed
// Solution in FEAT-011: a plain refresh loop against the existing HTTP
// logs endpoint, not a new WebSocket log-tail endpoint - simpler and
// good enough for log tailing (unlike the interactive terminal, where
// sub-second latency actually matters).
const LOG_POLL_MS = 3000;

export default function ContainerDetailPage() {
  const params = useParams();
  const router = useRouter();
  const [container, setContainer] = useState<any>(null);
  const [logs, setLogs] = useState("");
  const [stats, setStats] = useState<ContainerStats | null>(null);
  const [tab, setTab] = useState<Tab>("inspect");
  const [loading, setLoading] = useState(true);
  const { user, loading: authLoading } = useAuth();
  const id = params.id as string;

  useEffect(() => {
    if (!authLoading && !user) router.replace("/login");
  }, [user, authLoading, router]);

  const fetchContainer = () => {
    if (!user || !id) return;
    setLoading(true);
    getContainer(id)
      .then(setContainer)
      .catch(console.error)
      .finally(() => setLoading(false));
  };

  useEffect(fetchContainer, [id, user]);

  const fetchLogs = useCallback(async () => {
    try {
      const res = await getContainerLogs(id);
      setLogs(res.logs);
    } catch (e) {
      console.error(e);
    }
  }, [id]);

  const fetchStats = useCallback(async () => {
    try {
      const s = await getContainerStats(id);
      setStats(s);
    } catch (e) {
      console.error(e);
    }
  }, [id]);

  useEffect(() => {
    if (tab === "logs") fetchLogs();
    if (tab === "stats") fetchStats();
  }, [tab, fetchLogs, fetchStats]);

  // Poll for new logs while the Logs tab is open, so the viewer feels
  // "live" without a WebSocket connection.
  useEffect(() => {
    if (tab !== "logs") return;
    const interval = setInterval(fetchLogs, LOG_POLL_MS);
    return () => clearInterval(interval);
  }, [tab, fetchLogs]);

  const handleAction = async (action: "start" | "stop" | "restart" | "remove") => {
    try {
      if (action === "start") await startContainer(id);
      else if (action === "stop") await stopContainer(id);
      else if (action === "restart") await restartContainer(id);
      else {
        await removeContainer(id);
        router.push("/containers");
        return;
      }
      fetchContainer();
    } catch (e) {
      console.error(e);
    }
  };

  if (authLoading || !user) return null;

  return (
    <div className="p-6 max-w-6xl mx-auto">
      <Button variant="ghost" onClick={() => router.push("/containers")} className="mb-4">
        &larr; Containers
      </Button>

      {loading ? (
        <p className="text-muted-foreground">Loading...</p>
      ) : !container ? (
        <p className="text-muted-foreground">Container not found.</p>
      ) : (
        <>
          <div className="flex items-center justify-between mb-6">
            <div>
              <h1 className="text-2xl font-bold font-mono">{container.Name?.replace(/^\//, "") || id.slice(0, 12)}</h1>
              <p className="text-sm text-muted-foreground mt-1">{container.Config?.Image}</p>
            </div>
            <div className="flex gap-2">
              {container.State?.Status === "running" ? (
                <Button variant="outline" size="sm" onClick={() => handleAction("stop")}>Stop</Button>
              ) : (
                <Button variant="outline" size="sm" onClick={() => handleAction("start")}>Start</Button>
              )}
              <Button variant="outline" size="sm" onClick={() => handleAction("restart")}>Restart</Button>
              <Button variant="destructive" size="sm" onClick={() => handleAction("remove")}>Remove</Button>
            </div>
          </div>

          <Tabs value={tab} onValueChange={(v) => setTab(v as Tab)}>
            <TabsList className="mb-6">
              <TabsTrigger value="inspect">Inspect</TabsTrigger>
              <TabsTrigger value="logs">Logs</TabsTrigger>
              <TabsTrigger value="stats">Stats</TabsTrigger>
              <TabsTrigger value="resources">Resources</TabsTrigger>
            </TabsList>

            <TabsContent value="inspect">
              <Card>
                <CardHeader>
                  <CardTitle className="text-sm">Container Config</CardTitle>
                </CardHeader>
                <CardContent>
                  <ScrollArea className="h-[70vh]">
                    <pre className="bg-code-block rounded p-4 text-xs text-success overflow-auto font-mono whitespace-pre-wrap">
                      {JSON.stringify(container, null, 2)}
                    </pre>
                  </ScrollArea>
                </CardContent>
              </Card>
            </TabsContent>

            <TabsContent value="logs">
              <Card>
                <CardHeader className="flex flex-row items-center justify-between">
                  <CardTitle className="text-sm">Logs</CardTitle>
                  <Button variant="ghost" size="sm" onClick={fetchLogs}>Refresh</Button>
                </CardHeader>
                <CardContent>
                  <ScrollArea className="h-[70vh]">
                    <pre className="bg-code-block rounded p-4 text-xs text-success overflow-auto font-mono whitespace-pre-wrap">
                      {logs || "(no output)"}
                    </pre>
                  </ScrollArea>
                </CardContent>
              </Card>
            </TabsContent>

            <TabsContent value="stats">
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
            </TabsContent>

            <TabsContent value="resources">
              <ResourcesTab id={id} container={container} onUpdate={fetchContainer} />
            </TabsContent>
          </Tabs>
        </>
      )}
    </div>
  );
}

function ResourcesTab({ id, container, onUpdate }: { id: string; container: any; onUpdate: () => void }) {
  const [memoryMiB, setMemoryMiB] = useState("");
  const [cpus, setCpus] = useState("");
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");

  useEffect(() => {
    const hostConfig = container?.HostConfig;
    setMemoryMiB(hostConfig?.Memory > 0 ? (hostConfig.Memory / 1024 ** 2).toString() : "");
    setCpus(hostConfig?.NanoCpus > 0 ? (hostConfig.NanoCpus / 1_000_000_000).toString() : "");
  }, [container]);

  const handleSave = async () => {
    const memMiB = parseFloat(memoryMiB);
    const cpuCores = parseFloat(cpus);
    if (!(memMiB > 0) && !(cpuCores > 0)) {
      setError("Enter a memory limit and/or CPU limit.");
      return;
    }
    setError("");
    setSaving(true);
    try {
      await updateContainerResources(id, {
        ...(memMiB > 0 ? { memory: Math.round(memMiB * 1024 ** 2) } : {}),
        ...(cpuCores > 0 ? { nano_cpus: Math.round(cpuCores * 1_000_000_000) } : {}),
      });
      onUpdate();
    } catch (e) {
      setError(e instanceof Error ? e.message : "Failed to update resources.");
    } finally {
      setSaving(false);
    }
  };

  return (
    <Card className="max-w-xl">
      <CardHeader>
        <CardTitle className="text-sm">Resource Limits</CardTitle>
      </CardHeader>
      <CardContent className="space-y-3">
        <p className="text-xs text-muted-foreground">
          Update this container&apos;s live memory and CPU limits. Applied
          immediately via Docker, no restart required.
        </p>
        <div className="flex gap-3">
          <div className="space-y-1 flex-1">
            <Label className="text-xs">Memory (MiB)</Label>
            <Input
              type="number"
              min="0"
              step="128"
              value={memoryMiB}
              onChange={(e) => setMemoryMiB(e.target.value)}
            />
          </div>
          <div className="space-y-1 flex-1">
            <Label className="text-xs">CPUs (cores)</Label>
            <Input
              type="number"
              min="0"
              step="0.25"
              value={cpus}
              onChange={(e) => setCpus(e.target.value)}
            />
          </div>
        </div>
        {error && <p className="text-xs text-destructive">{error}</p>}
        <Button size="sm" onClick={handleSave} disabled={saving}>
          {saving ? "Saving..." : "Save"}
        </Button>
      </CardContent>
    </Card>
  );
}
