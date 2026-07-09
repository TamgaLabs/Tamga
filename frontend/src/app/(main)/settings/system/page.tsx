"use client";

import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { systemInfo, systemPrune, type DockerInfo } from "@/lib/api";
import { useAuth } from "@/lib/auth";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Separator } from "@/components/ui/separator";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog";

export default function SystemSettingsPage() {
  const [info, setInfo] = useState<DockerInfo | null>(null);
  const [pruneDialogOpen, setPruneDialogOpen] = useState(false);
  const { user, loading: authLoading } = useAuth();
  const router = useRouter();

  useEffect(() => {
    if (!authLoading && !user) router.replace("/login");
  }, [user, authLoading, router]);

  useEffect(() => {
    if (!user) return;
    systemInfo().then(setInfo).catch(console.error);
  }, [user]);

  const handlePrune = async () => {
    try {
      await systemPrune();
    } catch (e) {
      console.error(e);
    } finally {
      setPruneDialogOpen(false);
    }
  };

  if (authLoading || !user) return null;

  return (
    <div className="p-6 max-w-3xl mx-auto">
      <h1 className="text-2xl font-bold mb-6">System</h1>

      <div className="grid gap-4">
        <Card>
          <CardHeader>
            <CardTitle className="text-sm">Docker</CardTitle>
          </CardHeader>
          <CardContent>
            {info ? (
              <div className="text-sm space-y-2 text-muted-foreground">
                <div className="flex justify-between">
                  <span>Version</span>
                  <span className="text-foreground">{info.version}</span>
                </div>
                <div className="flex justify-between">
                  <span>OS</span>
                  <span className="text-foreground">{info.os}</span>
                </div>
                <div className="flex justify-between">
                  <span>Architecture</span>
                  <span className="text-foreground">{info.architecture}</span>
                </div>
                <div className="flex justify-between">
                  <span>Kernel</span>
                  <span className="text-foreground">{info.kernel}</span>
                </div>
                <div className="flex justify-between">
                  <span>Storage Driver</span>
                  <span className="text-foreground">{info.driver}</span>
                </div>
                <div className="flex justify-between">
                  <span>Name</span>
                  <span className="text-foreground">{info.name}</span>
                </div>
                <Separator />
                <div className="flex justify-between">
                  <span>CPU</span>
                  <span className="text-foreground">{info.cpus} cores</span>
                </div>
                <div className="flex justify-between">
                  <span>Memory</span>
                  <span className="text-foreground">{(info.memory / 1024 / 1024 / 1024).toFixed(1)} GB</span>
                </div>
                <div className="flex justify-between">
                  <span>Containers</span>
                  <span className="text-foreground">{info.containers} ({info.running} running, {info.paused} paused, {info.stopped} stopped)</span>
                </div>
                <div className="flex justify-between">
                  <span>Images</span>
                  <span className="text-foreground">{info.images}</span>
                </div>
              </div>
            ) : (
              <p className="text-sm text-muted-foreground">Loading...</p>
            )}
            <div className="mt-4 pt-4">
              <Button variant="destructive" size="sm" onClick={() => setPruneDialogOpen(true)}>
                Prune All
              </Button>
            </div>
          </CardContent>
        </Card>
      </div>

      <AlertDialog open={pruneDialogOpen} onOpenChange={setPruneDialogOpen}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Prune Docker resources?</AlertDialogTitle>
            <AlertDialogDescription>
              This will remove all unused containers, images, volumes, and
              networks. This action cannot be undone.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>Cancel</AlertDialogCancel>
            <AlertDialogAction onClick={handlePrune}>Prune</AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  );
}
