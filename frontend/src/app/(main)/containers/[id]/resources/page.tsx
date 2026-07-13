"use client";

import { useEffect, useState } from "react";
import { updateContainerResources } from "@/lib/api";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { PageHeader, PageHeaderDescription, PageHeaderTitle } from "@/components/page-header";
import { toast } from "sonner";
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
    const toastId = toast.loading("Updating resource limits...");
    try {
      await updateContainerResources(id, {
        ...(memMiB > 0 ? { memory: Math.round(memMiB * 1024 ** 2) } : {}),
        ...(cpuCores > 0 ? { nano_cpus: Math.round(cpuCores * 1_000_000_000) } : {}),
      });
      refetch();
      toast.success("Resource limits updated.", { id: toastId });
    } catch (e) {
      const message = e instanceof Error ? e.message : "Failed to update resources.";
      setError(message);
      toast.error(message, { id: toastId });
    } finally {
      setSaving(false);
    }
  };

  return (
    <div className="space-y-6">
      <PageHeader><div><PageHeaderTitle className="text-lg">Resource limits</PageHeaderTitle><PageHeaderDescription>Update live Docker CPU and memory limits without restarting this container.</PageHeaderDescription></div></PageHeader>
      <Card className="max-w-xl">
        <CardHeader>
          <CardTitle className="text-sm">Resource Limits</CardTitle>
        </CardHeader>
        <CardContent className="space-y-3">
          <div className="grid gap-3 sm:grid-cols-2">
            <div className="space-y-1 flex-1">
              <Label htmlFor="container-memory-mib" className="text-xs">Memory (MiB)</Label>
              <Input
                id="container-memory-mib"
                type="number"
                min="0"
                step="128"
                value={memoryMiB}
                onChange={(e) => setMemoryMiB(e.target.value)}
              />
            </div>
            <div className="space-y-1 flex-1">
              <Label htmlFor="container-cpu-cores" className="text-xs">CPUs (cores)</Label>
              <Input
                id="container-cpu-cores"
                type="number"
                min="0"
                step="0.25"
                value={cpus}
                onChange={(e) => setCpus(e.target.value)}
              />
            </div>
          </div>
          {error && <p role="alert" className="rounded-md border border-destructive/30 bg-destructive/10 p-3 text-sm text-destructive">{error}</p>}
          <Button size="sm" onClick={handleSave} disabled={saving}>
            {saving ? "Saving..." : "Save"}
          </Button>
        </CardContent>
      </Card>
    </div>
  );
}
