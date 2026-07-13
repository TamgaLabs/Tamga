"use client";

import Link from "next/link";
import { Card, CardContent } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { MoreVertical, Play, RotateCw, Square } from "lucide-react";
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
  actionPending = false,
}: {
  container: ContainerInfo;
  onAction: (id: string, action: "start" | "stop" | "restart") => void | Promise<void>;
  onDelete?: (container: ContainerInfo) => void;
  actionPending?: boolean;
}) {
  const name = container.name || container.id.slice(0, 12);
  const ports = container.ports || [];

  return (
    <Card className="transition-colors hover:bg-muted/40">
      <CardContent className="flex flex-col gap-3 p-4 sm:flex-row sm:items-center sm:justify-between">
          <div className="flex min-w-0 items-center gap-3">
            <Link href={`/containers/${container.id}`} className="min-w-0 font-mono text-sm text-foreground underline-offset-4 hover:underline focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2">
              <span className="block truncate">{name}</span>
              <span className="mt-1 block text-xs text-muted-foreground sm:hidden">{container.image}</span>
            </Link>
            <Badge variant={statusVariant[container.state] || "default"}>
              {container.state}
            </Badge>
          </div>
          <div className="flex flex-wrap items-center gap-2 text-sm text-muted-foreground sm:justify-end">
            <span className="hidden md:inline truncate max-w-40">{container.image}</span>
            {ports.length > 0 && (
              <span className="hidden lg:inline text-xs font-mono text-muted-foreground">
                {ports.join(", ")}
              </span>
            )}
            <div className="flex gap-1 items-center" onClick={(e) => e.stopPropagation()}>
              {container.state === "running" && (
                <Button variant="outline" size="sm" disabled={actionPending} onClick={() => void onAction(container.id, "stop")}>
                  <Square className="size-3.5" aria-hidden="true" />{actionPending ? "Working..." : "Stop"}
                </Button>
              )}
              {container.state === "exited" && (
                <Button variant="outline" size="sm" disabled={actionPending} onClick={() => void onAction(container.id, "start")}>
                  <Play className="size-3.5" aria-hidden="true" />{actionPending ? "Working..." : "Start"}
                </Button>
              )}
              <Button variant="outline" size="sm" disabled={actionPending} onClick={() => void onAction(container.id, "restart")}>
                <RotateCw className="size-3.5" aria-hidden="true" />{actionPending ? "Working..." : "Restart"}
              </Button>
              {onDelete && (
                <DropdownMenu>
                  <DropdownMenuTrigger asChild>
                    <Button variant="ghost" size="icon" className="h-8 w-8" disabled={actionPending} aria-label="Container actions">
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
      </CardContent>
    </Card>
  );
}
