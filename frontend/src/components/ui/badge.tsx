import { cn } from "@/lib/utils";

const variants: Record<string, string> = {
  default: "bg-neutral-700 text-neutral-200",
  success: "bg-green-600/20 text-green-400 border-green-600/30",
  warning: "bg-yellow-600/20 text-yellow-400 border-yellow-600/30",
  error: "bg-red-600/20 text-red-400 border-red-600/30",
  info: "bg-blue-600/20 text-blue-400 border-blue-600/30",
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
