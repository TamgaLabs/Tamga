"use client";

import { useEffect, useState } from "react";
import { useParams, useRouter } from "next/navigation";
import { getProject, deleteProject, restartProject, type Project } from "@/lib/api";
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

export default function ProjectDetailPage() {
  const params = useParams();
  const router = useRouter();
  const [project, setProject] = useState<Project | null>(null);
  const [tab, setTab] = useState<"overview" | "settings" | "agent">("overview");
  const { user, loading: authLoading } = useAuth();

  useEffect(() => {
    if (!authLoading && !user) router.replace("/login");
  }, [user, authLoading, router]);

  useEffect(() => {
    if (user && params.id) {
      getProject(Number(params.id))
        .then(setProject)
        .catch(console.error);
    }
  }, [user, params.id]);

  const handleRestart = async () => {
    if (!project) return;
    await restartProject(project.id);
    getProject(project.id).then(setProject).catch(console.error);
  };

  const handleRedeploy = async () => {
    if (!project) return;
    await restartProject(project.id);
    getProject(project.id).then(setProject).catch(console.error);
  };

  const handleDelete = async () => {
    if (!project || !confirm("Delete this project?")) return;
    await deleteProject(project.id);
    router.push("/dashboard");
  };

  if (authLoading || !user || !project) return null;

  const tabs = [
    { id: "overview" as const, label: "Overview" },
    { id: "settings" as const, label: "Settings" },
    { id: "agent" as const, label: "Agent" },
  ];

  return (
    <div className="min-h-screen p-6 max-w-5xl mx-auto">
      <Button variant="ghost" onClick={() => router.push("/dashboard")} className="mb-4">
        &larr; Dashboard
      </Button>

      <div className="flex items-center justify-between mb-6">
        <div>
          <h1 className="text-2xl font-bold">{project.name}</h1>
          <p className="text-sm text-neutral-400 mt-1">{project.repo_url}</p>
        </div>
        <Badge variant={statusVariant[project.status] || "default"}>{project.status}</Badge>
      </div>

      <div className="flex gap-1 mb-6 border-b border-neutral-800">
        {tabs.map((t) => (
          <button
            key={t.id}
            onClick={() => setTab(t.id)}
            className={`px-4 py-2 text-sm font-medium transition-colors ${
              tab === t.id
                ? "border-b-2 border-white text-white"
                : "text-neutral-500 hover:text-neutral-300"
            }`}
          >
            {t.label}
          </button>
        ))}
      </div>

      {tab === "overview" && (
        <div className="grid gap-4 md:grid-cols-2">
          <Card>
            <CardHeader>
              <CardTitle className="text-sm">Details</CardTitle>
            </CardHeader>
            <CardContent className="text-sm space-y-2 text-neutral-400">
              <div className="flex justify-between">
                <span>Domain</span>
                <span className="text-blue-400">{project.domain || "-"}</span>
              </div>
              <div className="flex justify-between">
                <span>Branch</span>
                <span>{project.branch}</span>
              </div>
              <div className="flex justify-between">
                <span>Container</span>
                <span className="font-mono text-xs">{project.container_id?.slice(0, 12) || "-"}</span>
              </div>
              <div className="flex justify-between">
                <span>Created</span>
                <span>{new Date(project.created_at).toLocaleDateString()}</span>
              </div>
            </CardContent>
          </Card>

          <Card>
            <CardHeader>
              <CardTitle className="text-sm">Actions</CardTitle>
            </CardHeader>
            <CardContent className="space-y-2">
              <Button variant="outline" size="sm" className="w-full" onClick={handleRestart}>
                Restart
              </Button>
              <Button variant="outline" size="sm" className="w-full" onClick={handleRedeploy}>
                Redeploy
              </Button>
              <Button variant="destructive" size="sm" className="w-full" onClick={handleDelete}>
                Delete
              </Button>
            </CardContent>
          </Card>
        </div>
      )}

      {tab === "settings" && (
        <Card>
          <CardHeader>
            <CardTitle className="text-sm">Settings</CardTitle>
          </CardHeader>
          <CardContent>
            <p className="text-sm text-neutral-500">Settings will be available in the next update.</p>
          </CardContent>
        </Card>
      )}

      {tab === "agent" && (
        <Card>
          <CardHeader>
            <CardTitle className="text-sm">AI Agent</CardTitle>
          </CardHeader>
          <CardContent>
            <p className="text-sm text-neutral-500">Agent chat will be available in the next update.</p>
          </CardContent>
        </Card>
      )}
    </div>
  );
}
