"use client";

import { useCallback, useEffect, useState } from "react";
import {
  listDeployments,
  listContainers,
  startContainer,
  stopContainer,
  restartContainer,
  type Deployment,
  type ContainerInfo,
} from "@/lib/api";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { useProjectContext } from "./project-context";
import { ContainerRow } from "./container-row";

const statusVariant: Record<string, "success" | "warning" | "error" | "info" | "default"> = {
  running: "success",
  building: "warning",
  cloning: "info",
  created: "info",
  error: "error",
};

export default function ProjectOverviewPage() {
  const { project } = useProjectContext();
  const [deployments, setDeployments] = useState<Deployment[]>([]);
  const [containers, setContainers] = useState<ContainerInfo[]>([]);
  const [containersLoading, setContainersLoading] = useState(true);

  useEffect(() => {
    listDeployments(project.id).then(setDeployments).catch(console.error);
  }, [project.id]);

  const fetchContainers = useCallback(() => {
    setContainersLoading(true);
    listContainers()
      .then((all) => setContainers(all.filter((c) => c.project_id === project.id)))
      .catch(console.error)
      .finally(() => setContainersLoading(false));
  }, [project.id]);

  useEffect(fetchContainers, [fetchContainers]);

  const handleContainerAction = async (id: string, action: "start" | "stop" | "restart") => {
    try {
      if (action === "start") await startContainer(id);
      else if (action === "stop") await stopContainer(id);
      else await restartContainer(id);
      fetchContainers();
    } catch (e) {
      console.error(e);
    }
  };

  return (
    <div className="p-6 max-w-5xl mx-auto">
      <div className="flex items-center justify-between mb-6">
        <div>
          <h1 className="text-2xl font-bold">{project.name}</h1>
          <p className="text-sm text-muted-foreground mt-1">{project.repo_url}</p>
        </div>
        <Badge variant={statusVariant[project.status] || "default"}>{project.status}</Badge>
      </div>

      <div className="grid gap-4 md:grid-cols-2">
        <Card>
          <CardHeader>
            <CardTitle className="text-sm">Details</CardTitle>
          </CardHeader>
          <CardContent className="text-sm space-y-2 text-muted-foreground">
            <div className="flex justify-between">
              <span>Domain</span>
              <span className="text-accent">{project.domain || "-"}</span>
            </div>
            {project.compose_yaml ? (
              <div className="flex justify-between">
                <span>Exposed Service</span>
                <span className="font-mono text-xs">{project.exposed_service || "-"}</span>
              </div>
            ) : (
              <div className="flex justify-between">
                <span>Branch</span>
                <span>{project.branch}</span>
              </div>
            )}
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
            <CardTitle className="text-sm">Containers</CardTitle>
          </CardHeader>
          <CardContent>
            {containersLoading ? (
              <p className="text-sm text-muted-foreground">Loading...</p>
            ) : containers.length === 0 ? (
              <p className="text-sm text-muted-foreground">No containers for this project.</p>
            ) : (
              <div className="space-y-2">
                {containers.map((c) => (
                  <ContainerRow key={c.id} container={c} onAction={handleContainerAction} />
                ))}
              </div>
            )}
          </CardContent>
        </Card>

        <Card className="md:col-span-2">
          <CardHeader>
            <CardTitle className="text-sm">Deployments</CardTitle>
          </CardHeader>
          <CardContent>
            {deployments.length === 0 ? (
              <p className="text-sm text-muted-foreground">No deployments yet.</p>
            ) : (
              <div className="text-sm space-y-2">
                {deployments.map((d) => (
                  <div key={d.id} className="flex items-center justify-between py-1 border-b border-border last:border-0">
                    <div className="flex items-center gap-2">
                      <Badge variant={d.status === "success" ? "success" : d.status === "failed" ? "error" : "warning"}>
                        {d.status}
                      </Badge>
                      <span className="text-muted-foreground">
                        {new Date(d.created_at).toLocaleString()}
                      </span>
                    </div>
                    {d.commit_sha && (
                      <span className="font-mono text-xs text-muted-foreground">{d.commit_sha.slice(0, 7)}</span>
                    )}
                  </div>
                ))}
              </div>
            )}
          </CardContent>
        </Card>
      </div>
    </div>
  );
}
