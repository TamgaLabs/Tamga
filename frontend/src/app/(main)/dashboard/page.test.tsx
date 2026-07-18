import React, { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

const mocks = vi.hoisted(() => ({
  listContainers: vi.fn(),
  listSeals: vi.fn(),
  listProjects: vi.fn(),
  startContainer: vi.fn(),
  stopContainer: vi.fn(),
  restartContainer: vi.fn(),
  removeContainer: vi.fn(),
  replace: vi.fn(),
  push: vi.fn(),
  useAuth: vi.fn(),
}));

vi.mock("next/navigation", () => ({
  useRouter: () => ({ replace: mocks.replace, push: mocks.push }),
}));

vi.mock("@/lib/api", () => ({
  listContainers: mocks.listContainers,
  listSeals: mocks.listSeals,
  listProjects: mocks.listProjects,
  startContainer: mocks.startContainer,
  stopContainer: mocks.stopContainer,
  restartContainer: mocks.restartContainer,
  removeContainer: mocks.removeContainer,
}));

vi.mock("@/lib/auth", () => ({ useAuth: mocks.useAuth }));
vi.mock("@/hooks/useMetrics", () => ({ useSystemMetrics: () => ({ data: null, loading: false }) }));
vi.mock("@/lib/settings", () => ({ getShowSystem: () => true }));

import GlobalDashboardPage from "./page";

describe("GlobalDashboardPage", () => {
  let container: HTMLDivElement;
  let root: Root;

  beforeEach(() => {
    vi.clearAllMocks();
    mocks.useAuth.mockReturnValue({ user: { user_id: 1 }, loading: false });
    mocks.listContainers.mockResolvedValue([
      {
        id: "container-1",
        name: "web",
        image: "nginx:latest",
        status: "running",
        state: "running",
        ports: [],
        created: "2026-07-18T00:00:00Z",
        labels: {},
        seal_id: 42,
      },
    ]);
    mocks.listSeals.mockResolvedValue([
      {
        id: 42,
        name: "Customer Portal",
        source_type: "remote",
        repo_url: "https://example.test/portal.git",
        branch: "main",
        domain: "",
        status: "ready",
        config_authority: "generated",
        config_revision: 1,
        build_revision: 1,
        created_at: "2026-07-18T00:00:00Z",
        updated_at: "2026-07-18T00:00:00Z",
      },
    ]);
    mocks.listProjects.mockRejectedValue(new Error("legacy /api/projects must not be requested"));
    container = document.createElement("div");
    document.body.appendChild(container);
    root = createRoot(container);
  });

  afterEach(async () => {
    await act(async () => root.unmount());
    container.remove();
  });

  it("loads Seal data without a legacy project collection request and links the Seal configuration", async () => {
    await act(async () => { root.render(<GlobalDashboardPage />); });
    await act(async () => {});

    expect(mocks.listContainers).toHaveBeenCalledOnce();
    expect(mocks.listSeals).toHaveBeenCalledOnce();
    expect(mocks.listProjects).not.toHaveBeenCalled();
    expect(container.textContent).toContain("Customer Portal");
    expect(container.textContent).toContain("web");
    expect(container.querySelector('a[href="/seals/42/configure"]')).not.toBeNull();
    expect(container.textContent).not.toContain("Containers could not be loaded");
  });
});
