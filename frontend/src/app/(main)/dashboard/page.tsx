"use client";

import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { listProjects, type Project } from "@/lib/api";
import { useAuth } from "@/lib/auth";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";

const statusVariant: Record<string, "success" | "warning" | "error" | "info" | "default"> = {
  running: "success",
  building: "warning",
  cloning: "info",
  created: "info",
  error: "error",
};

export default function DashboardPage() {
  const [projects, setProjects] = useState<Project[]>([]);
  const [loading, setLoading] = useState(true);
  const { user, loading: authLoading } = useAuth();
  const router = useRouter();

  useEffect(() => {
    if (!authLoading && !user) router.replace("/login");
  }, [user, authLoading, router]);

  useEffect(() => {
    if (user) {
      listProjects()
        .then(setProjects)
        .catch(console.error)
        .finally(() => setLoading(false));
    }
  }, [user]);

  if (authLoading || !user) return null;

  return (
    <div className="min-h-screen p-6 max-w-6xl mx-auto">
      <div className="flex items-center justify-between mb-8">
        <h1 className="text-2xl font-bold">Projects</h1>
        <Button onClick={() => router.push("/dashboard/new")}>New Project</Button>
      </div>

      {loading ? (
        <div className="text-muted-foreground text-center py-20">Loading...</div>
      ) : projects.length === 0 ? (
        <div className="text-muted-foreground text-center py-20">
          <p className="text-lg mb-2">No projects yet</p>
          <p className="text-sm">Create your first project to get started</p>
        </div>
      ) : (
        <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-3">
          {projects.map((p) => (
            <Card
              key={p.id}
              className="cursor-pointer hover:border-muted-foreground transition-colors"
              onClick={() => router.push(`/projects/${p.id}`)}
            >
              <CardHeader>
                <div className="flex items-center justify-between">
                  <CardTitle className="text-base">{p.name}</CardTitle>
                  <Badge variant={statusVariant[p.status] || "default"}>{p.status}</Badge>
                </div>
              </CardHeader>
              <CardContent>
                <div className="space-y-1 text-sm text-muted-foreground">
                  <p className="truncate">{p.repo_url}</p>
                  {p.domain && <p className="text-accent">{p.domain}</p>}
                </div>
              </CardContent>
            </Card>
          ))}
        </div>
      )}
    </div>
  );
}
