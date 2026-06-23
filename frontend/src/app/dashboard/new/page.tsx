"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { createProject } from "@/lib/api";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";

export default function NewProjectPage() {
  const [name, setName] = useState("");
  const [repoUrl, setRepoUrl] = useState("");
  const [domain, setDomain] = useState("");
  const [error, setError] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const router = useRouter();

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError("");
    setSubmitting(true);
    try {
      const project = await createProject({ name, repo_url: repoUrl, domain });
      router.push(`/projects/${project.id}`);
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : "Failed to create project");
      setSubmitting(false);
    }
  };

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
              <label className="text-sm text-neutral-400">Project Name</label>
              <Input
                value={name}
                onChange={(e) => setName(e.target.value)}
                placeholder="my-app"
                required
              />
            </div>
            <div className="space-y-2">
              <label className="text-sm text-neutral-400">Repository URL</label>
              <Input
                value={repoUrl}
                onChange={(e) => setRepoUrl(e.target.value)}
                placeholder="https://github.com/user/repo.git"
                required
              />
            </div>
            <div className="space-y-2">
              <label className="text-sm text-neutral-400">Domain</label>
              <Input
                value={domain}
                onChange={(e) => setDomain(e.target.value)}
                placeholder="my-app.example.com"
                required
              />
            </div>
            {error && <p className="text-sm text-red-400">{error}</p>}
            <Button type="submit" disabled={submitting}>
              {submitting ? "Creating..." : "Create & Deploy"}
            </Button>
          </form>
        </CardContent>
      </Card>
    </div>
  );
}
