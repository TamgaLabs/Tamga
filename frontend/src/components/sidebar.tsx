"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import {
  BarChart3,
  Code2,
  Container,
  Globe,
  LayoutDashboard,
  LogOut,
  Network,
  Server,
  Settings,
} from "lucide-react";

import { useAuth } from "@/lib/auth";
import {
  Sidebar,
  SidebarContent,
  SidebarFooter,
  SidebarGroup,
  SidebarGroupLabel,
  SidebarHeader,
  SidebarMenu,
  SidebarMenuButton,
  SidebarMenuItem,
  SidebarRail,
  useSidebar,
} from "@/components/ui/sidebar";
import { ProjectSelector } from "@/components/project-selector";
import { useWorkspace } from "@/contexts/workspace-context";

type NavItem = { href: string; label: string; icon: typeof LayoutDashboard };

const globalNav: NavItem[] = [
  { href: "/dashboard", label: "Dashboard", icon: LayoutDashboard },
  { href: "/containers", label: "Containers", icon: Container },
  { href: "/analytics", label: "Analytics", icon: BarChart3 },
  { href: "/infrastructure", label: "Topology", icon: Network },
];

const projectNav: NavItem[] = [
  { href: "/projects/$id", label: "Dashboard", icon: LayoutDashboard },
  { href: "/projects/$id/containers", label: "Containers", icon: Container },
  { href: "/projects/$id/environment", label: "Environment", icon: Server },
  { href: "/projects/$id/analytics", label: "Analytics", icon: BarChart3 },
  { href: "/projects/$id/map", label: "Topology", icon: Network },
  { href: "/projects/$id/code", label: "Code", icon: Code2 },
];

const nonProjectNav: NavItem[] = [
  { href: "/dashboard/non-project", label: "Dashboard", icon: Globe },
];

function isCurrentRoute(pathname: string, href: string) {
  return pathname === href || pathname.startsWith(`${href}/`);
}

export function AppSidebar() {
  const pathname = usePathname();
  const { logout } = useAuth();
  const { isMobile, setOpenMobile, state } = useSidebar();
  const { view, selectedProject } = useWorkspace();
  const showLabels = isMobile || state === "expanded";
  const closeMobileNavigation = () => {
    if (isMobile) setOpenMobile(false);
  };

  const navItems: NavItem[] = (() => {
    if (view === "all") return globalNav;
    if (view === "non-project") return nonProjectNav;
    return projectNav;
  })();

  const resolveHref = (href: string) => {
    if (typeof view === "number" && href.includes("$id")) {
      return href.replace("$id", String(view));
    }
    return href;
  };

  return (
    <Sidebar collapsible="icon">
      <SidebarHeader className="border-b border-sidebar-border p-3">
        <Link
          href="/dashboard"
          className="flex min-h-8 items-center gap-2 rounded-md px-1 text-sidebar-foreground outline-none focus-visible:ring-2 focus-visible:ring-sidebar-ring"
          aria-label="Tamga Console home"
          onClick={closeMobileNavigation}
        >
          <span className="flex size-7 shrink-0 items-center justify-center rounded-md bg-sidebar-primary font-display text-xs text-sidebar-primary-foreground">
            T
          </span>
          {showLabels && <span className="font-display text-lg tracking-wide">Tamga Console</span>}
        </Link>
        {showLabels && (
          <div className="mt-2">
            <ProjectSelector />
          </div>
        )}
      </SidebarHeader>

      <SidebarContent>
        <SidebarGroup>
          {showLabels && <SidebarGroupLabel>{selectedProject?.name ?? "Workspace"}</SidebarGroupLabel>}
          <SidebarMenu>
            {navItems.map(({ href, label, icon: Icon }) => {
              const resolvedHref = resolveHref(href);
              const active = isCurrentRoute(pathname, resolvedHref);
              return (
                <SidebarMenuItem key={href}>
                  <SidebarMenuButton asChild isActive={active}>
                    <Link
                      href={resolvedHref}
                      aria-current={active ? "page" : undefined}
                      onClick={closeMobileNavigation}
                    >
                      <Icon aria-hidden="true" />
                      {showLabels && <span>{label}</span>}
                    </Link>
                  </SidebarMenuButton>
                </SidebarMenuItem>
              );
            })}
          </SidebarMenu>
        </SidebarGroup>
      </SidebarContent>

      <SidebarFooter className="border-t border-sidebar-border">
        <SidebarMenu>
          <SidebarMenuItem>
            <SidebarMenuButton asChild isActive={isCurrentRoute(pathname, "/settings")}>
              <Link
                href="/settings"
                aria-current={isCurrentRoute(pathname, "/settings") ? "page" : undefined}
                onClick={closeMobileNavigation}
              >
                <Settings aria-hidden="true" />
                {showLabels && <span>Settings</span>}
              </Link>
            </SidebarMenuButton>
          </SidebarMenuItem>
          <SidebarMenuItem>
            <SidebarMenuButton
              type="button"
              onClick={() => {
                logout();
                window.location.assign("/login");
              }}
            >
              <LogOut aria-hidden="true" />
              {showLabels && <span>Logout</span>}
            </SidebarMenuButton>
          </SidebarMenuItem>
        </SidebarMenu>
      </SidebarFooter>
      <SidebarRail />
    </Sidebar>
  );
}
