"use client";

import { useCallback, useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { deleteGitCredential, getGitCredential, setGitCredential, type GitCredential } from "@/lib/api";
import { useAuth } from "@/lib/auth";
import { PageHeader, PageHeaderDescription, PageHeaderTitle } from "@/components/page-header";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Field, FieldError, FieldGroup, FieldLabel } from "@/components/ui/field";
import {
  AlertDialog, AlertDialogAction, AlertDialogCancel, AlertDialogContent,
  AlertDialogDescription, AlertDialogFooter, AlertDialogHeader, AlertDialogTitle,
} from "@/components/ui/alert-dialog";
import { toast } from "sonner";

export default function GitSettingsPage() {
  const [gitCredential, setGitCredentialState] = useState<GitCredential | null>(null);
  const { user, loading: authLoading } = useAuth();
  const router = useRouter();

  useEffect(() => { if (!authLoading && !user) router.replace("/login"); }, [user, authLoading, router]);
  const loadGitCredential = useCallback(() => { getGitCredential().then(setGitCredentialState).catch(console.error); }, []);
  useEffect(() => { if (user) loadGitCredential(); }, [user, loadGitCredential]);
  if (authLoading || !user) return null;

  return (
    <div className="mx-auto max-w-3xl space-y-6 p-4 sm:p-6">
      <PageHeader>
        <div className="space-y-1">
          <PageHeaderTitle>Git</PageHeaderTitle>
          <PageHeaderDescription>Manage the credential used for private repository access and sandbox Git operations.</PageHeaderDescription>
        </div>
      </PageHeader>
      <GitCredentialCard credential={gitCredential} onUpdate={loadGitCredential} />
    </div>
  );
}

function GitCredentialCard({ credential, onUpdate }: { credential: GitCredential | null; onUpdate: () => void }) {
  const [showForm, setShowForm] = useState(false);
  const [provider, setProvider] = useState("");
  const [username, setUsername] = useState("");
  const [token, setToken] = useState("");
  const [saving, setSaving] = useState(false);
  const [deleteOpen, setDeleteOpen] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const resetForm = () => {
    setProvider(credential?.provider || "");
    setUsername(credential?.username || "");
    setToken("");
    setError(null);
    setShowForm(false);
  };

  const handleSave = async () => {
    if (!token) { setError("A personal access token is required."); return; }
    setSaving(true); setError(null);
    try {
      await setGitCredential({ provider, username: username || undefined, token });
      resetForm(); onUpdate(); toast.success("Git credential saved");
    } catch (e) {
      console.error(e);
      setError(e instanceof Error ? e.message : "Could not save the Git credential.");
      toast.error("Could not save the Git credential");
    } finally { setSaving(false); }
  };

  const handleDelete = async () => {
    setSaving(true); setError(null);
    try {
      await deleteGitCredential(); onUpdate(); setDeleteOpen(false); toast.success("Git credential deleted");
    } catch (e) {
      console.error(e);
      setError(e instanceof Error ? e.message : "Could not delete the Git credential.");
      toast.error("Could not delete the Git credential");
    } finally { setSaving(false); }
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
