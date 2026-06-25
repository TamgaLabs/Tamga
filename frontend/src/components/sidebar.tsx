"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { useAuth } from "@/lib/auth";

export function Sidebar() {
  const pathname = usePathname();
  const { logout } = useAuth();

  const navItems = [
    { href: "/dashboard", label: "Projects", icon: "▦" },
    { href: "/containers", label: "Containers", icon: "▭" },
    { href: "/code", label: "Code", icon: "◇" },
    { href: "/settings", label: "Settings", icon: "⚙" },
  ];

  return (
    <aside className="fixed left-0 top-0 h-full w-56 bg-neutral-900 border-r border-neutral-800 flex flex-col z-50">
      <div className="p-4 border-b border-neutral-800">
        <Link href="/dashboard" className="text-lg font-bold text-white">
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
                  ? "bg-neutral-800 text-white"
                  : "text-neutral-400 hover:text-white hover:bg-neutral-800/50"
              }`}
            >
              <span className="w-5 text-center">{item.icon}</span>
              {item.label}
            </Link>
          );
        })}
      </nav>

      <div className="p-2 border-t border-neutral-800">
        <button
          onClick={() => { logout(); window.location.href = "/login"; }}
          className="flex items-center gap-3 px-3 py-2 rounded-md text-sm text-neutral-400 hover:text-white hover:bg-neutral-800/50 w-full transition-colors"
        >
          <span className="w-5 text-center">⎋</span>
          Logout
        </button>
      </div>
    </aside>
  );
}
