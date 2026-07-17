"use client";

import { useCallback, useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { Globe2, HardDrive, Plus, Trash2 } from "lucide-react";
import {
  addBlacklistDomain, addWhitelistDomain, deleteBlacklistDomain, deleteWhitelistDomain,
  getEgressMode, listBlacklist, listWhitelist, setEgressMode,
  deleteGitCredential, getGitCredential, setGitCredential,
  getResourceLimit, updateResourceLimit, getIdleTimeout, setIdleTimeout,
  systemInfo, systemPrune,
  type BlacklistDomain, type DockerInfo, type EgressMode, type GitCredential,
  type IdleTimeoutSettings, type ResourceLimit, type WhitelistDomain,
} from "@/lib/api";
import { useAuth } from "@/lib/auth";
import { useTheme, type Theme } from "@/lib/theme";
import { getShowSystem, setShowSystem } from "@/lib/settings";
import { PageHeader, PageHeaderDescription, PageHeaderTitle } from "@/components/page-header";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Field, FieldDescription, FieldError, FieldGroup, FieldLabel } from "@/components/ui/field";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { RadioGroup, RadioGroupItem } from "@/components/ui/radio-group";
import { Switch } from "@/components/ui/switch";
import { Badge } from "@/components/ui/badge";
import { Skeleton } from "@/components/ui/skeleton";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import {
  Select, SelectContent, SelectItem, SelectTrigger, SelectValue,
} from "@/components/ui/select";
import {
  AlertDialog, AlertDialogAction, AlertDialogCancel, AlertDialogContent,
  AlertDialogDescription, AlertDialogFooter, AlertDialogHeader, AlertDialogTitle,
} from "@/components/ui/alert-dialog";
import { toast } from "sonner";

export default function SettingsPage() {
  const { user, loading: authLoading } = useAuth();
  const router = useRouter();
  useEffect(() => { if (!authLoading && !user) router.replace("/login"); }, [user, authLoading, router]);
  if (authLoading || !user) return null;

  return (
    <div className="mx-auto max-w-3xl space-y-8 p-4 sm:p-6">
      <PageHeader>
        <div className="space-y-1">
          <PageHeaderTitle>Settings</PageHeaderTitle>
          <PageHeaderDescription>Configure your Tamga Console preferences in one place.</PageHeaderDescription>
        </div>
      </PageHeader>

      <Section id="appearance" title="Appearance">
        <AppearanceSection />
      </Section>

      <Section id="git" title="Git">
        <GitSection />
      </Section>

      <Section id="network" title="Network">
        <NetworkSection />
      </Section>

      <Section id="sandbox" title="Sandbox">
        <SandboxSection />
      </Section>

      <Section id="system" title="System">
        <SystemSection />
      </Section>
    </div>
  );
}

function Section({ id, title, children }: { id: string; title: string; children: React.ReactNode }) {
  return (
    <section id={id} className="space-y-4 scroll-mt-20">
      <h2 className="text-lg font-semibold text-foreground">{title}</h2>
      {children}
    </section>
  );
}

/* ── Appearance ──────────────────────────────────────────────── */

function AppearanceSection() {
  const [showSystemState, setShowSystemState] = useState(true);
  const { theme, setTheme } = useTheme();

  useEffect(() => { setShowSystemState(getShowSystem()); }, []);

  const handleToggleSystem = () => {
    const next = !showSystemState;
    setShowSystemState(next);
    setShowSystem(next);
  };

  return (
    <div className="grid gap-4">
      <Card>
        <CardHeader className="space-y-1">
          <CardTitle>Theme</CardTitle>
          <p className="text-sm text-muted-foreground">Choose how Tamga Console appears on this device.</p>
        </CardHeader>
        <CardContent>
          <RadioGroup value={theme} onValueChange={(v) => setTheme(v as Theme)} className="space-y-2">
            {[
              ["light", "Light"], ["dark", "Dark"], ["system", "System"],
            ].map(([value, label]) => (
              <div key={value} className="flex items-center gap-3 rounded-lg border p-3 has-[[data-state=checked]]:border-primary has-[[data-state=checked]]:bg-primary/5">
                <RadioGroupItem value={value} id={`theme-${value}`} />
                <Label htmlFor={`theme-${value}`}>{label}</Label>
              </div>
            ))}
          </RadioGroup>
          <p className="mt-3 text-xs text-muted-foreground">System follows your OS preference and updates live if it changes.</p>
        </CardContent>
      </Card>

      <Card>
        <CardHeader className="space-y-1">
          <CardTitle>Display</CardTitle>
          <p className="text-sm text-muted-foreground">Control whether internal Tamga resources appear throughout the console.</p>
        </CardHeader>
        <CardContent>
          <div className="flex items-center justify-between gap-4 rounded-lg border p-4">
            <div className="space-y-1">
              <Label htmlFor="show-system">Show Tamga System</Label>
              <p className="text-sm text-muted-foreground">Tamga system containers and codebases appear across the console.</p>
            </div>
            <Switch id="show-system" checked={showSystemState} onCheckedChange={handleToggleSystem} />
          </div>
        </CardContent>
      </Card>
    </div>
  );
}

