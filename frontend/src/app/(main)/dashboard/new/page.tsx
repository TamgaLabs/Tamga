"use client";

import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { CloudDownload, FolderPlus, Globe2, Plus, Server, X } from "lucide-react";

import { createSeal, createSealRepository } from "@/lib/api";
import { useAuth } from "@/lib/auth";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Field, FieldDescription, FieldError, FieldGroup, FieldLabel } from "@/components/ui/field";
import { InputGroup, InputGroupAddon, InputGroupInput } from "@/components/ui/input-group";
import { RadioGroup, RadioGroupItem } from "@/components/ui/radio-group";

import { PageHeader, PageHeaderActions, PageHeaderDescription, PageHeaderTitle } from "@/components/page-header";
import { useWorkspace } from "@/contexts/workspace-context";
import { cn } from "@/lib/utils";

type CreationMode = "empty" | "repository";

const creationOptions: { value: CreationMode; label: string; description: string; icon: typeof FolderPlus }[] = [
  { value: "empty", label: "Empty Seal", description: "Create a workspace now and add repositories or services later.", icon: FolderPlus },
  { value: "repository", label: "From repositories", description: "Create a Seal and clone its first Git repository.", icon: CloudDownload },
];

function validationErrors(mode: CreationMode, name: string, repositoryURL: string, branch: string) {
  const errors: Record<string, string> = {};
  if (!name.trim()) errors.name = "Seal name is required.";
  if (mode === "repository") {
    if (!repositoryURL.trim()) errors.repositoryURL = "Repository URL is required.";
    if (!branch.trim()) errors.branch = "Branch is required.";
    if (name.trim() && !/^[a-zA-Z0-9._-]+$/.test(name.trim())) errors.name = "Repository-backed Seal names may contain letters, numbers, periods, hyphens, and underscores only.";
  }
  return errors;
}

