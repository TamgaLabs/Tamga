"use client";

import { useState } from "react";
import { updateProject } from "@/lib/api";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { useProjectContext } from "../project-context";

// Parse the compose_yaml to extract service names from the top-level services: block.
// Safe minimal extraction without adding a YAML parser dependency.
function extractServices(composeYaml: string | undefined): string[] {
  if (!composeYaml) return [];

  const lines = composeYaml.split("\n");
  const services: string[] = [];
  let inServices = false;
  let servicesBlockIndent: number | null = null;
  let serviceIndentLevel: number | null = null;

  for (const line of lines) {
    const trimmed = line.trimStart();

    // Skip empty lines and comments
    if (!trimmed || trimmed.startsWith("#")) continue;

    // Calculate indentation
    const indent = line.length - trimmed.length;

    // Check if we found the services: block
    if (trimmed.startsWith("services:")) {
      inServices = true;
      servicesBlockIndent = indent;
      // Service names will be at servicesBlockIndent + 2 (standard YAML indent)
      serviceIndentLevel = indent + 2;
      continue;
    }

    if (inServices && servicesBlockIndent !== null) {
      // If we hit something at the same level as services: or less indented, we're done
      if (indent <= servicesBlockIndent && trimmed) {
        break;
      }

      // Service names are at serviceIndentLevel and have a colon
      if (indent === serviceIndentLevel && trimmed.includes(":")) {
        // Get the key name (before the colon)
        const parts = trimmed.split(":");
        const serviceName = parts[0].trim();
        if (serviceName && !serviceName.startsWith("#")) {
          services.push(serviceName);
        }
      }
    }
  }

  return services;
}

export default function ProjectSettingsPage() {
  const { project, refetch } = useProjectContext();
  const [editName, setEditName] = useState(project.name);
  const [editDomain, setEditDomain] = useState(project.domain);
  const [editBranch, setEditBranch] = useState(project.branch);
  const [editExposedService, setEditExposedService] = useState(project.exposed_service || "");
  const [error, setError] = useState("");
  const [saving, setSaving] = useState(false);

  const services = extractServices(project.compose_yaml);
  const hasCompose = !!project.compose_yaml;

  const handleSave = async () => {
    setError("");
    setSaving(true);
    try {
      await updateProject(project.id, {
        name: editName,
        domain: editDomain,
        branch: editBranch,
        ...(hasCompose ? { exposed_service: editExposedService } : {}),
      });
      refetch();
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : "Failed to save settings");
    } finally {
      setSaving(false);
    }
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
            <Input
              value={editDomain}
              onChange={(e) => setEditDomain(e.target.value)}
              placeholder="example.com"
            />
          </div>
          <div className="space-y-1">
            <Label className="text-xs">Branch</Label>
            <Input value={editBranch} onChange={(e) => setEditBranch(e.target.value)} />
          </div>

          {hasCompose && (
            <div className="space-y-1">
              <Label className="text-xs">Expose Service</Label>
              <div className="space-y-1">
                <Select value={editExposedService} onValueChange={setEditExposedService}>
                  <SelectTrigger>
                    <SelectValue placeholder="Select a service (optional)" />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="">None</SelectItem>
                    {services.map((service) => (
                      <SelectItem key={service} value={service}>
                        {service}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
              {project.exposed_service && (
                <p className="text-xs text-muted-foreground">
                  Currently exposed: <strong>{project.exposed_service}</strong>
                </p>
              )}
            </div>
          )}

          {error && (
            <p className="text-sm text-destructive whitespace-pre-wrap">{error}</p>
          )}

          <Button size="sm" onClick={handleSave} disabled={saving}>
            {saving ? "Saving..." : "Save"}
          </Button>
        </CardContent>
      </Card>
    </div>
  );
}
