import React, { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

const mocks = vi.hoisted(() => ({
  getProjectConfiguration: vi.fn(),
  listProjectRoutes: vi.fn(),
  refetch: vi.fn(),
}));

vi.mock("next/dynamic", () => ({
  default: () => () => null,
}));

vi.mock("@/lib/api", () => ({
  buildProject: vi.fn(),
  createProjectSource: vi.fn(),
  deleteProjectSource: vi.fn(),
  deployProject: vi.fn(),
  getProjectConfiguration: mocks.getProjectConfiguration,
  listProjectRoutes: mocks.listProjectRoutes,
  refreshAllProjectSources: vi.fn(),
  refreshProjectSource: vi.fn(),
  saveProjectConfiguration: vi.fn(),
  setProjectRoutes: vi.fn(),
}));

vi.mock("../project-context", () => ({
  useProjectContext: () => ({
    project: { id: 42, status: "configured" },
    refetch: mocks.refetch,
  }),
}));

import ProjectConfigurePage from "./page";

describe("ProjectConfigurePage", () => {
  let container: HTMLDivElement;
  let root: Root;

  beforeEach(() => {
    mocks.getProjectConfiguration.mockResolvedValue({
      sources: [{
        id: 7,
        display_name: "worker",
        remote_url: "https://example.test/worker.git",
        branch: "main",
        workspace_path: "sources/worker",
        status: "clone_failed",
        error_summary: "repository access failed",
      }],
      facts: [],
      parse_errors: [],
      services: [{ name: "api", context: "sources/worker", dockerfile: "Dockerfile" }],
      build_permitted: false,
      environment_owner: "Tamga",
    });
    mocks.listProjectRoutes.mockResolvedValue([]);
    container = document.createElement("div");
    document.body.appendChild(container);
    root = createRoot(container);
  });

  afterEach(async () => {
    await act(async () => root.unmount());
    container.remove();
  });

  it("shows source failure and uses configured services for route eligibility and build gating", async () => {
    await act(async () => { root.render(<ProjectConfigurePage />); });
    await act(async () => {});

    expect(mocks.getProjectConfiguration).toHaveBeenCalledWith(42);
    expect(container.textContent).toContain("repository access failed");
    expect(container.textContent).toContain("Eligible service");
    expect(container.querySelector("#route-service")).not.toBeNull();
    expect(container.querySelector("input#route-service")).toBeNull();
    expect(container.querySelector("button")?.textContent).toContain("Pull again");
    expect([...container.querySelectorAll("button")].find((button) => button.textContent === "Build")?.disabled).toBe(true);
    expect(container.textContent).toContain("Accept a valid configuration after all sources are ready to build.");
  });
});
