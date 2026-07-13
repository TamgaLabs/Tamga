"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { Settings2 } from "lucide-react";

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
    <div className="flex min-h-full flex-col md:min-h-[calc(100svh-3.5rem)] md:flex-row">
      <aside className="w-full shrink-0 border-b border-border bg-muted/20 p-3 md:w-56 md:border-r md:border-b-0 md:p-4">
        <div className="mb-3 flex items-center gap-2 px-2 pt-1">
          <Settings2 className="size-4 text-muted-foreground" aria-hidden="true" />
          <h1 className="font-semibold tracking-tight">Settings</h1>
        </div>
        <nav aria-label="Settings sections" className="grid grid-cols-2 gap-1 sm:grid-cols-3 md:block md:space-y-1">
          {sections.map((s) => {
            const active = pathname.startsWith(s.href);
            return (
              <Link
                key={s.href}
                href={s.href}
                aria-current={active ? "page" : undefined}
                className={`block rounded-md px-3 py-2 text-sm font-medium transition-colors ${
                  active
                    ? "bg-background text-foreground shadow-sm"
                    : "text-muted-foreground hover:bg-background/70 hover:text-foreground"
                }`}
              >
                {s.label}
              </Link>
            );
          })}
        </nav>
      </aside>
      <div className="min-w-0 flex-1">{children}</div>
    </div>
  );
}
