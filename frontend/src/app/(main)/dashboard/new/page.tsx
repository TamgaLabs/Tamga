"use client";

import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { createProject } from "@/lib/api";
import { useAuth } from "@/lib/auth";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { RadioGroup, RadioGroupItem } from "@/components/ui/radio-group";

type SourceType = "local" | "remote" | "compose";

const COMPOSE_PLACEHOLDER = `services:
  web:
    image: nginx:latest
    ports:
      - "80:80"
`;

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
        ...(sourceType === "compose"
          ? {
              compose_yaml: composeYaml,
              ...(exposedService ? { exposed_service: exposedService } : {}),
            }
          : {}),
      });
      router.push(`/projects/${project.id}`);
    } catch (err: unknown) {
      // The backend validates compose_yaml/exposed_service synchronously
      // on create (FEAT-027's parse errors, e.g. "build: not supported")
      // and returns the message as plain text - surfaced here as-is so
      // the user sees exactly why the compose was rejected.
      setError(err instanceof Error ? err.message : "Failed to create project");
      setSubmitting(false);
    }
  };

  if (authLoading || !user) return null;

  return (
    <div className="min-h-screen p-6 max-w-2xl mx-auto">
      <Button variant="ghost" onClick={() => router.back()} className="mb-4">
        &larr; Back
      </Button>
      <Card>
        <CardHeader>
          <CardTitle>New Project</CardTitle>
        </CardHeader>
        <CardContent>
          <form onSubmit={handleSubmit} className="space-y-4">
            <div className="space-y-2">
              <Label htmlFor="name">Project Name</Label>
              <Input
                id="name"
                value={name}
                onChange={(e) => setName(e.target.value)}
                placeholder="my-app"
                required
              />
            </div>

            <div className="space-y-2">
              <Label>Source</Label>
              <RadioGroup
                value={sourceType}
                onValueChange={(v) => {
                  setSourceType(v as SourceType);
                  if (v === "local") setRepoUrl("");
                }}
                className="flex gap-4"
              >
                <div className="flex items-center gap-2">
                  <RadioGroupItem value="local" id="local" />
                  <Label htmlFor="local">Local</Label>
                </div>
                <div className="flex items-center gap-2">
                  <RadioGroupItem value="remote" id="remote" />
                  <Label htmlFor="remote">Remote</Label>
                </div>
                <div className="flex items-center gap-2">
                  <RadioGroupItem value="compose" id="compose" />
                  <Label htmlFor="compose">Compose</Label>
                </div>
              </RadioGroup>
            </div>

            {sourceType === "remote" && (
              <div className="space-y-2">
                <Label htmlFor="repo_url">Repository URL</Label>
                <Input
                  id="repo_url"
                  value={repoUrl}
                  onChange={(e) => setRepoUrl(e.target.value)}
                  placeholder="https://github.com/user/repo.git"
                  required
                />
              </div>
            )}

            {sourceType === "compose" && (
              <>
                <div className="space-y-2">
                  <Label htmlFor="compose_yaml">docker-compose.yml</Label>
                  <Textarea
                    id="compose_yaml"
                    value={composeYaml}
                    onChange={(e) => setComposeYaml(e.target.value)}
                    placeholder={COMPOSE_PLACEHOLDER}
                    rows={12}
                    required
                  />
                  <p className="text-xs text-muted-foreground">
                    A subset of compose is supported: image, ports, environment,
                    volumes, networks, depends_on. build:, profiles:, secrets:
                    and healthcheck: are not supported.
                  </p>
                </div>

                <div className="space-y-2">
                  <Label htmlFor="exposed_service">Exposed Service (optional)</Label>
                  <Input
                    id="exposed_service"
                    value={exposedService}
                    onChange={(e) => setExposedService(e.target.value)}
                    placeholder="web"
                  />
                  <p className="text-xs text-muted-foreground">
                    Which service the domain below should route to. Leave blank
                    to let Tamga pick it automatically (a single service, or the
                    one service that declares a port).
                  </p>
                </div>
              </>
            )}

            <div className="space-y-2">
              <Label htmlFor="domain">Domain</Label>
              <Input
                id="domain"
                value={domain}
                onChange={(e) => setDomain(e.target.value)}
                placeholder="my-app.example.com"
                required
              />
            </div>

            {error && <p className="text-sm text-destructive whitespace-pre-wrap">{error}</p>}
            <Button type="submit" disabled={submitting}>
              {submitting ? "Creating..." : "Create & Deploy"}
            </Button>
          </form>
        </CardContent>
      </Card>
    </div>
  );
}
