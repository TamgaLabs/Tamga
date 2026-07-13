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
import { Field, FieldDescription, FieldError, FieldGroup, FieldLabel } from "@/components/ui/field";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { PageHeader, PageHeaderDescription, PageHeaderTitle } from "@/components/page-header";
import { toast } from "sonner";

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
    <div className="mx-auto max-w-3xl space-y-6 p-4 sm:p-6">
      <PageHeader>
        <div className="space-y-1">
          <PageHeaderTitle>Sandbox</PageHeaderTitle>
          <PageHeaderDescription>Set default resource budgets and detached terminal lifetime for new agent sandboxes.</PageHeaderDescription>
        </div>
      </PageHeader>

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
  const [error, setError] = useState<string | null>(null);

  const handleChange = async (value: string) => {
    setSaving(true);
    setError(null);
    try {
      await setIdleTimeout(parseInt(value, 10));
      onUpdate();
      toast.success("Session idle timeout saved");
    } catch (e) {
      console.error(e);
      const message = e instanceof Error ? e.message : "Could not save the session idle timeout.";
      setError(message);
      toast.error("Could not save the session idle timeout");
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
      <CardHeader className="space-y-1">
        <CardTitle>Session Idle Timeout</CardTitle>
        <p className="text-sm text-muted-foreground">Choose when detached terminal sessions end automatically.</p>
      </CardHeader>
      <CardContent className="space-y-3">
        <p className="text-xs text-muted-foreground">
          How long a detached terminal session (no browser tab attached) may
          sit before it&apos;s automatically terminated. Never (default)
          means sessions persist until you close them.
        </p>
        <Field className="max-w-sm">
          <FieldLabel htmlFor="idle-timeout">Timeout</FieldLabel>
          <Select value={value} onValueChange={handleChange} disabled={saving}>
            <SelectTrigger id="idle-timeout" aria-describedby="idle-timeout-description">
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
          <FieldDescription id="idle-timeout-description">Changes apply during the next background sweep; no restart is needed.</FieldDescription>
          {error && <FieldError>{error}</FieldError>}
        </Field>
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
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!limit) return;
    setMemoryGiB((limit.memory_bytes / (1024 ** 3)).toString());
    setCpus((limit.nano_cpus / 1_000_000_000).toString());
  }, [limit]);

  const handleSave = async () => {
    const memGiB = parseFloat(memoryGiB);
    const cpuCores = parseFloat(cpus);
    if (!(memGiB > 0) || !(cpuCores > 0)) {
      setError("Memory and CPU values must both be greater than zero.");
      return;
    }
    setSaving(true);
    setError(null);
    try {
      await updateResourceLimit({
        memory_bytes: Math.round(memGiB * 1024 ** 3),
        nano_cpus: Math.round(cpuCores * 1_000_000_000),
      });
      onUpdate();
      toast.success("Sandbox resource limits saved");
    } catch (e) {
      console.error(e);
      const message = e instanceof Error ? e.message : "Could not save sandbox resource limits.";
      setError(message);
      toast.error("Could not save sandbox resource limits");
    } finally {
      setSaving(false);
    }
  };

  return (
    <Card>
      <CardHeader className="space-y-1">
        <CardTitle>Sandbox Resource Limits</CardTitle>
        <p className="text-sm text-muted-foreground">Defaults are applied when new agent sandbox containers are created.</p>
      </CardHeader>
      <CardContent className="space-y-3">
        <FieldGroup className="sm:grid-cols-2">
          <Field>
            <FieldLabel htmlFor="sandbox-memory">Memory (GiB)</FieldLabel>
            <Input
              id="sandbox-memory"
              type="number"
              min="0"
              step="0.5"
              value={memoryGiB}
              onChange={(e) => setMemoryGiB(e.target.value)}
            />
          </Field>
          <Field>
            <FieldLabel htmlFor="sandbox-cpus">CPUs (cores)</FieldLabel>
            <Input
              id="sandbox-cpus"
              type="number"
              min="0"
              step="0.5"
              value={cpus}
              onChange={(e) => setCpus(e.target.value)}
            />
          </Field>
        </FieldGroup>
        <p className="text-sm text-muted-foreground">Existing sandboxes keep their current limits until they are recreated.</p>
        {error && <FieldError>{error}</FieldError>}
        <Button onClick={handleSave} disabled={saving}>
          {saving ? "Saving..." : "Save"}
        </Button>
      </CardContent>
    </Card>
  );
}
