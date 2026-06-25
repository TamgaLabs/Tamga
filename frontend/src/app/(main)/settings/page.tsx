"use client";

import { useEffect, useState, useCallback } from "react";
import { useRouter } from "next/navigation";
import {
  systemInfo,
  systemPrune,
  listAgentProviders,
  createAgentProvider,
  updateAgentProvider,
  deleteAgentProvider,
  type DockerInfo,
  type AgentProvider,
} from "@/lib/api";
import { useAuth } from "@/lib/auth";
import { getShowSystem, setShowSystem } from "@/lib/settings";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";

export default function SettingsPage() {
  const [info, setInfo] = useState<DockerInfo | null>(null);
  const [showSystem, setShowSystemState] = useState(true);
  const [providers, setProviders] = useState<AgentProvider[]>([]);
  const { user, loading: authLoading } = useAuth();
  const router = useRouter();

  useEffect(() => {
    if (!authLoading && !user) router.replace("/login");
  }, [user, authLoading, router]);

  useEffect(() => {
    if (!user) return;
    systemInfo().then(setInfo).catch(console.error);
    setShowSystemState(getShowSystem());
    loadProviders();
  }, [user]);

  const loadProviders = useCallback(() => {
    listAgentProviders().then(setProviders).catch(console.error);
  }, []);

  const handleToggleSystem = () => {
    const next = !showSystem;
    setShowSystemState(next);
    setShowSystem(next);
  };

  const handlePrune = async () => {
    if (!confirm("Prune all unused containers, images, volumes, and networks?")) return;
    try {
      await systemPrune();
    } catch (e) {
      console.error(e);
    }
  };

  if (authLoading || !user) return null;

  return (
    <div className="p-6 max-w-3xl mx-auto">
      <h1 className="text-2xl font-bold mb-6">Settings</h1>

      <div className="grid gap-4">
        <Card>
          <CardHeader>
            <CardTitle className="text-sm">Display</CardTitle>
          </CardHeader>
          <CardContent>
              <label className="flex items-center gap-2 text-sm cursor-pointer">
                <input
                  type="checkbox"
                  checked={showSystem}
                  onChange={handleToggleSystem}
                  className="accent-accent"
                />
              Show Tamga System
            </label>
              <p className="text-xs text-muted-foreground mt-1">
              When disabled, Tamga system containers and codebases are hidden from all pages.
            </p>
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle className="text-sm">Docker</CardTitle>
          </CardHeader>
          <CardContent>
              {info ? (
                <div className="text-sm space-y-2 text-muted-foreground">
                  <div className="flex justify-between">
                    <span>Version</span>
                    <span className="text-foreground">{info.version}</span>
                  </div>
                  <div className="flex justify-between">
                    <span>OS</span>
                    <span className="text-foreground">{info.os}</span>
                  </div>
                  <div className="flex justify-between">
                    <span>Architecture</span>
                    <span className="text-foreground">{info.architecture}</span>
                  </div>
                  <div className="flex justify-between">
                    <span>Kernel</span>
                    <span className="text-foreground">{info.kernel}</span>
                  </div>
                  <div className="flex justify-between">
                    <span>Storage Driver</span>
                    <span className="text-foreground">{info.driver}</span>
                  </div>
                  <div className="flex justify-between">
                    <span>Name</span>
                    <span className="text-foreground">{info.name}</span>
                  </div>
                  <div className="border-t border-border my-2" />
                  <div className="flex justify-between">
                    <span>CPU</span>
                    <span className="text-foreground">{info.cpus} cores</span>
                  </div>
                  <div className="flex justify-between">
                    <span>Memory</span>
                    <span className="text-foreground">{(info.memory / 1024 / 1024 / 1024).toFixed(1)} GB</span>
                  </div>
                  <div className="flex justify-between">
                    <span>Containers</span>
                    <span className="text-foreground">{info.containers} ({info.running} running, {info.paused} paused, {info.stopped} stopped)</span>
                  </div>
                  <div className="flex justify-between">
                    <span>Images</span>
                    <span className="text-foreground">{info.images}</span>
                  </div>
                </div>
              ) : (
                <p className="text-sm text-muted-foreground">Loading...</p>
              )}
              <div className="mt-4 pt-4 border-t border-border">
              <Button variant="destructive" size="sm" onClick={handlePrune}>
                Prune All
              </Button>
            </div>
          </CardContent>
        </Card>
        <AgentProvidersCard providers={providers} onUpdate={loadProviders} />
      </div>
    </div>
  );
}

