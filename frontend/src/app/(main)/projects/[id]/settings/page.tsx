"use client";

import { useState } from "react";
import { updateProject } from "@/lib/api";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { useProjectContext } from "../project-context";

export default function ProjectSettingsPage() {
  const { project, refetch } = useProjectContext();
  const [editName, setEditName] = useState(project.name);
  const [editDomain, setEditDomain] = useState(project.domain);
  const [editBranch, setEditBranch] = useState(project.branch);

  const handleSave = async () => {
    await updateProject(project.id, {
      name: editName,
      domain: editDomain,
      branch: editBranch,
    });
    refetch();
  };

  return (
    <div className="p-6 max-w-xl mx-auto">
      <h1 className="text-2xl font-bold mb-6">Settings</h1>
      <Card>
        <CardHeader>
          <CardTitle className="text-sm">Project Settings</CardTitle>
        </CardHeader>
        <CardContent className="space-y-3">
          <div className="space-y-1">
            <Label className="text-xs">Name</Label>
            <Input value={editName} onChange={(e) => setEditName(e.target.value)} />
          </div>
          <div className="space-y-1">
            <Label className="text-xs">Domain</Label>
            <Input value={editDomain} onChange={(e) => setEditDomain(e.target.value)} />
          </div>
          <div className="space-y-1">
            <Label className="text-xs">Branch</Label>
            <Input value={editBranch} onChange={(e) => setEditBranch(e.target.value)} />
          </div>
          <Button size="sm" onClick={handleSave}>Save</Button>
        </CardContent>
      </Card>
    </div>
  );
}