export default function NewSealPage() {
  const [creationMode, setCreationMode] = useState<CreationMode>("empty");
  const [name, setName] = useState("");
  const [repositoryURL, setRepositoryURL] = useState("");
  const [branch, setBranch] = useState("main");
  const [domain, setDomain] = useState("");
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [outcome, setOutcome] = useState("");
  const [error, setError] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [createdSealID, setCreatedSealID] = useState<number | null>(null);
  const router = useRouter();
  const { user, loading: authLoading } = useAuth();
  const { addSeal } = useWorkspace();

  useEffect(() => {
    if (!authLoading && !user) router.replace("/login");
  }, [user, authLoading, router]);

  const resetForAnotherSeal = () => {
    setName("");
    setRepositoryURL("");
    setBranch("main");
    setDomain("");
    setErrors({});
    setOutcome("");
    setError("");
    setCreatedSealID(null);
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError("");
    setOutcome("");
    setCreatedSealID(null);
    const nextErrors = validationErrors(creationMode, name, repositoryURL, branch);
    setErrors(nextErrors);
    if (Object.keys(nextErrors).length) return;
    setSubmitting(true);
    try {
      const seal = await createSeal({ name: name.trim(), ...(domain.trim() ? { domain: domain.trim() } : {}) });
      addSeal(seal);
      setCreatedSealID(seal.id);
      if (creationMode === "repository") {
        await createSealRepository(seal.id, { display_name: name.trim(), remote_url: repositoryURL.trim(), branch: branch.trim() });
        setOutcome("Repository added. Next, select services for this Seal after the repository finishes cloning.");
      } else {
        setOutcome("Empty Seal created. Add a repository or define services when you are ready.");
      }
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : "Failed to create Seal.");
    } finally {
      setSubmitting(false);
    }
  };

  if (authLoading || !user) return null;

  return (
    <main className="mx-auto w-full max-w-3xl space-y-6 p-4 sm:p-6 lg:p-8">
      <PageHeader>
        <div className="space-y-1"><PageHeaderTitle>New Seal</PageHeaderTitle><PageHeaderDescription>Start with an empty workspace or attach a repository to configure services next.</PageHeaderDescription></div>
        <PageHeaderActions><Button variant="ghost" onClick={() => router.back()}><X className="size-4" aria-hidden="true" />Cancel</Button></PageHeaderActions>
      </PageHeader>

      <Card>
        <CardHeader>
          <CardTitle>Seal details</CardTitle>
          <CardDescription>Choose how to start. You can add services after repository details are saved.</CardDescription>
        </CardHeader>
        <CardContent>
          <form onSubmit={handleSubmit} className="space-y-7">
            <FieldGroup>
              <Field>
                <FieldLabel htmlFor="name">Seal name</FieldLabel>
                <InputGroup><InputGroupAddon aria-hidden="true"><Server className="size-4" /></InputGroupAddon><InputGroupInput id="name" value={name} onChange={(e) => { setName(e.target.value); setErrors((current) => ({ ...current, name: "" })); }} placeholder="my-app" aria-invalid={!!errors.name} aria-describedby={errors.name ? "name-error" : undefined} required /></InputGroup>
                {errors.name && <FieldError id="name-error">{errors.name}</FieldError>}
              </Field>

              <Field>
                <FieldLabel>Creation mode</FieldLabel>
                <FieldDescription>Select whether to create an empty Seal or begin with a repository.</FieldDescription>
                <RadioGroup value={creationMode} onValueChange={(value) => { setCreationMode(value as CreationMode); setErrors({}); }} className="grid gap-3 sm:grid-cols-2" aria-label="Seal creation mode">
                  {creationOptions.map(({ value, label, description, icon: Icon }) => (
                    <label key={value} htmlFor={value} className={cn("flex cursor-pointer gap-3 rounded-lg border p-4 transition-colors hover:bg-accent/50", creationMode === value && "border-primary bg-primary/5") }>
                      <RadioGroupItem value={value} id={value} className="mt-0.5 shrink-0" />
                      <span className="space-y-1"><span className="flex items-center gap-2 text-sm font-medium"><Icon className="size-4" aria-hidden="true" />{label}</span><span className="block text-xs leading-5 text-muted-foreground">{description}</span></span>
                    </label>
                  ))}
                </RadioGroup>
              </Field>

              {creationMode === "repository" && <>
                <Field>
                  <FieldLabel htmlFor="repository-url">Repository URL</FieldLabel>
                  <InputGroup><InputGroupAddon>git</InputGroupAddon><InputGroupInput id="repository-url" value={repositoryURL} onChange={(e) => { setRepositoryURL(e.target.value); setErrors((current) => ({ ...current, repositoryURL: "" })); }} placeholder="https://github.com/user/repo.git" autoComplete="url" aria-invalid={!!errors.repositoryURL} aria-describedby={errors.repositoryURL ? "repository-url-error" : undefined} required /></InputGroup>
                  {errors.repositoryURL && <FieldError id="repository-url-error">{errors.repositoryURL}</FieldError>}
                </Field>
                <Field>
                  <FieldLabel htmlFor="branch">Branch</FieldLabel>
                  <InputGroup><InputGroupAddon>git</InputGroupAddon><InputGroupInput id="branch" value={branch} onChange={(e) => { setBranch(e.target.value); setErrors((current) => ({ ...current, branch: "" })); }} placeholder="main" aria-invalid={!!errors.branch} aria-describedby={errors.branch ? "branch-error" : undefined} required /></InputGroup>
                  {errors.branch && <FieldError id="branch-error">{errors.branch}</FieldError>}
                  <FieldDescription>After this repository is saved, select the services it should provide.</FieldDescription>
                </Field>
              </>}

              <Field>
                <FieldLabel htmlFor="domain">Domain <span className="font-normal text-muted-foreground">(optional)</span></FieldLabel>
                <InputGroup><InputGroupAddon aria-hidden="true"><Globe2 className="size-4" /></InputGroupAddon><InputGroupInput id="domain" value={domain} onChange={(e) => setDomain(e.target.value)} placeholder="my-app.example.com" autoComplete="url" /></InputGroup>
              </Field>
            </FieldGroup>

            {error && <FieldError role="alert" className="rounded-md border border-destructive/30 bg-destructive/5 p-3 whitespace-pre-wrap">{error}</FieldError>}
            {outcome && <p role="status" className="rounded-md border border-success/30 bg-success/5 p-3 text-sm text-success">{outcome}</p>}
            <div className="flex flex-col-reverse gap-3 border-t pt-5 sm:flex-row sm:justify-end"><Button type="button" variant="outline" onClick={() => router.back()}>Cancel</Button>{createdSealID && <><Button type="button" variant="outline" onClick={() => router.push(`/seals/${createdSealID}/configure`)}>Configure Seal</Button><Button type="button" variant="outline" onClick={resetForAnotherSeal}><Plus className="size-4" aria-hidden="true" />Add another Seal</Button></>}<Button type="submit" disabled={submitting}>{submitting ? "Creating..." : creationMode === "repository" ? "Create Seal & add repository" : "Create Empty Seal"}</Button></div>
          </form>
        </CardContent>
      </Card>
    </main>
  );
}
