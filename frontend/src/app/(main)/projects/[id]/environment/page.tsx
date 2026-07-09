"use client";

import { useEffect, useState } from "react";
import { listEnvVars, createEnvVar, deleteEnvVar, type EnvVar } from "@/lib/api";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { useProjectContext } from "../project-context";

export default function ProjectEnvironmentPage() {
  const { project } = useProjectContext();
  const projectId = project.id;
  const [envVars, setEnvVars] = useState<EnvVar[]>([]);
  const [newKey, setNewKey] = useState("");
  const [newValue, setNewValue] = useState("");

  useEffect(() => {
    listEnvVars(projectId).then(setEnvVars).catch(console.error);
  }, [projectId]);

  const handleAddEnvVar = async () => {
    if (!newKey) return;
    await createEnvVar(projectId, newKey, newValue);
    setNewKey("");
    setNewValue("");
    listEnvVars(projectId).then(setEnvVars).catch(console.error);
  };

  const handleDeleteEnvVar = async (id: number) => {
    await deleteEnvVar(projectId, id);
    listEnvVars(projectId).then(setEnvVars).catch(console.error);
  };

  return (
    <div className="p-6 max-w-xl mx-auto">
      <h1 className="text-2xl font-bold mb-6">Environment</h1>
      <Card>
        <CardHeader>
          <CardTitle className="text-sm">Environment Variables</CardTitle>
        </CardHeader>
        <CardContent className="space-y-3">
          {envVars.length === 0 && (
            <p className="text-sm text-muted-foreground">No environment variables configured.</p>
          )}
          {envVars.map((ev) => (
            <div key={ev.id} className="flex items-center gap-2 text-sm">
              <span className="font-mono text-accent min-w-24">{ev.key}</span>
              <span className="text-muted-foreground">=</span>
              <span className="font-mono text-card-foreground flex-1 truncate">{ev.value}</span>
              <Button variant="ghost" size="sm" className="text-destructive" onClick={() => handleDeleteEnvVar(ev.id)}>
                &times;
              </Button>
            </div>
          ))}
          <div className="flex gap-2 pt-2 border-t border-border">
            <Input
              placeholder="KEY"
              className="font-mono text-xs flex-1"
              value={newKey}
              onChange={(e) => setNewKey(e.target.value)}
            />
            <Input
              placeholder="value"
              className="font-mono text-xs flex-1"
              value={newValue}
              onChange={(e) => setNewValue(e.target.value)}
            />
            <Button size="sm" onClick={handleAddEnvVar}>Add</Button>
          </div>
        </CardContent>
      </Card>
    </div>
  );
}
