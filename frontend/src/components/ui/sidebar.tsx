"use client"

import * as React from "react"
import { Slot } from "@radix-ui/react-slot"
import { PanelLeft } from "lucide-react"

import { Sheet, SheetContent, SheetDescription, SheetHeader, SheetTitle } from "@/components/ui/sheet"
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from "@/components/ui/tooltip"
import { cn } from "@/lib/utils"

const SIDEBAR_COOKIE_NAME = "sidebar_state"
const SIDEBAR_COOKIE_MAX_AGE = 60 * 60 * 24 * 7
const SIDEBAR_WIDTH = "16rem"
const SIDEBAR_WIDTH_ICON = "3.5rem"
const SIDEBAR_KEYBOARD_SHORTCUT = "b"

type SidebarContextValue = {
  state: "expanded" | "collapsed"
  open: boolean
  setOpen: (open: boolean) => void
  openMobile: boolean
  setOpenMobile: (open: boolean) => void
  isMobile: boolean
  toggleSidebar: () => void
}

const SidebarContext = React.createContext<SidebarContextValue | null>(null)

function useIsMobile() {
  const [isMobile, setIsMobile] = React.useState(false)

  React.useEffect(() => {
    const media = window.matchMedia("(max-width: 767px)")
    const update = () => setIsMobile(media.matches)
    update()
    media.addEventListener("change", update)
    return () => media.removeEventListener("change", update)
  }, [])

  return isMobile
}

function SidebarProvider({
  defaultOpen = true,
  open: openProp,
  onOpenChange,
  children,
}: {
  defaultOpen?: boolean
  open?: boolean
  onOpenChange?: (open: boolean) => void
  children: React.ReactNode
}) {
  const isMobile = useIsMobile()
  const [openState, setOpenState] = React.useState(defaultOpen)
  const [openMobile, setOpenMobile] = React.useState(false)
  const open = openProp ?? openState

  const setOpen = React.useCallback((value: boolean) => {
    onOpenChange?.(value)
    if (openProp === undefined) setOpenState(value)
    document.cookie = `${SIDEBAR_COOKIE_NAME}=${value}; path=/; max-age=${SIDEBAR_COOKIE_MAX_AGE}`
  }, [onOpenChange, openProp])

  const toggleSidebar = React.useCallback(() => {
    if (isMobile) setOpenMobile((value) => !value)
    else setOpen(!open)
  }, [isMobile, open, setOpen])

  React.useEffect(() => {
    const onKeyDown = (event: KeyboardEvent) => {
      if ((event.metaKey || event.ctrlKey) && event.key.toLowerCase() === SIDEBAR_KEYBOARD_SHORTCUT) {
        event.preventDefault()
        toggleSidebar()
      }
    }
    window.addEventListener("keydown", onKeyDown)
    return () => window.removeEventListener("keydown", onKeyDown)
  }, [toggleSidebar])

  const value = React.useMemo<SidebarContextValue>(() => ({
    state: open ? "expanded" : "collapsed",
    open,
    setOpen,
    openMobile,
    setOpenMobile,
    isMobile,
    toggleSidebar,
  }), [open, setOpen, openMobile, isMobile, toggleSidebar])

  return <SidebarContext.Provider value={value}><TooltipProvider delayDuration={0}>{children}</TooltipProvider></SidebarContext.Provider>
}

function useSidebar() {
  const context = React.useContext(SidebarContext)
  if (!context) throw new Error("useSidebar must be used within a SidebarProvider.")
  return context
}

const Sidebar = React.forwardRef<HTMLElement, React.ComponentProps<"aside"> & {
  side?: "left" | "right"
  collapsible?: "offcanvas" | "icon" | "none"
}>(({ side = "left", collapsible = "offcanvas", className, children, ...props }, ref) => {
  const { isMobile, state, openMobile, setOpenMobile } = useSidebar()
  const sideClasses = side === "left" ? "left-0 border-r" : "right-0 border-l"
  const widthClass = collapsible === "icon" && state === "collapsed" ? "w-14" : "w-64"

  if (isMobile) {
    return (
      <Sheet open={openMobile} onOpenChange={setOpenMobile}>
        <SheetContent side={side} className="w-72 p-0">
          <SheetHeader className="sr-only"><SheetTitle>Navigation</SheetTitle><SheetDescription>Application navigation</SheetDescription></SheetHeader>
          <aside ref={ref} className={cn("flex h-full w-full flex-col bg-sidebar text-sidebar-foreground", className)} {...props}>{children}</aside>
        </SheetContent>
      </Sheet>
    )
  }

  if (collapsible === "offcanvas" && state === "collapsed") return null

  return (
    <aside
      ref={ref}
      data-slot="sidebar"
      data-state={state}
      data-collapsible={state === "collapsed" ? collapsible : ""}
      className={cn("fixed inset-y-0 z-40 hidden h-svh shrink-0 flex-col bg-sidebar text-sidebar-foreground transition-[width,transform] duration-200 md:flex", sideClasses, widthClass, className)}
      style={{ "--sidebar-width": SIDEBAR_WIDTH, "--sidebar-width-icon": SIDEBAR_WIDTH_ICON } as React.CSSProperties}
      {...props}
    >
      {children}
    </aside>
  )
})
Sidebar.displayName = "Sidebar"

