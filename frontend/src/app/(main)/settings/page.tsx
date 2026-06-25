"use client";

import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { systemInfo, systemPrune, type DockerInfo } from "@/lib/api";
import { useAuth } from "@/lib/auth";
import { getShowSystem, setShowSystem } from "@/lib/settings";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";

export default function SettingsPage() {
  const [info, setInfo] = useState<DockerInfo | null>(null);
  const [showSystem, setShowSystemState] = useState(true);
  const { user, loading: authLoading } = useAuth();
  const router = useRouter();

  useEffect(() => {
    if (!authLoading && !user) router.replace("/login");
  }, [user, authLoading, router]);

  useEffect(() => {
    if (!user) return;
    systemInfo().then(setInfo).catch(console.error);
    setShowSystemState(getShowSystem());
  }, [user]);

  const handleToggleSystem = () => {
    const next = !showSystem;
    setShowSystemState(next);
    setShowSystem(next);
  };

  const handlePrune = async () => {
    if (!confirm("Prune all unused containers, images, volumes, and networks?")) return;
    try {
      await systemPrune();
    } catch (e) {
      console.error(e);
    }
  };

  if (authLoading || !user) return null;

  return (
    <div className="p-6 max-w-3xl mx-auto">
      <h1 className="text-2xl font-bold mb-6">Settings</h1>

      <div className="grid gap-4">
        <Card>
          <CardHeader>
            <CardTitle className="text-sm">Display</CardTitle>
          </CardHeader>
          <CardContent>
            <label className="flex items-center gap-2 text-sm cursor-pointer">
              <input
                type="checkbox"
                checked={showSystem}
                onChange={handleToggleSystem}
                className="accent-neutral-400"
              />
              Show Tamga System
            </label>
            <p className="text-xs text-neutral-500 mt-1">
              When disabled, Tamga system containers and codebases are hidden from all pages.
            </p>
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle className="text-sm">Docker</CardTitle>
          </CardHeader>
          <CardContent>
            {info ? (
              <div className="text-sm space-y-2 text-neutral-400">
                <div className="flex justify-between">
                  <span>Version</span>
                  <span className="text-neutral-200">{info.version}</span>
                </div>
                <div className="flex justify-between">
                  <span>OS</span>
                  <span className="text-neutral-200">{info.os}</span>
                </div>
                <div className="flex justify-between">
                  <span>Architecture</span>
                  <span className="text-neutral-200">{info.architecture}</span>
                </div>
                <div className="flex justify-between">
                  <span>Kernel</span>
                  <span className="text-neutral-200">{info.kernel}</span>
                </div>
                <div className="flex justify-between">
                  <span>Storage Driver</span>
                  <span className="text-neutral-200">{info.driver}</span>
                </div>
                <div className="flex justify-between">
                  <span>Name</span>
                  <span className="text-neutral-200">{info.name}</span>
                </div>
                <div className="border-t border-neutral-800 my-2" />
                <div className="flex justify-between">
                  <span>CPU</span>
                  <span className="text-neutral-200">{info.cpus} cores</span>
                </div>
                <div className="flex justify-between">
                  <span>Memory</span>
                  <span className="text-neutral-200">{(info.memory / 1024 / 1024 / 1024).toFixed(1)} GB</span>
                </div>
                <div className="flex justify-between">
                  <span>Containers</span>
                  <span className="text-neutral-200">{info.containers} ({info.running} running, {info.paused} paused, {info.stopped} stopped)</span>
                </div>
                <div className="flex justify-between">
                  <span>Images</span>
                  <span className="text-neutral-200">{info.images}</span>
                </div>
              </div>
            ) : (
              <p className="text-sm text-neutral-500">Loading...</p>
            )}
            <div className="mt-4 pt-4 border-t border-neutral-800">
              <Button variant="destructive" size="sm" onClick={handlePrune}>
                Prune All
              </Button>
            </div>
          </CardContent>
        </Card>
      </div>
    </div>
  );
}
