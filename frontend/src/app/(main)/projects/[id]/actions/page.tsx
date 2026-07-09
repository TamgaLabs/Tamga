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

export default function ProjectActionsPage() {
  const { project, refetch } = useProjectContext();
  const router = useRouter();
  const [restarting, setRestarting] = useState(false);
  const [restartError, setRestartError] = useState("");
  const [logs, setLogs] = useState("");
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
    try {
      const result = await getProjectLogs(project.id);
      setLogs(result.logs);
      setShowLogs(true);
    } catch (e) {
      console.error(e);
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
    <div className="p-6 max-w-3xl mx-auto">
      <h1 className="text-2xl font-bold mb-6">Actions</h1>

      {deleteSuccess && (
        <div className="mb-4 p-3 bg-success/10 border border-success/20 rounded text-sm text-success">
          Project deleted successfully. Redirecting...
        </div>
      )}
      {deleteError && (
        <div className="mb-4 p-3 bg-destructive/10 border border-destructive/20 rounded text-sm text-destructive flex items-center justify-between">
          <span>{deleteError}</span>
          <button onClick={() => setDeleteError("")} className="text-destructive hover:text-destructive/80 ml-2">&times;</button>
        </div>
      )}
      {restartError && (
        <div className="mb-4 p-3 bg-destructive/10 border border-destructive/20 rounded text-sm text-destructive flex items-center justify-between">
          <span>{restartError}</span>
          <button onClick={() => setRestartError("")} className="text-destructive hover:text-destructive/80 ml-2">&times;</button>
        </div>
      )}

      <Card>
        <CardHeader>
          <CardTitle className="text-sm">Project Actions</CardTitle>
        </CardHeader>
        <CardContent className="flex flex-wrap gap-2">
          <Button variant="outline" size="sm" onClick={handleRestart} disabled={restarting}>
            {restarting ? "Restarting..." : "Restart"}
          </Button>
          <Button variant="outline" size="sm" onClick={loadLogs}>
            View Logs
          </Button>
          <Button variant="destructive" size="sm" onClick={() => setDeleteDialogOpen(true)}>
            Delete
          </Button>
        </CardContent>
      </Card>

      {showLogs && (
        <Card className="mt-4">
          <CardHeader className="flex flex-row items-center justify-between">
            <CardTitle className="text-sm">Container Logs</CardTitle>
            <Button variant="ghost" size="sm" onClick={() => setShowLogs(false)}>Close</Button>
          </CardHeader>
          <CardContent>
            <pre className="bg-code-block rounded p-4 text-xs text-success overflow-auto max-h-80 font-mono whitespace-pre-wrap">
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
