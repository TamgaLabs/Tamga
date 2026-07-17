"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { BarChart3, Wrench, Globe, LayoutDashboard, LogOut, Network, Settings } from "lucide-react";

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
import { SealSelector } from "@/components/seal-selector";
import { useWorkspace, TAMGA_SYSTEM_ID } from "@/contexts/workspace-context";

type NavItem = { href: string; label: string; icon: typeof LayoutDashboard };

const globalNav: NavItem[] = [
  { href: "/dashboard", label: "Dashboard", icon: LayoutDashboard },
  { href: "/analytics", label: "Analytics", icon: BarChart3 },
  { href: "/infrastructure", label: "Topology", icon: Network },
];

const sealNav: NavItem[] = [
  { href: "/seals/$id/configure", label: "Configure", icon: Wrench },
];

const nonSealNav: NavItem[] = [
  { href: "/dashboard/non-project", label: "Dashboard", icon: Globe },
];

const systemNav: NavItem[] = [
  { href: "/dashboard/system", label: "Dashboard", icon: LayoutDashboard },
];

function isCurrentRoute(pathname: string, href: string) {
  return pathname === href || pathname.startsWith(`${href}/`);
}

export function AppSidebar() {
  const pathname = usePathname();
  const { logout } = useAuth();
  const { isMobile, setOpenMobile, state } = useSidebar();
  const { view, selectedSeal } = useWorkspace();
  const showLabels = isMobile || state === "expanded";
  const closeMobileNavigation = () => {
    if (isMobile) setOpenMobile(false);
  };

  const navItems: NavItem[] = (() => {
    if (view === "all") return globalNav;
    if (view === "non-seal") return nonSealNav;
    if (view === TAMGA_SYSTEM_ID) return systemNav;
    return sealNav;
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
          <span className="flex size-7 shrink-0 items-center justify-center overflow-hidden rounded-md bg-sidebar-primary">
            {/* eslint-disable-next-line @next/next/no-img-element */}
            <img src="/icon.svg" alt="" className="h-full w-full object-contain" />
          </span>
          {showLabels && <span className="font-display text-xl tracking-wide">Tamga Console</span>}
        </Link>
        {showLabels && (
          <div className="mt-2">
            <SealSelector />
          </div>
        )}
      </SidebarHeader>

      <SidebarContent>
        <SidebarGroup>
          {showLabels && <SidebarGroupLabel>{selectedSeal?.name ?? "Workspace"}</SidebarGroupLabel>}
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
