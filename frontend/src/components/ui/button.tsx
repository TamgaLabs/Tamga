import * as React from "react";
import { cn } from "@/lib/utils";

const variants = {
  default: "bg-neutral-900 text-white hover:bg-neutral-800",
  destructive: "bg-red-600 text-white hover:bg-red-500",
  outline: "border border-neutral-600 bg-transparent hover:bg-neutral-800",
  secondary: "bg-neutral-800 text-white hover:bg-neutral-700",
  ghost: "hover:bg-neutral-800",
  link: "text-neutral-300 underline-offset-4 hover:underline",
};

const sizes = {
  default: "h-10 px-4 py-2",
  sm: "h-9 rounded-md px-3",
  lg: "h-11 rounded-md px-8",
  icon: "h-10 w-10",
};

type ButtonProps = React.ButtonHTMLAttributes<HTMLButtonElement> & {
  variant?: keyof typeof variants;
  size?: keyof typeof sizes;
};

export function Button({
  className,
  variant = "default",
  size = "default",
  ...props
}: ButtonProps) {
  return (
    <button
      className={cn(
        "inline-flex items-center justify-center whitespace-nowrap rounded-md text-sm font-medium transition-colors focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-neutral-400 disabled:pointer-events-none disabled:opacity-50",
        variants[variant],
        sizes[size],
        className
      )}
      {...props}
    />
  );
}
