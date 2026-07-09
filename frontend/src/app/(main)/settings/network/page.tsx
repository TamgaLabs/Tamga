"use client";

import { useEffect, useState, useCallback } from "react";
import { useRouter } from "next/navigation";
import {
  getEgressMode,
  setEgressMode,
  listWhitelist,
  addWhitelistDomain,
  deleteWhitelistDomain,
  listBlacklist,
  addBlacklistDomain,
  deleteBlacklistDomain,
  type EgressMode,
  type WhitelistDomain,
  type BlacklistDomain,
} from "@/lib/api";
import { useAuth } from "@/lib/auth";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { RadioGroup, RadioGroupItem } from "@/components/ui/radio-group";
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

export default function NetworkSettingsPage() {
  const [mode, setModeState] = useState<EgressMode>("open");
  const [whitelist, setWhitelist] = useState<WhitelistDomain[]>([]);
  const [blacklist, setBlacklist] = useState<BlacklistDomain[]>([]);
  const { user, loading: authLoading } = useAuth();
  const router = useRouter();

  useEffect(() => {
    if (!authLoading && !user) router.replace("/login");
  }, [user, authLoading, router]);

  const loadMode = useCallback(() => {
    getEgressMode().then((s) => setModeState(s.mode)).catch(console.error);
  }, []);

  const loadWhitelist = useCallback(() => {
    listWhitelist().then(setWhitelist).catch(console.error);
  }, []);

  const loadBlacklist = useCallback(() => {
    listBlacklist().then(setBlacklist).catch(console.error);
  }, []);

  useEffect(() => {
    if (!user) return;
    loadMode();
    loadWhitelist();
    loadBlacklist();
  }, [user, loadMode, loadWhitelist, loadBlacklist]);

  const handleModeChange = async (newMode: EgressMode) => {
    try {
      await setEgressMode(newMode);
      setModeState(newMode);
    } catch (e) {
      console.error(e);
    }
  };

  if (authLoading || !user) return null;

  return (
    <div className="p-6 max-w-3xl mx-auto">
      <h1 className="text-2xl font-bold mb-6">Network</h1>

      <div className="grid gap-4">
        <Card>
          <CardHeader>
            <CardTitle className="text-sm">Egress Mode</CardTitle>
          </CardHeader>
          <CardContent>
            <RadioGroup value={mode} onValueChange={(v) => handleModeChange(v as EgressMode)} className="space-y-2">
              <div className="flex items-center gap-2">
                <RadioGroupItem value="open" id="mode-open" />
                <Label htmlFor="mode-open">Open</Label>
              </div>
              <div className="flex items-center gap-2">
                <RadioGroupItem value="whitelist" id="mode-whitelist" />
                <Label htmlFor="mode-whitelist">Whitelist</Label>
              </div>
              <div className="flex items-center gap-2">
                <RadioGroupItem value="blacklist" id="mode-blacklist" />
                <Label htmlFor="mode-blacklist">Blacklist</Label>
              </div>
            </RadioGroup>
            <p className="text-xs text-muted-foreground mt-3">
              Policy applies on next sandbox start.
            </p>
          </CardContent>
        </Card>

        <div className={mode !== "whitelist" ? "opacity-50 pointer-events-none" : ""}>
          <WhitelistCard domains={whitelist} onUpdate={loadWhitelist} />
        </div>

        <div className={mode !== "blacklist" ? "opacity-50 pointer-events-none" : ""}>
          <BlacklistCard domains={blacklist} onUpdate={loadBlacklist} />
        </div>
      </div>
    </div>
  );
}

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

function BlacklistCard({ domains, onUpdate }: { domains: BlacklistDomain[]; onUpdate: () => void }) {
  const [domain, setDomain] = useState("");
  const [showForm, setShowForm] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);
  const [deleteTarget, setDeleteTarget] = useState<BlacklistDomain | null>(null);

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
      await addBlacklistDomain(domain);
      resetForm();
      onUpdate();
    } catch (e) {
      const errMsg = e instanceof Error ? e.message : String(e);
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
      await deleteBlacklistDomain(id);
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
        <CardTitle className="text-sm">Egress Blacklist</CardTitle>
        <Button size="sm" variant="outline" onClick={() => { resetForm(); setShowForm(!showForm); }}>
          {showForm ? "Cancel" : "Add Domain"}
        </Button>
      </CardHeader>
      <CardContent className="space-y-3">
        <p className="text-xs text-muted-foreground">
          Domains the agent sandbox egress proxy will deny outbound requests to.
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
          <p className="text-sm text-muted-foreground">No domains in blacklist.</p>
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
            <AlertDialogTitle>Remove domain from blacklist?</AlertDialogTitle>
            <AlertDialogDescription>
              &quot;{deleteTarget?.domain}&quot; will no longer be blocked by the egress proxy.
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