function AgentProvidersCard({ providers, onUpdate }: { providers: AgentProvider[]; onUpdate: () => void }) {
  const [showForm, setShowForm] = useState(false);
  const [editId, setEditId] = useState<string | null>(null);
  const [name, setName] = useState("");
  const [providerType, setProviderType] = useState<"docker" | "http">("docker");
  const [image, setImage] = useState("");
  const [command, setCommand] = useState("");
  const [endpoint, setEndpoint] = useState("");

  const resetForm = () => {
    setName("");
    setProviderType("docker");
    setImage("");
    setCommand("");
    setEndpoint("");
    setShowForm(false);
    setEditId(null);
  };

  const handleEdit = (p: AgentProvider) => {
    setName(p.name);
    setProviderType(p.provider_type);
    setImage(p.image || "");
    setCommand(p.command || "");
    setEndpoint(p.endpoint || "");
    setEditId(p.id);
    setShowForm(true);
  };

  const handleSave = async () => {
    try {
      if (editId) {
        await updateAgentProvider(editId, { name, provider_type: providerType, image, command, endpoint });
      } else {
        await createAgentProvider({ name, provider_type: providerType, image, command, endpoint });
      }
      resetForm();
      onUpdate();
    } catch (e) {
      console.error(e);
    }
  };

  const handleDelete = async (id: string) => {
    if (!confirm("Delete this agent provider?")) return;
    try {
      await deleteAgentProvider(id);
      onUpdate();
    } catch (e) {
      console.error(e);
    }
  };

  return (
    <Card>
      <CardHeader className="flex flex-row items-center justify-between">
        <CardTitle className="text-sm">Agent Providers</CardTitle>
        <Button size="sm" variant="outline" onClick={() => { resetForm(); setShowForm(!showForm); }}>
          {showForm ? "Cancel" : "Add Provider"}
        </Button>
      </CardHeader>
      <CardContent className="space-y-3">
        {showForm && (
          <div className="space-y-2 p-3 border border-border rounded bg-card">
            <div>
              <label className="text-xs text-muted-foreground block mb-1">Name</label>
              <Input value={name} onChange={(e) => setName(e.target.value)} placeholder="my-agent" />
            </div>
            <div>
              <label className="text-xs text-muted-foreground block mb-1">Type</label>
              <select
                className="w-full h-9 rounded-md border border-border bg-card px-3 text-sm"
                value={providerType}
                onChange={(e) => setProviderType(e.target.value as "docker" | "http")}
              >
                <option value="docker">Docker</option>
                <option value="http">HTTP</option>
              </select>
            </div>
            {providerType === "docker" && (
              <>
                <div>
                  <label className="text-xs text-muted-foreground block mb-1">Image</label>
                  <Input value={image} onChange={(e) => setImage(e.target.value)} placeholder="tamga-agent" />
                </div>
                <div>
                  <label className="text-xs text-muted-foreground block mb-1">Command</label>
                  <Input value={command} onChange={(e) => setCommand(e.target.value)} placeholder="opencode --stdin --diff" />
                </div>
              </>
            )}
            {providerType === "http" && (
              <div>
                <label className="text-xs text-muted-foreground block mb-1">Endpoint</label>
                <Input value={endpoint} onChange={(e) => setEndpoint(e.target.value)} placeholder="http://agent:9000/chat" />
              </div>
            )}
            <Button size="sm" onClick={handleSave}>{editId ? "Update" : "Create"}</Button>
          </div>
        )}
        {providers.length === 0 ? (
          <p className="text-sm text-muted-foreground">No custom providers configured.</p>
        ) : (
          <div className="text-sm space-y-2">
            {providers.map((p) => (
              <div key={p.id} className="flex items-center justify-between py-1.5 border-b border-border last:border-0">
                <div className="flex items-center gap-2">
                  <span className="font-medium">{p.name}</span>
                  <Badge variant="info" className="text-xs">{p.provider_type}</Badge>
                  {p.is_default && <Badge variant="success" className="text-xs">default</Badge>}
                </div>
                <div className="flex gap-1">
                  <Button variant="ghost" size="sm" onClick={() => handleEdit(p)}>Edit</Button>
                  {!p.is_default && (
                    <Button variant="ghost" size="sm" className="text-destructive" onClick={() => handleDelete(p.id)}>
                      Delete
                    </Button>
                  )}
                </div>
              </div>
            ))}
          </div>
        )}
      </CardContent>
    </Card>
  );
}
