"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { Command } from "lucide-react";

import { AppSidebar } from "@/components/sidebar";
import {
  Breadcrumb,
  BreadcrumbItem,
  BreadcrumbLink,
  BreadcrumbList,
  BreadcrumbPage,
  BreadcrumbSeparator,
} from "@/components/ui/breadcrumb";
import {
  SidebarInset,
  SidebarProvider,
  SidebarTrigger,
  useSidebar,
} from "@/components/ui/sidebar";
import { cn } from "@/lib/utils";

const routeLabels: Record<string, string> = {
  analytics: "Analytics",
  appearance: "Appearance",
  code: "Code",
  containers: "Containers",
  dashboard: "Projects",
  environment: "Environment",
  infrastructure: "Infrastructure",
  logs: "Logs",
  map: "Map",
  network: "Network",
  new: "New project",
  projects: "Projects",
  resources: "Resources",
  sandbox: "Sandbox",
  settings: "Settings",
  stats: "Stats",
  system: "System",
  git: "Git",
  actions: "Actions",
};

function pageContext(pathname: string) {
  const segments = pathname.split("/").filter(Boolean);
  const last = segments.at(-1);
  if (!last) return "Projects";
  return routeLabels[last] ?? (Number.isNaN(Number(last)) ? last.replace(/-/g, " ") : "Details");
}

function ShellContent({ children }: { children: React.ReactNode }) {
  const pathname = usePathname();
  const { state } = useSidebar();
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
            <BreadcrumbItem className="shrink-0">
              <BreadcrumbLink asChild>
                <Link href="/dashboard" className="font-display text-xs tracking-wide text-foreground">
                  Tamga Console
                </Link>
              </BreadcrumbLink>
            </BreadcrumbItem>
            <BreadcrumbSeparator className="shrink-0" />
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
    <SidebarProvider>
      <AppSidebar />
      <ShellContent>{children}</ShellContent>
    </SidebarProvider>
  );
}
