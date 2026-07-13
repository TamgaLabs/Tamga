"use client";

import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { Boxes, CloudDownload, FolderUp, Globe2, Server, X } from "lucide-react";

import { createProject } from "@/lib/api";
import { useAuth } from "@/lib/auth";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Field, FieldDescription, FieldError, FieldGroup, FieldLabel } from "@/components/ui/field";
import { InputGroup, InputGroupAddon, InputGroupInput } from "@/components/ui/input-group";
import { RadioGroup, RadioGroupItem } from "@/components/ui/radio-group";
import { Textarea } from "@/components/ui/textarea";
import { PageHeader, PageHeaderActions, PageHeaderDescription, PageHeaderTitle } from "@/components/page-header";
import { cn } from "@/lib/utils";

type SourceType = "local" | "remote" | "compose";

const COMPOSE_PLACEHOLDER = `services:
  web:
    image: nginx:latest
    ports:
      - "80:80"
`;

const sourceOptions: { value: SourceType; label: string; description: string; icon: typeof FolderUp }[] = [
  { value: "remote", label: "Repository", description: "Clone from a remote Git URL.", icon: CloudDownload },
  { value: "local", label: "Local", description: "Use a project already available locally.", icon: FolderUp },
  { value: "compose", label: "Compose", description: "Deploy a supported Compose stack.", icon: Boxes },
];

export default function NewProjectPage() {
  const [sourceType, setSourceType] = useState<SourceType>("remote");
  const [name, setName] = useState("");
  const [repoUrl, setRepoUrl] = useState("");
  const [domain, setDomain] = useState("");
  const [composeYaml, setComposeYaml] = useState("");
  const [exposedService, setExposedService] = useState("");
  const [error, setError] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const router = useRouter();
  const { user, loading: authLoading } = useAuth();

  useEffect(() => {
    if (!authLoading && !user) router.replace("/login");
  }, [user, authLoading, router]);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError("");
    setSubmitting(true);
    try {
      const project = await createProject({
        name,
        source_type: sourceType,
        repo_url: sourceType === "compose" ? "" : repoUrl,
        domain,
        ...(sourceType === "compose" ? { compose_yaml: composeYaml, ...(exposedService ? { exposed_service: exposedService } : {}) } : {}),
      });
      router.push(`/projects/${project.id}`);
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : "Failed to create project");
      setSubmitting(false);
    }
  };

  if (authLoading || !user) return null;

  return (
    <main className="mx-auto w-full max-w-3xl space-y-6 p-4 sm:p-6 lg:p-8">
      <PageHeader>
        <div className="space-y-1"><PageHeaderTitle>New Project</PageHeaderTitle><PageHeaderDescription>Choose a source, then configure how Tamga Console should expose it.</PageHeaderDescription></div>
        <PageHeaderActions><Button variant="ghost" onClick={() => router.back()}><X className="size-4" aria-hidden="true" />Cancel</Button></PageHeaderActions>
      </PageHeader>

      <Card>
        <CardHeader>
          <CardTitle>Project details</CardTitle>
          <CardDescription>All deployments need a name and a public domain.</CardDescription>
        </CardHeader>
        <CardContent>
          <form onSubmit={handleSubmit} className="space-y-7">
            <FieldGroup>
              <Field>
                <FieldLabel htmlFor="name">Project name</FieldLabel>
                <InputGroup><InputGroupAddon aria-hidden="true"><Server className="size-4" /></InputGroupAddon><InputGroupInput id="name" value={name} onChange={(e) => setName(e.target.value)} placeholder="my-app" required /></InputGroup>
              </Field>

              <Field>
                <FieldLabel>Source</FieldLabel>
                <FieldDescription>Select where Tamga should get your application.</FieldDescription>
                <RadioGroup value={sourceType} onValueChange={(value) => { const next = value as SourceType; setSourceType(next); if (next === "local") setRepoUrl(""); }} className="grid gap-3 sm:grid-cols-3">
                  {sourceOptions.map(({ value, label, description, icon: Icon }) => (
                    <label key={value} htmlFor={value} className={cn("flex cursor-pointer gap-3 rounded-lg border p-4 transition-colors hover:bg-accent/50", sourceType === value && "border-primary bg-primary/5") }>
                      <RadioGroupItem value={value} id={value} className="mt-0.5 shrink-0" />
                      <span className="space-y-1"><span className="flex items-center gap-2 text-sm font-medium"><Icon className="size-4" aria-hidden="true" />{label}</span><span className="block text-xs leading-5 text-muted-foreground">{description}</span></span>
                    </label>
                  ))}
                </RadioGroup>
              </Field>

              {sourceType === "remote" && (
                <Field>
                  <FieldLabel htmlFor="repo_url">Repository URL</FieldLabel>
                  <InputGroup><InputGroupAddon>git</InputGroupAddon><InputGroupInput id="repo_url" value={repoUrl} onChange={(e) => setRepoUrl(e.target.value)} placeholder="https://github.com/user/repo.git" autoComplete="url" required /></InputGroup>
                </Field>
              )}

              {sourceType === "compose" && <>
                <Field>
                  <FieldLabel htmlFor="compose_yaml">docker-compose.yml</FieldLabel>
                  <Textarea id="compose_yaml" value={composeYaml} onChange={(e) => setComposeYaml(e.target.value)} placeholder={COMPOSE_PLACEHOLDER} rows={12} required />
                  <FieldDescription>A subset of Compose is supported: image, ports, environment, volumes, networks, and depends_on. build:, profiles:, secrets:, and healthcheck: are not supported.</FieldDescription>
                </Field>
                <Field>
                  <FieldLabel htmlFor="exposed_service">Exposed service <span className="font-normal text-muted-foreground">(optional)</span></FieldLabel>
                  <InputGroup><InputGroupAddon aria-hidden="true"><Globe2 className="size-4" /></InputGroupAddon><InputGroupInput id="exposed_service" value={exposedService} onChange={(e) => setExposedService(e.target.value)} placeholder="web" /></InputGroup>
                  <FieldDescription>Leave blank to let Tamga select the service automatically.</FieldDescription>
                </Field>
              </>}

              <Field>
                <FieldLabel htmlFor="domain">Domain</FieldLabel>
                <InputGroup><InputGroupAddon aria-hidden="true"><Globe2 className="size-4" /></InputGroupAddon><InputGroupInput id="domain" value={domain} onChange={(e) => setDomain(e.target.value)} placeholder="my-app.example.com" autoComplete="url" required /></InputGroup>
              </Field>
            </FieldGroup>

            {error && <FieldError className="rounded-md border border-destructive/30 bg-destructive/5 p-3 whitespace-pre-wrap">{error}</FieldError>}
            <div className="flex flex-col-reverse gap-3 border-t pt-5 sm:flex-row sm:justify-end"><Button type="button" variant="outline" onClick={() => router.back()}>Cancel</Button><Button type="submit" disabled={submitting}>{submitting ? "Creating..." : "Create & Deploy"}</Button></div>
          </form>
        </CardContent>
      </Card>
    </main>
  );
}