const SidebarTrigger = React.forwardRef<HTMLButtonElement, React.ComponentProps<"button">>(({ className, onClick, ...props }, ref) => {
  const { toggleSidebar, state, isMobile, openMobile } = useSidebar()
  return <button ref={ref} data-sidebar="trigger" aria-label="Toggle sidebar" aria-expanded={isMobile ? openMobile : state === "expanded"} className={cn("inline-flex size-8 items-center justify-center rounded-md outline-none hover:bg-accent focus-visible:ring-2 focus-visible:ring-ring", className)} onClick={(event) => { onClick?.(event); toggleSidebar() }} {...props}><PanelLeft className="size-4" /><span className="sr-only">Toggle sidebar</span></button>
})
SidebarTrigger.displayName = "SidebarTrigger"

const SidebarRail = React.forwardRef<HTMLButtonElement, React.ComponentProps<"button">>(({ className, ...props }, ref) => {
  const { toggleSidebar } = useSidebar()
  return <button ref={ref} type="button" tabIndex={-1} aria-label="Toggle sidebar" title="Toggle sidebar" onClick={toggleSidebar} className={cn("absolute inset-y-0 z-50 hidden w-4 -translate-x-1/2 cursor-ew-resize md:block", className)} {...props} />
})
SidebarRail.displayName = "SidebarRail"

const SidebarInset = React.forwardRef<HTMLDivElement, React.ComponentProps<"main">>(({ className, ...props }, ref) => <main ref={ref} className={cn("relative flex w-full flex-1 flex-col bg-background", className)} {...props} />)
SidebarInset.displayName = "SidebarInset"
const SidebarHeader = ({ className, ...props }: React.ComponentProps<"div">) => <div data-sidebar="header" className={cn("flex flex-col gap-2 p-2", className)} {...props} />
const SidebarContent = ({ className, ...props }: React.ComponentProps<"div">) => <div data-sidebar="content" className={cn("flex min-h-0 flex-1 flex-col gap-2 overflow-auto", className)} {...props} />
const SidebarFooter = ({ className, ...props }: React.ComponentProps<"div">) => <div data-sidebar="footer" className={cn("flex flex-col gap-2 p-2", className)} {...props} />
const SidebarGroup = ({ className, ...props }: React.ComponentProps<"div">) => <div data-sidebar="group" className={cn("relative flex w-full min-w-0 flex-col p-2", className)} {...props} />
const SidebarGroupLabel = ({ className, ...props }: React.ComponentProps<"div">) => <div data-sidebar="group-label" className={cn("flex h-8 shrink-0 items-center rounded-md px-2 text-xs font-medium text-muted-foreground outline-none", className)} {...props} />
const SidebarMenu = ({ className, ...props }: React.ComponentProps<"ul">) => <ul data-sidebar="menu" className={cn("flex w-full min-w-0 flex-col gap-1", className)} {...props} />
const SidebarMenuItem = ({ className, ...props }: React.ComponentProps<"li">) => <li data-sidebar="menu-item" className={cn("group/menu-item relative", className)} {...props} />
const SidebarMenuButton = React.forwardRef<HTMLButtonElement, React.ComponentProps<"button"> & { asChild?: boolean; isActive?: boolean; tooltip?: string }>(({ asChild = false, isActive, tooltip, className, ...props }, ref) => {
  const Comp = asChild ? Slot : "button"
  const button = <Comp ref={ref} data-sidebar="menu-button" data-active={isActive} className={cn("peer/menu-button flex w-full items-center gap-2 overflow-hidden rounded-md px-2 py-2 text-left text-sm outline-none transition-colors hover:bg-sidebar-accent hover:text-sidebar-accent-foreground focus-visible:ring-2 focus-visible:ring-sidebar-ring data-[active=true]:bg-sidebar-accent data-[active=true]:font-medium data-[active=true]:text-sidebar-accent-foreground", className)} {...props} />
  return tooltip ? <Tooltip><TooltipTrigger asChild>{button}</TooltipTrigger><TooltipContent side="right" align="center" hidden={false}>{tooltip}</TooltipContent></Tooltip> : button
})
SidebarMenuButton.displayName = "SidebarMenuButton"

export { SidebarProvider, useSidebar, Sidebar, SidebarTrigger, SidebarRail, SidebarInset, SidebarHeader, SidebarContent, SidebarFooter, SidebarGroup, SidebarGroupLabel, SidebarMenu, SidebarMenuItem, SidebarMenuButton }
