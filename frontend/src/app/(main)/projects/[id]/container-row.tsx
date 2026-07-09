"use client";

import { useRouter } from "next/navigation";
import { Card, CardContent } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { MoreVertical } from "lucide-react";
import type { ContainerInfo } from "@/lib/api";

const statusVariant: Record<string, "success" | "warning" | "error" | "info" | "default"> = {
  running: "success",
  paused: "warning",
  exited: "error",
  created: "info",
};

// Shared row treatment for a container, reused by the project overview's
// containers summary and the project's full containers sub-page (both
// filter the same listContainers() response by project_id) - mirrors the
// row rendered on the main containers page.
export function ContainerRow({
  container,
  onAction,
  onDelete,
}: {
  container: ContainerInfo;
  onAction: (id: string, action: "start" | "stop" | "restart") => void;
  onDelete?: (container: ContainerInfo) => void;
}) {
  const router = useRouter();
  const name = container.name || container.id.slice(0, 12);
  const ports = container.ports || [];

  return (
    <Card
      className="cursor-pointer hover:bg-muted/50 transition-colors"
      onClick={() => router.push(`/containers/${container.id}`)}
    >
      <CardContent className="p-4">
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-3 min-w-0">
            <span className="font-mono text-sm text-foreground truncate max-w-48">
              {name}
            </span>
            <Badge variant={statusVariant[container.state] || "default"}>
              {container.state}
            </Badge>
          </div>
          <div className="flex items-center gap-4 text-sm text-muted-foreground">
            <span className="hidden md:inline truncate max-w-40">{container.image}</span>
            {ports.length > 0 && (
              <span className="hidden lg:inline text-xs font-mono text-muted-foreground">
                {ports.join(", ")}
              </span>
            )}
            <div className="flex gap-1 items-center" onClick={(e) => e.stopPropagation()}>
              {container.state === "running" && (
                <Button variant="outline" size="sm" onClick={() => onAction(container.id, "stop")}>
                  Stop
                </Button>
              )}
              {container.state === "exited" && (
                <Button variant="outline" size="sm" onClick={() => onAction(container.id, "start")}>
                  Start
                </Button>
              )}
              <Button variant="outline" size="sm" onClick={() => onAction(container.id, "restart")}>
                Restart
              </Button>
              {onDelete && (
                <DropdownMenu>
                  <DropdownMenuTrigger asChild>
                    <Button variant="ghost" size="icon" className="h-8 w-8">
                      <MoreVertical className="h-4 w-4" />
                    </Button>
                  </DropdownMenuTrigger>
                  <DropdownMenuContent align="end">
                    <DropdownMenuItem
                      className="text-destructive"
                      onClick={() => onDelete(container)}
                    >
                      Delete
                    </DropdownMenuItem>
                  </DropdownMenuContent>
                </DropdownMenu>
              )}
            </div>
          </div>
        </div>
      </CardContent>
    </Card>
  );
}
