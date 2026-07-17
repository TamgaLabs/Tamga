"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover";
import { Check, ChevronsUpDown, FolderPlus, Search, Globe, Box, LayoutGrid, Server } from "lucide-react";
import { useWorkspace, type WorkspaceView, TAMGA_SYSTEM_ID } from "@/contexts/workspace-context";

export function SealSelector() {
  const { view, setView, seals } = useWorkspace();
  const [open, setOpen] = useState(false);
  const [search, setSearch] = useState("");
  const router = useRouter();

  const filtered = seals.filter((seal) =>
    seal.name.toLowerCase().includes(search.toLowerCase())
  );
  const systemSeal = filtered.find((seal) => seal.id === TAMGA_SYSTEM_ID);
  const realSeals = filtered.filter((seal) => seal.id !== TAMGA_SYSTEM_ID);

  const currentLabel = (() => {
    if (view === "all") return "All";
    if (view === "non-seal") return "Non-Seal";
    const seal = seals.find((candidate) => candidate.id === view);
    return seal?.name ?? "Select Seal";
  })();

  const handleSelect = (nextView: WorkspaceView) => {
    setView(nextView);
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
          <Search className="absolute left-2 top-1/2 h-3.5 w-3.5 -translate-y-1/2 text-muted-foreground" />
          <Input
            placeholder="Search Seals..."
            value={search}
            onChange={(event) => setSearch(event.target.value)}
            className="h-8 pl-7 text-sm"
            autoFocus
          />
        </div>
        <div role="listbox" aria-label="Seals" className="max-h-64 space-y-0.5 overflow-auto">
          <button
            role="option"
            aria-selected={view === "all"}
            onClick={() => handleSelect("all")}
            className={`flex w-full items-center gap-2 rounded px-2 py-2 text-left text-sm transition-colors ${
              view === "all"
                ? "bg-muted text-foreground"
                : "text-muted-foreground hover:bg-muted hover:text-foreground"
            }`}
          >
            <LayoutGrid className="size-4 shrink-0" aria-hidden="true" />
            <span className="truncate">All</span>
            {view === "all" && <Check className="ml-auto size-4 shrink-0" aria-hidden="true" />}
          </button>

          {systemSeal && (
            <button
              role="option"
              aria-selected={view === TAMGA_SYSTEM_ID}
              onClick={() => handleSelect(TAMGA_SYSTEM_ID)}
              className={`flex w-full items-center gap-2 rounded px-2 py-2 text-left text-sm transition-colors ${
                view === TAMGA_SYSTEM_ID
                  ? "bg-muted text-foreground"
                  : "text-muted-foreground hover:bg-muted hover:text-foreground"
              }`}
            >
              <Server className="size-4 shrink-0" aria-hidden="true" />
              <span className="truncate">Tamga System</span>
              {view === TAMGA_SYSTEM_ID && <Check className="ml-auto size-4 shrink-0" aria-hidden="true" />}
            </button>
          )}

          {realSeals.length > 0 && (
            <div className="pt-1">
              {realSeals.map((seal) => (
                <button
                  key={seal.id}
                  role="option"
                  aria-selected={view === seal.id}
                  onClick={() => handleSelect(seal.id)}
                  className={`flex w-full items-center gap-2 rounded px-2 py-2 text-left text-sm transition-colors ${
                    view === seal.id
                      ? "bg-muted text-foreground"
                      : "text-muted-foreground hover:bg-muted hover:text-foreground"
                  }`}
                >
                  <Box className="size-4 shrink-0" aria-hidden="true" />
                  <span className="truncate">{seal.name}</span>
                  {view === seal.id && <Check className="ml-auto size-4 shrink-0" aria-hidden="true" />}
                </button>
              ))}
            </div>
          )}

          <button
            role="option"
            aria-selected={view === "non-seal"}
            onClick={() => handleSelect("non-seal")}
            className={`flex w-full items-center gap-2 rounded px-2 py-2 text-left text-sm transition-colors ${
              view === "non-seal"
                ? "bg-muted text-foreground"
                : "text-muted-foreground hover:bg-muted hover:text-foreground"
            }`}
          >
            <Globe className="size-4 shrink-0" aria-hidden="true" />
            <span className="truncate">Non-Seal</span>
            {view === "non-seal" && <Check className="ml-auto size-4 shrink-0" aria-hidden="true" />}
          </button>
        </div>
        <div className="mt-2 border-t border-border pt-2">
          <button
            onClick={() => {
              setOpen(false);
              router.push("/dashboard/new");
            }}
            className="flex w-full items-center gap-2 rounded px-2 py-2 text-left text-sm font-medium text-primary transition-colors hover:bg-muted"
          >
            <FolderPlus className="size-4" aria-hidden="true" /> New Seal
          </button>
        </div>
      </PopoverContent>
    </Popover>
  );
}
