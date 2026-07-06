"use client";

import { useEffect, useState, useCallback } from "react";
import { useRouter } from "next/navigation";
import {
  systemInfo,
  systemPrune,
  listAgentProviders,
  createAgentProvider,
  updateAgentProvider,
  deleteAgentProvider,
  listApiKeys,
  setApiKey,
  deleteApiKey,
  getResourceLimit,
  updateResourceLimit,
  type DockerInfo,
  type AgentProvider,
  type ApiKeyEntry,
  type ResourceLimit,
} from "@/lib/api";
import { useAuth } from "@/lib/auth";
import { getShowSystem, setShowSystem } from "@/lib/settings";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Checkbox } from "@/components/ui/checkbox";
import { Separator } from "@/components/ui/separator";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
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

export default function SettingsPage() {
  const [info, setInfo] = useState<DockerInfo | null>(null);
  const [showSystemState, setShowSystemState] = useState(true);
  const [providers, setProviders] = useState<AgentProvider[]>([]);
  const [apiKeys, setApiKeys] = useState<ApiKeyEntry[]>([]);
  const [resourceLimit, setResourceLimit] = useState<ResourceLimit | null>(null);
  const [pruneDialogOpen, setPruneDialogOpen] = useState(false);
  const { user, loading: authLoading } = useAuth();
  const router = useRouter();

  useEffect(() => {
    if (!authLoading && !user) router.replace("/login");
  }, [user, authLoading, router]);

  const loadProviders = useCallback(() => {
    listAgentProviders().then(setProviders).catch(console.error);
  }, []);
  const loadApiKeys = useCallback(() => {
    listApiKeys().then(setApiKeys).catch(console.error);
  }, []);
  const loadResourceLimit = useCallback(() => {
    getResourceLimit().then(setResourceLimit).catch(console.error);
  }, []);

  useEffect(() => {
    if (!user) return;
    systemInfo().then(setInfo).catch(console.error);
    setShowSystemState(getShowSystem());
    loadProviders();
    loadApiKeys();
    loadResourceLimit();
  }, [user, loadProviders, loadApiKeys, loadResourceLimit]);

  const handleToggleSystem = () => {
    const next = !showSystemState;
    setShowSystemState(next);
    setShowSystem(next);
  };

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
      <h1 className="text-2xl font-bold mb-6">Settings</h1>

      <div className="grid gap-4">
        <Card>
          <CardHeader>
            <CardTitle className="text-sm">Display</CardTitle>
          </CardHeader>
          <CardContent>
            <div className="flex items-center gap-2">
              <Checkbox
                id="show-system"
                checked={showSystemState}
                onCheckedChange={handleToggleSystem}
              />
              <Label htmlFor="show-system">Show Tamga System</Label>
            </div>
            <p className="text-xs text-muted-foreground mt-2">
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
        <AgentProvidersCard providers={providers} onUpdate={loadProviders} />
        <ApiKeysCard keys={apiKeys} onUpdate={loadApiKeys} />
        <ResourceLimitCard limit={resourceLimit} onUpdate={loadResourceLimit} />
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

const PROVIDER_OPTIONS = [
  { value: "anthropic", label: "Anthropic" },
  { value: "openai", label: "OpenAI" },
  { value: "google", label: "Google" },
  { value: "groq", label: "Groq" },
  { value: "deepseek", label: "DeepSeek" },
  { value: "mistral", label: "Mistral" },
  { value: "cohere", label: "Cohere" },
  { value: "together", label: "Together" },
  { value: "openrouter", label: "OpenRouter" },
  { value: "xai", label: "xAI" },
  { value: "huggingface", label: "HuggingFace" },
];

function ApiKeysCard({ keys, onUpdate }: { keys: ApiKeyEntry[]; onUpdate: () => void }) {
  const [provider, setProvider] = useState("");
  const [keyValue, setKeyValue] = useState("");
  const [showForm, setShowForm] = useState(false);
  const [deleteTarget, setDeleteTarget] = useState<ApiKeyEntry | null>(null);

  const resetForm = () => {
    setProvider("");
    setKeyValue("");
    setShowForm(false);
  };

  const handleSave = async () => {
    if (!provider || !keyValue) return;
    try {
      await setApiKey(provider, keyValue);
      resetForm();
      onUpdate();
    } catch (e) {
      console.error(e);
    }
  };

  const handleDelete = async (id: string) => {
    try {
      await deleteApiKey(id);
      onUpdate();
    } catch (e) {
      console.error(e);
    }
  };

  const confirmDelete = async () => {
    if (!deleteTarget) return;
    await handleDelete(deleteTarget.id);
    setDeleteTarget(null);
  };

  return (
    <Card>
      <CardHeader className="flex flex-row items-center justify-between">
        <CardTitle className="text-sm">API Keys</CardTitle>
        <Button size="sm" variant="outline" onClick={() => { resetForm(); setShowForm(!showForm); }}>
          {showForm ? "Cancel" : "Add Key"}
        </Button>
      </CardHeader>
      <CardContent className="space-y-3">
        {showForm && (
          <div className="space-y-2 p-3 border border-border rounded bg-card">
            <div className="space-y-1">
              <Label className="text-xs">Provider</Label>
              <Select value={provider} onValueChange={setProvider}>
                <SelectTrigger className="w-full">
                  <SelectValue placeholder="Select provider" />
                </SelectTrigger>
                <SelectContent>
                  {PROVIDER_OPTIONS.map((opt) => (
                    <SelectItem key={opt.value} value={opt.value}>{opt.label}</SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
            <div className="space-y-1">
              <Label className="text-xs">API Key</Label>
              <Input
                value={keyValue}
                onChange={(e) => setKeyValue(e.target.value)}
                placeholder="sk-..."
                type="password"
              />
            </div>
            <Button size="sm" onClick={handleSave}>Set Key</Button>
          </div>
        )}
        {keys.length === 0 ? (
          <p className="text-sm text-muted-foreground">No API keys configured. Add keys for your LLM providers.</p>
        ) : (
          <div className="text-sm space-y-2">
            {keys.map((k) => (
              <div key={k.id} className="flex items-center justify-between py-1.5 border-b border-border last:border-0">
                <div className="flex items-center gap-2">
                  <span className="font-medium capitalize">{k.provider}</span>
                  <Badge variant="outline" className="text-xs font-mono">
                    {k.has_key ? "••••••••" : "not set"}
                  </Badge>
                </div>
                <div className="flex gap-1">
                  <Button variant="ghost" size="sm" onClick={() => { setProvider(k.provider); setKeyValue(""); setShowForm(true); }}>
                    Update
                  </Button>
                  <Button variant="ghost" size="sm" className="text-destructive" onClick={() => setDeleteTarget(k)}>
                    Delete
                  </Button>
                </div>
              </div>
            ))}
          </div>
        )}
      </CardContent>

      <AlertDialog open={!!deleteTarget} onOpenChange={(open) => !open && setDeleteTarget(null)}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Delete API key?</AlertDialogTitle>
            <AlertDialogDescription>
              This will delete the API key for &quot;{deleteTarget?.provider}&quot;.
              This action cannot be undone.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>Cancel</AlertDialogCancel>
            <AlertDialogAction onClick={confirmDelete}>Delete</AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </Card>
  );
}

function AgentProvidersCard({ providers, onUpdate }: { providers: AgentProvider[]; onUpdate: () => void }) {
  const [showForm, setShowForm] = useState(false);
  const [editId, setEditId] = useState<string | null>(null);
  const [name, setName] = useState("");
  const [image, setImage] = useState("");
  const [deleteTarget, setDeleteTarget] = useState<AgentProvider | null>(null);

  const resetForm = () => {
    setName("");
    setImage("");
    setShowForm(false);
    setEditId(null);
  };

  const handleEdit = (p: AgentProvider) => {
    setName(p.name);
    setImage(p.image || "");
    setEditId(p.id);
    setShowForm(true);
  };

  const handleSave = async () => {
    const data = { name, image, type: "docker" as const };
    try {
      if (editId) {
        await updateAgentProvider(editId, data);
      } else {
        await createAgentProvider(data);
      }
      resetForm();
      onUpdate();
    } catch (e) {
      console.error(e);
    }
  };

  const handleDelete = async (id: string) => {
    try {
      await deleteAgentProvider(id);
      onUpdate();
    } catch (e) {
      console.error(e);
    }
  };

  const confirmDelete = async () => {
    if (!deleteTarget) return;
    await handleDelete(deleteTarget.id);
    setDeleteTarget(null);
  };

  return (
    <Card>
      <CardHeader className="flex flex-row items-center justify-between">
        <CardTitle className="text-sm">Agent Providers</CardTitle>
        <Button size="sm" variant="outline" onClick={() => { resetForm(); setShowForm(!showForm); }}>
          {showForm ? "Cancel" : "Add Provider"}
        </Button>
      </CardHeader>
      <CardContent className="space-y-3">
        {showForm && (
          <div className="space-y-2 p-3 border border-border rounded bg-card">
            <div className="space-y-1">
              <Label className="text-xs">Name</Label>
              <Input value={name} onChange={(e) => setName(e.target.value)} placeholder="my-agent" />
            </div>
            <div className="space-y-1">
              <Label className="text-xs">Image</Label>
              <Input value={image} onChange={(e) => setImage(e.target.value)} placeholder="tamga-agent" />
            </div>
            <Button size="sm" onClick={handleSave}>{editId ? "Update" : "Create"}</Button>
          </div>
        )}
        {providers.length === 0 ? (
          <p className="text-sm text-muted-foreground">No custom providers configured.</p>
        ) : (
          <div className="text-sm space-y-2">
            {providers.map((p) => (
              <div key={p.id} className="flex items-center justify-between py-1.5 border-b border-border last:border-0">
                <div className="flex items-center gap-2">
                  <span className="font-medium">{p.name}</span>
                  <Badge variant="outline" className="text-xs">docker</Badge>
                  {p.is_default && <Badge variant="success" className="text-xs">default</Badge>}
                </div>
                <div className="flex gap-1">
                  <Button variant="ghost" size="sm" onClick={() => handleEdit(p)}>Edit</Button>
                  {!p.is_default && (
                    <Button variant="ghost" size="sm" className="text-destructive" onClick={() => setDeleteTarget(p)}>
                      Delete
                    </Button>
                  )}
                </div>
              </div>
            ))}
          </div>
        )}
      </CardContent>

      <AlertDialog open={!!deleteTarget} onOpenChange={(open) => !open && setDeleteTarget(null)}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Delete agent provider?</AlertDialogTitle>
            <AlertDialogDescription>
              This will permanently delete &quot;{deleteTarget?.name}&quot;. This
              action cannot be undone.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>Cancel</AlertDialogCancel>
            <AlertDialogAction onClick={confirmDelete}>Delete</AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
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