/* ── Git ─────────────────────────────────────────────────────── */

function GitSection() {
  const [credential, setCredential] = useState<GitCredential | null>(null);
  const load = useCallback(() => { getGitCredential().then(setCredential).catch(console.error); }, []);
  useEffect(() => { load(); }, [load]);

  return <GitCredentialCard credential={credential} onUpdate={load} />;
}

function GitCredentialCard({ credential, onUpdate }: { credential: GitCredential | null; onUpdate: () => void }) {
  const [showForm, setShowForm] = useState(false);
  const [provider, setProvider] = useState("");
  const [username, setUsername] = useState("");
  const [token, setToken] = useState("");
  const [saving, setSaving] = useState(false);
  const [deleteOpen, setDeleteOpen] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const resetForm = () => { setProvider(credential?.provider || ""); setUsername(credential?.username || ""); setToken(""); setError(null); setShowForm(false); };

  const handleSave = async () => {
    if (!token) { setError("A personal access token is required."); return; }
    setSaving(true); setError(null);
    try { await setGitCredential({ provider, username: username || undefined, token }); resetForm(); onUpdate(); toast.success("Git credential saved"); }
    catch (e) { console.error(e); setError(e instanceof Error ? e.message : "Could not save the Git credential."); toast.error("Could not save the Git credential"); }
    finally { setSaving(false); }
  };

  const handleDelete = async () => {
    setSaving(true); setError(null);
    try { await deleteGitCredential(); onUpdate(); setDeleteOpen(false); toast.success("Git credential deleted"); }
    catch (e) { console.error(e); setError(e instanceof Error ? e.message : "Could not delete the Git credential."); toast.error("Could not delete the Git credential"); }
    finally { setSaving(false); }
  };

  return (
    <Card>
      <CardHeader className="flex flex-row items-start justify-between gap-4">
        <div className="space-y-1">
          <CardTitle>Git Credential</CardTitle>
          <p className="text-sm text-muted-foreground">One credential is available to private clones and sandbox Git.</p>
        </div>
        <Button size="sm" variant="outline" onClick={() => {
          if (showForm) resetForm(); else { setProvider(credential?.provider || ""); setUsername(credential?.username || ""); setToken(""); setError(null); setShowForm(true); }
        }}>
          {showForm ? "Cancel" : credential?.has_token ? "Update" : "Add credential"}
        </Button>
      </CardHeader>
      <CardContent className="space-y-4">
        {showForm && (
          <div className="rounded-lg border bg-muted/20 p-4">
            <FieldGroup>
              <Field><FieldLabel htmlFor="git-provider">Provider</FieldLabel><Input id="git-provider" value={provider} onChange={(e) => setProvider(e.target.value)} placeholder="github" autoComplete="organization" /></Field>
              <Field><FieldLabel htmlFor="git-username">Username <span className="text-muted-foreground">(optional)</span></FieldLabel><Input id="git-username" value={username} onChange={(e) => setUsername(e.target.value)} placeholder="octocat" autoComplete="username" /></Field>
              <Field><FieldLabel htmlFor="git-token">Token</FieldLabel><Input id="git-token" value={token} onChange={(e) => setToken(e.target.value)} placeholder="ghp_..." type="password" autoComplete="new-password" /></Field>
              {error && <FieldError>{error}</FieldError>}
              <Button className="w-fit" onClick={() => void handleSave()} disabled={saving}>{saving ? "Saving..." : "Save credential"}</Button>
            </FieldGroup>
          </div>
        )}
        {!credential?.has_token ? (
          <p className="rounded-lg border border-dashed p-4 text-sm text-muted-foreground">No Git credential configured.</p>
        ) : (
          <div className="flex flex-wrap items-center justify-between gap-3 rounded-lg border p-4">
            <div className="flex items-center gap-2 text-sm"><span className="font-medium capitalize">{credential.provider || "git"}</span>{credential.username && <span className="text-muted-foreground">{credential.username}</span>}<Badge variant="outline" className="font-mono">••••••••</Badge></div>
            <Button variant="outline" size="sm" className="border-destructive/40 text-destructive hover:bg-destructive hover:text-destructive-foreground" onClick={() => { setError(null); setDeleteOpen(true); }}>Delete</Button>
          </div>
        )}
      </CardContent>
      <AlertDialog open={deleteOpen} onOpenChange={(open) => !saving && setDeleteOpen(open)}>
        <AlertDialogContent>
          <AlertDialogHeader><AlertDialogTitle>Delete Git credential?</AlertDialogTitle><AlertDialogDescription>Private repository clone/pull and sandbox Git push operations stop working until a new credential is configured. This action cannot be undone.</AlertDialogDescription></AlertDialogHeader>
          {error && <FieldError>{error}</FieldError>}
          <AlertDialogFooter><AlertDialogCancel disabled={saving}>Cancel</AlertDialogCancel><AlertDialogAction disabled={saving} onClick={(event) => { event.preventDefault(); void handleDelete(); }}>{saving ? "Deleting..." : "Delete"}</AlertDialogAction></AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </Card>
  );
}

