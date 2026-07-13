"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { deleteProject, restartProject, getProjectLogs } from "@/lib/api";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog";
import { useProjectContext } from "../project-context";
import { PageHeader, PageHeaderDescription, PageHeaderTitle } from "@/components/page-header";

export default function ProjectActionsPage() {
  const { project, refetch } = useProjectContext();
  const router = useRouter();
  const [restarting, setRestarting] = useState(false);
  const [restartError, setRestartError] = useState("");
  const [logs, setLogs] = useState("");
  const [logsError, setLogsError] = useState("");
  const [showLogs, setShowLogs] = useState(false);
  const [deleteDialogOpen, setDeleteDialogOpen] = useState(false);
  const [deleting, setDeleting] = useState(false);
  const [deleteError, setDeleteError] = useState("");
  const [deleteSuccess, setDeleteSuccess] = useState(false);

  const handleRestart = async () => {
    setRestartError("");
    setRestarting(true);
    try {
      await restartProject(project.id);
      refetch();
    } catch (err: unknown) {
      setRestartError(err instanceof Error ? err.message : "Failed to restart project");
    } finally {
      setRestarting(false);
    }
  };

  const loadLogs = async () => {
    setLogsError("");
    try {
      const result = await getProjectLogs(project.id);
      setLogs(result.logs);
      setShowLogs(true);
    } catch (err: unknown) {
      console.error(err);
      setLogsError(err instanceof Error ? err.message : "Failed to load project logs");
    }
  };

  const handleDelete = async () => {
    setDeleteError("");
    setDeleting(true);
    try {
      await deleteProject(project.id);
      setDeleteSuccess(true);
      setTimeout(() => router.push("/dashboard"), 1500);
    } catch (err: unknown) {
      setDeleteError(err instanceof Error ? err.message : "Failed to delete project");
      setDeleting(false);
    }
  };

  return (
    <div className="mx-auto max-w-4xl space-y-6 p-4 sm:p-6">
      <PageHeader><div><PageHeaderTitle>Actions</PageHeaderTitle><PageHeaderDescription>Operational controls and logs for {project.name}.</PageHeaderDescription></div></PageHeader>

      {deleteSuccess && (
        <div role="status" className="rounded-md border border-success/20 bg-success/10 p-3 text-sm text-success">
          Project deleted successfully. Redirecting...
        </div>
      )}
      {deleteError && (
        <div role="alert" className="flex items-center justify-between rounded-md border border-destructive/20 bg-destructive/10 p-3 text-sm text-destructive">
          <span>{deleteError}</span>
          <button onClick={() => setDeleteError("")} className="text-destructive hover:text-destructive/80 ml-2">&times;</button>
        </div>
      )}
      {restartError && (
        <div role="alert" className="flex items-center justify-between rounded-md border border-destructive/20 bg-destructive/10 p-3 text-sm text-destructive">
          <span>{restartError}</span>
          <button onClick={() => setRestartError("")} className="text-destructive hover:text-destructive/80 ml-2">&times;</button>
        </div>
      )}

      <Card>
        <CardHeader>
          <CardTitle className="text-sm">Project operations</CardTitle>
        </CardHeader>
        <CardContent className="flex flex-wrap gap-2">
          <Button variant="outline" size="sm" onClick={handleRestart} disabled={restarting}>
            {restarting ? "Restarting..." : "Restart"}
          </Button>
          <Button variant="outline" size="sm" onClick={() => void loadLogs()}>
            View Logs
          </Button>
          <Button variant="destructive" size="sm" onClick={() => setDeleteDialogOpen(true)}>
            Delete
          </Button>
        </CardContent>
      </Card>

      {logsError && <div role="alert" className="rounded-md border border-destructive/20 bg-destructive/10 p-3 text-sm text-destructive">{logsError}</div>}

      {showLogs && (
        <Card className="mt-4">
          <CardHeader className="flex flex-row items-center justify-between">
            <CardTitle className="text-sm">Container Logs</CardTitle>
            <Button variant="ghost" size="sm" onClick={() => setShowLogs(false)}>Close</Button>
          </CardHeader>
          <CardContent>
            <pre className="max-h-80 overflow-auto rounded-md bg-code-block p-4 font-mono text-xs text-success whitespace-pre-wrap">
              {logs || "(no output)"}
            </pre>
          </CardContent>
        </Card>
      )}

      <AlertDialog open={deleteDialogOpen} onOpenChange={setDeleteDialogOpen}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Delete project?</AlertDialogTitle>
            <AlertDialogDescription>
              This will permanently delete &quot;{project.name}&quot; and its
              associated container. This action cannot be undone.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>Cancel</AlertDialogCancel>
            <AlertDialogAction onClick={handleDelete} disabled={deleting}>
              {deleting ? "Deleting..." : "Delete"}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  );
}
