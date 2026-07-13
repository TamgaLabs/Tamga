"use client";

import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { HardDrive, Trash2 } from "lucide-react";
import { systemInfo, systemPrune, type DockerInfo } from "@/lib/api";
import { useAuth } from "@/lib/auth";
import { PageHeader, PageHeaderDescription, PageHeaderTitle } from "@/components/page-header";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { FieldError } from "@/components/ui/field";
import { Skeleton } from "@/components/ui/skeleton";
import {
  AlertDialog, AlertDialogAction, AlertDialogCancel, AlertDialogContent,
  AlertDialogDescription, AlertDialogFooter, AlertDialogHeader, AlertDialogTitle,
} from "@/components/ui/alert-dialog";
import { toast } from "sonner";

export default function SystemSettingsPage() {
  const [info, setInfo] = useState<DockerInfo | null>(null);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [actionError, setActionError] = useState<string | null>(null);
  const [pruneDialogOpen, setPruneDialogOpen] = useState(false);
  const [pruning, setPruning] = useState(false);
  const { user, loading: authLoading } = useAuth();
  const router = useRouter();

  useEffect(() => { if (!authLoading && !user) router.replace("/login"); }, [user, authLoading, router]);
  useEffect(() => {
    if (!user) return;
    setLoadError(null);
    systemInfo().then(setInfo).catch((error) => {
      console.error(error);
      setLoadError(error instanceof Error ? error.message : "Could not load Docker system information.");
    });
  }, [user]);

  const handlePrune = async () => {
    setPruning(true); setActionError(null);
    try {
      await systemPrune();
      setPruneDialogOpen(false);
      toast.success("Unused Docker resources pruned");
    } catch (error) {
      console.error(error);
      setActionError(error instanceof Error ? error.message : "Could not prune Docker resources.");
      toast.error("Could not prune Docker resources");
    } finally { setPruning(false); }
  };

  if (authLoading || !user) return null;
  const rows = info && [
    ["Version", info.version], ["Operating system", info.os], ["Architecture", info.architecture], ["Kernel", info.kernel], ["Storage driver", info.driver], ["Docker host", info.name],
    ["CPU", `${info.cpus} cores`], ["Memory", `${(info.memory / 1024 / 1024 / 1024).toFixed(1)} GB`], ["Containers", `${info.containers} (${info.running} running, ${info.paused} paused, ${info.stopped} stopped)`], ["Images", String(info.images)],
  ];

  return (
    <div className="mx-auto max-w-3xl space-y-6 p-4 sm:p-6">
      <PageHeader><div className="space-y-1"><PageHeaderTitle>System</PageHeaderTitle><PageHeaderDescription>Review Docker host capacity and safely reclaim unused local resources.</PageHeaderDescription></div></PageHeader>
      <Card>
        <CardHeader className="space-y-1"><CardTitle className="flex items-center gap-2"><HardDrive className="size-4" aria-hidden="true" />Docker host</CardTitle><p className="text-sm text-muted-foreground">Live information from the Docker daemon running Tamga Console.</p></CardHeader>
        <CardContent className="space-y-5">
          {loadError ? <FieldError>{loadError}</FieldError> : rows ? (
            <dl className="divide-y rounded-lg border">
              {rows.map(([label, value]) => <div key={label} className="grid gap-1 px-4 py-3 text-sm sm:grid-cols-[10rem_1fr] sm:gap-4"><dt className="text-muted-foreground">{label}</dt><dd className="break-words font-medium text-foreground">{value}</dd></div>)}
            </dl>
          ) : <div className="space-y-3"><Skeleton className="h-10 w-full" /><Skeleton className="h-10 w-full" /><Skeleton className="h-10 w-4/5" /></div>}
          <div className="flex flex-col items-start justify-between gap-4 rounded-lg border border-destructive/30 bg-destructive/5 p-4 sm:flex-row sm:items-center">
            <div className="space-y-1"><p className="font-medium">Prune unused Docker resources</p><p className="text-sm text-muted-foreground">Removes unused containers, images, volumes, and networks. Running resources are not removed.</p></div>
            <Button variant="destructive" className="shrink-0" onClick={() => { setActionError(null); setPruneDialogOpen(true); }}><Trash2 className="size-4" aria-hidden="true" />Prune resources</Button>
          </div>
        </CardContent>
      </Card>
      <AlertDialog open={pruneDialogOpen} onOpenChange={(open) => !pruning && setPruneDialogOpen(open)}>
        <AlertDialogContent>
          <AlertDialogHeader><AlertDialogTitle>Prune Docker resources?</AlertDialogTitle><AlertDialogDescription>This removes all unused containers, images, volumes, and networks. This action cannot be undone.</AlertDialogDescription></AlertDialogHeader>
          {actionError && <FieldError>{actionError}</FieldError>}
          <AlertDialogFooter><AlertDialogCancel disabled={pruning}>Cancel</AlertDialogCancel><AlertDialogAction disabled={pruning} onClick={(event) => { event.preventDefault(); void handlePrune(); }}>{pruning ? "Pruning..." : "Prune resources"}</AlertDialogAction></AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  );
}
