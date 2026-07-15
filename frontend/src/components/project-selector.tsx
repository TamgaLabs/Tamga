"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { listProjects, type Project } from "@/lib/api";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover";
import { Check, ChevronsUpDown, FolderPlus, Search, Globe, Box, LayoutGrid, Server } from "lucide-react";
import { useWorkspace, type WorkspaceView, TAMGA_SYSTEM_ID } from "@/contexts/workspace-context";

export function ProjectSelector() {
  const { view, setView, projects } = useWorkspace();
  const [open, setOpen] = useState(false);
  const [search, setSearch] = useState("");
  const router = useRouter();

  const filtered = projects.filter((p) =>
    p.name.toLowerCase().includes(search.toLowerCase())
  );
  const systemProject = filtered.find((p) => p.id === TAMGA_SYSTEM_ID);
  const realProjects = filtered.filter((p) => p.id !== TAMGA_SYSTEM_ID);

  const currentLabel = (() => {
    if (view === "all") return "All";
    if (view === "non-project") return "Non-project";
    const p = projects.find((proj) => proj.id === view);
    return p?.name ?? "Select project";
  })();

  const handleSelect = (v: WorkspaceView) => {
    setView(v);
    setOpen(false);
    setSearch("");
  };

  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger asChild>
        <Button variant="outline" className="w-full justify-between">
          <span className="truncate">{currentLabel}</span>
          <ChevronsUpDown className="h-4 w-4 shrink-0 opacity-50" />
        </Button>
      </PopoverTrigger>
      <PopoverContent className="w-[min(20rem,calc(100vw-2rem))] p-2" align="start">
        <div className="relative mb-2">
          <Search className="absolute left-2 top-1/2 -translate-y-1/2 h-3.5 w-3.5 text-muted-foreground" />
          <Input
            placeholder="Search projects..."
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            className="pl-7 h-8 text-sm"
            autoFocus
          />
        </div>
        <div role="listbox" aria-label="Projects" className="max-h-64 overflow-auto space-y-0.5">
          <button
            role="option"
            aria-selected={view === "all"}
            onClick={() => handleSelect("all")}
            className={`flex w-full items-center gap-2 rounded px-2 py-2 text-left text-sm transition-colors ${
              view === "all"
                ? "bg-muted text-foreground"
                : "text-muted-foreground hover:text-foreground hover:bg-muted"
            }`}
          >
            <LayoutGrid className="size-4 shrink-0" aria-hidden="true" />
            <span className="truncate">All</span>
            {view === "all" && <Check className="size-4 shrink-0 ml-auto" aria-hidden="true" />}
          </button>

          {systemProject && (
            <button
              role="option"
              aria-selected={view === TAMGA_SYSTEM_ID}
              onClick={() => handleSelect(TAMGA_SYSTEM_ID)}
              className={`flex w-full items-center gap-2 rounded px-2 py-2 text-left text-sm transition-colors ${
                view === TAMGA_SYSTEM_ID
                  ? "bg-muted text-foreground"
                  : "text-muted-foreground hover:text-foreground hover:bg-muted"
              }`}
            >
              <Server className="size-4 shrink-0" aria-hidden="true" />
              <span className="truncate">Tamga System</span>
              {view === TAMGA_SYSTEM_ID && <Check className="size-4 shrink-0 ml-auto" aria-hidden="true" />}
            </button>
          )}

          {realProjects.length > 0 && (
            <div className="pt-1">
              {realProjects.map((p) => (
                <button
                  key={p.id}
                  role="option"
                  aria-selected={view === p.id}
                  onClick={() => handleSelect(p.id)}
                  className={`flex w-full items-center gap-2 rounded px-2 py-2 text-left text-sm transition-colors ${
                    view === p.id
                      ? "bg-muted text-foreground"
                      : "text-muted-foreground hover:text-foreground hover:bg-muted"
                  }`}
                >
                  <Box className="size-4 shrink-0" aria-hidden="true" />
                  <span className="truncate">{p.name}</span>
                  {view === p.id && <Check className="size-4 shrink-0 ml-auto" aria-hidden="true" />}
                </button>
              ))}
            </div>
          )}

          <button
            role="option"
            aria-selected={view === "non-project"}
            onClick={() => handleSelect("non-project")}
            className={`flex w-full items-center gap-2 rounded px-2 py-2 text-left text-sm transition-colors ${
              view === "non-project"
                ? "bg-muted text-foreground"
                : "text-muted-foreground hover:text-foreground hover:bg-muted"
            }`}
          >
            <Globe className="size-4 shrink-0" aria-hidden="true" />
            <span className="truncate">Non-project</span>
            {view === "non-project" && <Check className="size-4 shrink-0 ml-auto" aria-hidden="true" />}
          </button>
        </div>
        <div className="border-t border-border mt-2 pt-2">
          <button
            onClick={() => {
              setOpen(false);
              router.push("/dashboard/new");
            }}
            className="flex w-full items-center gap-2 rounded px-2 py-2 text-left text-sm font-medium text-primary transition-colors hover:bg-muted"
          >
            <FolderPlus className="size-4" aria-hidden="true" /> New project
          </button>
        </div>
      </PopoverContent>
    </Popover>
  );
}
