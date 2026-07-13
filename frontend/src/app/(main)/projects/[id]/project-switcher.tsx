"use client";

import { useEffect, useState } from "react";
import { useRouter, usePathname } from "next/navigation";
import { listProjects, type Project } from "@/lib/api";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover";
import { Check, ChevronsUpDown, FolderPlus, Search } from "lucide-react";

export function ProjectSwitcher({ current }: { current: Project }) {
  const [open, setOpen] = useState(false);
  const [projects, setProjects] = useState<Project[]>([]);
  const [search, setSearch] = useState("");
  const router = useRouter();
  const pathname = usePathname();

  useEffect(() => {
    if (!open) return;
    listProjects().then(setProjects).catch(console.error);
  }, [open]);

  const filtered = projects.filter((p) =>
    p.name.toLowerCase().includes(search.toLowerCase())
  );

  // Preserve the current sub-section: /projects/3/settings -> /projects/5/settings
  const navigateTo = (id: number) => {
    const rest = pathname.replace(/^\/projects\/\d+/, "");
    setOpen(false);
    router.push(`/projects/${id}${rest}`);
  };

  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger asChild>
        <Button variant="outline" className="w-full justify-between">
          <span className="truncate">{current.name}</span>
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
          {filtered.length === 0 ? (
            <p className="px-2 py-1.5 text-sm text-muted-foreground">No projects found.</p>
          ) : (
            filtered.map((p) => (
              <button
                key={p.id}
                role="option"
                aria-selected={p.id === current.id}
                onClick={() => navigateTo(p.id)}
                className={`flex w-full items-center justify-between rounded px-2 py-2 text-left text-sm transition-colors ${
                  p.id === current.id
                    ? "bg-muted text-foreground"
                    : "text-muted-foreground hover:text-foreground hover:bg-muted"
                }`}
              >
                <span className="truncate">{p.name}</span>
                {p.id === current.id && <Check className="size-4 shrink-0" aria-hidden="true" />}
              </button>
            ))
          )}
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
