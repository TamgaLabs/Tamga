"use client";

import { useCallback, useEffect, useState } from "react";
import { listEnvVars, createEnvVar, deleteEnvVar, type EnvVar } from "@/lib/api";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { useProjectContext } from "../project-context";
import { PageHeader, PageHeaderDescription, PageHeaderTitle } from "@/components/page-header";
import { Field, FieldError, FieldGroup, FieldLabel } from "@/components/ui/field";
import { Skeleton } from "@/components/ui/skeleton";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";

export default function ProjectEnvironmentPage() {
  const { project } = useProjectContext();
  const projectId = project.id;
  const [envVars, setEnvVars] = useState<EnvVar[]>([]);
  const [newKey, setNewKey] = useState("");
  const [newValue, setNewValue] = useState("");
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");

  const loadEnvVars = useCallback(() => {
    setLoading(true);
    setError("");
    return listEnvVars(projectId).then(setEnvVars).catch((requestError) => {
      console.error(requestError);
      setError(requestError instanceof Error ? requestError.message : "Failed to load environment variables.");
    }).finally(() => setLoading(false));
  }, [projectId]);

  useEffect(() => { void loadEnvVars(); }, [loadEnvVars]);

  const handleAddEnvVar = async () => {
    if (!newKey || saving) return;
    setSaving(true);
    setError("");
    try {
      await createEnvVar(projectId, newKey, newValue);
      setNewKey("");
      setNewValue("");
      await loadEnvVars();
    } catch (requestError) {
      setError(requestError instanceof Error ? requestError.message : "Failed to add environment variable.");
    } finally { setSaving(false); }
  };

  const handleDeleteEnvVar = async (id: number) => {
    setError("");
    try { await deleteEnvVar(projectId, id); await loadEnvVars(); }
    catch (requestError) { setError(requestError instanceof Error ? requestError.message : "Failed to delete environment variable."); }
  };

  return (
    <div className="mx-auto max-w-4xl space-y-6 p-4 sm:p-6">
      <PageHeader><div><PageHeaderTitle>Environment</PageHeaderTitle><PageHeaderDescription>Runtime configuration for this project.</PageHeaderDescription></div></PageHeader>
      <Card>
        <CardHeader>
          <CardTitle className="text-sm">Environment Variables</CardTitle>
        </CardHeader>
        <CardContent className="space-y-5">
          {error && <FieldError>{error}</FieldError>}
          {loading ? <div className="space-y-2"><Skeleton className="h-10 w-full" /><Skeleton className="h-10 w-full" /></div> : envVars.length === 0 ? <p className="text-sm text-muted-foreground">No environment variables configured.</p> : (
            <Table><TableHeader><TableRow><TableHead>Key</TableHead><TableHead>Value</TableHead><TableHead className="w-20"><span className="sr-only">Actions</span></TableHead></TableRow></TableHeader><TableBody>{envVars.map((ev) => <TableRow key={ev.id}><TableCell className="font-mono text-xs font-medium text-primary">{ev.key}</TableCell><TableCell className="max-w-64 truncate font-mono text-xs">{ev.value}</TableCell><TableCell><Button variant="ghost" size="sm" className="text-destructive" onClick={() => void handleDeleteEnvVar(ev.id)}>Delete</Button></TableCell></TableRow>)}</TableBody></Table>
          )}
          <FieldGroup className="border-t pt-5 sm:grid-cols-[1fr_1fr_auto] sm:items-end"><Field><FieldLabel htmlFor="environment-key">Key</FieldLabel><Input id="environment-key" placeholder="KEY" className="font-mono text-xs" value={newKey} onChange={(e) => setNewKey(e.target.value)} /></Field><Field><FieldLabel htmlFor="environment-value">Value</FieldLabel><Input id="environment-value" placeholder="value" className="font-mono text-xs" value={newValue} onChange={(e) => setNewValue(e.target.value)} /></Field><Button onClick={() => void handleAddEnvVar()} disabled={!newKey || saving}>{saving ? "Adding..." : "Add variable"}</Button></FieldGroup>
        </CardContent>
      </Card>
    </div>
  );
}
