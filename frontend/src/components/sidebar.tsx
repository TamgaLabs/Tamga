"use client";

"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { useAuth } from "@/lib/auth";
import { useTheme } from "@/lib/theme";

export function Sidebar() {
  const pathname = usePathname();
  const { logout } = useAuth();
  const { theme, toggleTheme } = useTheme();

  const navItems = [
    { href: "/dashboard", label: "Projects", icon: "▦" },
    { href: "/containers", label: "Containers", icon: "▭" },
    { href: "/code", label: "Code", icon: "◇" },
    { href: "/settings", label: "Settings", icon: "⚙" },
  ];

  return (
    <aside className="fixed left-0 top-0 h-full w-56 bg-card border-r border-border flex flex-col z-50">
      <div className="p-4 border-b border-border">
        <Link href="/dashboard" className="text-lg font-bold text-foreground">
          Tamga
        </Link>
      </div>

      <nav className="flex-1 p-2 space-y-1">
        {navItems.map((item) => {
          const active = pathname.startsWith(item.href);
          return (
            <Link
              key={item.href}
              href={item.href}
              className={`flex items-center gap-3 px-3 py-2 rounded-md text-sm transition-colors ${
                active
                  ? "bg-muted text-foreground"
                  : "text-muted-foreground hover:text-foreground hover:bg-muted"
              }`}
            >
              <span className="w-5 text-center">{item.icon}</span>
              {item.label}
            </Link>
          );
        })}
      </nav>

      <div className="p-2 border-t border-border space-y-1">
        <button
          onClick={toggleTheme}
          className="flex items-center gap-3 px-3 py-2 rounded-md text-sm text-muted-foreground hover:text-foreground hover:bg-muted w-full transition-colors"
        >
          <span className="w-5 text-center">{theme === "dark" ? "☀️" : "🌙"}</span>
          {theme === "dark" ? "Light Mode" : "Dark Mode"}
        </button>
        <button
          onClick={() => { logout(); window.location.href = "/login"; }}
          className="flex items-center gap-3 px-3 py-2 rounded-md text-sm text-muted-foreground hover:text-foreground hover:bg-muted w-full transition-colors"
        >
          <span className="w-5 text-center">⎋</span>
          Logout
        </button>
      </div>
    </aside>
  );
}
