"use client";

import { useEffect, useState, useRef, useCallback } from "react";
import { useParams, useRouter } from "next/navigation";
import {
  getFileTree,
  readFile,
  writeFile,
  chatWithCodeAgent,
  getCodeTask,
  listSessions,
  createSession,
  renameSession,
  deleteSession,
  listSessionTasks,
  getCodeAgentStatus,
  startCodeAgent,
  stopCodeAgent,
  type FileEntry,
  type AgentTask,
  type AgentSession,
} from "@/lib/api";
import { useAuth } from "@/lib/auth";
import { useTheme } from "@/lib/theme";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import dynamic from "next/dynamic";

const MonacoEditor = dynamic(() => import("@monaco-editor/react"), { ssr: false });

export default function CodeIDEPage() {
  const params = useParams();
  const router = useRouter();
  const projectId = Number(params.id);
  const { user, loading: authLoading } = useAuth();
  const { theme } = useTheme();

  // View mode: "chat" or "code"
  const [mode, setMode] = useState<"chat" | "code">("chat");

  // Code editor state
  const [files, setFiles] = useState<FileEntry[]>([]);
  const [currentPath, setCurrentPath] = useState("");
  const [content, setContent] = useState("");
  const [originalContent, setOriginalContent] = useState("");
  const [dirty, setDirty] = useState(false);
  const [expandedDirs, setExpandedDirs] = useState<Set<string>>(new Set());
  const [showFileTree, setShowFileTree] = useState(false);

  // Agent state
  const [sessions, setSessions] = useState<AgentSession[]>([]);
  const [activeSessionId, setActiveSessionId] = useState<string | null>(null);
  const [messages, setMessages] = useState<{ role: "user" | "agent"; content: string; taskId?: string; diff?: string }[]>([]);
  const [input, setInput] = useState("");
  const [agentLoading, setAgentLoading] = useState(false);
  const [selectedDiff, setSelectedDiff] = useState<string | null>(null);
  const [agentRunning, setAgentRunning] = useState(false);
  const [sessionMenu, setSessionMenu] = useState<string | null>(null);
  const [renamingSession, setRenamingSession] = useState<string | null>(null);
  const [renameValue, setRenameValue] = useState("");

  const pollingRef = useRef<ReturnType<typeof setInterval> | null>(null);
  const healthRef = useRef<ReturnType<typeof setInterval> | null>(null);
  const chatRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!authLoading && !user) router.replace("/login");
  }, [user, authLoading, router]);

  // Fetch file tree for code mode
  useEffect(() => {
    if (!user || mode !== "code") return;
    getFileTree(projectId).then(setFiles).catch(console.error);
  }, [projectId, user, mode]);

  // Check agent status
  const checkAgentStatus = useCallback(() => {
    if (!user) return;
    getCodeAgentStatus(projectId).then((s) => setAgentRunning(s.running)).catch(console.error);
  }, [projectId, user]);

  useEffect(() => {
    checkAgentStatus();
    healthRef.current = setInterval(checkAgentStatus, 10000);
    return () => {
      if (healthRef.current) clearInterval(healthRef.current);
    };
  }, [checkAgentStatus]);

  // Load sessions
  const loadSessions = useCallback(() => {
    if (!user) return;
    listSessions(projectId).then(setSessions).catch(console.error);
  }, [projectId, user]);

  useEffect(loadSessions, [loadSessions]);

  // Load messages when active session changes
  useEffect(() => {
    if (!user || !activeSessionId) {
      setMessages([]);
      setSelectedDiff(null);
      return;
    }
    listSessionTasks(projectId, activeSessionId)
      .then((tasks) => {
        const history: { role: "user" | "agent"; content: string; taskId?: string; diff?: string }[] = [];
        let latestDiff: string | null = null;
        for (const t of tasks) {
          history.push({ role: "user", content: t.message });
          if (t.response) {
            history.push({ role: "agent", content: t.response, taskId: t.id, diff: t.diff });
          }
          if (t.diff) latestDiff = t.diff;
        }
        setMessages(history);
        if (latestDiff) setSelectedDiff(latestDiff);
      })
      .catch(console.error);
  }, [projectId, user, activeSessionId]);

  // Scroll to bottom of chat
  useEffect(() => {
    if (chatRef.current) {
      chatRef.current.scrollTop = chatRef.current.scrollHeight;
    }
  }, [messages]);

  // Select first session on load
  useEffect(() => {
    if (sessions.length > 0 && !activeSessionId) {
      setActiveSessionId(sessions[0].id);
    }
  }, [sessions, activeSessionId]);

  // Cleanup
  useEffect(() => {
    return () => {
      if (pollingRef.current) clearInterval(pollingRef.current);
      if (healthRef.current) clearInterval(healthRef.current);
    };
  }, []);

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
              className={`flex items-center gap-1 px-2 py-0.5 text-xs cursor-pointer rounded hover:bg-muted ${
                currentPath === entry?.path ? "bg-muted text-accent" : "text-muted-foreground"
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
                ? { ...m, role: "agent", content: task.response || "(no response)", taskId, diff: task.diff }
                : m
            )
          );
          if (task.diff) setSelectedDiff(task.diff);
          setAgentLoading(false);
        }
      } catch {}
    }, 2000);
  }, [projectId]);

  const handleSend = async () => {
    if (!input.trim() || agentLoading) return;
    const msg = input.trim();
    setInput("");

    try {
      // If no active session, create one first
      let sessionId = activeSessionId;
      if (!sessionId) {
        const session = await createSession(projectId, msg.slice(0, 60));
        setSessions((prev) => [session, ...prev]);
        setActiveSessionId(session.id);
        sessionId = session.id;
      }

      const { task_id } = await chatWithCodeAgent(projectId, msg, sessionId || undefined);
      setMessages((prev) => [
        ...prev,
        { role: "user", content: msg },
        { role: "agent", content: "Processing...", taskId: task_id },
      ]);
      setAgentLoading(true);
      startPolling(task_id);

      // Refresh sessions to get updated name
      loadSessions();
    } catch (e) {
      console.error(e);
    }
  };

  const handleNewSession = async () => {
    try {
      const session = await createSession(projectId, "New Chat");
      setSessions((prev) => [session, ...prev]);
      setActiveSessionId(session.id);
    } catch (e) {
      console.error(e);
    }
  };

  const handleDeleteSession = async (id: string) => {
    try {
      await deleteSession(projectId, id);
      setSessions((prev) => prev.filter((s) => s.id !== id));
      if (activeSessionId === id) {
        setActiveSessionId(null);
        setMessages([]);
        setSelectedDiff(null);
      }
    } catch (e) {
      console.error(e);
    }
  };

  const handleRenameSession = async (id: string) => {
    if (!renameValue.trim()) return;
    try {
      await renameSession(projectId, id, renameValue.trim());
      setSessions((prev) => prev.map((s) => s.id === id ? { ...s, name: renameValue.trim() } : s));
      setRenamingSession(null);
      setRenameValue("");
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
    <div className="h-full flex flex-col">
      {/* Top bar with mode toggle and agent status */}
      <div className="flex items-center justify-between px-4 py-2 bg-card border-b border-border">
        <div className="flex items-center gap-2">
          <button
            onClick={() => setMode("chat")}
            className={`px-3 py-1 text-xs font-medium rounded transition-colors ${
              mode === "chat" ? "bg-accent text-accent-foreground" : "text-muted-foreground hover:text-foreground"
            }`}
          >
            Chat
          </button>
          <button
            onClick={() => { setMode("code"); getFileTree(projectId).then(setFiles).catch(console.error); }}
            className={`px-3 py-1 text-xs font-medium rounded transition-colors ${
              mode === "code" ? "bg-accent text-accent-foreground" : "text-muted-foreground hover:text-foreground"
            }`}
          >
            Code
          </button>
        </div>
        <div className="flex items-center gap-3">
          <div className="flex items-center gap-2">
            <span className={`w-2 h-2 rounded-full ${agentRunning ? "bg-success" : "bg-destructive"}`} />
            <span className="text-xs text-muted-foreground">Agent</span>
          </div>
          {agentRunning ? (
            <Button size="sm" variant="ghost" className="text-xs text-destructive h-auto py-0.5" onClick={handleStopAgent}>
              Stop
            </Button>
          ) : (
            <Button size="sm" variant="ghost" className="text-xs text-success h-auto py-0.5" onClick={handleStartAgent}>
              Start
            </Button>
          )}
        </div>
      </div>

      {mode === "chat" ? (
        /* ===== CHAT MODE ===== */
        <div className="flex flex-1 min-h-0">
          {/* Session sidebar */}
          <div className="w-56 bg-card border-r border-border flex flex-col flex-shrink-0">
            <div className="px-3 py-2 text-xs font-semibold text-muted-foreground uppercase tracking-wider border-b border-border flex items-center justify-between">
              <span>Sessions</span>
              <Button size="sm" variant="ghost" className="text-xs h-auto py-0.5" onClick={handleNewSession}>
                + New
              </Button>
            </div>
            <div className="flex-1 overflow-auto">
              {sessions.length === 0 && (
                <p className="p-3 text-xs text-muted-foreground">No sessions yet.</p>
              )}
              {sessions.map((s) => (
                <div key={s.id} className="relative">
                  <div
                    onClick={() => setActiveSessionId(s.id)}
                    className={`flex items-center justify-between px-3 py-2 text-xs cursor-pointer transition-colors ${
                      activeSessionId === s.id
                        ? "bg-muted text-accent"
                        : "text-muted-foreground hover:bg-muted/50"
                    }`}
                  >
                    <span className="truncate flex-1">{s.name}</span>
                    <button
                      onClick={(e) => { e.stopPropagation(); setSessionMenu(sessionMenu === s.id ? null : s.id); }}
                      className="text-muted-foreground/50 hover:text-foreground ml-1"
                    >
                      ⋯
                    </button>
                  </div>
                  {sessionMenu === s.id && (
                    <div className="absolute right-2 top-8 bg-card border border-border rounded shadow-lg z-10 min-w-24">
                      <button
                        className="block w-full text-left px-3 py-1.5 text-xs text-card-foreground hover:bg-muted"
                        onClick={() => { setRenamingSession(s.id); setRenameValue(s.name); setSessionMenu(null); }}
                      >
                        Rename
                      </button>
                      <button
                        className="block w-full text-left px-3 py-1.5 text-xs text-destructive hover:bg-muted"
                        onClick={() => { handleDeleteSession(s.id); setSessionMenu(null); }}
                      >
                        Delete
                      </button>
                    </div>
                  )}
                  {renamingSession === s.id && (
                    <div className="absolute inset-x-0 top-0 bg-card p-2 z-10 flex gap-1">
                      <Input
                        value={renameValue}
                        onChange={(e) => setRenameValue(e.target.value)}
                        onKeyDown={(e) => {
                          if (e.key === "Enter") handleRenameSession(s.id);
                          if (e.key === "Escape") setRenamingSession(null);
                        }}
                        className="text-xs h-7"
                        autoFocus
                      />
                      <Button size="sm" className="text-xs h-7" onClick={() => handleRenameSession(s.id)}>
                        Save
                      </Button>
                    </div>
                  )}
                </div>
              ))}
            </div>
          </div>

          {/* Chat area */}
          <div className="flex-1 flex flex-col min-w-0">
            <div ref={chatRef} className="flex-1 overflow-auto p-4 space-y-4">
              {messages.length === 0 && (
                <div className="flex items-center justify-center h-full">
                  <p className="text-sm text-muted-foreground">
                    {activeSessionId
                      ? "Start a conversation with the AI agent."
                      : "Create or select a session to start chatting."}
                  </p>
                </div>
              )}
              {messages.map((m, i) => (
                <div key={i} className={`flex ${m.role === "user" ? "justify-end" : "justify-start"}`}>
                  <div
                    className={`max-w-[80%] rounded-lg px-4 py-2 text-sm ${
                      m.role === "user"
                        ? "bg-accent text-accent-foreground"
                        : "bg-muted text-card-foreground"
                    }`}
                  >
                    <div className="whitespace-pre-wrap break-words">{m.content}</div>
                  </div>
                </div>
              ))}
              {agentLoading && (
                <div className="flex justify-start">
                  <div className="bg-muted rounded-lg px-4 py-2 text-sm text-muted-foreground">
                    <span className="animate-pulse">Agent is thinking...</span>
                  </div>
                </div>
              )}
            </div>
            <div className="p-3 border-t border-border flex gap-2">
              <Input
                value={input}
                onChange={(e) => setInput(e.target.value)}
                onKeyDown={(e) => e.key === "Enter" && handleSend()}
                placeholder="Message the AI agent..."
                disabled={agentLoading}
                className="text-sm"
              />
              <Button size="sm" onClick={handleSend} disabled={agentLoading}>
                Send
              </Button>
            </div>
          </div>

          {/* Diff panel */}
          <div className="w-96 bg-card border-l border-border flex flex-col flex-shrink-0">
            <div className="px-3 py-2 text-xs font-semibold text-muted-foreground uppercase tracking-wider border-b border-border">
              Diff
            </div>
            <div className="flex-1 overflow-auto p-3">
              {selectedDiff ? (
                <pre className="text-xs text-success font-mono whitespace-pre-wrap">{selectedDiff}</pre>
              ) : (
                <div className="flex items-center justify-center h-full">
                  <span className="text-xs text-muted-foreground">Waiting for changes...</span>
                </div>
              )}
            </div>
          </div>
        </div>
      ) : (
        /* ===== CODE MODE ===== */
        <div className="flex flex-1 min-h-0">
          {/* File tree toggle button */}
          {!showFileTree && (
            <button
              onClick={() => setShowFileTree(true)}
              className="w-6 bg-card border-r border-border hover:bg-muted flex items-center justify-center text-xs text-muted-foreground flex-shrink-0"
              title="Show file tree"
            >
              ▶
            </button>
          )}

          {/* File tree */}
          {showFileTree && (
            <div className="w-64 bg-card border-r border-border overflow-auto flex-shrink-0">
              <div className="p-3 text-xs font-semibold text-muted-foreground uppercase tracking-wider border-b border-border flex items-center justify-between">
                <span>Files</span>
                <button
                  onClick={() => setShowFileTree(false)}
                  className="text-muted-foreground hover:text-foreground text-xs"
                >
                  ◀
                </button>
              </div>
              <div className="py-1">{buildTree(files)}</div>
            </div>
          )}

          {/* Editor */}
          <div className="flex-1 flex flex-col min-w-0">
            {currentPath ? (
              <>
                <div className="flex items-center justify-between px-4 py-2 border-b border-border bg-card">
                  <span className="text-sm text-card-foreground font-mono">{currentPath}</span>
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
                    theme={theme === "dark" ? "vs-dark" : "vs"}
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
              <div className="flex-1 flex items-center justify-center text-muted-foreground">
                <div className="text-center">
                  <p className="mb-2">Select a file from the tree to edit</p>
                  {!showFileTree && (
                    <Button size="sm" variant="outline" onClick={() => setShowFileTree(true)}>
                      Show file tree
                    </Button>
                  )}
                </div>
              </div>
            )}
          </div>
        </div>
      )}
    </div>
  );
}
