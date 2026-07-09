"use client";

import { useEffect, useState, useCallback } from "react";
import { useRouter } from "next/navigation";
import {
  getGitCredential,
  setGitCredential,
  deleteGitCredential,
  type GitCredential,
} from "@/lib/api";
import { useAuth } from "@/lib/auth";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
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

export default function GitSettingsPage() {
  const [gitCredential, setGitCredentialState] = useState<GitCredential | null>(null);
  const { user, loading: authLoading } = useAuth();
  const router = useRouter();

  useEffect(() => {
    if (!authLoading && !user) router.replace("/login");
  }, [user, authLoading, router]);

  const loadGitCredential = useCallback(() => {
    getGitCredential().then(setGitCredentialState).catch(console.error);
  }, []);

  useEffect(() => {
    if (!user) return;
    loadGitCredential();
  }, [user, loadGitCredential]);

  if (authLoading || !user) return null;

  return (
    <div className="p-6 max-w-3xl mx-auto">
      <h1 className="text-2xl font-bold mb-6">Git</h1>

      <div className="grid gap-4">
        <GitCredentialCard credential={gitCredential} onUpdate={loadGitCredential} />
      </div>
    </div>
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
