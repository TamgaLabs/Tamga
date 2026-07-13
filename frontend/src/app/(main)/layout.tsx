import { ConsoleShell } from "@/components/console-shell";

export default function MainLayout({ children }: { children: React.ReactNode }) {
  return (
    <ConsoleShell>{children}</ConsoleShell>
  );
}
