"use client";

import { useEffect, useRef } from "react";
import { Terminal } from "@xterm/xterm";
import { FitAddon } from "@xterm/addon-fit";
import "@xterm/xterm/css/xterm.css";
import { agentTerminalUrl, listAgentSessions } from "@/lib/api";
import { isOfflineMode } from "@/lib/offline-api";

// Wire protocol: the server streams raw shell output as WebSocket binary
// frames. The browser sends JSON text frames for keystrokes and resize
// events - see backend/internal/handler/terminal_handler.go.
type ClientMessage =
  | { type: "input"; data: string }
  | { type: "resize"; cols: number; rows: number };

export function AgentTerminal({
  projectId,
  sessionId,
  knownSessionIds,
  onSessionResolved,
  onConnectFailed,
  onSessionTerminated,
}: {
  projectId: number;
  sessionId?: string;
  knownSessionIds?: string[];
  onSessionResolved?: (id: string) => void;
  onConnectFailed?: () => void;
  /** Fires when an established connection closes (session killed server-side). */
  onSessionTerminated?: () => void;
}) {
  const containerRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const el = containerRef.current;
    if (!el) return;

    const isNewSession = !sessionId;

    const term = new Terminal({
      cursorBlink: true,
      fontSize: 13,
      fontFamily: "monospace",
      convertEol: true,
    });
    const fitAddon = new FitAddon();
    term.loadAddon(fitAddon);
    term.open(el);
    fitAddon.fit();

    if (isOfflineMode()) {
      term.writeln("\x1b[90m[Offline preview: terminal connections are disabled]\x1b[0m");
      return () => term.dispose();
    }

    const ws = new WebSocket(agentTerminalUrl(projectId, sessionId));
    ws.binaryType = "arraybuffer";
    let opened = false;

    const send = (msg: ClientMessage) => {
      if (ws.readyState === WebSocket.OPEN) ws.send(JSON.stringify(msg));
    };

    ws.onopen = () => {
      opened = true;
      fitAddon.fit();
      send({ type: "resize", cols: term.cols, rows: term.rows });

      // The WS handshake doesn't tell us the new session's id (server ->
      // client is a raw byte stream, no envelope - see terminal_handler.go).
      // So once the socket is open (meaning the backend has already created
      // and registered the session), we re-fetch the session list and treat
      // whichever id we didn't already know about as ours. This assumes the
      // caller isn't racing another tab creation for the same project at
      // the same instant, which is fine for a single-user local tool.
      if (isNewSession && onSessionResolved) {
        const known = new Set(knownSessionIds || []);
        listAgentSessions(projectId)
          .then((sessions) => {
            const fresh = sessions.filter((s) => !known.has(s.id));
            if (fresh.length === 0) return;
            const newest = fresh.reduce((a, b) =>
              a.created_at > b.created_at ? a : b
            );
            onSessionResolved(newest.id);
          })
          .catch(() => {});
      }
    };
    ws.onmessage = (ev) => {
      term.write(ev.data instanceof ArrayBuffer ? new Uint8Array(ev.data) : ev.data);
    };
    ws.onclose = () => {
      term.writeln("\r\n\x1b[90m[connection closed]\x1b[0m");
      if (isNewSession && !opened) {
        onConnectFailed?.();
      } else if (opened) {
        onSessionTerminated?.();
      }
    };
    ws.onerror = () => {
      term.writeln("\r\n\x1b[31m[connection error]\x1b[0m");
    };

    const onData = term.onData((data) => send({ type: "input", data }));

    const handleResize = () => {
      fitAddon.fit();
      send({ type: "resize", cols: term.cols, rows: term.rows });
    };
    const resizeObserver = new ResizeObserver(handleResize);
    resizeObserver.observe(el);

    return () => {
      resizeObserver.disconnect();
      onData.dispose();
      // Closing the socket only detaches - the session itself keeps running
      // server-side (FEAT-015) until explicitly terminated via the REST
      // endpoint, so this is safe on tab switch / unmount / navigation.
      ws.close();
      term.dispose();
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [projectId, sessionId]);

  return <div ref={containerRef} className="h-full w-full bg-black p-2" />;
}
