"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import {
  BarChart3,
  Code2,
  Container,
  LayoutDashboard,
  LogOut,
  Network,
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

const navigation = [
  { href: "/dashboard", label: "Projects", icon: LayoutDashboard },
  { href: "/containers", label: "Containers", icon: Container },
  { href: "/code", label: "Code", icon: Code2 },
  { href: "/analytics", label: "Analytics", icon: BarChart3 },
  { href: "/infrastructure", label: "Infrastructure", icon: Network },
  { href: "/settings", label: "Settings", icon: Settings },
];

function isCurrentRoute(pathname: string, href: string) {
  return pathname === href || pathname.startsWith(`${href}/`);
}

export function AppSidebar() {
  const pathname = usePathname();
  const { logout } = useAuth();
  const { isMobile, setOpenMobile, state } = useSidebar();
  const showLabels = isMobile || state === "expanded";
  const closeMobileNavigation = () => {
    if (isMobile) setOpenMobile(false);
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
          {showLabels && <span className="font-display text-sm tracking-wide">Tamga Console</span>}
        </Link>
      </SidebarHeader>

      <SidebarContent>
        <SidebarGroup>
          {showLabels && <SidebarGroupLabel>Workspace</SidebarGroupLabel>}
          <SidebarMenu>
            {navigation.map(({ href, label, icon: Icon }) => {
              const active = isCurrentRoute(pathname, href);
              return (
                <SidebarMenuItem key={href}>
                  <SidebarMenuButton asChild isActive={active} tooltip={label}>
                    <Link
                      href={href}
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
            <SidebarMenuButton
              type="button"
              tooltip="Logout"
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
