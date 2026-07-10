"use client";

import { useEffect, useState, useCallback } from "react";
import { useRouter } from "next/navigation";
import {
  getResourceLimit,
  updateResourceLimit,
  getIdleTimeout,
  setIdleTimeout,
  type ResourceLimit,
  type IdleTimeoutSettings,
} from "@/lib/api";
import { useAuth } from "@/lib/auth";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";

export default function SandboxSettingsPage() {
  const [resourceLimit, setResourceLimit] = useState<ResourceLimit | null>(null);
  const [idleTimeout, setIdleTimeoutState] = useState<IdleTimeoutSettings | null>(null);
  const { user, loading: authLoading } = useAuth();
  const router = useRouter();

  useEffect(() => {
    if (!authLoading && !user) router.replace("/login");
  }, [user, authLoading, router]);

  const loadResourceLimit = useCallback(() => {
    getResourceLimit().then(setResourceLimit).catch(console.error);
  }, []);

  const loadIdleTimeout = useCallback(() => {
    getIdleTimeout().then(setIdleTimeoutState).catch(console.error);
  }, []);

  useEffect(() => {
    if (!user) return;
    loadResourceLimit();
    loadIdleTimeout();
  }, [user, loadResourceLimit, loadIdleTimeout]);

  if (authLoading || !user) return null;

  return (
    <div className="p-6 max-w-3xl mx-auto">
      <h1 className="text-2xl font-bold mb-6">Sandbox</h1>

      <div className="grid gap-4">
        <ResourceLimitCard limit={resourceLimit} onUpdate={loadResourceLimit} />
        <IdleTimeoutCard settings={idleTimeout} onUpdate={loadIdleTimeout} />
      </div>
    </div>
  );
}

// Presets for the detached terminal session idle timeout (see FEAT-022).
// "0" means Never - it must always be present and default.
const IDLE_TIMEOUT_PRESETS: { value: string; label: string }[] = [
  { value: "0", label: "Never" },
  { value: String(30 * 60), label: "30 minutes" },
  { value: String(60 * 60), label: "1 hour" },
  { value: String(8 * 60 * 60), label: "8 hours" },
  { value: String(24 * 60 * 60), label: "24 hours" },
];

// How long a detached terminal session (no attached WebSocket) may sit
// before the backend auto-terminates it (see FEAT-022). Global setting,
// applies on the next background sweep - no restart needed. Defaults to
// Never: sessions persist until explicitly terminated.
function IdleTimeoutCard({ settings, onUpdate }: { settings: IdleTimeoutSettings | null; onUpdate: () => void }) {
  const [saving, setSaving] = useState(false);

  const handleChange = async (value: string) => {
    setSaving(true);
    try {
      await setIdleTimeout(parseInt(value, 10));
      onUpdate();
    } catch (e) {
      console.error(e);
    } finally {
      setSaving(false);
    }
  };

  const value = settings ? String(settings.timeout_seconds) : "0";
  // A saved value that isn't one of the presets (shouldn't normally
  // happen, since this UI only ever writes presets) still needs *a*
  // matching item to render a label instead of a blank trigger.
  const knownValues = IDLE_TIMEOUT_PRESETS.map((p) => p.value);
  const options = knownValues.includes(value)
    ? IDLE_TIMEOUT_PRESETS
    : [...IDLE_TIMEOUT_PRESETS, { value, label: `${value}s` }];

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-sm">Session Idle Timeout</CardTitle>
      </CardHeader>
      <CardContent className="space-y-3">
        <p className="text-xs text-muted-foreground">
          How long a detached terminal session (no browser tab attached) may
          sit before it&apos;s automatically terminated. Never (default)
          means sessions persist until you close them.
        </p>
        <div className="max-w-xs">
          <Select value={value} onValueChange={handleChange} disabled={saving}>
            <SelectTrigger>
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              {options.map((opt) => (
                <SelectItem key={opt.value} value={opt.value}>
                  {opt.label}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
      </CardContent>
    </Card>
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
