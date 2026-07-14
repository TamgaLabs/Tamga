"use client";

import { useEffect, useRef, useState } from "react";
import { useParams, useRouter } from "next/navigation";
import {
  getFileTree,
  readFile,
  writeFile,
  listAgentSessions,
  terminateAgentSession,
  listContainers,
  startContainer,
  stopContainer,
  type FileEntry,
  type AgentSession,
  type ContainerInfo,
} from "@/lib/api";
import { AgentTerminal } from "@/components/agent-terminal";
import { mergeTerminalTabs, removeTerminalTab, type TerminalTab } from "@/lib/terminal-tabs";
import { useAuth } from "@/lib/auth";
import { useTheme } from "@/lib/theme";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { ScrollArea } from "@/components/ui/scroll-area";
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from "@/components/ui/tooltip";
import {
  Tabs,
  TabsList,
  TabsTrigger,
} from "@/components/ui/tabs";
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
  Empty,
  EmptyContent,
  EmptyDescription,
  EmptyHeader,
  EmptyMedia,
  EmptyTitle,
} from "@/components/ui/empty";
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
  Save,
  Lock,
  Play,
  Square,
  Loader2,
} from "lucide-react";
import { toast } from "sonner";
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

  const [tabs, setTabs] = useState<TerminalTab[]>([]);
  const [activeTabId, setActiveTabId] = useState<string | null>(null);
  const terminatedSessionIds = useRef(new Set<string>());
  const [terminalError, setTerminalError] = useState<string | null>(null);
  const [saveError, setSaveError] = useState<string | null>(null);
  const [tabPendingClose, setTabPendingClose] = useState<string | null>(null);

  const [container, setContainer] = useState<ContainerInfo | null>(null);
  const [containerActionPending, setContainerActionPending] = useState(false);

  const isReadOnly = !isProject;

  useEffect(() => {
    if (!authLoading && !user) router.replace("/login");
  }, [user, authLoading, router]);

  useEffect(() => {
    if (!user || !isProject) return;
    listAgentSessions(projectId)
      .then((sessions: AgentSession[]) => {
        const sorted = [...sessions].sort((a, b) => a.created_at.localeCompare(b.created_at));
        setTabs((prev) => mergeTerminalTabs(prev, sorted, terminatedSessionIds.current));
        setActiveTabId((cur) => {
          if (cur !== null) return cur;
          return sorted.find((session) => !terminatedSessionIds.current.has(session.id))?.id || null;
        });
      })
      .catch((e) => setTerminalError(e instanceof Error ? e.message : "Failed to load terminal sessions"));
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [projectId, user, isProject]);

  useEffect(() => {
    if (!user || mode !== "code") return;
    getFileTree(projectId).then(setFiles).catch(console.error);
  }, [projectId, user, mode]);

  const fetchContainer = () => {
    if (!isProject) return;
    listContainers()
      .then((all) => setContainer(all.find((c) => c.project_id === projectId) || null))
      .catch(() => {});
  };

  useEffect(() => {
    if (!user || !isProject) return;
    fetchContainer();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [projectId, user, isProject]);

  const handleContainerAction = async (action: "start" | "stop") => {
    if (!container || containerActionPending) return;
    const toastId = toast.loading(`${action === "start" ? "Starting" : "Stopping"} sandbox...`);
    setContainerActionPending(true);
    try {
      if (action === "start") await startContainer(container.id);
      else await stopContainer(container.id);
      fetchContainer();
      toast.success(`Sandbox ${action === "start" ? "started" : "stopped"}.`, { id: toastId });
    } catch (e) {
      toast.error(e instanceof Error ? e.message : `Failed to ${action} sandbox.`, { id: toastId });
    } finally {
      setContainerActionPending(false);
    }
  };

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
      const realIdAlreadyExists = prev.some((t) => !t.pending && t.id === realId);
      if (realIdAlreadyExists) {
        return prev.filter((t) => t.id !== pendingId);
      }
      return prev.map((t) => (t.id === pendingId ? { id: realId, pending: false } : t));
    });
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
      setActiveTabId((current) => removeTerminalTab(prev, current, id).activeTabId);
      return removeTerminalTab(prev, null, id).tabs;
    });
  };

  const activeTab = tabs.find((t) => t.id === activeTabId) || null;

  const openFile = async (path: string) => {
    if (isReadOnly) return;
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
    if (!currentPath || isReadOnly) return;
    const toastId = toast.loading("Saving…");
    try {
      await writeFile(projectId, currentPath, content);
      setOriginalContent(content);
      setDirty(false);
      toast.success("File saved.", { id: toastId });
    } catch (e) {
      const message = e instanceof Error ? e.message : "Failed to save file";
      setSaveError(message);
      toast.error(message, { id: toastId });
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
        const isActive = currentPath === entry?.path;

        return (
          <div key={entry?.path || key}>
            <button
              type="button"
              className={`flex w-full items-center gap-1.5 px-2 py-0.5 text-xs text-left transition-colors rounded hover:bg-muted ${
                isActive ? "bg-muted text-accent" : "text-muted-foreground"
              } ${isReadOnly && !hasChildren ? "opacity-60" : ""}`}
              style={{ paddingLeft: `${12 + depth * 14}px` }}
              onClick={() => {
                if (hasChildren) toggleDir(entry.path);
                else if (entry) openFile(entry.path);
              }}
              tabIndex={0}
              aria-label={hasChildren ? (isExpanded ? `Collapse ${key}` : `Expand ${key}`) : `Open ${key}`}
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
            </button>
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
    <TooltipProvider delayDuration={300}>
      <div className="h-[calc(100dvh-3.5rem)] flex flex-col">
        {/* Top bar: mode switcher + project context */}
        <div className="flex items-center justify-between px-4 py-2 bg-card border-b border-border">
          <Tabs value={mode} onValueChange={(v) => {
            const next = v as "terminal" | "code";
            setMode(next);
            if (next === "code") {
              getFileTree(projectId).then(setFiles).catch(console.error);
            }
          }}>
            <TabsList className="h-8">
              {isProject && (
                <TabsTrigger value="terminal" className="gap-1.5 text-xs h-6 px-3">
                  <SquareTerminal className="h-3.5 w-3.5" aria-hidden="true" />
                  Terminal
                </TabsTrigger>
              )}
              <TabsTrigger value="code" className="gap-1.5 text-xs h-6 px-3">
                <Code2 className="h-3.5 w-3.5" aria-hidden="true" />
                Code
              </TabsTrigger>
            </TabsList>
          </Tabs>

          {isReadOnly && (
            <Badge variant="warning" className="gap-1 text-xs">
              <Lock className="h-3 w-3" aria-hidden="true" />
              Read-only
            </Badge>
          )}
          {isProject && container && (
            <div className="flex items-center gap-2">
              <Badge variant={container.state === "running" ? "success" : container.state === "exited" ? "error" : "default"} className="gap-1 text-xs">
                {container.state}
              </Badge>
              {container.state === "running" ? (
                <Tooltip>
                  <TooltipTrigger asChild>
                    <Button
                      variant="outline"
                      size="sm"
                      className="h-6 gap-1 text-xs"
                      disabled={containerActionPending}
                      onClick={() => void handleContainerAction("stop")}
                    >
                      {containerActionPending ? <Loader2 className="h-3 w-3 animate-spin" /> : <Square className="h-3 w-3" />}
                      Stop
                    </Button>
                  </TooltipTrigger>
                  <TooltipContent>Stop the sandbox container</TooltipContent>
                </Tooltip>
              ) : container.state === "exited" ? (
                <Tooltip>
                  <TooltipTrigger asChild>
                    <Button
                      variant="outline"
                      size="sm"
                      className="h-6 gap-1 text-xs"
                      disabled={containerActionPending}
                      onClick={() => void handleContainerAction("start")}
                    >
                      {containerActionPending ? <Loader2 className="h-3 w-3 animate-spin" /> : <Play className="h-3 w-3" />}
                      Start
                    </Button>
                  </TooltipTrigger>
                  <TooltipContent>Start the sandbox container</TooltipContent>
                </Tooltip>
              ) : null}
            </div>
          )}
        </div>

        {mode === "terminal" ? (
          <div className="flex-1 min-h-0 flex flex-col">
            {/* Terminal tab strip */}
            <div className="flex items-center gap-1 px-2 py-1 bg-card border-b border-border overflow-x-auto">
              {tabs.map((tab, i) => (
                <div
                  key={tab.id}
                  className={`group flex items-center gap-1.5 pl-3 pr-1 py-1 rounded text-xs cursor-pointer shrink-0 transition-colors ${
                    activeTabId === tab.id
                      ? "bg-muted text-foreground"
                      : "text-muted-foreground hover:bg-muted/50"
                  }`}
                  onClick={() => setActiveTabId(tab.id)}
                  role="tab"
                  aria-selected={activeTabId === tab.id}
                  tabIndex={0}
                  onKeyDown={(e) => {
                    if (e.key === "Enter" || e.key === " ") {
                      e.preventDefault();
                      setActiveTabId(tab.id);
                    }
                  }}
                >
                  <SquareTerminal className="h-3 w-3" aria-hidden="true" />
                  <span>{tab.pending ? "Connecting…" : `Session ${i + 1}`}</span>
                  <Tooltip>
                    <TooltipTrigger asChild>
                      <Button
                        variant="ghost"
                        size="icon"
                        className="h-4 w-4 opacity-60 hover:!opacity-100"
                        disabled={tab.pending}
                        onClick={(e) => {
                          e.stopPropagation();
                          setTabPendingClose(tab.id);
                        }}
                      >
                        <X className="h-3 w-3" />
                      </Button>
                    </TooltipTrigger>
                    <TooltipContent side="bottom">Terminate session</TooltipContent>
                  </Tooltip>
                </div>
              ))}
              <Tooltip>
                <TooltipTrigger asChild>
                  <Button
                    variant="ghost"
                    size="icon"
                    className="h-6 w-6 shrink-0"
                    disabled={isProject && container !== null && container.state !== "running"}
                    onClick={handleNewTab}
                  >
                    <Plus className="h-4 w-4" />
                  </Button>
                </TooltipTrigger>
                <TooltipContent side="bottom">
                  {isProject && container !== null && container.state !== "running"
                    ? "Sandbox is stopped. Start it to open a new session."
                    : "New terminal session"}
                </TooltipContent>
              </Tooltip>
            </div>

            {terminalError && (
              <div role="alert" className="px-3 py-1.5 text-xs text-destructive bg-destructive/10 border-b border-border flex items-center justify-between">
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
                  onSessionTerminated={() => {
                    terminatedSessionIds.current.add(activeTab.id);
                    setTabs((prev) => {
                      setActiveTabId((current) => removeTerminalTab(prev, current, activeTab.id).activeTabId);
                      return removeTerminalTab(prev, null, activeTab.id).tabs;
                    });
                  }}
                />
              ) : (
                <Empty className="min-h-0 flex-1 border-dashed rounded-none">
                  <EmptyHeader>
                    <EmptyMedia><SquareTerminal className="size-5" aria-hidden="true" /></EmptyMedia>
                    <EmptyTitle>No terminal sessions</EmptyTitle>
                    <EmptyDescription>Create a session to open a shell in this project&apos;s sandbox.</EmptyDescription>
                  </EmptyHeader>
                  <EmptyContent>
                    <Tooltip>
                      <TooltipTrigger asChild>
                        <Button
                          size="sm"
                          disabled={isProject && container !== null && container.state !== "running"}
                          onClick={handleNewTab}
                        >
                          <Plus className="h-4 w-4 mr-1" aria-hidden="true" />
                          New session
                        </Button>
                      </TooltipTrigger>
                      {isProject && container !== null && container.state !== "running" && (
                        <TooltipContent>
                          Sandbox is stopped. Start it to open a new session.
                        </TooltipContent>
                      )}
                    </Tooltip>
                  </EmptyContent>
                </Empty>
              )}
            </div>

            <AlertDialog open={tabPendingClose !== null} onOpenChange={(open) => !open && setTabPendingClose(null)}>
              <AlertDialogContent>
                <AlertDialogHeader>
                  <AlertDialogTitle>Terminate this session?</AlertDialogTitle>
                  <AlertDialogDescription>
                    This ends the shell process for real &mdash; it isn&apos;t just closing the tab. If it&apos;s the
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
            {/* File tree panel */}
            {!showFileTree && (
              <Tooltip>
                <TooltipTrigger asChild>
                  <Button
                    variant="ghost"
                    size="icon"
                    onClick={() => setShowFileTree(true)}
                    className="w-8 h-full rounded-none border-r border-border flex-shrink-0"
                  >
                    <PanelLeftOpen className="h-4 w-4" />
                  </Button>
                </TooltipTrigger>
                <TooltipContent side="right">Show file tree</TooltipContent>
              </Tooltip>
            )}

            {showFileTree && (
              <div className="w-64 bg-card border-r border-border flex flex-col flex-shrink-0">
                <div className="flex items-center justify-between px-3 py-2 border-b border-border">
                  <span className="text-xs font-semibold text-muted-foreground uppercase tracking-wider">Files</span>
                  <Button
                    variant="ghost"
                    size="icon"
                    className="h-5 w-5"
                    onClick={() => setShowFileTree(false)}
                    title="Hide file tree"
                  >
                    <PanelLeftClose className="h-3 w-3" />
                  </Button>
                </div>
                <ScrollArea className="flex-1">
                  <div className="py-1">{buildTree(files)}</div>
                </ScrollArea>
              </div>
            )}

            {/* Editor area */}
            <div className="flex-1 flex flex-col min-w-0">
              {currentPath ? (
                <>
                  <div className="flex items-center justify-between px-4 py-2 border-b border-border bg-card gap-2">
                    <div className="flex items-center gap-2 min-w-0">
                      <span className="text-sm text-card-foreground font-mono truncate">{currentPath}</span>
                      {dirty && <Badge variant="warning" className="shrink-0 text-[10px] px-1.5 py-0">Unsaved</Badge>}
                    </div>
                    <div className="flex items-center gap-1.5 shrink-0">
                      {!isReadOnly && dirty && (
                        <Button size="sm" variant="default" onClick={handleSave} className="gap-1.5">
                          <Save className="h-3.5 w-3.5" aria-hidden="true" />
                          Save
                        </Button>
                      )}
                      <Button
                        size="sm"
                        variant="ghost"
                        onClick={() => { setCurrentPath(""); setContent(""); setOriginalContent(""); setDirty(false); }}
                      >
                        Close
                      </Button>
                    </div>
                  </div>

                  {saveError && (
                    <div role="alert" className="px-3 py-1.5 text-xs text-destructive bg-destructive/10 border-b border-border flex items-center justify-between">
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
                        readOnly: isReadOnly,
                      }}
                    />
                  </div>
                </>
              ) : (
                <Empty className="min-h-0 flex-1 border-dashed rounded-none">
                  <EmptyHeader>
                    <EmptyMedia><Code2 className="size-5" aria-hidden="true" /></EmptyMedia>
                    <EmptyTitle>Select a file to edit</EmptyTitle>
                    <EmptyDescription>
                      {files.length === 0
                        ? "No files found in this codebase."
                        : "Choose a file from the tree to open it in the editor."}
                    </EmptyDescription>
                  </EmptyHeader>
                  <EmptyContent>
                    {files.length > 0 && !showFileTree && (
                      <Button size="sm" variant="outline" onClick={() => setShowFileTree(true)}>
                        <PanelLeftOpen className="h-4 w-4 mr-1" aria-hidden="true" />
                        Show file tree
                      </Button>
                    )}
                  </EmptyContent>
                </Empty>
              )}
            </div>
          </div>
        )}
      </div>
    </TooltipProvider>
  );
}
