"use client";

import { useEffect, useState, useCallback } from "react";
import { useRouter } from "next/navigation";
import { listCodebases, type Codebase } from "@/lib/api";
import { useAuth } from "@/lib/auth";
import { getShowSystem } from "@/lib/settings";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Empty, EmptyContent, EmptyDescription, EmptyHeader, EmptyMedia, EmptyTitle } from "@/components/ui/empty";
import { PageHeader, PageHeaderActions, PageHeaderDescription, PageHeaderTitle } from "@/components/page-header";
import { Skeleton } from "@/components/ui/skeleton";
import { AlertCircle, Code2, FolderOpen } from "lucide-react";

function CodeListLoading() {
  return (
    <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-3" aria-label="Loading codebases" aria-busy="true">
      {["cb-one", "cb-two", "cb-three"].map((key) => (
        <Skeleton key={key} className="h-32 rounded-xl" />
      ))}
    </div>
  );
}

export default function CodeListPage() {
  const [codebases, setCodebases] = useState<Codebase[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const { user, loading: authLoading } = useAuth();
  const router = useRouter();

  useEffect(() => {
    if (!authLoading && !user) router.replace("/login");
  }, [user, authLoading, router]);

  const fetch = useCallback(() => {
    if (!user) return;
    setLoading(true);
    setError("");
    listCodebases()
      .then((all) => {
        const showSystem = getShowSystem();
        if (!showSystem) {
          setCodebases(all.filter((cb) => cb.type !== "system"));
        } else {
          setCodebases(all);
        }
      })
      .catch((err: unknown) => {
        setError(err instanceof Error ? err.message : "Unable to load codebases");
      })
      .finally(() => setLoading(false));
  }, [user]);

  useEffect(fetch, [fetch]);

  if (authLoading || !user) return null;

  return (
    <main className="mx-auto w-full max-w-5xl space-y-6 p-4 sm:p-6 lg:p-8">
      <PageHeader>
        <div className="space-y-1">
          <PageHeaderTitle>Code</PageHeaderTitle>
          <PageHeaderDescription>Browse and open project and system codebases.</PageHeaderDescription>
        </div>
        <PageHeaderActions>
          <Badge variant="secondary">{codebases.length} codebase{codebases.length !== 1 ? "s" : ""}</Badge>
        </PageHeaderActions>
      </PageHeader>

      {loading ? (
        <CodeListLoading />
      ) : error ? (
        <Empty className="min-h-56 border-destructive/30">
          <EmptyHeader>
            <EmptyMedia className="bg-destructive/10 text-destructive">
              <AlertCircle className="size-5" aria-hidden="true" />
            </EmptyMedia>
            <EmptyTitle>Codebases could not be loaded</EmptyTitle>
            <EmptyDescription className="whitespace-pre-wrap">{error}</EmptyDescription>
          </EmptyHeader>
          <EmptyContent>
            <Button variant="outline" onClick={() => fetch()}>Try again</Button>
          </EmptyContent>
        </Empty>
      ) : codebases.length === 0 ? (
        <Empty className="min-h-56">
          <EmptyHeader>
            <EmptyMedia><FolderOpen className="size-5" aria-hidden="true" /></EmptyMedia>
            <EmptyTitle>No codebases available</EmptyTitle>
            <EmptyDescription>Codebases will appear here when projects with source code are added.</EmptyDescription>
          </EmptyHeader>
        </Empty>
      ) : (
        <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-3">
          {codebases.map((cb) => (
            <button
              key={`${cb.type}-${cb.id}`}
              className="text-left"
              onClick={() => router.push(`/code/${cb.id}`)}
            >
              <Card className="group h-full transition-colors hover:border-primary/50">
                <CardHeader className="flex-row items-start justify-between gap-3 space-y-0">
                  <div className="min-w-0 space-y-1">
                    <CardTitle className="truncate text-base">{cb.name}</CardTitle>
                  </div>
                  <Badge variant={cb.type === "system" ? "warning" : "secondary"}>
                    {cb.type === "system" ? "System" : "Project"}
                  </Badge>
                </CardHeader>
                <CardContent>
                  <div className="flex items-center gap-2 text-xs text-muted-foreground">
                    <Code2 className="size-3.5 shrink-0" aria-hidden="true" />
                    <span className="truncate font-mono">{cb.path}</span>
                  </div>
                </CardContent>
              </Card>
            </button>
          ))}
        </div>
      )}
    </main>
  );
}
