"use client";

import dynamic from "next/dynamic";
import { useCallback, useEffect, useState } from "react";
import { useParams } from "next/navigation";
import {
  createSealService, createSealServiceRoute, deleteSealServiceRoute, deploySeal, getSealConfiguration,
  listSealRepositories, listSealServiceRoutes, refreshSealRepository, saveSealConfiguration,
  type SealConfiguration, type SealRepository, type SealServiceRoute,
} from "@/lib/api";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Field, FieldError, FieldGroup, FieldLabel } from "@/components/ui/field";
import { Input } from "@/components/ui/input";
import { PageHeader, PageHeaderDescription, PageHeaderTitle } from "@/components/page-header";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Skeleton } from "@/components/ui/skeleton";

const MonacoEditor = dynamic(() => import("@monaco-editor/react"), { ssr: false });

function errorMessage(error: unknown, fallback: string) {
  return error instanceof Error ? error.message : fallback;
}

export default function SealConfigurePage() {
  const params = useParams();
  const sealId = Number(params.id);
  const [configuration, setConfiguration] = useState<SealConfiguration | null>(null);
  const [repositories, setRepositories] = useState<SealRepository[]>([]);
  const [routes, setRoutes] = useState<Record<number, SealServiceRoute[]>>({});
  const [selectedService, setSelectedService] = useState("");
  const [selectedRepository, setSelectedRepository] = useState("");
  const [serviceName, setServiceName] = useState("");
  const [servicePort, setServicePort] = useState("3000");
  const [domain, setDomain] = useState("");
  const [compose, setCompose] = useState("");
  const [advanced, setAdvanced] = useState(false);
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState("");
  const [error, setError] = useState("");
  const [outcome, setOutcome] = useState("");

  const load = useCallback(async () => {
    if (!Number.isSafeInteger(sealId) || sealId < 1) {
      setError("Invalid Seal identifier.");
      setLoading(false);
      return;
    }
    setLoading(true);
    setError("");
    try {
      const [nextConfiguration, nextRepositories] = await Promise.all([
        getSealConfiguration(sealId), listSealRepositories(sealId),
      ]);
      const entries = await Promise.all(nextConfiguration.services.map(async (service) => [
        service.id, await listSealServiceRoutes(sealId, service.id),
      ] as const));
      setConfiguration(nextConfiguration);
      setRepositories(nextRepositories);
      setRoutes(Object.fromEntries(entries));
      setCompose(nextConfiguration.direct_compose || "");
      setAdvanced(nextConfiguration.authority === "direct");
      setSelectedService((current) => nextConfiguration.services.some((service) => String(service.id) === current)
        ? current : String(nextConfiguration.services[0]?.id || ""));
      setSelectedRepository((current) => nextRepositories.some((repository) => String(repository.id) === current)
        ? current : String(nextRepositories[0]?.id || ""));
    } catch (requestError) {
      setError(errorMessage(requestError, "Failed to load Seal configuration."));
    } finally {
      setLoading(false);
    }
  }, [sealId]);

  useEffect(() => { void load(); }, [load]);

  const run = async (label: string, action: () => Promise<unknown>) => {
    setBusy(label);
    setError("");
    setOutcome("");
    try {
      await action();
      await load();
      setOutcome(`${label} completed.`);
    } catch (requestError) {
      setError(errorMessage(requestError, `Failed to ${label.toLowerCase()}.`));
    } finally {
      setBusy("");
    }
  };

  const selectedServiceId = Number(selectedService);
  const selectedServiceIsPreconfigured = configuration?.services.some((service) => service.id === selectedServiceId && configuration.facts.some((fact) => fact.repository_id === service.repository_id && fact.preconfigured)) || false;

  return <div className="mx-auto max-w-6xl space-y-6 p-4 sm:p-6">
    <PageHeader><div><PageHeaderTitle>Configure Seal</PageHeaderTitle><PageHeaderDescription>Use the verified Next.js path when available, or opt in to direct Compose for an advanced deployment.</PageHeaderDescription></div></PageHeader>
    {error && <FieldError className="whitespace-pre-wrap">{error}</FieldError>}
    {outcome && <p role="status" className="rounded-md border border-success/30 bg-success/5 p-3 text-sm text-success">{outcome}</p>}
    {loading || !configuration ? <div className="space-y-4"><Skeleton className="h-48 w-full" /><Skeleton className="h-64 w-full" /></div> : <>
      <Card><CardHeader><CardTitle className="text-sm">Repositories</CardTitle></CardHeader><CardContent className="space-y-3">
        {repositories.length === 0 ? <p className="text-sm text-muted-foreground">No repositories yet. Add a repository before creating a service.</p> : repositories.map((repository) => <div key={repository.id} className="flex flex-wrap items-center justify-between gap-3 rounded-md border p-3 text-sm"><div><p className="font-medium">{repository.display_name}</p><p className="font-mono text-xs text-muted-foreground">{repository.remote_url} · {repository.branch}</p>{repository.error_summary && <p role="alert" className="mt-1 text-destructive">{repository.error_summary}</p>}</div><div className="flex items-center gap-2"><Badge variant={repository.status === "ready" ? "success" : repository.status === "clone_failed" ? "error" : "warning"}>{repository.status}</Badge><Button size="sm" variant="outline" disabled={!!busy} onClick={() => void run(`Refresh ${repository.display_name}`, () => refreshSealRepository(sealId, repository.id))}>{busy === `Refresh ${repository.display_name}` ? "Refreshing..." : "Pull / refresh"}</Button></div></div>)}
      </CardContent></Card>

      <Card><CardHeader><CardTitle className="text-sm">Services</CardTitle></CardHeader><CardContent className="space-y-4">
        <p className="text-sm text-muted-foreground">Define the repository-backed services that this Seal should build. Services remain private until you add a domain below.</p>
        <FieldGroup className="sm:grid-cols-[minmax(10rem,1fr)_minmax(8rem,1fr)_8rem_auto] sm:items-end"><Field><FieldLabel htmlFor="service-repository">Repository</FieldLabel><Select value={selectedRepository} onValueChange={setSelectedRepository}><SelectTrigger id="service-repository"><SelectValue placeholder="Select repository" /></SelectTrigger><SelectContent>{repositories.map((repository) => <SelectItem key={repository.id} value={String(repository.id)}>{repository.display_name}</SelectItem>)}</SelectContent></Select></Field><Field><FieldLabel htmlFor="service-name">Service name</FieldLabel><Input id="service-name" placeholder="web" value={serviceName} onChange={(event) => setServiceName(event.target.value)} /></Field><Field><FieldLabel htmlFor="service-port">Internal port</FieldLabel><Input id="service-port" inputMode="numeric" value={servicePort} onChange={(event) => setServicePort(event.target.value)} /></Field><Button disabled={!!busy || !selectedRepository || !serviceName.trim() || !Number.isInteger(Number(servicePort)) || Number(servicePort) < 1 || Number(servicePort) > 65535} onClick={() => void run("Create service", async () => { await createSealService(sealId, { repository_id: Number(selectedRepository), name: serviceName.trim(), build_context: ".", internal_port: Number(servicePort) }); setServiceName(""); })}>{busy === "Create service" ? "Creating..." : "Create service"}</Button></FieldGroup>
        {configuration.services.length === 0 ? <p className="rounded-md border border-dashed p-3 text-sm text-muted-foreground">No services yet. Create one to enable the generated Next.js path or advanced deployment.</p> : <ul className="space-y-2">{configuration.services.map((service) => <li key={service.id} className="flex items-center justify-between rounded-md border p-3 text-sm"><span className="font-medium">{service.name}</span><span className="font-mono text-xs text-muted-foreground">{repositories.find((repository) => repository.id === service.repository_id)?.display_name || `Repository #${service.repository_id}`} · port {service.internal_port}</span></li>)}</ul>}
      </CardContent></Card>

      <Card><CardHeader><CardTitle className="text-sm">Next.js deployment</CardTitle></CardHeader><CardContent className="space-y-4">
        {configuration.facts.length === 0 ? <p className="text-sm text-muted-foreground">Add and refresh a repository to check for supported Next.js configuration.</p> : configuration.facts.map((fact) => { const repository = repositories.find((item) => item.id === fact.repository_id); return <div key={fact.repository_id} className="flex flex-wrap items-center justify-between gap-2 rounded-md border p-3 text-sm"><span>{repository?.display_name || `Repository #${fact.repository_id}`}</span>{fact.detected ? <Badge variant={fact.preconfigured ? "success" : "warning"}>{fact.preconfigured ? "Next.js detected · preconfigured" : "Next.js detected · needs advanced configuration"}</Badge> : <Badge variant="default">Next.js not detected</Badge>}</div>; })}
        <Field className="max-w-sm"><FieldLabel htmlFor="nextjs-service">Service to deploy</FieldLabel><Select value={selectedService} onValueChange={setSelectedService}><SelectTrigger id="nextjs-service"><SelectValue placeholder="Create a service first" /></SelectTrigger><SelectContent>{configuration.services.map((service) => <SelectItem key={service.id} value={String(service.id)}>{service.name} · port {service.internal_port}</SelectItem>)}</SelectContent></Select></Field>
        <p className="text-sm text-muted-foreground">{selectedServiceIsPreconfigured ? "This service matches Tamga’s verified Next.js blueprint. Deploy it without editing Compose." : "Select a service from a preconfigured Next.js repository to enable one-click deployment."}</p>
        <Button disabled={!!busy || !selectedServiceIsPreconfigured} onClick={() => void run("Apply one-click Next.js configuration", () => saveSealConfiguration(sealId, { apply_nextjs_template: true, service_id: selectedServiceId }))}>{busy === "Apply one-click Next.js configuration" ? "Applying..." : "Use one-click Next.js configuration"}</Button>
      </CardContent></Card>

      <Card><CardHeader><CardTitle className="text-sm">Advanced Compose</CardTitle></CardHeader><CardContent className="space-y-4">
        {!advanced ? <><p className="text-sm text-muted-foreground">Direct Compose is for professional users who need to define the deployment themselves. It replaces generated configuration for this Seal.</p><Button size="sm" variant="outline" disabled={!!busy} onClick={() => setAdvanced(true)}>Edit Compose in advanced mode</Button></> : <><p className="text-sm text-muted-foreground">Advanced mode is active. Host port publishing is not allowed; services remain private unless a domain is added below.</p><div className="h-80 overflow-hidden rounded-md border"><MonacoEditor language="yaml" theme="vs-dark" value={compose} onChange={(value) => setCompose(value || "")} options={{ minimap: { enabled: false }, automaticLayout: true, fontSize: 13 }} /></div><div className="flex flex-wrap gap-2"><Button disabled={!!busy || !compose.trim()} onClick={() => void run("Save advanced Compose", () => saveSealConfiguration(sealId, { compose_yaml: compose }))}>{busy === "Save advanced Compose" ? "Saving..." : "Save advanced Compose"}</Button>{configuration.authority === "direct" && <Button variant="outline" disabled={!!busy} onClick={() => void run("Restore generated configuration", () => saveSealConfiguration(sealId, { regenerate: true }))}>Restore generated configuration</Button>}</div></>}
      </CardContent></Card>

      <Card><CardHeader><CardTitle className="text-sm">Service domains</CardTitle></CardHeader><CardContent className="space-y-4">
        <p className="text-sm text-muted-foreground">Services are private by default. Add an exact domain only for a service that should accept public traffic.</p>
        {configuration.services.length === 0 ? <p className="rounded-md border border-dashed p-3 text-sm text-muted-foreground">Create a service before assigning a public domain.</p> : configuration.services.map((service) => <div key={service.id} className="rounded-md border p-3"><div className="flex flex-wrap items-center justify-between gap-2"><span className="font-medium">{service.name}</span><Badge variant={(routes[service.id] || []).length ? "success" : "default"}>{(routes[service.id] || []).length ? "Public domains configured" : "Private"}</Badge></div>{(routes[service.id] || []).length === 0 ? <p className="mt-2 text-sm text-muted-foreground">No public domains. This service remains private.</p> : <ul className="mt-2 space-y-2">{(routes[service.id] || []).map((route) => <li key={route.id} className="flex items-center justify-between gap-2 font-mono text-sm"><span>{route.domain}</span><Button size="sm" variant="ghost" className="text-destructive" disabled={!!busy} onClick={() => void run(`Remove ${route.domain}`, () => deleteSealServiceRoute(sealId, service.id, route.id))}>Remove</Button></li>)}</ul>}</div>)}
        <FieldGroup className="sm:grid-cols-[minmax(12rem,1fr)_minmax(14rem,2fr)_auto] sm:items-end"><Field><FieldLabel htmlFor="domain-service">Service</FieldLabel><Select value={selectedService} onValueChange={setSelectedService}><SelectTrigger id="domain-service"><SelectValue placeholder="Select service" /></SelectTrigger><SelectContent>{configuration.services.map((service) => <SelectItem key={service.id} value={String(service.id)}>{service.name}</SelectItem>)}</SelectContent></Select></Field><Field><FieldLabel htmlFor="service-domain">Exact public domain</FieldLabel><Input id="service-domain" placeholder="api.example.com" value={domain} onChange={(event) => setDomain(event.target.value)} /></Field><Button disabled={!!busy || !selectedServiceId || !domain.trim()} onClick={() => void run("Add public domain", async () => { await createSealServiceRoute(sealId, selectedServiceId, domain); setDomain(""); })}>{busy === "Add public domain" ? "Adding..." : "Add public domain"}</Button></FieldGroup>
      </CardContent></Card>

      <Card><CardHeader><CardTitle className="text-sm">Deploy readiness</CardTitle></CardHeader><CardContent className="flex flex-wrap items-center gap-3"><Button disabled={!!busy || !configuration.build_permitted} onClick={() => void run("Deploy Seal", () => deploySeal(sealId))}>{busy === "Deploy Seal" ? "Deploying..." : "Deploy Seal"}</Button><span className="text-sm text-muted-foreground">{configuration.build_permitted ? "Configuration is ready for the Seal build and deploy lifecycle." : "Complete a valid generated or advanced configuration before deploying."}</span></CardContent></Card>
    </>}
  </div>;
}
