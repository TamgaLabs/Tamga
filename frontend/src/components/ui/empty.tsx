import * as React from "react"
import { cn } from "@/lib/utils"

const Empty = ({ className, ...props }: React.ComponentProps<"div">) => <div className={cn("flex min-h-40 flex-1 flex-col items-center justify-center gap-4 rounded-lg border border-dashed p-8 text-center", className)} {...props} />
const EmptyHeader = ({ className, ...props }: React.ComponentProps<"div">) => <div className={cn("flex max-w-sm flex-col items-center gap-2", className)} {...props} />
const EmptyMedia = ({ className, ...props }: React.ComponentProps<"div">) => <div className={cn("flex size-10 items-center justify-center rounded-full bg-muted", className)} {...props} />
const EmptyTitle = ({ className, ...props }: React.ComponentProps<"h3">) => <h3 className={cn("text-sm font-medium", className)} {...props} />
const EmptyDescription = ({ className, ...props }: React.ComponentProps<"p">) => <p className={cn("text-sm text-muted-foreground", className)} {...props} />
const EmptyContent = ({ className, ...props }: React.ComponentProps<"div">) => <div className={cn("flex items-center gap-2", className)} {...props} />
export { Empty, EmptyHeader, EmptyMedia, EmptyTitle, EmptyDescription, EmptyContent }
