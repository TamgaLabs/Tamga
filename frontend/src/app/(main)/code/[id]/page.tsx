"use client";

import { useEffect, useState, useRef, useCallback } from "react";
import { useParams, useRouter } from "next/navigation";
import {
  getFileTree,
  readFile,
  writeFile,
  chatWithCodeAgent,
  getCodeTask,
  listCodeTasks,
  getCodeAgentStatus,
  startCodeAgent,
  stopCodeAgent,
  type FileEntry,
  type AgentTask,
} from "@/lib/api";
import { useAuth } from "@/lib/auth";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import dynamic from "next/dynamic";

const MonacoEditor = dynamic(() => import("@monaco-editor/react"), { ssr: false });

export default function CodeIDEPage() {
  const params = useParams();
  const router = useRouter();
  const projectId = Number(params.id);
  const { user, loading: authLoading } = useAuth();

  const [files, setFiles] = useState<FileEntry[]>([]);
  const [currentPath, setCurrentPath] = useState("");
  const [content, setContent] = useState("");
  const [originalContent, setOriginalContent] = useState("");
  const [dirty, setDirty] = useState(false);
  const [messages, setMessages] = useState<{ role: "user" | "agent"; content: string; taskId?: string }[]>([]);
  const [input, setInput] = useState("");
  const [agentLoading, setAgentLoading] = useState(false);
  const [selectedDiff, setSelectedDiff] = useState<string | null>(null);
  const [expandedDirs, setExpandedDirs] = useState<Set<string>>(new Set());
  const [agentRunning, setAgentRunning] = useState(false);
  const pollingRef = useRef<ReturnType<typeof setInterval> | null>(null);

  useEffect(() => {
    if (!authLoading && !user) router.replace("/login");
  }, [user, authLoading, router]);

  useEffect(() => {
    if (!user) return;
    getFileTree(projectId).then(setFiles).catch(console.error);
  }, [projectId, user]);

  // Check agent status on mount
  const checkAgentStatus = useCallback(() => {
    if (!user) return;
    getCodeAgentStatus(projectId).then((s) => setAgentRunning(s.running)).catch(console.error);
  }, [projectId, user]);

  useEffect(() => {
    checkAgentStatus();
  }, [checkAgentStatus]);

  // Load chat history on mount
  useEffect(() => {
    if (!user) return;
    listCodeTasks(projectId)
      .then((tasks) => {
        const history: { role: "user" | "agent"; content: string; taskId?: string }[] = [];
        for (const t of tasks) {
          history.push({ role: "user", content: t.message });
          if (t.response) {
            history.push({ role: "agent", content: t.response, taskId: t.id });
          }
          if (t.diff && !selectedDiff) {
            setSelectedDiff(t.diff);
          }
        }
        setMessages(history);
      })
      .catch(console.error);
  }, [projectId, user]);

  const openFile = async (path: string) => {
    if (dirty && !confirm("Discard unsaved changes?")) return;
    try {
      const res = await readFile(projectId, path);
      setCurrentPath(path);
      setContent(res.content);
      setOriginalContent(res.content);
      setDirty(false);
    } catch (e) {
      console.error(e);
    }
  };

  const handleSave = async () => {
    if (!currentPath) return;
    try {
      await writeFile(projectId, currentPath, content);
      setOriginalContent(content);
      setDirty(false);
    } catch (e) {
      console.error(e);
    }
  };

  const toggleDir = (path: string) => {
    setExpandedDirs((prev) => {
      const next = new Set(prev);
      if (next.has(path)) next.delete(path);
      else next.add(path);
      return next;
    });
  };

  const buildTree = (entries: FileEntry[]) => {
    const root: Record<string, any> = {};
    for (const e of entries) {
      const parts = e.path.split("/");
      let current = root;
      for (let i = 0; i < parts.length; i++) {
        const part = parts[i];
        if (!current[part]) current[part] = {};
        current = current[part];
      }
      current._entry = e;
    }

    const render = (obj: Record<string, any>, depth = 0): React.ReactNode[] => {
      const keys = Object.keys(obj).filter((k) => !k.startsWith("_"));
      keys.sort((a, b) => {
        const ae = obj[a]._entry;
        const be = obj[b]._entry;
        if (ae?.type !== be?.type) return ae?.type === "dir" ? -1 : 1;
        return a.localeCompare(b);
      });

      return keys.map((key) => {
        const entry = obj[key]._entry;
        const hasChildren = entry?.type === "dir";
        const isExpanded = expandedDirs.has(entry?.path);

        return (
          <div key={entry?.path || key}>
            <div
              className={`flex items-center gap-1 px-2 py-0.5 text-xs cursor-pointer rounded hover:bg-neutral-800 ${
                currentPath === entry?.path ? "bg-neutral-800 text-blue-400" : "text-neutral-400"
              }`}
              style={{ paddingLeft: `${12 + depth * 12}px` }}
              onClick={() => {
                if (hasChildren) toggleDir(entry.path);
                else if (entry) openFile(entry.path);
              }}
            >
              <span className="w-4 text-center">
                {hasChildren ? (isExpanded ? "▼" : "▶") : "📄"}
              </span>
              <span className="truncate">{key}</span>
            </div>
            {hasChildren && isExpanded && render(obj[key], depth + 1)}
          </div>
        );
      });
    };

    const children: Record<string, any> = {};
    const sorted = Object.keys(root).sort();
    for (const k of sorted) children[k] = root[k];
    return render(children);
  };

  const startPolling = useCallback((taskId: string) => {
    if (pollingRef.current) clearInterval(pollingRef.current);
    pollingRef.current = setInterval(async () => {
      try {
        const task: AgentTask = await getCodeTask(projectId, taskId);
        if (task.status === "completed" || task.status === "failed") {
          if (pollingRef.current) clearInterval(pollingRef.current);
          setMessages((prev) =>
            prev.map((m) =>
              m.taskId === taskId
                ? { ...m, role: "agent", content: task.response || "(no response)", taskId }
                : m
            )
          );
          if (task.diff) setSelectedDiff(task.diff);
          setAgentLoading(false);
          setAgentRunning(false);
          getFileTree(projectId).then(setFiles).catch(console.error);
        }
      } catch {}
    }, 2000);
  }, [projectId]);

  const handleSend = async () => {
    if (!input.trim() || agentLoading) return;
    const msg = input.trim();
    setInput("");
    try {
      const { task_id } = await chatWithCodeAgent(projectId, msg);
      setMessages((prev) => [
        ...prev,
        { role: "user", content: msg },
        { role: "agent", content: "Processing...", taskId: task_id },
      ]);
      setAgentLoading(true);
      setAgentRunning(true);
      startPolling(task_id);
    } catch (e) {
      console.error(e);
    }
  };

  const handleStartAgent = async () => {
    try {
      await startCodeAgent(projectId);
      setAgentRunning(true);
    } catch (e) {
      console.error(e);
    }
  };

  const handleStopAgent = async () => {
    try {
      await stopCodeAgent(projectId);
      setAgentRunning(false);
    } catch (e) {
      console.error(e);
    }
  };

  useEffect(() => {
    return () => {
      if (pollingRef.current) clearInterval(pollingRef.current);
    };
  }, []);

  const detectLanguage = (path: string): string => {
    const ext = path.split(".").pop()?.toLowerCase() || "";
    const map: Record<string, string> = {
      ts: "typescript", tsx: "typescript", js: "javascript", jsx: "javascript",
      go: "go", py: "python", rs: "rust", rb: "ruby", java: "java",
      json: "json", yaml: "yaml", yml: "yaml", md: "markdown",
      css: "css", scss: "scss", html: "html", xml: "xml",
      sql: "sql", sh: "shell", bash: "shell", dockerfile: "dockerfile",
    };
    return map[ext] || "plaintext";
  };

  if (authLoading || !user) return null;

  return (
    <div className="h-screen flex">
      {/* File tree */}
      <div className="w-64 bg-neutral-900 border-r border-neutral-800 overflow-auto flex-shrink-0">
        <div className="p-3 text-xs font-semibold text-neutral-500 uppercase tracking-wider border-b border-neutral-800">
          Files
        </div>
        <div className="py-1">{buildTree(files)}</div>
      </div>

      {/* Editor */}
      <div className="flex-1 flex flex-col min-w-0">
        {currentPath ? (
          <>
            <div className="flex items-center justify-between px-4 py-2 border-b border-neutral-800 bg-neutral-900">
              <span className="text-sm text-neutral-300 font-mono">{currentPath}</span>
              <div className="flex gap-2">
                {dirty && (
                  <Button size="sm" variant="outline" onClick={handleSave}>
                    Save
                  </Button>
                )}
                <Button
                  size="sm"
                  variant="ghost"
                  onClick={() => { setCurrentPath(""); setContent(""); setOriginalContent(""); }}
                >
                  Close
                </Button>
              </div>
            </div>
            <div className="flex-1">
              <MonacoEditor
                language={detectLanguage(currentPath)}
                theme="vs-dark"
                value={content}
                onChange={(v) => {
                  setContent(v || "");
                  setDirty(v !== originalContent);
                }}
                options={{
                  minimap: { enabled: false },
                  fontSize: 13,
                  lineNumbers: "on",
                  scrollBeyondLastLine: false,
                  automaticLayout: true,
                }}
              />
            </div>
          </>
        ) : (
          <div className="flex-1 flex items-center justify-center text-neutral-500">
            Select a file to edit
          </div>
        )}
      </div>

      {/* Agent Chat + Diff */}
      <div className="w-80 bg-neutral-900 border-l border-neutral-800 flex flex-col flex-shrink-0">
        {/* Chat */}
        <div className="flex-1 flex flex-col min-h-0">
          <div className="px-3 py-2 text-xs font-semibold text-neutral-500 uppercase tracking-wider border-b border-neutral-800 flex items-center justify-between">
            <div className="flex items-center gap-2">
              <span className={`w-2 h-2 rounded-full ${agentRunning ? "bg-green-500" : "bg-red-500"}`} />
              <span>Agent Chat</span>
            </div>
            <div className="flex gap-1">
              {agentRunning ? (
                <Button size="sm" variant="ghost" className="text-xs text-red-400 h-auto py-0.5" onClick={handleStopAgent}>
                  Stop
                </Button>
              ) : (
                <Button size="sm" variant="ghost" className="text-xs text-green-400 h-auto py-0.5" onClick={handleStartAgent}>
                  Start
                </Button>
              )}
            </div>
          </div>
          <div className="flex-1 overflow-auto p-3 space-y-3">
            {messages.length === 0 && (
              <p className="text-xs text-neutral-500">
                Ask the AI agent to make changes to this codebase.
              </p>
            )}
            {messages.map((m, i) => (
              <div key={i} className={`text-xs ${m.role === "user" ? "text-blue-400" : "text-green-400"}`}>
                <span className="font-bold">{m.role === "user" ? "You" : "Agent"}:</span> {m.content}
              </div>
            ))}
            {agentLoading && <p className="text-xs text-neutral-500">Agent is thinking...</p>}
          </div>
          <div className="p-3 border-t border-neutral-800 flex gap-2">
            <Input
              value={input}
              onChange={(e) => setInput(e.target.value)}
              onKeyDown={(e) => e.key === "Enter" && handleSend()}
              placeholder="Message..."
              disabled={agentLoading}
              className="text-xs"
            />
            <Button size="sm" onClick={handleSend} disabled={agentLoading}>
              Send
            </Button>
          </div>
        </div>

        {/* Diff */}
        <div className="h-1/2 border-t border-neutral-800 flex flex-col">
          <div className="px-3 py-2 text-xs font-semibold text-neutral-500 uppercase tracking-wider border-b border-neutral-800">
            Diff
          </div>
          <div className="flex-1 overflow-auto p-3">
            {selectedDiff ? (
              <pre className="text-xs text-green-400 font-mono whitespace-pre-wrap">{selectedDiff}</pre>
            ) : (
              <span className="text-xs text-neutral-500">Waiting for changes...</span>
            )}
          </div>
        </div>
      </div>
    </div>
  );
}
