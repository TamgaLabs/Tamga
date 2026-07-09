"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";

const sections = [
  { href: "/settings/appearance", label: "Appearance" },
  { href: "/settings/sandbox", label: "Sandbox" },
  { href: "/settings/network", label: "Network" },
  { href: "/settings/git", label: "Git" },
  { href: "/settings/system", label: "System" },
];

export default function SettingsLayout({ children }: { children: React.ReactNode }) {
  const pathname = usePathname();

  return (
    <div className="flex min-h-screen">
      <aside className="w-48 shrink-0 border-r border-border p-4">
        <h1 className="text-lg font-bold px-2 mb-4">Settings</h1>
        <nav className="space-y-1">
          {sections.map((s) => {
            const active = pathname.startsWith(s.href);
            return (
              <Link
                key={s.href}
                href={s.href}
                className={`block px-3 py-2 rounded-md text-sm transition-colors ${
                  active
                    ? "bg-muted text-foreground"
                    : "text-muted-foreground hover:text-foreground hover:bg-muted"
                }`}
              >
                {s.label}
              </Link>
            );
          })}
        </nav>
      </aside>
      <div className="flex-1">{children}</div>
    </div>
  );
}
