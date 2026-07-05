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
import { Input } from "@/components/ui/input";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
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
import { MoreVertical, Search } from "lucide-react";

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
  const [deleteTarget, setDeleteTarget] = useState<ContainerInfo | null>(null);
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
    try {
      await removeContainer(id);
      fetch();
    } catch (e) {
      console.error(e);
    }
  };

  const confirmDelete = async () => {
    if (!deleteTarget) return;
    await handleDelete(deleteTarget.id);
    setDeleteTarget(null);
  };

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
        <div className="relative max-w-xs">
          <Search className="absolute left-2.5 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground" />
          <Input
            type="text"
            placeholder="Search by name..."
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            className="pl-8"
          />
        </div>
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
                      <span className="font-mono text-sm text-foreground truncate max-w-48">
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
                        <DropdownMenu>
                          <DropdownMenuTrigger asChild>
                            <Button variant="ghost" size="icon" className="h-8 w-8">
                              <MoreVertical className="h-4 w-4" />
                            </Button>
                          </DropdownMenuTrigger>
                          <DropdownMenuContent align="end">
                            <DropdownMenuItem
                              className="text-destructive"
                              onClick={() => setDeleteTarget(c)}
                            >
                              Delete
                            </DropdownMenuItem>
                          </DropdownMenuContent>
                        </DropdownMenu>
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

      <AlertDialog open={!!deleteTarget} onOpenChange={(open) => !open && setDeleteTarget(null)}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Delete container?</AlertDialogTitle>
            <AlertDialogDescription>
              This will permanently delete &quot;
              {deleteTarget?.name || deleteTarget?.id.slice(0, 12)}&quot;. This action
              cannot be undone.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>Cancel</AlertDialogCancel>
            <AlertDialogAction onClick={confirmDelete}>Delete</AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  );
}
