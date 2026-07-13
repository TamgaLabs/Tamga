"use client";

import { useEffect, useRef, useState } from "react";
import { useParams, useRouter } from "next/navigation";
import {
  getFileTree,
  readFile,
  writeFile,
  listAgentSessions,
  terminateAgentSession,
  type FileEntry,
  type AgentSession,
} from "@/lib/api";
import { AgentTerminal } from "@/components/agent-terminal";
import { mergeTerminalTabs, removeTerminalTab, type TerminalTab } from "@/lib/terminal-tabs";
import { useAuth } from "@/lib/auth";
import { useTheme } from "@/lib/theme";
import { Button } from "@/components/ui/button";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog";
import {
  SquareTerminal,
  Code2,
  PanelLeftOpen,
  PanelLeftClose,
  FileIcon,
  FolderIcon,
  FolderOpenIcon,
  Plus,
  X,
} from "lucide-react";
import dynamic from "next/dynamic";

const MAX_TERMINAL_SESSIONS = 10;

const MonacoEditor = dynamic(() => import("@monaco-editor/react"), { ssr: false });

export default function CodeIDEPage() {
  const params = useParams();
  const router = useRouter();
  const projectId = Number(params.id);
  const isProject = projectId > 0;
  const { user, loading: authLoading } = useAuth();
  const { resolvedTheme } = useTheme();

  const [mode, setMode] = useState<"terminal" | "code">(isProject ? "terminal" : "code");

  const [files, setFiles] = useState<FileEntry[]>([]);
  const [currentPath, setCurrentPath] = useState("");
  const [content, setContent] = useState("");
  const [originalContent, setOriginalContent] = useState("");
  const [dirty, setDirty] = useState(false);
  const [expandedDirs, setExpandedDirs] = useState<Set<string>>(new Set());
  const [showFileTree, setShowFileTree] = useState(true);

  // Terminal tabs: fetched once on entry (mirrors the server's session
  // list from then on via local updates on create/terminate), not
  // re-fetched on every mode/tab switch - see Proposed Solution.
  const [tabs, setTabs] = useState<TerminalTab[]>([]);
  const [activeTabId, setActiveTabId] = useState<string | null>(null);
  // The entry-time session request can resolve after a successful DELETE.
  // Keep those ids out of that stale snapshot so it cannot resurrect a tab.
  const terminatedSessionIds = useRef(new Set<string>());
  const [terminalError, setTerminalError] = useState<string | null>(null);
  const [saveError, setSaveError] = useState<string | null>(null);
  const [tabPendingClose, setTabPendingClose] = useState<string | null>(null);

  useEffect(() => {
    if (!authLoading && !user) router.replace("/login");
  }, [user, authLoading, router]);

  useEffect(() => {
    if (!user || !isProject) return;
    listAgentSessions(projectId)
      .then((sessions: AgentSession[]) => {
        const sorted = [...sessions].sort((a, b) => a.created_at.localeCompare(b.created_at));
        // Merge server sessions with any existing tabs (e.g., pending tabs created
        // while this fetch was in flight). Preserve pending tabs, add server sessions
        // that aren't already tracked.
        setTabs((prev) => mergeTerminalTabs(prev, sorted, terminatedSessionIds.current));
        // Only set activeTabId if nothing is selected yet (user hasn't created or
        // switched to a tab).
        setActiveTabId((cur) => {
          if (cur !== null) return cur;
          return sorted.find((session) => !terminatedSessionIds.current.has(session.id))?.id || null;
        });
      })
      .catch((e) => setTerminalError(e instanceof Error ? e.message : "Failed to load terminal sessions"));
    // Fetch once on entry only - tabs are then kept in sync locally.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [projectId, user, isProject]);

  useEffect(() => {
    if (!user || mode !== "code") return;
    getFileTree(projectId).then(setFiles).catch(console.error);
  }, [projectId, user, mode]);

  const handleNewTab = () => {
    setTerminalError(null);
    if (tabs.length >= MAX_TERMINAL_SESSIONS) {
      setTerminalError(`Maximum of ${MAX_TERMINAL_SESSIONS} terminal sessions reached for this project.`);
      return;
    }
    const pendingId = `pending-${Date.now()}-${Math.random().toString(36).slice(2, 8)}`;
    setTabs((prev) => [...prev, { id: pendingId, pending: true }]);
    setActiveTabId(pendingId);
  };

  const handleSessionResolved = (pendingId: string, realId: string) => {
    setTabs((prev) => {
      // Dedup: if realId already exists as a real tab (e.g., from the seed
      // merge during concurrent WS creation), drop the pending tab instead of
      // renaming it into a duplicate.
      const realIdAlreadyExists = prev.some((t) => !t.pending && t.id === realId);
      if (realIdAlreadyExists) {
        return prev.filter((t) => t.id !== pendingId);
      }
      // Normal case: rename pending tab to real id
      return prev.map((t) => (t.id === pendingId ? { id: realId, pending: false } : t));
    });
    // Switch activeTabId if the pending tab was active (points to realId either way)
    setActiveTabId((prev) => (prev === pendingId ? realId : prev));
  };

  const handleConnectFailed = (pendingId: string) => {
    setTerminalError("Could not open a new terminal session (the server's 10-session limit may have been reached).");
    setTabs((prev) => {
      const next = prev.filter((t) => t.id !== pendingId);
      setActiveTabId((cur) => (cur === pendingId ? (next[0]?.id ?? null) : cur));
      return next;
    });
  };

  const handleTerminateTab = async (id: string) => {
    setTabPendingClose(null);
    try {
      await terminateAgentSession(projectId, id);
    } catch (e) {
      setTerminalError(e instanceof Error ? e.message : "Failed to terminate session");
      return;
    }
    terminatedSessionIds.current.add(id);
    setTabs((prev) => {
      // Use the active state's functional updater: the user may have selected
      // another tab while DELETE was in flight, and that newer selection must
      // not be overwritten by the async handler's old closure.
      setActiveTabId((current) => removeTerminalTab(prev, current, id).activeTabId);
      return removeTerminalTab(prev, null, id).tabs;
    });
  };

  const activeTab = tabs.find((t) => t.id === activeTabId) || null;

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
      setSaveError(e instanceof Error ? e.message : "Failed to save file");
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
              <span className="w-4 flex items-center justify-center shrink-0">
                {hasChildren ? (
                  isExpanded ? (
                    <FolderOpenIcon className="h-3.5 w-3.5" />
                  ) : (
                    <FolderIcon className="h-3.5 w-3.5" />
                  )
                ) : (
                  <FileIcon className="h-3.5 w-3.5" />
                )}
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
      <div className="flex items-center justify-between px-4 py-2 bg-card border-b border-border">
        <div className="flex items-center gap-2">
          {isProject && (
            <Button
              variant={mode === "terminal" ? "default" : "ghost"}
              size="sm"
              onClick={() => setMode("terminal")}
            >
              <SquareTerminal className="h-4 w-4 mr-1" />
              Terminal
            </Button>
          )}
          <Button
            variant={mode === "code" ? "default" : "ghost"}
            size="sm"
            onClick={() => { setMode("code"); getFileTree(projectId).then(setFiles).catch(console.error); }}
          >
            <Code2 className="h-4 w-4 mr-1" />
            Code
          </Button>
        </div>
      </div>

      {mode === "terminal" ? (
        <div className="flex-1 min-h-0 flex flex-col">
          <div className="flex items-center gap-1 px-2 py-1 bg-card border-b border-border overflow-x-auto">
            {tabs.map((tab, i) => (
              <div
                key={tab.id}
                className={`group flex items-center gap-1 pl-3 pr-1 py-1 rounded text-xs cursor-pointer shrink-0 ${
                  activeTabId === tab.id
                    ? "bg-muted text-foreground"
                    : "text-muted-foreground hover:bg-muted/50"
                }`}
                onClick={() => setActiveTabId(tab.id)}
              >
                <SquareTerminal className="h-3 w-3" />
                <span>{tab.pending ? "connecting…" : `Session ${i + 1}`}</span>
                <Button
                  variant="ghost"
                  size="icon"
                  className="h-4 w-4 opacity-60 hover:opacity-100"
                  disabled={tab.pending}
                  title="Terminate session"
                  onClick={(e) => {
                    e.stopPropagation();
                    setTabPendingClose(tab.id);
                  }}
                >
                  <X className="h-3 w-3" />
                </Button>
              </div>
            ))}
            <Button
              variant="ghost"
              size="icon"
              className="h-6 w-6 shrink-0"
              title="New terminal session"
              onClick={handleNewTab}
            >
              <Plus className="h-4 w-4" />
            </Button>
          </div>

          {terminalError && (
            <div className="px-3 py-1.5 text-xs text-destructive bg-destructive/10 border-b border-border flex items-center justify-between">
              <span>{terminalError}</span>
              <Button variant="ghost" size="icon" className="h-4 w-4" onClick={() => setTerminalError(null)}>
                <X className="h-3 w-3" />
              </Button>
            </div>
          )}

          <div className="flex-1 min-h-0">
            {activeTab ? (
              <AgentTerminal
                key={activeTab.id}
                projectId={projectId}
                sessionId={activeTab.pending ? undefined : activeTab.id}
                knownSessionIds={tabs.filter((t) => !t.pending).map((t) => t.id)}
                onSessionResolved={(realId) => handleSessionResolved(activeTab.id, realId)}
                onConnectFailed={() => handleConnectFailed(activeTab.id)}
              />
            ) : (
              <div className="h-full w-full flex items-center justify-center text-muted-foreground text-sm">
                <Button variant="outline" size="sm" onClick={handleNewTab}>
                  <Plus className="h-4 w-4 mr-1" />
                  New terminal session
                </Button>
              </div>
            )}
          </div>

          <AlertDialog open={tabPendingClose !== null} onOpenChange={(open) => !open && setTabPendingClose(null)}>
            <AlertDialogContent>
              <AlertDialogHeader>
                <AlertDialogTitle>Terminate this session?</AlertDialogTitle>
                <AlertDialogDescription>
                  This ends the shell process for real - it isn&apos;t just closing the tab. If it&apos;s the
                  project&apos;s last session, the sandbox container is stopped too.
                </AlertDialogDescription>
              </AlertDialogHeader>
              <AlertDialogFooter>
                <AlertDialogCancel>Cancel</AlertDialogCancel>
                <AlertDialogAction onClick={() => tabPendingClose && handleTerminateTab(tabPendingClose)}>
                  Terminate
                </AlertDialogAction>
              </AlertDialogFooter>
            </AlertDialogContent>
          </AlertDialog>
        </div>
      ) : (
        <div className="flex flex-1 min-h-0">
          {!showFileTree && (
            <Button
              variant="ghost"
              size="icon"
              onClick={() => setShowFileTree(true)}
              className="w-6 h-full rounded-none border-r border-border flex-shrink-0"
              title="Show file tree"
            >
              <PanelLeftOpen className="h-4 w-4" />
            </Button>
          )}

          {showFileTree && (
            <div className="w-64 bg-card border-r border-border overflow-auto flex-shrink-0">
              <div className="p-3 text-xs font-semibold text-muted-foreground uppercase tracking-wider border-b border-border flex items-center justify-between">
                <span>Files</span>
                <Button
                  variant="ghost"
                  size="icon"
                  className="h-5 w-5"
                  onClick={() => setShowFileTree(false)}
                >
                  <PanelLeftClose className="h-3 w-3" />
                </Button>
              </div>
              <div className="h-[calc(100%-36px)] overflow-auto">
                <div className="py-1">{buildTree(files)}</div>
              </div>
            </div>
          )}

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

                {saveError && (
                  <div className="px-3 py-1.5 text-xs text-destructive bg-destructive/10 border-b border-border flex items-center justify-between">
                    <span>{saveError}</span>
                    <Button variant="ghost" size="icon" className="h-4 w-4" onClick={() => setSaveError(null)}>
                      <X className="h-3 w-3" />
                    </Button>
                  </div>
                )}

                <div className="flex-1">
                  <MonacoEditor
                    language={detectLanguage(currentPath)}
                    theme={resolvedTheme === "dark" ? "vs-dark" : "vs"}
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
