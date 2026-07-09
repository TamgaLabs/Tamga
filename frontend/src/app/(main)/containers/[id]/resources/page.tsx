"use client";

import { useEffect, useState } from "react";
import { updateContainerResources } from "@/lib/api";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { useContainerContext } from "../container-context";

export default function ContainerResourcesPage() {
  const { id, container, refetch } = useContainerContext();
  const [memoryMiB, setMemoryMiB] = useState("");
  const [cpus, setCpus] = useState("");
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");

  useEffect(() => {
    const hostConfig = container?.HostConfig;
    setMemoryMiB(hostConfig?.Memory > 0 ? (hostConfig.Memory / 1024 ** 2).toString() : "");
    setCpus(hostConfig?.NanoCpus > 0 ? (hostConfig.NanoCpus / 1_000_000_000).toString() : "");
  }, [container]);

  const handleSave = async () => {
    const memMiB = parseFloat(memoryMiB);
    const cpuCores = parseFloat(cpus);
    if (!(memMiB > 0) && !(cpuCores > 0)) {
      setError("Enter a memory limit and/or CPU limit.");
      return;
    }
    setError("");
    setSaving(true);
    try {
      await updateContainerResources(id, {
        ...(memMiB > 0 ? { memory: Math.round(memMiB * 1024 ** 2) } : {}),
        ...(cpuCores > 0 ? { nano_cpus: Math.round(cpuCores * 1_000_000_000) } : {}),
      });
      refetch();
    } catch (e) {
      setError(e instanceof Error ? e.message : "Failed to update resources.");
    } finally {
      setSaving(false);
    }
  };

  return (
    <div className="p-6 max-w-6xl mx-auto">
      <Card className="max-w-xl">
        <CardHeader>
          <CardTitle className="text-sm">Resource Limits</CardTitle>
        </CardHeader>
        <CardContent className="space-y-3">
          <p className="text-xs text-muted-foreground">
            Update this container&apos;s live memory and CPU limits. Applied
            immediately via Docker, no restart required.
          </p>
          <div className="flex gap-3">
            <div className="space-y-1 flex-1">
              <Label className="text-xs">Memory (MiB)</Label>
              <Input
                type="number"
                min="0"
                step="128"
                value={memoryMiB}
                onChange={(e) => setMemoryMiB(e.target.value)}
              />
            </div>
            <div className="space-y-1 flex-1">
              <Label className="text-xs">CPUs (cores)</Label>
              <Input
                type="number"
                min="0"
                step="0.25"
                value={cpus}
                onChange={(e) => setCpus(e.target.value)}
              />
            </div>
          </div>
          {error && <p className="text-xs text-destructive">{error}</p>}
          <Button size="sm" onClick={handleSave} disabled={saving}>
            {saving ? "Saving..." : "Save"}
          </Button>
        </CardContent>
      </Card>
    </div>
  );
}