/* ── Network ─────────────────────────────────────────────────── */

type Domain = WhitelistDomain | BlacklistDomain;

function NetworkSection() {
  const [mode, setModeState] = useState<EgressMode>("open");
  const [whitelist, setWhitelist] = useState<WhitelistDomain[]>([]);
  const [blacklist, setBlacklist] = useState<BlacklistDomain[]>([]);
  const [modeError, setModeError] = useState<string | null>(null);
  const [modeSaving, setModeSaving] = useState(false);

  const loadMode = useCallback(() => { getEgressMode().then((state) => setModeState(state.mode)).catch(console.error); }, []);
  const loadWhitelist = useCallback(() => { listWhitelist().then(setWhitelist).catch(console.error); }, []);
  const loadBlacklist = useCallback(() => { listBlacklist().then(setBlacklist).catch(console.error); }, []);
  useEffect(() => { loadMode(); loadWhitelist(); loadBlacklist(); }, [loadMode, loadWhitelist, loadBlacklist]);

  const handleModeChange = async (nextMode: EgressMode) => {
    if (nextMode === mode) return;
    setModeSaving(true); setModeError(null);
    try { await setEgressMode(nextMode); setModeState(nextMode); toast.success("Egress mode saved"); }
    catch (error) { console.error(error); setModeError(error instanceof Error ? error.message : "Could not save egress mode."); toast.error("Could not save egress mode"); }
    finally { setModeSaving(false); }
  };

  return (
    <div className="grid gap-4">
      <Card>
        <CardHeader className="space-y-1"><CardTitle>Egress Mode</CardTitle><p className="text-sm text-muted-foreground">Policy changes apply when the next sandbox starts.</p></CardHeader>
        <CardContent className="space-y-3">
          <RadioGroup value={mode} onValueChange={(value) => void handleModeChange(value as EgressMode)} disabled={modeSaving} className="grid gap-2 sm:grid-cols-3">
            {[
              ["open", "Open", "Allow outbound requests to any domain."],
              ["whitelist", "Whitelist", "Allow only listed domains."],
              ["blacklist", "Blacklist", "Block listed domains."],
            ].map(([value, label, description]) => (
              <Label key={value} htmlFor={`mode-${value}`} className="flex cursor-pointer flex-col gap-1 rounded-lg border p-3 has-[[data-state=checked]]:border-primary has-[[data-state=checked]]:bg-primary/5">
                <span className="flex items-center gap-2"><RadioGroupItem value={value} id={`mode-${value}`} /><span className="font-medium text-foreground">{label}</span></span>
                <span className="pl-6 text-xs font-normal text-muted-foreground">{description}</span>
              </Label>
            ))}
          </RadioGroup>
          {modeError && <FieldError>{modeError}</FieldError>}
        </CardContent>
      </Card>
      <DomainListCard title="Egress Whitelist" description="Domains the sandbox egress proxy permits outbound requests to." empty="No domains in the whitelist." active={mode === "whitelist"} domains={whitelist} onUpdate={loadWhitelist} addDomain={addWhitelistDomain} deleteDomain={deleteWhitelistDomain} />
      <DomainListCard title="Egress Blacklist" description="Domains the sandbox egress proxy denies outbound requests to." empty="No domains in the blacklist." active={mode === "blacklist"} domains={blacklist} onUpdate={loadBlacklist} addDomain={addBlacklistDomain} deleteDomain={deleteBlacklistDomain} />
    </div>
  );
}

