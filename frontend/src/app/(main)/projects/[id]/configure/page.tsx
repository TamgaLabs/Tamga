"use client";

import dynamic from "next/dynamic";
import { useCallback, useEffect, useState } from "react";
import {
  buildProject, createProjectSource, deleteProjectSource, deployProject,
  getProjectConfiguration, listProjectRoutes, refreshAllProjectSources,
  refreshProjectSource, saveProjectConfiguration, setProjectRoutes,
  type ProjectConfiguration, type ProjectRoute,
} from "@/lib/api";
import { useProjectContext } from "../project-context";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Badge } from "@/components/ui/badge";
import { Field, FieldError, FieldGroup, FieldLabel } from "@/components/ui/field";
import { PageHeader, PageHeaderDescription, PageHeaderTitle } from "@/components/page-header";
import { Skeleton } from "@/components/ui/skeleton";

const MonacoEditor = dynamic(() => import("@monaco-editor/react"), { ssr: false });

function message(error: unknown, fallback: string) { return error instanceof Error ? error.message : fallback; }

export default function ProjectConfigurePage() {
  const { project, refetch } = useProjectContext();
  const [configuration, setConfiguration] = useState<ProjectConfiguration | null>(null);
  const [routes, setRoutes] = useState<ProjectRoute[]>([]);
  const [compose, setCompose] = useState("");
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState("");
  const [error, setError] = useState("");
  const [source, setSource] = useState({ display_name: "", remote_url: "", branch: "main", workspace_path: "" });
  const [route, setRoute] = useState({ service: "", domain: "" });

  const load = useCallback(async () => {
    setLoading(true); setError("");
    try {
      const [nextConfiguration, nextRoutes] = await Promise.all([getProjectConfiguration(project.id), listProjectRoutes(project.id)]);
      setConfiguration(nextConfiguration); setRoutes(nextRoutes);
      setCompose(nextConfiguration.accepted_compose || nextConfiguration.pending_compose || "");
    } catch (requestError) { setError(message(requestError, "Failed to load project configuration.")); }
    finally { setLoading(false); }
  }, [project.id]);
  useEffect(() => { void load(); }, [load]);

  const run = async (label: string, action: () => Promise<unknown>, reload = true) => {
    setBusy(label); setError("");
    try { await action(); if (reload) await load(); refetch(); }
    catch (requestError) { setError(message(requestError, `Failed to ${label.toLowerCase()}.`)); }
    finally { setBusy(""); }
  };
  const addRoute = () => {
    if (!route.service || !route.domain) return;
    setRoutes((current) => [...current, route]); setRoute({ service: "", domain: "" });
  };

  const canDeploy = project.status === "ready_to_deploy";
  return <div className="mx-auto max-w-6xl space-y-6 p-4 sm:p-6">
    <PageHeader><div><PageHeaderTitle>Configure</PageHeaderTitle><PageHeaderDescription>Approve the Compose configuration, then build and deploy this project.</PageHeaderDescription></div></PageHeader>
    {error && <FieldError className="whitespace-pre-wrap">{error}</FieldError>}
    {loading || !configuration ? <div className="space-y-4"><Skeleton className="h-48 w-full" /><Skeleton className="h-64 w-full" /></div> : <>
      <Card><CardHeader className="flex flex-row items-center justify-between"><CardTitle className="text-sm">Sources</CardTitle><Button size="sm" variant="outline" disabled={!!busy || configuration.sources.length === 0} onClick={() => void run("Refresh all", () => refreshAllProjectSources(project.id))}>{busy === "Refresh all" ? "Refreshing..." : "Pull again"}</Button></CardHeader><CardContent className="space-y-4">
        {configuration.sources.map((item) => <div key={item.id} className="rounded-md border p-3 text-sm"><div className="flex flex-wrap items-center justify-between gap-2"><div><span className="font-medium">{item.display_name}</span> <span className="font-mono text-xs text-muted-foreground">{item.workspace_path}</span></div><div className="flex items-center gap-2"><Badge variant={item.status === "ready" ? "success" : item.status === "clone_failed" ? "error" : "warning"}>{item.status}</Badge><Button size="sm" variant="outline" disabled={!!busy} onClick={() => void run(`Refresh ${item.id}`, () => refreshProjectSource(project.id, item.id))}>Refresh</Button><Button size="sm" variant="ghost" className="text-destructive" disabled={!!busy} onClick={() => void run(`Remove ${item.id}`, () => deleteProjectSource(project.id, item.id))}>Remove</Button></div></div>{item.error_summary && <p role="alert" className="mt-2 text-destructive">{item.error_summary}</p>}<p className="mt-1 truncate text-xs text-muted-foreground">{item.remote_url} · {item.branch}</p></div>)}
        <FieldGroup className="border-t pt-4 sm:grid-cols-2"><Field><FieldLabel htmlFor="source-name">Name</FieldLabel><Input id="source-name" value={source.display_name} onChange={(e) => setSource({ ...source, display_name: e.target.value })} /></Field><Field><FieldLabel htmlFor="source-url">Repository URL</FieldLabel><Input id="source-url" value={source.remote_url} onChange={(e) => setSource({ ...source, remote_url: e.target.value })} /></Field><Field><FieldLabel htmlFor="source-branch">Branch</FieldLabel><Input id="source-branch" value={source.branch} onChange={(e) => setSource({ ...source, branch: e.target.value })} /></Field><Field><FieldLabel htmlFor="source-path">Workspace path</FieldLabel><Input id="source-path" placeholder="sources/worker" value={source.workspace_path} onChange={(e) => setSource({ ...source, workspace_path: e.target.value })} /></Field></FieldGroup><Button size="sm" disabled={!!busy || !source.display_name || !source.remote_url || !source.workspace_path} onClick={() => void run("Add source", async () => { await createProjectSource(project.id, source); setSource({ display_name: "", remote_url: "", branch: "main", workspace_path: "" }); })}>Add source</Button>
      </CardContent></Card>
      <Card><CardHeader><CardTitle className="text-sm">Detected configuration</CardTitle></CardHeader><CardContent className="space-y-4"><div className="text-sm text-muted-foreground">{configuration.facts.map((fact) => `${fact.workspace_path}: ${fact.compose_file || "no Compose file"}${fact.dockerfile ? ", Dockerfile" : ""}${fact.nextjs ? ", Next.js" : ""}`).join(" · ") || "No source facts available."}</div>{configuration.parse_errors.map((item) => <p key={item} role="alert" className="text-sm text-destructive">{item}</p>)}{configuration.pending_compose && !configuration.accepted_compose && <Button size="sm" variant="outline" disabled={!!busy} onClick={() => void run("Accept detected configuration", () => saveProjectConfiguration(project.id, { accept_detected: true }))}>Accept detected Compose</Button>}{configuration.recommendation?.kind === "nextjs" && <Button size="sm" variant="outline" disabled={!!busy} onClick={() => void run("Apply Next.js template", () => saveProjectConfiguration(project.id, { apply_nextjs_template: true }))}>Apply Next.js template</Button>}<div className="h-80 overflow-hidden rounded-md border"><MonacoEditor language="yaml" theme="vs-dark" value={compose} onChange={(value) => setCompose(value || "")} options={{ minimap: { enabled: false }, automaticLayout: true, fontSize: 13 }} /></div><Button size="sm" disabled={!!busy || !compose.trim()} onClick={() => void run("Save Compose", () => saveProjectConfiguration(project.id, { compose_yaml: compose }))}>{busy === "Save Compose" ? "Saving..." : "Save Compose"}</Button><p className="text-xs text-muted-foreground">Environment values are owned by {configuration.environment_owner}.</p></CardContent></Card>
      <Card><CardHeader><CardTitle className="text-sm">Build and deploy</CardTitle></CardHeader><CardContent className="flex flex-wrap items-center gap-3"><Button disabled={!!busy || !configuration.build_permitted} onClick={() => void run("Build", () => buildProject(project.id))}>{busy === "Build" ? "Building..." : "Build"}</Button><Button disabled={!!busy || !canDeploy} onClick={() => void run("Deploy", () => deployProject(project.id))}>{busy === "Deploy" ? "Deploying..." : "Deploy"}</Button><span className="text-sm text-muted-foreground">{!configuration.build_permitted ? "Accept a valid configuration after all sources are ready to build." : !canDeploy ? "Deploy is enabled after a successful build." : "Build is current and ready to deploy."}</span></CardContent></Card>
      <Card><CardHeader><CardTitle className="text-sm">Service subdomains</CardTitle></CardHeader><CardContent className="space-y-3">{routes.map((item, index) => <div key={`${item.service}-${item.domain}-${index}`} className="flex items-center justify-between rounded border p-2 text-sm"><span><strong>{item.service}</strong> → {item.domain}</span><Button size="sm" variant="ghost" className="text-destructive" disabled={!!busy} onClick={() => setRoutes((current) => current.filter((_, currentIndex) => currentIndex !== index))}>Delete</Button></div>)}<FieldGroup className="sm:grid-cols-2"><Field><FieldLabel htmlFor="route-service">Service</FieldLabel><Input id="route-service" list="configure-services" value={route.service} onChange={(e) => setRoute({ ...route, service: e.target.value })} /><datalist id="configure-services">{configuration.services.map((item) => <option key={item.name} value={item.name} />)}</datalist></Field><Field><FieldLabel htmlFor="route-domain">Subdomain</FieldLabel><Input id="route-domain" placeholder="api.example.com" value={route.domain} onChange={(e) => setRoute({ ...route, domain: e.target.value })} /></Field></FieldGroup><div className="flex gap-2"><Button size="sm" variant="outline" disabled={!route.service || !route.domain || !!busy} onClick={addRoute}>Add route</Button><Button size="sm" disabled={!!busy} onClick={() => void run("Save routes", () => setProjectRoutes(project.id, routes))}>{busy === "Save routes" ? "Saving..." : "Save routes"}</Button></div></CardContent></Card>
    </>}
  </div>;
}
