"use client";

import { useCallback, useEffect, useState } from "react";
import { getContainerLogs } from "@/lib/api";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { ScrollArea } from "@/components/ui/scroll-area";
import { Skeleton } from "@/components/ui/skeleton";
import { useContainerContext } from "../container-context";

// Log tail polling interval while this route is mounted. See Proposed
// Solution in FEAT-011: a plain refresh loop against the existing HTTP
// logs endpoint, not a new WebSocket log-tail endpoint - simpler and
// good enough for log tailing (unlike the interactive terminal, where
// sub-second latency actually matters).
const LOG_POLL_MS = 3000;

export default function ContainerLogsPage() {
  const { id } = useContainerContext();
  const [logs, setLogs] = useState("");
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");

  const fetchLogs = useCallback(async () => {
    try {
      setError("");
      const res = await getContainerLogs(id);
      setLogs(res.logs);
    } catch (requestError) {
      console.error(requestError);
      setError(requestError instanceof Error ? requestError.message : "Failed to load container logs.");
    } finally { setLoading(false); }
  }, [id]);

  useEffect(() => {
    fetchLogs();
  }, [fetchLogs]);

  // Poll for new logs while this route is mounted, so the viewer feels
  // "live" without a WebSocket connection. Cleared on unmount (i.e. on
  // navigating away from /logs) so there's no background polling from
  // other routes.
  useEffect(() => {
    const interval = setInterval(fetchLogs, LOG_POLL_MS);
    return () => clearInterval(interval);
  }, [fetchLogs]);

  return (
    <div className="space-y-4">
      <Card>
        <CardHeader className="flex flex-row items-center justify-between">
          <CardTitle className="text-sm">Logs</CardTitle>
          <Button variant="ghost" size="sm" onClick={fetchLogs} disabled={loading}>{loading ? "Loading..." : "Refresh"}</Button>
        </CardHeader>
        <CardContent>
          {error ? <p role="alert" className="rounded-md border border-destructive/30 bg-destructive/10 p-3 text-sm text-destructive">{error}</p> : loading && !logs ? <Skeleton className="h-[50vh] w-full" /> : <ScrollArea className="h-[70vh] rounded-md border"><pre className="min-w-max bg-code-block p-4 text-xs text-success font-mono whitespace-pre-wrap">{logs || "(no output)"}</pre></ScrollArea>}
        </CardContent>
      </Card>
    </div>
  );
}
