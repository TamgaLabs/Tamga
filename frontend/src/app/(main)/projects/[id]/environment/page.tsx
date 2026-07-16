"use client";

import { useCallback, useEffect, useState } from "react";
import { getProjectConfiguration, listEnvVars, createEnvVar, deleteEnvVar, deleteServiceEnvVar, listServiceEnvVars, upsertServiceEnvVar, type EnvVar, type ServiceEnvVar } from "@/lib/api";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { useProjectContext } from "../project-context";
import { PageHeader, PageHeaderDescription, PageHeaderTitle } from "@/components/page-header";
import { Field, FieldError, FieldGroup, FieldLabel } from "@/components/ui/field";
import { Skeleton } from "@/components/ui/skeleton";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";

export default function ProjectEnvironmentPage() {
  const { project } = useProjectContext();
  const projectId = project.id;
  const [envVars, setEnvVars] = useState<EnvVar[]>([]);
  const [services, setServices] = useState<string[]>([]);
  const [service, setService] = useState("");
  const [serviceVars, setServiceVars] = useState<ServiceEnvVar[]>([]);
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
  const loadServices = useCallback(() => {
    return getProjectConfiguration(projectId).then((configuration) => {
      const names = configuration.services.map(({ name }) => name);
      setServices(names);
      setService((current) => names.includes(current) ? current : (names[0] || ""));
    }).catch((requestError) => {
      setError(requestError instanceof Error ? requestError.message : "Failed to load configured services.");
    });
  }, [projectId]);
  useEffect(() => { void loadServices(); }, [loadServices]);
  const loadServiceVars = useCallback(() => {
    if (!service) { setServiceVars([]); return Promise.resolve(); }
    setError("");
    return listServiceEnvVars(projectId, service).then(setServiceVars).catch((requestError) => setError(requestError instanceof Error ? requestError.message : "Failed to load service environment variables."));
  }, [projectId, service]);
  useEffect(() => { void loadServiceVars(); }, [loadServiceVars]);

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
      <PageHeader><div><PageHeaderTitle>Environment</PageHeaderTitle><PageHeaderDescription>Global values apply to every service; a service value with the same key overrides its global value.</PageHeaderDescription></div></PageHeader>
      <Card>
        <CardHeader>
          <CardTitle className="text-sm">Global environment variables</CardTitle>
        </CardHeader>
        <CardContent className="space-y-5">
          {error && <FieldError>{error}</FieldError>}
          {loading ? <div className="space-y-2"><Skeleton className="h-10 w-full" /><Skeleton className="h-10 w-full" /></div> : envVars.length === 0 ? <p className="text-sm text-muted-foreground">No environment variables configured.</p> : (
            <Table><TableHeader><TableRow><TableHead>Key</TableHead><TableHead>Value</TableHead><TableHead className="w-20"><span className="sr-only">Actions</span></TableHead></TableRow></TableHeader><TableBody>{envVars.map((ev) => <TableRow key={ev.id}><TableCell className="font-mono text-xs font-medium text-primary">{ev.key}</TableCell><TableCell className="max-w-64 truncate font-mono text-xs">{ev.value}</TableCell><TableCell><Button variant="ghost" size="sm" className="text-destructive" onClick={() => void handleDeleteEnvVar(ev.id)}>Delete</Button></TableCell></TableRow>)}</TableBody></Table>
          )}
          <FieldGroup className="border-t pt-5 sm:grid-cols-[1fr_1fr_auto] sm:items-end"><Field><FieldLabel htmlFor="environment-key">Key</FieldLabel><Input id="environment-key" placeholder="KEY" className="font-mono text-xs" value={newKey} onChange={(e) => setNewKey(e.target.value)} /></Field><Field><FieldLabel htmlFor="environment-value">Value</FieldLabel><Input id="environment-value" placeholder="value" className="font-mono text-xs" value={newValue} onChange={(e) => setNewValue(e.target.value)} /></Field><Button onClick={() => void handleAddEnvVar()} disabled={!newKey || saving}>{saving ? "Adding..." : "Add variable"}</Button></FieldGroup>
        </CardContent>
      </Card>
      <Card>
        <CardHeader><CardTitle className="text-sm">Service environment variables</CardTitle></CardHeader>
        <CardContent className="space-y-5">
          {services.length === 0 ? <p className="text-sm text-muted-foreground">Save a Compose configuration to configure service-specific values.</p> : <>
            <Field className="max-w-xs"><FieldLabel htmlFor="environment-service">Service</FieldLabel><Select value={service} onValueChange={setService}><SelectTrigger id="environment-service"><SelectValue /></SelectTrigger><SelectContent>{services.map((name) => <SelectItem key={name} value={name}>{name}</SelectItem>)}</SelectContent></Select></Field>
            {serviceVars.length === 0 ? <p className="text-sm text-muted-foreground">No overrides for {service}.</p> : <Table><TableHeader><TableRow><TableHead>Key</TableHead><TableHead>Value</TableHead><TableHead className="w-20"><span className="sr-only">Actions</span></TableHead></TableRow></TableHeader><TableBody>{serviceVars.map((ev) => <TableRow key={ev.id}><TableCell className="font-mono text-xs font-medium text-primary">{ev.key}</TableCell><TableCell className="max-w-64 truncate font-mono text-xs">{ev.value}</TableCell><TableCell><Button variant="ghost" size="sm" className="text-destructive" onClick={() => void deleteServiceEnvVar(projectId, service, ev.id).then(loadServiceVars).catch((requestError) => setError(requestError instanceof Error ? requestError.message : "Failed to delete service environment variable."))}>Delete</Button></TableCell></TableRow>)}</TableBody></Table>}
            <FieldGroup className="border-t pt-5 sm:grid-cols-[1fr_1fr_auto] sm:items-end"><Field><FieldLabel htmlFor="service-environment-key">Key</FieldLabel><Input id="service-environment-key" placeholder="KEY" className="font-mono text-xs" value={newKey} onChange={(e) => setNewKey(e.target.value)} /></Field><Field><FieldLabel htmlFor="service-environment-value">Value</FieldLabel><Input id="service-environment-value" placeholder="value" className="font-mono text-xs" value={newValue} onChange={(e) => setNewValue(e.target.value)} /></Field><Button disabled={!newKey || saving} onClick={() => { if (!service) return; setSaving(true); setError(""); void upsertServiceEnvVar(projectId, service, newKey, newValue).then(() => { setNewKey(""); setNewValue(""); return loadServiceVars(); }).catch((requestError) => setError(requestError instanceof Error ? requestError.message : "Failed to save service environment variable.")).finally(() => setSaving(false)); }}>{saving ? "Saving..." : "Save override"}</Button></FieldGroup>
          </>}
        </CardContent>
      </Card>
    </div>
  );
}
