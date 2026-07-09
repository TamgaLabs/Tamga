"use client";

import { useEffect, useState, useCallback } from "react";
import { useRouter } from "next/navigation";
import {
  systemInfo,
  systemPrune,
  getResourceLimit,
  updateResourceLimit,
  getGitCredential,
  setGitCredential,
  deleteGitCredential,
  listWhitelist,
  addWhitelistDomain,
  deleteWhitelistDomain,
  type DockerInfo,
  type ResourceLimit,
  type GitCredential,
  type WhitelistDomain,
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
  const [resourceLimit, setResourceLimit] = useState<ResourceLimit | null>(null);
  const [gitCredential, setGitCredentialState] = useState<GitCredential | null>(null);
  const [whitelist, setWhitelist] = useState<WhitelistDomain[]>([]);
  const [pruneDialogOpen, setPruneDialogOpen] = useState(false);
  const { user, loading: authLoading } = useAuth();
  const router = useRouter();

  useEffect(() => {
    if (!authLoading && !user) router.replace("/login");
  }, [user, authLoading, router]);

  const loadResourceLimit = useCallback(() => {
    getResourceLimit().then(setResourceLimit).catch(console.error);
  }, []);
  const loadGitCredential = useCallback(() => {
    getGitCredential().then(setGitCredentialState).catch(console.error);
  }, []);
  const loadWhitelist = useCallback(() => {
    listWhitelist().then(setWhitelist).catch(console.error);
  }, []);

  useEffect(() => {
    if (!user) return;
    systemInfo().then(setInfo).catch(console.error);
    setShowSystemState(getShowSystem());
    loadResourceLimit();
    loadGitCredential();
    loadWhitelist();
  }, [user, loadResourceLimit, loadGitCredential, loadWhitelist]);

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
        <ResourceLimitCard limit={resourceLimit} onUpdate={loadResourceLimit} />
        <GitCredentialCard credential={gitCredential} onUpdate={loadGitCredential} />
        <WhitelistCard domains={whitelist} onUpdate={loadWhitelist} />
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

// The single global git credential (see FEAT-008), used both by the
// backend to `git clone`/`pull` private repos and injected into every
// agent sandbox so `git commit`/`push` works from the terminal. Single
// value, not a list - shown/edited like ResourceLimitCard.
function GitCredentialCard({ credential, onUpdate }: { credential: GitCredential | null; onUpdate: () => void }) {
  const [showForm, setShowForm] = useState(false);
  const [provider, setProvider] = useState("");
  const [username, setUsername] = useState("");
  const [token, setToken] = useState("");
  const [saving, setSaving] = useState(false);
  const [deleteOpen, setDeleteOpen] = useState(false);

  const resetForm = () => {
    setProvider(credential?.provider || "");
    setUsername(credential?.username || "");
    setToken("");
    setShowForm(false);
  };

  const handleSave = async () => {
    if (!token) return;
    setSaving(true);
    try {
      await setGitCredential({ provider, username: username || undefined, token });
      resetForm();
      onUpdate();
    } catch (e) {
      console.error(e);
    } finally {
      setSaving(false);
    }
  };

  const handleDelete = async () => {
    try {
      await deleteGitCredential();
      onUpdate();
    } catch (e) {
      console.error(e);
    } finally {
      setDeleteOpen(false);
    }
  };

  return (
    <Card>
      <CardHeader className="flex flex-row items-center justify-between">
        <CardTitle className="text-sm">Git Credential</CardTitle>
        <Button
          size="sm"
          variant="outline"
          onClick={() => {
            if (showForm) {
              resetForm();
            } else {
              setProvider(credential?.provider || "");
              setUsername(credential?.username || "");
              setToken("");
              setShowForm(true);
            }
          }}
        >
          {showForm ? "Cancel" : credential?.has_token ? "Update" : "Add Credential"}
        </Button>
      </CardHeader>
      <CardContent className="space-y-3">
        <p className="text-xs text-muted-foreground">
          Used to clone/pull private repositories and to authenticate
          `git commit`/`push` from an agent sandbox terminal. Only one
          credential is stored globally.
        </p>
        {showForm && (
          <div className="space-y-2 p-3 border border-border rounded bg-card">
            <div className="space-y-1">
              <Label className="text-xs">Provider</Label>
              <Input
                value={provider}
                onChange={(e) => setProvider(e.target.value)}
                placeholder="github"
              />
            </div>
            <div className="space-y-1">
              <Label className="text-xs">Username (optional)</Label>
              <Input
                value={username}
                onChange={(e) => setUsername(e.target.value)}
                placeholder="octocat"
              />
            </div>
            <div className="space-y-1">
              <Label className="text-xs">Token</Label>
              <Input
                value={token}
                onChange={(e) => setToken(e.target.value)}
                placeholder="ghp_..."
                type="password"
              />
            </div>
            <Button size="sm" onClick={handleSave} disabled={saving}>
              {saving ? "Saving..." : "Save"}
            </Button>
          </div>
        )}
        {!credential?.has_token ? (
          <p className="text-sm text-muted-foreground">No git credential configured.</p>
        ) : (
          <div className="flex items-center justify-between py-1.5">
            <div className="flex items-center gap-2 text-sm">
              <span className="font-medium capitalize">{credential.provider || "git"}</span>
              {credential.username && (
                <span className="text-muted-foreground">{credential.username}</span>
              )}
              <Badge variant="outline" className="text-xs font-mono">••••••••</Badge>
            </div>
            <Button variant="ghost" size="sm" className="text-destructive" onClick={() => setDeleteOpen(true)}>
              Delete
            </Button>
          </div>
        )}
      </CardContent>

      <AlertDialog open={deleteOpen} onOpenChange={setDeleteOpen}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Delete git credential?</AlertDialogTitle>
            <AlertDialogDescription>
              Private repo clones/pulls and sandbox `git commit`/`push` will
              stop working until a new credential is configured. This action
              cannot be undone.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>Cancel</AlertDialogCancel>
            <AlertDialogAction onClick={handleDelete}>Delete</AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </Card>
  );
}

// Agent sandbox egress whitelist (see FEAT-006): domains the sandbox egress
// proxy will permit outbound requests to. Listed/added/removed here.
function WhitelistCard({ domains, onUpdate }: { domains: WhitelistDomain[]; onUpdate: () => void }) {
  const [domain, setDomain] = useState("");
  const [showForm, setShowForm] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);
  const [deleteTarget, setDeleteTarget] = useState<WhitelistDomain | null>(null);

  const resetForm = () => {
    setDomain("");
    setError(null);
    setShowForm(false);
  };

  const handleAdd = async () => {
    if (!domain) return;
    setSaving(true);
    setError(null);
    try {
      await addWhitelistDomain(domain);
      resetForm();
      onUpdate();
    } catch (e) {
      const errMsg = e instanceof Error ? e.message : String(e);
      // Check if it's a 409 duplicate domain error from the backend
      if (errMsg.includes("domain already exists")) {
        setError("Domain already exists");
      } else {
        setError(errMsg || "Failed to add domain");
      }
    } finally {
      setSaving(false);
    }
  };

  const handleDelete = async (id: number) => {
    try {
      await deleteWhitelistDomain(id);
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
        <CardTitle className="text-sm">Egress Whitelist</CardTitle>
        <Button size="sm" variant="outline" onClick={() => { resetForm(); setShowForm(!showForm); }}>
          {showForm ? "Cancel" : "Add Domain"}
        </Button>
      </CardHeader>
      <CardContent className="space-y-3">
        <p className="text-xs text-muted-foreground">
          Domains the agent sandbox egress proxy will permit outbound requests to.
        </p>
        {showForm && (
          <div className="space-y-2 p-3 border border-border rounded bg-card">
            <div className="space-y-1">
              <Label className="text-xs">Domain</Label>
              <Input
                value={domain}
                onChange={(e) => setDomain(e.target.value)}
                placeholder="example.com"
              />
            </div>
            {error && (
              <p className="text-xs text-destructive">{error}</p>
            )}
            <Button size="sm" onClick={handleAdd} disabled={saving}>
              {saving ? "Adding..." : "Add"}
            </Button>
          </div>
        )}
        {domains.length === 0 ? (
          <p className="text-sm text-muted-foreground">No domains in whitelist.</p>
        ) : (
          <div className="text-sm space-y-2">
            {domains.map((d) => (
              <div key={d.id} className="flex items-center justify-between py-1.5 border-b border-border last:border-0">
                <span className="font-mono text-sm">{d.domain}</span>
                <Button variant="ghost" size="sm" className="text-destructive" onClick={() => setDeleteTarget(d)}>
                  Delete
                </Button>
              </div>
            ))}
          </div>
        )}
      </CardContent>

      <AlertDialog open={!!deleteTarget} onOpenChange={(open) => !open && setDeleteTarget(null)}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Remove domain from whitelist?</AlertDialogTitle>
            <AlertDialogDescription>
              &quot;{deleteTarget?.domain}&quot; will no longer be accessible from agent sandboxes.
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
