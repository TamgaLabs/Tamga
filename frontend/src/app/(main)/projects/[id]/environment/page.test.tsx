import React, { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

const mocks = vi.hoisted(() => ({
  getProjectConfiguration: vi.fn(),
  listEnvVars: vi.fn(),
  listServiceEnvVars: vi.fn(),
}));

vi.mock("@/lib/api", () => ({
  getProjectConfiguration: mocks.getProjectConfiguration,
  listEnvVars: mocks.listEnvVars,
  listServiceEnvVars: mocks.listServiceEnvVars,
  createEnvVar: vi.fn(),
  deleteEnvVar: vi.fn(),
  deleteServiceEnvVar: vi.fn(),
  upsertServiceEnvVar: vi.fn(),
}));

vi.mock("../project-context", () => ({
  useProjectContext: () => ({
    project: {
      id: 42,
      // This indentation deliberately does not match the old client regex.
      compose_yaml: "services:\n    ignored-by-client-parser:\n      build: .\n",
    },
  }),
}));

import ProjectEnvironmentPage from "./page";

describe("ProjectEnvironmentPage", () => {
  let container: HTMLDivElement;
  let root: Root;

  beforeEach(() => {
    mocks.getProjectConfiguration.mockResolvedValue({
      services: [{ name: "api", context: ".", dockerfile: "Dockerfile" }],
    });
    mocks.listEnvVars.mockResolvedValue([]);
    mocks.listServiceEnvVars.mockResolvedValue([]);
    container = document.createElement("div");
    document.body.appendChild(container);
    root = createRoot(container);
  });

  afterEach(async () => {
    await act(async () => root.unmount());
    container.remove();
  });

  it("uses the server configuration services instead of parsing Compose in the browser", async () => {
    await act(async () => { root.render(<ProjectEnvironmentPage />); });
    await act(async () => {});

    expect(mocks.getProjectConfiguration).toHaveBeenCalledWith(42);
    expect(container.textContent).toContain("api");
    expect(container.textContent).not.toContain("Save a Compose configuration");
  });
});
