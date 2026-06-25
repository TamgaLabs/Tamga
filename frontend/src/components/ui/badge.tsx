import { cn } from "@/lib/utils";

const variants: Record<string, string> = {
  default: "bg-muted text-card-foreground",
  success: "bg-success/20 text-success border-success/30",
  warning: "bg-yellow-600/20 text-yellow-400 border-yellow-600/30",
  error: "bg-destructive/20 text-destructive border-destructive/30",
  info: "bg-accent/20 text-accent border-accent/30",
};

export function Badge({
  className,
  variant = "default",
  children,
}: {
  className?: string;
  variant?: keyof typeof variants;
  children: React.ReactNode;
}) {
  return (
    <span
      className={cn(
        "inline-flex items-center rounded-full border px-2.5 py-0.5 text-xs font-semibold",
        variants[variant],
        className
      )}
    >
      {children}
    </span>
  );
}
