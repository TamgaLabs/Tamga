"use client";

import { usePathname } from "next/navigation";
import { Command } from "lucide-react";

import { AppSidebar } from "@/components/sidebar";
import {
  Breadcrumb,
  BreadcrumbItem,
  BreadcrumbList,
  BreadcrumbPage,
} from "@/components/ui/breadcrumb";
import {
  SidebarInset,
  SidebarProvider,
  SidebarTrigger,
  useSidebar,
} from "@/components/ui/sidebar";
import { cn } from "@/lib/utils";
import { WorkspaceProvider, useWorkspace } from "@/contexts/workspace-context";

const routeLabels: Record<string, string> = {
  analytics: "Analytics",
  appearance: "Appearance",
  code: "Code",
  containers: "Containers",
  dashboard: "Dashboard",
  environment: "Environment",
  infrastructure: "Topology",
  logs: "Logs",
  map: "Topology",
  network: "Network",
  new: "New Seal",
  seals: "Seals",
  resources: "Resources",
  sandbox: "Sandbox",
  settings: "Settings",
  stats: "Stats",
  system: "System",
  git: "Git",
};

function pageContext(pathname: string) {
  if (pathname === "/dashboard/non-project") return "Non-Seal";
  const segments = pathname.split("/").filter(Boolean);
  const last = segments.at(-1);
  if (!last) return "Dashboard";
  return routeLabels[last] ?? (Number.isNaN(Number(last)) ? last.replace(/-/g, " ") : "Details");
}

function ShellContent({ children }: { children: React.ReactNode }) {
  const pathname = usePathname();
  const { state } = useSidebar();
  const { selectedSeal } = useWorkspace();
  const context = pageContext(pathname);

  return (
    <SidebarInset
      className={cn(
        "min-h-svh transition-[margin] duration-200 md:ml-64",
        state === "collapsed" && "md:ml-14"
      )}
    >
      <header className="sticky top-0 z-30 flex h-14 shrink-0 items-center gap-3 border-b border-border bg-background/95 px-4 backdrop-blur supports-[backdrop-filter]:bg-background/80 sm:px-6">
        <SidebarTrigger />
        <div className="h-5 w-px bg-border" aria-hidden="true" />
        <Breadcrumb className="min-w-0">
          <BreadcrumbList className="flex-nowrap overflow-hidden">
            {selectedSeal && (
              <>
                <BreadcrumbItem className="shrink-0">
                  <BreadcrumbPage className="text-xs text-muted-foreground">{selectedSeal.name}</BreadcrumbPage>
                </BreadcrumbItem>
                <BreadcrumbItem className="shrink-0 text-muted-foreground">/</BreadcrumbItem>
              </>
            )}
            <BreadcrumbItem className="min-w-0">
              <BreadcrumbPage className="block truncate capitalize">{context}</BreadcrumbPage>
            </BreadcrumbItem>
          </BreadcrumbList>
        </Breadcrumb>
        <Command className="ml-auto hidden size-4 text-muted-foreground sm:block" aria-label="Console workspace" />
      </header>
      <div className="min-w-0 flex-1">{children}</div>
    </SidebarInset>
  );
}

export function ConsoleShell({ children }: { children: React.ReactNode }) {
  return (
    <WorkspaceProvider>
      <SidebarProvider>
        <AppSidebar />
        <ShellContent>{children}</ShellContent>
      </SidebarProvider>
    </WorkspaceProvider>
  );
}
