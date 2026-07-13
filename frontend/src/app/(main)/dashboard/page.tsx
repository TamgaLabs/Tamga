"use client";

import { useEffect, useMemo, useState } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { AlertCircle, ArrowUpRight, FolderKanban, Globe2, Plus } from "lucide-react";

import { listProjects, type Project } from "@/lib/api";
import { useAuth } from "@/lib/auth";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Empty, EmptyContent, EmptyDescription, EmptyHeader, EmptyMedia, EmptyTitle } from "@/components/ui/empty";
import { PageHeader, PageHeaderActions, PageHeaderDescription, PageHeaderTitle } from "@/components/page-header";
import { Skeleton } from "@/components/ui/skeleton";

const statusVariant: Record<string, "success" | "warning" | "error" | "info" | "default"> = {
  running: "success",
  building: "warning",
  cloning: "info",
  created: "info",
  error: "error",
};

function ProjectSummary({ label, value, description }: { label: string; value: number; description: string }) {
  return (
    <Card>
      <CardHeader className="space-y-1 pb-3">
        <CardDescription>{label}</CardDescription>
        <CardTitle className="text-2xl tabular-nums">{value}</CardTitle>
      </CardHeader>
      <CardContent className="text-xs text-muted-foreground">{description}</CardContent>
    </Card>
  );
}

function DashboardLoading() {
  return (
    <div className="space-y-6" aria-label="Loading projects" aria-busy="true">
      <div className="grid gap-4 sm:grid-cols-3">
        {["summary-one", "summary-two", "summary-three"].map((key) => <Skeleton key={key} className="h-28" />)}
      </div>
      <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-3">
        {["project-one", "project-two", "project-three"].map((key) => <Skeleton key={key} className="h-40" />)}
      </div>
    </div>
  );
}

export default function DashboardPage() {
  const [projects, setProjects] = useState<Project[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const { user, loading: authLoading } = useAuth();
  const router = useRouter();

  useEffect(() => {
    if (!authLoading && !user) router.replace("/login");
  }, [user, authLoading, router]);

  useEffect(() => {
    if (!user) return;
    setLoading(true);
    setError("");
    listProjects()
      .then(setProjects)
      .catch((err: unknown) => {
        setError(err instanceof Error ? err.message : "Unable to load projects");
      })
      .finally(() => setLoading(false));
  }, [user]);

  const summary = useMemo(() => ({
    running: projects.filter((project) => project.status === "running").length,
    attention: projects.filter((project) => project.status === "error").length,
  }), [projects]);

  if (authLoading || !user) return null;

  return (
    <main className="mx-auto w-full max-w-7xl space-y-6 p-4 sm:p-6 lg:p-8">
      <PageHeader>
        <div className="space-y-1">
          <PageHeaderTitle>Projects</PageHeaderTitle>
          <PageHeaderDescription>Deploy, monitor, and operate your applications.</PageHeaderDescription>
        </div>
        <PageHeaderActions>
          <Button onClick={() => router.push("/dashboard/new")}>
            <Plus className="size-4" aria-hidden="true" />
            New project
          </Button>
        </PageHeaderActions>
      </PageHeader>

      {loading ? <DashboardLoading /> : error ? (
        <Empty className="min-h-72 border-destructive/30">
          <EmptyHeader>
            <EmptyMedia className="bg-destructive/10 text-destructive"><AlertCircle className="size-5" aria-hidden="true" /></EmptyMedia>
            <EmptyTitle>Projects could not be loaded</EmptyTitle>
            <EmptyDescription className="whitespace-pre-wrap">{error}</EmptyDescription>
          </EmptyHeader>
          <EmptyContent><Button variant="outline" onClick={() => window.location.reload()}>Try again</Button></EmptyContent>
        </Empty>
      ) : projects.length === 0 ? (
        <Empty className="min-h-72">
          <EmptyHeader>
            <EmptyMedia><FolderKanban className="size-5" aria-hidden="true" /></EmptyMedia>
            <EmptyTitle>No projects yet</EmptyTitle>
            <EmptyDescription>Create your first project to deploy it from a repository or Compose file.</EmptyDescription>
          </EmptyHeader>
          <EmptyContent><Button onClick={() => router.push("/dashboard/new")}><Plus className="size-4" aria-hidden="true" />New project</Button></EmptyContent>
        </Empty>
      ) : (
        <div className="space-y-6">
          <section className="grid gap-4 sm:grid-cols-3" aria-label="Project summary">
            <ProjectSummary label="Total projects" value={projects.length} description="Across all sources" />
            <ProjectSummary label="Running" value={summary.running} description="Available and serving traffic" />
            <ProjectSummary label="Needs attention" value={summary.attention} description="Projects reporting an error" />
          </section>
          <section className="space-y-3" aria-labelledby="project-list-title">
            <div className="flex items-center justify-between"><h2 id="project-list-title" className="text-base font-semibold">All projects</h2><span className="text-sm text-muted-foreground">{projects.length} total</span></div>
            <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-3">
              {projects.map((project) => (
                <Link
                  key={project.id}
                  href={`/projects/${project.id}`}
                  aria-label={`Open project ${project.name}`}
                  className="block rounded-xl focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2"
                >
                  <Card className="group flex min-h-44 flex-col transition-colors hover:border-primary/50">
                    <CardHeader className="flex-row items-start justify-between gap-3 space-y-0">
                      <div className="min-w-0 space-y-1">
                        <CardTitle className="truncate text-base">{project.name}</CardTitle>
                        <CardDescription className="truncate">{project.repo_url || "Compose deployment"}</CardDescription>
                      </div>
                      <Badge variant={statusVariant[project.status] || "default"}>{project.status}</Badge>
                    </CardHeader>
                    <CardContent className="mt-auto space-y-4">
                      {project.domain && <div className="flex min-w-0 items-center gap-2 text-sm text-muted-foreground"><Globe2 className="size-4 shrink-0" aria-hidden="true" /><span className="truncate">{project.domain}</span></div>}
                      <span className="flex h-9 w-full items-center justify-between rounded-lg border border-border bg-background px-4 text-sm font-medium transition-colors group-hover:bg-accent group-hover:text-accent-foreground">
                        Open project <ArrowUpRight className="size-4" aria-hidden="true" />
                      </span>
                    </CardContent>
                  </Card>
                </Link>
              ))}
            </div>
          </section>
        </div>
      )}
    </main>
  );
}
