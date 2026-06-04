"use client";

import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import Link from "next/link";
import { projects, removeToken, isAuthenticated, type Project } from "@/lib/api";

export default function DashboardPage() {
  const router = useRouter();
  const [list, setList] = useState<Project[]>([]);
  const [loading, setLoading] = useState(true);
  const [showCreate, setShowCreate] = useState(false);
  const [name, setName] = useState("");
  const [desc, setDesc] = useState("");

  useEffect(() => {
    if (!isAuthenticated()) {
      router.replace("/login");
      return;
    }
    projects.list().then(setList).catch(() => router.replace("/login")).finally(() => setLoading(false));
  }, [router]);

  async function handleCreate(e: React.FormEvent) {
    e.preventDefault();
    const p = await projects.create(name, desc);
    setList((prev) => [p, ...prev]);
    setName("");
    setDesc("");
    setShowCreate(false);
  }

  async function handleLogout() {
    removeToken();
    router.push("/login");
  }

  if (loading) return <div className="p-8 text-zinc-400">Loading...</div>;

  return (
    <div className="mx-auto max-w-4xl p-8">
      <div className="mb-8 flex items-center justify-between">
        <h1 className="text-2xl font-bold">Projects</h1>
        <div className="flex gap-3">
          <button onClick={() => setShowCreate(!showCreate)} className="rounded-lg bg-blue-600 px-4 py-2 text-sm font-semibold hover:bg-blue-700">
            New Project
          </button>
          <button onClick={handleLogout} className="rounded-lg border border-zinc-700 px-4 py-2 text-sm text-zinc-400 hover:text-white">
            Logout
          </button>
        </div>
      </div>

      {showCreate && (
        <form onSubmit={handleCreate} className="mb-8 rounded-xl border border-zinc-800 bg-zinc-900 p-6 space-y-4">
          <input
            placeholder="Project name"
            value={name}
            onChange={(e) => setName(e.target.value)}
            required
            className="w-full rounded-lg border border-zinc-700 bg-zinc-800 px-4 py-2 text-sm focus:border-blue-500 focus:outline-none"
          />
          <textarea
            placeholder="Description (optional)"
            value={desc}
            onChange={(e) => setDesc(e.target.value)}
            rows={2}
            className="w-full rounded-lg border border-zinc-700 bg-zinc-800 px-4 py-2 text-sm focus:border-blue-500 focus:outline-none"
          />
          <div className="flex gap-2">
            <button type="submit" className="rounded-lg bg-green-600 px-4 py-2 text-sm font-semibold hover:bg-green-700">Create</button>
            <button type="button" onClick={() => setShowCreate(false)} className="rounded-lg border border-zinc-700 px-4 py-2 text-sm text-zinc-400 hover:text-white">Cancel</button>
          </div>
        </form>
      )}

      {list.length === 0 ? (
        <div className="rounded-xl border border-zinc-800 p-12 text-center text-zinc-500">
          No projects yet. Click &quot;New Project&quot; to get started.
        </div>
      ) : (
        <div className="space-y-3">
          {list.map((p) => (
            <Link
              key={p.id}
              href={`/projects/${p.id}`}
              className="block rounded-xl border border-zinc-800 bg-zinc-900 p-5 hover:border-zinc-700 transition-colors"
            >
              <h2 className="text-lg font-semibold">{p.name}</h2>
              {p.description && <p className="mt-1 text-sm text-zinc-400">{p.description}</p>}
              <p className="mt-2 text-xs text-zinc-600">Created {new Date(p.created_at).toLocaleDateString()}</p>
            </Link>
          ))}
        </div>
      )}
    </div>
  );
}
