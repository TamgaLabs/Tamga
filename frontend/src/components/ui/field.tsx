import * as React from "react"
import { cn } from "@/lib/utils"

const Field = ({ className, ...props }: React.ComponentProps<"div">) => <div className={cn("grid gap-2", className)} {...props} />
const FieldGroup = ({ className, ...props }: React.ComponentProps<"div">) => <div className={cn("grid gap-4", className)} {...props} />
const FieldLabel = ({ className, ...props }: React.ComponentProps<"label">) => <label className={cn("text-sm font-medium leading-none peer-disabled:cursor-not-allowed peer-disabled:opacity-70", className)} {...props} />
const FieldDescription = ({ className, ...props }: React.ComponentProps<"p">) => <p className={cn("text-sm text-muted-foreground", className)} {...props} />
const FieldError = ({ className, ...props }: React.ComponentProps<"p">) => <p role="alert" className={cn("text-sm font-medium text-destructive", className)} {...props} />
export { Field, FieldGroup, FieldLabel, FieldDescription, FieldError }
