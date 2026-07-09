"use client";

import { useEffect, useState } from "react";
import { useParams, useRouter } from "next/navigation";
import {
  getFileTree,
  readFile,
  writeFile,
  type FileEntry,
} from "@/lib/api";
import { AgentTerminal } from "@/components/agent-terminal";
import { useAuth } from "@/lib/auth";
import { useTheme } from "@/lib/theme";
import { Button } from "@/components/ui/button";
import {
  SquareTerminal,
  Code2,
  PanelLeftOpen,
  PanelLeftClose,
  FileIcon,
  FolderIcon,
  FolderOpenIcon,
} from "lucide-react";
import dynamic from "next/dynamic";

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
  const [showFileTree, setShowFileTree] = useState(false);

  useEffect(() => {
    if (!authLoading && !user) router.replace("/login");
  }, [user, authLoading, router]);

  useEffect(() => {
    if (!user || mode !== "code") return;
    getFileTree(projectId).then(setFiles).catch(console.error);
  }, [projectId, user, mode]);

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
        <div className="flex-1 min-h-0">
          <AgentTerminal projectId={projectId} />
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
