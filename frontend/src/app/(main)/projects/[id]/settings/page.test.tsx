import React, { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

const mocks = vi.hoisted(() => ({
  getProjectConfiguration: vi.fn(),
  updateProject: vi.fn(),
  refetch: vi.fn(),
}));

vi.mock("@/lib/api", () => ({
  getProjectConfiguration: mocks.getProjectConfiguration,
  updateProject: mocks.updateProject,
}));

vi.mock("../project-context", () => ({
  useProjectContext: () => ({
    project: {
      id: 42,
      name: "Demo",
      domain: "demo.test",
      branch: "main",
      exposed_service: "api",
      // This indentation deliberately does not match the removed client parser.
      compose_yaml: "services:\n    ignored-by-client-parser:\n      build: .\n",
    },
    refetch: mocks.refetch,
  }),
}));

import ProjectSettingsPage from "./page";

describe("ProjectSettingsPage", () => {
  let container: HTMLDivElement;
  let root: Root;

  beforeEach(() => {
    mocks.getProjectConfiguration.mockResolvedValue({
      services: [{ name: "api", context: ".", dockerfile: "Dockerfile" }],
    });
    container = document.createElement("div");
    document.body.appendChild(container);
    root = createRoot(container);
  });

  afterEach(async () => {
    await act(async () => root.unmount());
    container.remove();
  });

  it("uses server configuration services for the expose-service selector", async () => {
    await act(async () => { root.render(<ProjectSettingsPage />); });
    await act(async () => {});

    expect(mocks.getProjectConfiguration).toHaveBeenCalledWith(42);
    expect(container.textContent).toContain("Expose service");
    expect(container.textContent).toContain("api");
    expect(container.textContent).not.toContain("ignored-by-client-parser");
  });
});