function DomainListCard({ title, description, empty, active, domains, onUpdate, addDomain, deleteDomain }: {
  title: string; description: string; empty: string; active: boolean; domains: Domain[]; onUpdate: () => void;
  addDomain: (domain: string) => Promise<unknown>; deleteDomain: (id: number) => Promise<unknown>;
}) {
  const [domain, setDomain] = useState("");
  const [showForm, setShowForm] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);
  const [deleteTarget, setDeleteTarget] = useState<Domain | null>(null);
  const resetForm = () => { setDomain(""); setError(null); setShowForm(false); };
  useEffect(() => { if (!active) setDeleteTarget(null); }, [active]);

  const handleAdd = async () => {
    if (!active) return;
    if (!domain.trim()) { setError("Enter a domain before adding it."); return; }
    setSaving(true); setError(null);
    try { await addDomain(domain.trim()); resetForm(); onUpdate(); toast.success("Domain added"); }
    catch (error) { console.error(error); const message = error instanceof Error ? error.message : "Could not add the domain."; setError(message.includes("domain already exists") ? "Domain already exists." : message); toast.error("Could not add the domain"); }
    finally { setSaving(false); }
  };

  const handleDelete = async () => {
    if (!active || !deleteTarget) return;
    setSaving(true); setError(null);
    try { await deleteDomain(deleteTarget.id); onUpdate(); setDeleteTarget(null); toast.success("Domain removed"); }
    catch (error) { console.error(error); setError(error instanceof Error ? error.message : "Could not remove the domain."); toast.error("Could not remove the domain"); }
    finally { setSaving(false); }
  };

  const slug = title.includes("Whitelist") ? "whitelist" : "blacklist";
  return (
    <Card className={!active ? "opacity-60" : undefined} aria-disabled={!active}>
      <CardHeader className="flex flex-row items-start justify-between gap-4">
        <div className="space-y-1"><CardTitle>{title}</CardTitle><p className="text-sm text-muted-foreground">{description}</p></div>
        <Button size="sm" variant="outline" disabled={!active || saving} onClick={() => { if (showForm) resetForm(); else { setError(null); setShowForm(true); } }}>{showForm ? "Cancel" : <><Plus className="size-4" aria-hidden="true" />Add domain</>}</Button>
      </CardHeader>
      <CardContent className="space-y-4">
        {showForm && <div className="rounded-lg border bg-muted/20 p-4"><FieldGroup><Field><FieldLabel htmlFor={`${slug}-domain`}>Domain</FieldLabel><Input id={`${slug}-domain`} value={domain} onChange={(event) => setDomain(event.target.value)} placeholder="example.com" autoComplete="url" disabled={!active || saving} /></Field>{error && <FieldError>{error}</FieldError>}<Button className="w-fit" disabled={!active || saving} onClick={() => void handleAdd()}>{saving ? "Adding..." : "Add domain"}</Button></FieldGroup></div>}
        {domains.length === 0 ? <p className="rounded-lg border border-dashed p-4 text-sm text-muted-foreground">{empty}</p> : (
          <Table><TableHeader><TableRow><TableHead>Domain</TableHead><TableHead className="w-28 text-right">Action</TableHead></TableRow></TableHeader>
            <TableBody>{domains.map((item) => <TableRow key={item.id}><TableCell className="font-mono text-xs">{item.domain}</TableCell><TableCell className="text-right"><Button variant="ghost" size="sm" className="text-destructive hover:bg-destructive/10 hover:text-destructive" disabled={!active || saving} onClick={() => { setError(null); setDeleteTarget(item); }}><Trash2 className="size-4" aria-hidden="true" /><span className="sr-only">Remove {item.domain}</span></Button></TableCell></TableRow>)}</TableBody></Table>
        )}
      </CardContent>
      <AlertDialog open={!!deleteTarget} onOpenChange={(open) => !saving && !open && setDeleteTarget(null)}>
        <AlertDialogContent>
          <AlertDialogHeader><AlertDialogTitle>Remove domain from {slug}?</AlertDialogTitle><AlertDialogDescription>&quot;{deleteTarget?.domain}&quot; will no longer be {slug === "whitelist" ? "accessible from agent sandboxes" : "blocked by the egress proxy"}. This action cannot be undone.</AlertDialogDescription></AlertDialogHeader>
          {error && <FieldError>{error}</FieldError>}
          <AlertDialogFooter><AlertDialogCancel disabled={saving}>Cancel</AlertDialogCancel><AlertDialogAction disabled={saving} onClick={(event) => { event.preventDefault(); void handleDelete(); }}>{saving ? "Removing..." : "Remove domain"}</AlertDialogAction></AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </Card>
  );
}

