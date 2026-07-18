"use client";

import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { listSeals, type Seal } from "@/lib/api";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover";
import { Check, ChevronsUpDown, FolderPlus, Search } from "lucide-react";

export function ProjectSwitcher({ current }: { current: Seal }) {
  const [open, setOpen] = useState(false);
  const [seals, setSeals] = useState<Seal[]>([]);
  const [search, setSearch] = useState("");
  const router = useRouter();

  useEffect(() => {
    if (!open) return;
    listSeals().then(setSeals).catch(console.error);
  }, [open]);

  const filtered = seals.filter((seal) =>
    seal.name.toLowerCase().includes(search.toLowerCase())
  );

  const navigateTo = (id: number) => {
    setOpen(false);
    router.push(`/seals/${id}/configure`);
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
            placeholder="Search Seals..."
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            className="pl-7 h-8 text-sm"
            autoFocus
          />
        </div>
        <div role="listbox" aria-label="Seals" className="max-h-64 overflow-auto space-y-0.5">
          {filtered.length === 0 ? (
            <p className="px-2 py-1.5 text-sm text-muted-foreground">No Seals found.</p>
          ) : (
            filtered.map((seal) => (
              <button
                key={seal.id}
                role="option"
                aria-selected={seal.id === current.id}
                onClick={() => navigateTo(seal.id)}
                className={`flex w-full items-center justify-between rounded px-2 py-2 text-left text-sm transition-colors ${
                  seal.id === current.id
                    ? "bg-muted text-foreground"
                    : "text-muted-foreground hover:text-foreground hover:bg-muted"
                }`}
              >
                <span className="truncate">{seal.name}</span>
                {seal.id === current.id && <Check className="size-4 shrink-0" aria-hidden="true" />}
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
            <FolderPlus className="size-4" aria-hidden="true" /> New Seal
          </button>
        </div>
      </PopoverContent>
    </Popover>
  );
}
