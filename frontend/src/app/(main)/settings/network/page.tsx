"use client";

import { useCallback, useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { Globe2, Plus, Trash2 } from "lucide-react";
import {
  addBlacklistDomain, addWhitelistDomain, deleteBlacklistDomain, deleteWhitelistDomain,
  getEgressMode, listBlacklist, listWhitelist, setEgressMode,
  type BlacklistDomain, type EgressMode, type WhitelistDomain,
} from "@/lib/api";
import { useAuth } from "@/lib/auth";
import { PageHeader, PageHeaderDescription, PageHeaderTitle } from "@/components/page-header";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Field, FieldError, FieldGroup, FieldLabel } from "@/components/ui/field";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { RadioGroup, RadioGroupItem } from "@/components/ui/radio-group";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import {
  AlertDialog, AlertDialogAction, AlertDialogCancel, AlertDialogContent,
  AlertDialogDescription, AlertDialogFooter, AlertDialogHeader, AlertDialogTitle,
} from "@/components/ui/alert-dialog";
import { toast } from "sonner";

type Domain = WhitelistDomain | BlacklistDomain;

export default function NetworkSettingsPage() {
  const [mode, setModeState] = useState<EgressMode>("open");
  const [whitelist, setWhitelist] = useState<WhitelistDomain[]>([]);
  const [blacklist, setBlacklist] = useState<BlacklistDomain[]>([]);
  const [modeError, setModeError] = useState<string | null>(null);
  const [modeSaving, setModeSaving] = useState(false);
  const { user, loading: authLoading } = useAuth();
  const router = useRouter();
  useEffect(() => { if (!authLoading && !user) router.replace("/login"); }, [user, authLoading, router]);
  const loadMode = useCallback(() => { getEgressMode().then((state) => setModeState(state.mode)).catch(console.error); }, []);
  const loadWhitelist = useCallback(() => { listWhitelist().then(setWhitelist).catch(console.error); }, []);
  const loadBlacklist = useCallback(() => { listBlacklist().then(setBlacklist).catch(console.error); }, []);
  useEffect(() => { if (user) { loadMode(); loadWhitelist(); loadBlacklist(); } }, [user, loadMode, loadWhitelist, loadBlacklist]);

  const handleModeChange = async (nextMode: EgressMode) => {
    if (nextMode === mode) return;
    setModeSaving(true); setModeError(null);
    try { await setEgressMode(nextMode); setModeState(nextMode); toast.success("Egress mode saved"); }
    catch (error) { console.error(error); setModeError(error instanceof Error ? error.message : "Could not save egress mode."); toast.error("Could not save egress mode"); }
    finally { setModeSaving(false); }
  };
  if (authLoading || !user) return null;

  return (
    <div className="mx-auto max-w-3xl space-y-6 p-4 sm:p-6">
      <PageHeader><div className="space-y-1"><PageHeaderTitle>Network</PageHeaderTitle><PageHeaderDescription>Control outbound sandbox access and maintain the domain policies used at sandbox start.</PageHeaderDescription></div></PageHeader>
      <div className="grid gap-4">
        <Card>
          <CardHeader className="space-y-1"><CardTitle>Egress Mode</CardTitle><p className="text-sm text-muted-foreground">Policy changes apply when the next sandbox starts.</p></CardHeader>
          <CardContent className="space-y-3">
            <RadioGroup value={mode} onValueChange={(value) => void handleModeChange(value as EgressMode)} disabled={modeSaving} className="grid gap-2 sm:grid-cols-3">
              {[
                ["open", "Open", "Allow outbound requests to any domain."],
                ["whitelist", "Whitelist", "Allow only listed domains."],
                ["blacklist", "Blacklist", "Block listed domains."],
              ].map(([value, label, description]) => <Label key={value} htmlFor={`mode-${value}`} className="flex cursor-pointer flex-col gap-1 rounded-lg border p-3 has-[[data-state=checked]]:border-primary has-[[data-state=checked]]:bg-primary/5">
                <span className="flex items-center gap-2"><RadioGroupItem value={value} id={`mode-${value}`} /><span className="font-medium text-foreground">{label}</span></span><span className="pl-6 text-xs font-normal text-muted-foreground">{description}</span>
              </Label>)}
            </RadioGroup>
            {modeError && <FieldError>{modeError}</FieldError>}
          </CardContent>
        </Card>
        <DomainListCard title="Egress Whitelist" description="Domains the sandbox egress proxy permits outbound requests to." empty="No domains in the whitelist." active={mode === "whitelist"} domains={whitelist} onUpdate={loadWhitelist} addDomain={addWhitelistDomain} deleteDomain={deleteWhitelistDomain} />
        <DomainListCard title="Egress Blacklist" description="Domains the sandbox egress proxy denies outbound requests to." empty="No domains in the blacklist." active={mode === "blacklist"} domains={blacklist} onUpdate={loadBlacklist} addDomain={addBlacklistDomain} deleteDomain={deleteBlacklistDomain} />
      </div>
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
  useEffect(() => {
    if (!active) setDeleteTarget(null);
  }, [active]);
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
      <CardHeader className="flex flex-row items-start justify-between gap-4"><div className="space-y-1"><CardTitle>{title}</CardTitle><p className="text-sm text-muted-foreground">{description}</p></div><Button size="sm" variant="outline" disabled={!active || saving} onClick={() => { if (showForm) resetForm(); else { setError(null); setShowForm(true); } }}>{showForm ? "Cancel" : <><Plus className="size-4" aria-hidden="true" />Add domain</>}</Button></CardHeader>
      <CardContent className="space-y-4">
        {showForm && <div className="rounded-lg border bg-muted/20 p-4"><FieldGroup><Field><FieldLabel htmlFor={`${slug}-domain`}>Domain</FieldLabel><Input id={`${slug}-domain`} value={domain} onChange={(event) => setDomain(event.target.value)} placeholder="example.com" autoComplete="url" disabled={!active || saving} /></Field>{error && <FieldError>{error}</FieldError>}<Button className="w-fit" disabled={!active || saving} onClick={() => void handleAdd()}>{saving ? "Adding..." : "Add domain"}</Button></FieldGroup></div>}
        {domains.length === 0 ? <p className="rounded-lg border border-dashed p-4 text-sm text-muted-foreground">{empty}</p> : <Table><TableHeader><TableRow><TableHead>Domain</TableHead><TableHead className="w-28 text-right">Action</TableHead></TableRow></TableHeader><TableBody>{domains.map((item) => <TableRow key={item.id}><TableCell className="font-mono text-xs">{item.domain}</TableCell><TableCell className="text-right"><Button variant="ghost" size="sm" className="text-destructive hover:bg-destructive/10 hover:text-destructive" disabled={!active || saving} onClick={() => { setError(null); setDeleteTarget(item); }}><Trash2 className="size-4" aria-hidden="true" /><span className="sr-only">Remove {item.domain}</span></Button></TableCell></TableRow>)}</TableBody></Table>}
      </CardContent>
      <AlertDialog open={!!deleteTarget} onOpenChange={(open) => !saving && !open && setDeleteTarget(null)}><AlertDialogContent><AlertDialogHeader><AlertDialogTitle>Remove domain from {slug}?</AlertDialogTitle><AlertDialogDescription>&quot;{deleteTarget?.domain}&quot; will no longer be {slug === "whitelist" ? "accessible from agent sandboxes" : "blocked by the egress proxy"}. This action cannot be undone.</AlertDialogDescription></AlertDialogHeader>{error && <FieldError>{error}</FieldError>}<AlertDialogFooter><AlertDialogCancel disabled={saving}>Cancel</AlertDialogCancel><AlertDialogAction disabled={saving} onClick={(event) => { event.preventDefault(); void handleDelete(); }}>{saving ? "Removing..." : "Remove domain"}</AlertDialogAction></AlertDialogFooter></AlertDialogContent></AlertDialog>
    </Card>
  );
}
