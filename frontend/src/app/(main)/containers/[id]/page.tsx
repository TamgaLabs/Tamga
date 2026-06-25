"use client";

import { useEffect, useState } from "react";
import { useParams, useRouter } from "next/navigation";
import {
  getContainer,
  getContainerLogs,
  getContainerStats,
  startContainer,
  stopContainer,
  restartContainer,
  removeContainer,
  type ContainerStats,
} from "@/lib/api";
import { useAuth } from "@/lib/auth";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";

type Tab = "inspect" | "logs" | "stats";

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

  const fetchLogs = async () => {
    try {
      const res = await getContainerLogs(id);
      setLogs(res.logs);
    } catch (e) {
      console.error(e);
    }
  };

  const fetchStats = async () => {
    try {
      const s = await getContainerStats(id);
      setStats(s);
    } catch (e) {
      console.error(e);
    }
  };

  useEffect(() => {
    if (tab === "logs") fetchLogs();
    if (tab === "stats") fetchStats();
  }, [tab]);

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

  const tabs = [
    { id: "inspect" as const, label: "Inspect" },
    { id: "logs" as const, label: "Logs" },
    { id: "stats" as const, label: "Stats" },
  ];

  return (
    <div className="p-6 max-w-6xl mx-auto">
      <Button variant="ghost" onClick={() => router.push("/containers")} className="mb-4">
        &larr; Containers
      </Button>

      {loading ? (
        <p className="text-neutral-400">Loading...</p>
      ) : !container ? (
        <p className="text-neutral-500">Container not found.</p>
      ) : (
        <>
          <div className="flex items-center justify-between mb-6">
            <div>
              <h1 className="text-2xl font-bold font-mono">{container.Name?.replace(/^\//, "") || id.slice(0, 12)}</h1>
              <p className="text-sm text-neutral-400 mt-1">{container.Config?.Image}</p>
            </div>
            <div className="flex gap-2">
              {container.State?.Status === "running" && (
                <Button variant="outline" size="sm" onClick={() => handleAction("stop")}>Stop</Button>
              )}
              {container.State?.Status !== "running" && (
                <Button variant="outline" size="sm" onClick={() => handleAction("start")}>Start</Button>
              )}
              <Button variant="outline" size="sm" onClick={() => handleAction("restart")}>Restart</Button>
              <Button variant="destructive" size="sm" onClick={() => handleAction("remove")}>Remove</Button>
            </div>
          </div>

          <div className="flex gap-1 mb-6 border-b border-neutral-800">
            {tabs.map((t) => (
              <button
                key={t.id}
                onClick={() => setTab(t.id)}
                className={`px-4 py-2 text-sm font-medium transition-colors ${
                  tab === t.id
                    ? "border-b-2 border-white text-white"
                    : "text-neutral-500 hover:text-neutral-300"
                }`}
              >
                {t.label}
              </button>
            ))}
          </div>

          {tab === "inspect" && (
            <Card>
              <CardHeader>
                <CardTitle className="text-sm">Container Config</CardTitle>
              </CardHeader>
              <CardContent>
                <pre className="bg-black rounded p-4 text-xs text-green-400 overflow-auto max-h-[70vh] font-mono whitespace-pre-wrap">
                  {JSON.stringify(container, null, 2)}
                </pre>
              </CardContent>
            </Card>
          )}

          {tab === "logs" && (
            <Card>
              <CardHeader className="flex flex-row items-center justify-between">
                <CardTitle className="text-sm">Logs</CardTitle>
                <Button variant="ghost" size="sm" onClick={fetchLogs}>Refresh</Button>
              </CardHeader>
              <CardContent>
                <pre className="bg-black rounded p-4 text-xs text-green-400 overflow-auto max-h-[70vh] font-mono whitespace-pre-wrap">
                  {logs || "(no output)"}
                </pre>
              </CardContent>
            </Card>
          )}

          {tab === "stats" && (
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
                      <p className="text-xs text-neutral-400 mt-1">
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
                    <div className="text-xs space-y-1 text-neutral-400">
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
          )}
        </>
      )}
    </div>
  );
}
