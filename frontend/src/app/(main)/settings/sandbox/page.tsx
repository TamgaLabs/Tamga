"use client";

import { useEffect, useState, useCallback } from "react";
import { useRouter } from "next/navigation";
import { getResourceLimit, updateResourceLimit, type ResourceLimit } from "@/lib/api";
import { useAuth } from "@/lib/auth";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";

export default function SandboxSettingsPage() {
  const [resourceLimit, setResourceLimit] = useState<ResourceLimit | null>(null);
  const { user, loading: authLoading } = useAuth();
  const router = useRouter();

  useEffect(() => {
    if (!authLoading && !user) router.replace("/login");
  }, [user, authLoading, router]);

  const loadResourceLimit = useCallback(() => {
    getResourceLimit().then(setResourceLimit).catch(console.error);
  }, []);

  useEffect(() => {
    if (!user) return;
    loadResourceLimit();
  }, [user, loadResourceLimit]);

  if (authLoading || !user) return null;

  return (
    <div className="p-6 max-w-3xl mx-auto">
      <h1 className="text-2xl font-bold mb-6">Sandbox</h1>

      <div className="grid gap-4">
        <ResourceLimitCard limit={resourceLimit} onUpdate={loadResourceLimit} />
      </div>
    </div>
  );
}

// Default CPU/memory limit applied to every agent sandbox container at
// creation time (see FEAT-007). Displayed/edited in friendlier units
// (GiB, CPU cores) and converted to the bytes/nano_cpus the API expects.
function ResourceLimitCard({ limit, onUpdate }: { limit: ResourceLimit | null; onUpdate: () => void }) {
  const [memoryGiB, setMemoryGiB] = useState("");
  const [cpus, setCpus] = useState("");
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    if (!limit) return;
    setMemoryGiB((limit.memory_bytes / (1024 ** 3)).toString());
    setCpus((limit.nano_cpus / 1_000_000_000).toString());
  }, [limit]);

  const handleSave = async () => {
    const memGiB = parseFloat(memoryGiB);
    const cpuCores = parseFloat(cpus);
    if (!(memGiB > 0) || !(cpuCores > 0)) return;
    setSaving(true);
    try {
      await updateResourceLimit({
        memory_bytes: Math.round(memGiB * 1024 ** 3),
        nano_cpus: Math.round(cpuCores * 1_000_000_000),
      });
      onUpdate();
    } catch (e) {
      console.error(e);
    } finally {
      setSaving(false);
    }
  };

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-sm">Sandbox Resource Limits</CardTitle>
      </CardHeader>
      <CardContent className="space-y-3">
        <p className="text-xs text-muted-foreground">
          Default CPU/memory limit applied to every new agent sandbox
          container. Existing sandboxes aren&apos;t affected until recreated.
        </p>
        <div className="flex gap-3">
          <div className="space-y-1 flex-1">
            <Label className="text-xs">Memory (GiB)</Label>
            <Input
              type="number"
              min="0"
              step="0.5"
              value={memoryGiB}
              onChange={(e) => setMemoryGiB(e.target.value)}
            />
          </div>
          <div className="space-y-1 flex-1">
            <Label className="text-xs">CPUs (cores)</Label>
            <Input
              type="number"
              min="0"
              step="0.5"
              value={cpus}
              onChange={(e) => setCpus(e.target.value)}
            />
          </div>
        </div>
        <Button size="sm" onClick={handleSave} disabled={saving}>
          {saving ? "Saving..." : "Save"}
        </Button>
      </CardContent>
    </Card>
  );
}
