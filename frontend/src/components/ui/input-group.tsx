import * as React from "react"
import { cn } from "@/lib/utils"

const InputGroup = React.forwardRef<HTMLDivElement, React.HTMLAttributes<HTMLDivElement>>(({ className, ...props }, ref) => <div ref={ref} className={cn("flex h-9 w-full items-center rounded-md border border-input bg-transparent shadow-sm focus-within:ring-2 focus-within:ring-ring focus-within:ring-offset-2", className)} {...props} />)
InputGroup.displayName = "InputGroup"
const InputGroupAddon = ({ className, ...props }: React.ComponentProps<"div">) => <div className={cn("flex shrink-0 items-center gap-1 px-3 text-sm text-muted-foreground", className)} {...props} />
const InputGroupInput = React.forwardRef<HTMLInputElement, React.InputHTMLAttributes<HTMLInputElement>>(({ className, ...props }, ref) => <input ref={ref} className={cn("flex h-full min-w-0 flex-1 bg-transparent px-3 py-1 text-sm outline-none placeholder:text-muted-foreground disabled:cursor-not-allowed disabled:opacity-50", className)} {...props} />)
InputGroupInput.displayName = "InputGroupInput"
export { InputGroup, InputGroupAddon, InputGroupInput }
