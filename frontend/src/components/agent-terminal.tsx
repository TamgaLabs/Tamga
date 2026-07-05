"use client";

import { useEffect, useRef } from "react";
import { Terminal } from "@xterm/xterm";
import { FitAddon } from "@xterm/addon-fit";
import "@xterm/xterm/css/xterm.css";
import { agentTerminalUrl } from "@/lib/api";

// Wire protocol: the server streams raw shell output as WebSocket binary
// frames. The browser sends JSON text frames for keystrokes and resize
// events - see backend/internal/handler/terminal_handler.go.
type ClientMessage =
  | { type: "input"; data: string }
  | { type: "resize"; cols: number; rows: number };

export function AgentTerminal({ projectId }: { projectId: number }) {
  const containerRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const el = containerRef.current;
    if (!el) return;

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

    const ws = new WebSocket(agentTerminalUrl(projectId));
    ws.binaryType = "arraybuffer";

    const send = (msg: ClientMessage) => {
      if (ws.readyState === WebSocket.OPEN) ws.send(JSON.stringify(msg));
    };

    ws.onopen = () => {
      fitAddon.fit();
      send({ type: "resize", cols: term.cols, rows: term.rows });
    };
    ws.onmessage = (ev) => {
      term.write(ev.data instanceof ArrayBuffer ? new Uint8Array(ev.data) : ev.data);
    };
    ws.onclose = () => term.writeln("\r\n\x1b[90m[connection closed]\x1b[0m");
    ws.onerror = () => term.writeln("\r\n\x1b[31m[connection error]\x1b[0m");

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
      ws.close();
      term.dispose();
    };
  }, [projectId]);

  return <div ref={containerRef} className="h-full w-full bg-black p-2" />;
}
