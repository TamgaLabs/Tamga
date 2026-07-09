"use client";

import { useCallback, useEffect, useState } from "react";
import { getContainerLogs } from "@/lib/api";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { ScrollArea } from "@/components/ui/scroll-area";
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

  const fetchLogs = useCallback(async () => {
    try {
      const res = await getContainerLogs(id);
      setLogs(res.logs);
    } catch (e) {
      console.error(e);
    }
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
    <div className="p-6 max-w-6xl mx-auto">
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
    </div>
  );
}
