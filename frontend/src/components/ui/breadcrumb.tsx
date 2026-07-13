import * as React from "react"
import { Slot } from "@radix-ui/react-slot"
import { ChevronRight, MoreHorizontal } from "lucide-react"

import { cn } from "@/lib/utils"

const Breadcrumb = ({ className, ...props }: React.ComponentProps<"nav">) => (
  <nav aria-label="breadcrumb" className={cn("text-sm", className)} {...props} />
)
const BreadcrumbList = ({ className, ...props }: React.ComponentProps<"ol">) => (
  <ol className={cn("flex flex-wrap items-center gap-1.5 break-words text-muted-foreground sm:gap-2.5", className)} {...props} />
)
const BreadcrumbItem = ({ className, ...props }: React.ComponentProps<"li">) => <li className={cn("inline-flex items-center gap-1.5", className)} {...props} />
const BreadcrumbLink = ({ className, asChild, ...props }: React.ComponentProps<"a"> & { asChild?: boolean }) => {
  const Comp = asChild ? Slot : "a"
  return <Comp className={cn("transition-colors hover:text-foreground", className)} {...props} />
}
const BreadcrumbPage = ({ className, ...props }: React.ComponentProps<"span">) => <span role="link" aria-disabled="true" aria-current="page" className={cn("font-normal text-foreground", className)} {...props} />
const BreadcrumbSeparator = ({ children, className, ...props }: React.ComponentProps<"li">) => <li role="presentation" aria-hidden="true" className={cn("[&>svg]:size-3.5", className)} {...props}>{children ?? <ChevronRight />}</li>
const BreadcrumbEllipsis = ({ className, ...props }: React.ComponentProps<"span">) => <span role="presentation" aria-hidden="true" className={cn("flex size-9 items-center justify-center", className)} {...props}><MoreHorizontal className="size-4" /><span className="sr-only">More</span></span>

export { Breadcrumb, BreadcrumbList, BreadcrumbItem, BreadcrumbLink, BreadcrumbPage, BreadcrumbSeparator, BreadcrumbEllipsis }
