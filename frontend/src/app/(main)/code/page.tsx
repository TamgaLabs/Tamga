"use client";

import { useEffect, useState, useCallback } from "react";
import { useRouter } from "next/navigation";
import { listCodebases, type Codebase } from "@/lib/api";
import { useAuth } from "@/lib/auth";
import { getShowSystem } from "@/lib/settings";
import { Card, CardContent, CardTitle } from "@/components/ui/card";

export default function CodeListPage() {
  const [codebases, setCodebases] = useState<Codebase[]>([]);
  const [loading, setLoading] = useState(true);
  const { user, loading: authLoading } = useAuth();
  const router = useRouter();

  useEffect(() => {
    if (!authLoading && !user) router.replace("/login");
  }, [user, authLoading, router]);

  const fetch = useCallback(() => {
    if (!user) return;
    setLoading(true);
    listCodebases()
      .then((all) => {
        const showSystem = getShowSystem();
        if (!showSystem) {
          setCodebases(all.filter((cb) => cb.type !== "system"));
        } else {
          setCodebases(all);
        }
      })
      .catch(console.error)
      .finally(() => setLoading(false));
  }, [user]);

  useEffect(fetch, [fetch]);

  if (authLoading || !user) return null;

  return (
    <div className="p-6 max-w-4xl mx-auto">
      <h1 className="text-2xl font-bold mb-6">Code</h1>

      {loading ? (
        <p className="text-muted-foreground">Loading...</p>
      ) : codebases.length === 0 ? (
        <p className="text-muted-foreground">No codebases available.</p>
      ) : (
        <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-3">
          {codebases.map((cb) => (
            <Card
              key={`${cb.type}-${cb.id}`}
              className="cursor-pointer hover:bg-muted/50 transition-colors"
              onClick={() => router.push(`/code/${cb.id}`)}
            >
              <CardContent className="p-4">
                <CardTitle className="text-sm mb-2">{cb.name}</CardTitle>
                <p className="text-xs text-muted-foreground">{cb.type === "system" ? "System" : "Project"}</p>
                <p className="text-xs text-muted-foreground mt-1 font-mono truncate">{cb.path}</p>
              </CardContent>
            </Card>
          ))}
        </div>
      )}
    </div>
  );
}