/* ── Sandbox ─────────────────────────────────────────────────── */

const IDLE_TIMEOUT_PRESETS: { value: string; label: string }[] = [
  { value: "0", label: "Never" },
  { value: String(30 * 60), label: "30 minutes" },
  { value: String(60 * 60), label: "1 hour" },
  { value: String(8 * 60 * 60), label: "8 hours" },
  { value: String(24 * 60 * 60), label: "24 hours" },
];

function SandboxSection() {
  const [resourceLimit, setResourceLimit] = useState<ResourceLimit | null>(null);
  const [idleTimeout, setIdleTimeoutState] = useState<IdleTimeoutSettings | null>(null);
  const loadResourceLimit = useCallback(() => { getResourceLimit().then(setResourceLimit).catch(console.error); }, []);
  const loadIdleTimeout = useCallback(() => { getIdleTimeout().then(setIdleTimeoutState).catch(console.error); }, []);
  useEffect(() => { loadResourceLimit(); loadIdleTimeout(); }, [loadResourceLimit, loadIdleTimeout]);

  return (
    <div className="grid gap-4">
      <ResourceLimitCard limit={resourceLimit} onUpdate={loadResourceLimit} />
      <IdleTimeoutCard settings={idleTimeout} onUpdate={loadIdleTimeout} />
    </div>
  );
}

