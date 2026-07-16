"use client";

import { useEffect, useState } from "react";
import { getProjectConfiguration, updateProject } from "@/lib/api";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { useProjectContext } from "../project-context";
import { PageHeader, PageHeaderDescription, PageHeaderTitle } from "@/components/page-header";
import { Field, FieldError, FieldGroup, FieldLabel } from "@/components/ui/field";

export default function ProjectSettingsPage() {
  const { project, refetch } = useProjectContext();
  const [editName, setEditName] = useState(project.name);
  const [editDomain, setEditDomain] = useState(project.domain);
  const [editBranch, setEditBranch] = useState(project.branch);
  const [editExposedService, setEditExposedService] = useState(project.exposed_service || "");
  const [services, setServices] = useState<string[]>([]);
  const [error, setError] = useState("");
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    let active = true;
    getProjectConfiguration(project.id).then((configuration) => {
      if (active) setServices(configuration.services.map(({ name }) => name));
    }).catch(() => {
      if (active) setServices([]);
    });
    return () => { active = false; };
  }, [project.id]);

  const hasServices = services.length > 0;

  const handleSave = async () => {
    setError("");
    setSaving(true);
    try {
      await updateProject(project.id, {
        name: editName,
        domain: editDomain,
        branch: editBranch,
        ...(hasServices ? { exposed_service: editExposedService } : {}),
      });
      refetch();
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : "Failed to save settings");
    } finally {
      setSaving(false);
    }
  };

  return (
    <div className="mx-auto max-w-4xl space-y-6 p-4 sm:p-6">
      <PageHeader><div><PageHeaderTitle>Settings</PageHeaderTitle><PageHeaderDescription>Project identity, routing, and deployment source settings.</PageHeaderDescription></div></PageHeader>
      <Card>
        <CardHeader>
          <CardTitle className="text-sm">Project Settings</CardTitle>
        </CardHeader>
        <CardContent className="space-y-5">
          <FieldGroup>
          <Field>
            <FieldLabel htmlFor="project-name">Name</FieldLabel>
            <Input id="project-name" value={editName} onChange={(e) => setEditName(e.target.value)} />
          </Field>
          <Field>
            <FieldLabel htmlFor="project-domain">Domain</FieldLabel>
            <Input
              id="project-domain"
              value={editDomain}
              onChange={(e) => setEditDomain(e.target.value)}
              placeholder="example.com"
            />
          </Field>
          <Field>
            <FieldLabel htmlFor="project-branch">Branch</FieldLabel>
            <Input id="project-branch" value={editBranch} onChange={(e) => setEditBranch(e.target.value)} />
          </Field>

          {hasServices && (
            <Field>
              <FieldLabel>Expose service</FieldLabel>
              <div>
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
            </Field>
          )}
          </FieldGroup>

          {error && <FieldError className="whitespace-pre-wrap">{error}</FieldError>}

          <Button size="sm" onClick={handleSave} disabled={saving}>
            {saving ? "Saving..." : "Save"}
          </Button>
        </CardContent>
      </Card>
    </div>
  );
}
