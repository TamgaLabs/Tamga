"use client";

import { useEffect, useState, useCallback } from "react";
import { useRouter } from "next/navigation";
import {
  listContainers,
  startContainer,
  stopContainer,
  restartContainer,
  removeContainer,
  type ContainerInfo,
} from "@/lib/api";
import { useAuth } from "@/lib/auth";
import { getShowSystem } from "@/lib/settings";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";

const statusVariant: Record<string, "success" | "warning" | "error" | "info" | "default"> = {
  running: "success",
  paused: "warning",
  exited: "error",
  created: "info",
};

export default function ContainersPage() {
  const [containers, setContainers] = useState<ContainerInfo[]>([]);
  const [loading, setLoading] = useState(true);
  const [search, setSearch] = useState("");
  const [menuOpen, setMenuOpen] = useState<string | null>(null);
  const { user, loading: authLoading } = useAuth();
  const router = useRouter();

  useEffect(() => {
    if (!authLoading && !user) router.replace("/login");
  }, [user, authLoading, router]);

  const fetch = useCallback(() => {
    if (!user) return;
    setLoading(true);
    listContainers()
      .then(setContainers)
      .catch(console.error)
      .finally(() => setLoading(false));
  }, [user]);

  useEffect(fetch, [fetch]);

  const handleAction = async (id: string, action: "start" | "stop" | "restart") => {
    try {
      if (action === "start") await startContainer(id);
      else if (action === "stop") await stopContainer(id);
      else await restartContainer(id);
      fetch();
    } catch (e) {
      console.error(e);
    }
  };

  const handleDelete = async (id: string) => {
    if (!confirm("Delete this container? This action cannot be undone.")) return;
    try {
      await removeContainer(id);
      setMenuOpen(null);
      fetch();
    } catch (e) {
      console.error(e);
    }
  };

  useEffect(() => {
    const close = () => setMenuOpen(null);
    if (menuOpen) {
      document.addEventListener("click", close);
      return () => document.removeEventListener("click", close);
    }
  }, [menuOpen]);

  const showSystem = getShowSystem();

  const filtered = (containers || []).filter((c) => {
    const name = c.name || "";
    const isSystem = !!c.system_type;
    if (!showSystem && isSystem) return false;
    if (search && !name.toLowerCase().includes(search.toLowerCase())) return false;
    return true;
  });

  if (authLoading || !user) return null;

  return (
    <div className="p-6 max-w-6xl mx-auto">
      <div className="flex items-center justify-between mb-6">
        <h1 className="text-2xl font-bold">Containers</h1>
        <input
          type="text"
          placeholder="Search by name..."
          value={search}
          onChange={(e) => setSearch(e.target.value)}
          className="bg-background border border-border rounded px-3 py-1.5 text-sm text-foreground max-w-xs"
        />
      </div>

      {loading ? (
        <p className="text-muted-foreground">Loading...</p>
      ) : filtered.length === 0 ? (
        <p className="text-muted-foreground">No containers found.</p>
      ) : (
        <div className="space-y-2">
          {filtered.map((c) => {
            const name = c.name || c.id.slice(0, 12);
            const ports = c.ports || [];
            return (
              <Card
                key={c.id}
                className="cursor-pointer hover:bg-muted/50 transition-colors"
                onClick={() => router.push(`/containers/${c.id}`)}
              >
                <CardContent className="p-4">
                  <div className="flex items-center justify-between">
                    <div className="flex items-center gap-3 min-w-0">
                      <span className="font-mono text-sm text-accent truncate max-w-48">
                        {name}
                      </span>
                      <Badge variant={statusVariant[c.state] || "default"}>{c.state}</Badge>
                    </div>
                    <div className="flex items-center gap-4 text-sm text-muted-foreground">
                      <span className="hidden md:inline truncate max-w-40">{c.image}</span>
                      {ports.length > 0 && (
                        <span className="hidden lg:inline text-xs font-mono text-muted-foreground">
                          {ports.join(", ")}
                        </span>
                      )}
                      <div className="flex gap-1 items-center" onClick={(e) => e.stopPropagation()}>
                        {c.state === "running" && (
                          <Button variant="outline" size="sm" onClick={() => handleAction(c.id, "stop")}>
                            Stop
                          </Button>
                        )}
                        {c.state === "exited" && (
                          <Button variant="outline" size="sm" onClick={() => handleAction(c.id, "start")}>
                            Start
                          </Button>
                        )}
                        <Button variant="outline" size="sm" onClick={() => handleAction(c.id, "restart")}>
                          Restart
                        </Button>
                        <div className="relative">
                          <button
                            className="text-muted-foreground hover:text-foreground px-1 text-lg leading-none"
                            onClick={() => setMenuOpen(menuOpen === c.id ? null : c.id)}
                          >
                            ⋮
                          </button>
                          {menuOpen === c.id && (
                            <div className="absolute right-0 top-full mt-1 bg-card border border-border rounded shadow-lg z-10 min-w-28">
                              <button
                                className="w-full text-left px-3 py-1.5 text-sm text-destructive hover:bg-muted"
                                onClick={() => handleDelete(c.id)}
                              >
                                Delete
                              </button>
                            </div>
                          )}
                        </div>
                      </div>
                    </div>
                  </div>
                  {c.system_type && (
                    <p className="text-xs text-muted-foreground mt-1">system: {c.system_type}</p>
                  )}
                  {c.project_id && (
                    <p className="text-xs text-muted-foreground mt-1">project: {c.project_id}</p>
                  )}
                </CardContent>
              </Card>
            );
          })}
        </div>
      )}
    </div>
  );
}