function IdleTimeoutCard({ settings, onUpdate }: { settings: IdleTimeoutSettings | null; onUpdate: () => void }) {
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const handleChange = async (value: string) => {
    setSaving(true); setError(null);
    try { await setIdleTimeout(parseInt(value, 10)); onUpdate(); toast.success("Session idle timeout saved"); }
    catch (e) { console.error(e); setError(e instanceof Error ? e.message : "Could not save the session idle timeout."); toast.error("Could not save the session idle timeout"); }
    finally { setSaving(false); }
  };

  const value = settings ? String(settings.timeout_seconds) : "0";
  const knownValues = IDLE_TIMEOUT_PRESETS.map((p) => p.value);
  const options = knownValues.includes(value) ? IDLE_TIMEOUT_PRESETS : [...IDLE_TIMEOUT_PRESETS, { value, label: `${value}s` }];

  return (
    <Card>
      <CardHeader className="space-y-1">
        <CardTitle>Session Idle Timeout</CardTitle>
        <p className="text-sm text-muted-foreground">Choose when detached terminal sessions end automatically.</p>
      </CardHeader>
      <CardContent className="space-y-3">
        <p className="text-xs text-muted-foreground">How long a detached terminal session (no browser tab attached) may sit before it&apos;s automatically terminated. Never (default) means sessions persist until you close them.</p>
        <Field className="max-w-sm">
          <FieldLabel htmlFor="idle-timeout">Timeout</FieldLabel>
          <Select value={value} onValueChange={handleChange} disabled={saving}>
            <SelectTrigger id="idle-timeout" aria-describedby="idle-timeout-description"><SelectValue /></SelectTrigger>
            <SelectContent>{options.map((opt) => <SelectItem key={opt.value} value={opt.value}>{opt.label}</SelectItem>)}</SelectContent>
          </Select>
          <FieldDescription id="idle-timeout-description">Changes apply during the next background sweep; no restart is needed.</FieldDescription>
          {error && <FieldError>{error}</FieldError>}
        </Field>
      </CardContent>
    </Card>
  );
}

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
    if (!(memGiB > 0) || !(cpuCores > 0)) { setError("Memory and CPU values must both be greater than zero."); return; }
    setSaving(true); setError(null);
    try { await updateResourceLimit({ memory_bytes: Math.round(memGiB * 1024 ** 3), nano_cpus: Math.round(cpuCores * 1_000_000_000) }); onUpdate(); toast.success("Sandbox resource limits saved"); }
    catch (e) { console.error(e); setError(e instanceof Error ? e.message : "Could not save sandbox resource limits."); toast.error("Could not save sandbox resource limits"); }
    finally { setSaving(false); }
  };

  return (
    <Card>
      <CardHeader className="space-y-1">
        <CardTitle>Sandbox Resource Limits</CardTitle>
        <p className="text-sm text-muted-foreground">Defaults are applied when new agent sandbox containers are created.</p>
      </CardHeader>
      <CardContent className="space-y-3">
        <FieldGroup className="sm:grid-cols-2">
          <Field><FieldLabel htmlFor="sandbox-memory">Memory (GiB)</FieldLabel><Input id="sandbox-memory" type="number" min="0" step="0.5" value={memoryGiB} onChange={(e) => setMemoryGiB(e.target.value)} /></Field>
          <Field><FieldLabel htmlFor="sandbox-cpus">CPUs (cores)</FieldLabel><Input id="sandbox-cpus" type="number" min="0" step="0.5" value={cpus} onChange={(e) => setCpus(e.target.value)} /></Field>
        </FieldGroup>
        <p className="text-sm text-muted-foreground">Existing sandboxes keep their current limits until they are recreated.</p>
        {error && <FieldError>{error}</FieldError>}
        <Button onClick={handleSave} disabled={saving}>{saving ? "Saving..." : "Save"}</Button>
      </CardContent>
    </Card>
  );
}

/* ── System ──────────────────────────────────────────────────── */

function SystemSection() {
  const [info, setInfo] = useState<DockerInfo | null>(null);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [actionError, setActionError] = useState<string | null>(null);
  const [pruneDialogOpen, setPruneDialogOpen] = useState(false);
  const [pruning, setPruning] = useState(false);

  useEffect(() => {
    setLoadError(null);
    systemInfo().then(setInfo).catch((error) => {
      console.error(error);
      setLoadError(error instanceof Error ? error.message : "Could not load Docker system information.");
    });
  }, []);

  const handlePrune = async () => {
    setPruning(true); setActionError(null);
    try { await systemPrune(); setPruneDialogOpen(false); toast.success("Unused Docker resources pruned"); }
    catch (error) { console.error(error); setActionError(error instanceof Error ? error.message : "Could not prune Docker resources."); toast.error("Could not prune Docker resources"); }
    finally { setPruning(false); }
  };

  const rows = info && [
    ["Version", info.version], ["Operating system", info.os], ["Architecture", info.architecture], ["Kernel", info.kernel], ["Storage driver", info.driver], ["Docker host", info.name],
    ["CPU", `${info.cpus} cores`], ["Memory", `${(info.memory / 1024 / 1024 / 1024).toFixed(1)} GB`], ["Containers", `${info.containers} (${info.running} running, ${info.paused} paused, ${info.stopped} stopped)`], ["Images", String(info.images)],
  ];

  return (
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
      <AlertDialog open={pruneDialogOpen} onOpenChange={(open) => !pruning && setPruneDialogOpen(open)}>
        <AlertDialogContent>
          <AlertDialogHeader><AlertDialogTitle>Prune Docker resources?</AlertDialogTitle><AlertDialogDescription>This removes all unused containers, images, volumes, and networks. This action cannot be undone.</AlertDialogDescription></AlertDialogHeader>
          {actionError && <FieldError>{actionError}</FieldError>}
          <AlertDialogFooter><AlertDialogCancel disabled={pruning}>Cancel</AlertDialogCancel><AlertDialogAction disabled={pruning} onClick={(event) => { event.preventDefault(); void handlePrune(); }}>{pruning ? "Pruning..." : "Prune resources"}</AlertDialogAction></AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </Card>
  );
}
