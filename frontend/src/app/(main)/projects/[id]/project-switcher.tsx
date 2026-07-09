"use client";

import { useEffect, useState } from "react";
import { useRouter, usePathname } from "next/navigation";
import { listProjects, type Project } from "@/lib/api";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover";
import { ChevronsUpDown, Search } from "lucide-react";

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
      <PopoverContent className="w-64 p-2" align="start">
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
        <div className="max-h-64 overflow-auto space-y-0.5">
          {filtered.length === 0 ? (
            <p className="px-2 py-1.5 text-sm text-muted-foreground">No projects found.</p>
          ) : (
            filtered.map((p) => (
              <button
                key={p.id}
                onClick={() => navigateTo(p.id)}
                className={`w-full text-left px-2 py-1.5 rounded text-sm transition-colors ${
                  p.id === current.id
                    ? "bg-muted text-foreground"
                    : "text-muted-foreground hover:text-foreground hover:bg-muted"
                }`}
              >
                {p.name}
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
            className="w-full text-left px-2 py-1.5 rounded text-sm text-accent hover:bg-muted transition-colors"
          >
            + New Project
          </button>
        </div>
      </PopoverContent>
    </Popover>
  );
}
