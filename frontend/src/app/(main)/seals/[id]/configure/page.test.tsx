import React, { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

const mocks = vi.hoisted(() => ({
  getSealConfiguration: vi.fn(),
  listSealRepositories: vi.fn(),
  listSealServiceRoutes: vi.fn(),
  refreshSealRepository: vi.fn(),
  createSealService: vi.fn(),
  saveSealConfiguration: vi.fn(),
  deploySeal: vi.fn(),
  createSealServiceRoute: vi.fn(),
  deleteSealServiceRoute: vi.fn(),
}));

vi.mock("next/navigation", () => ({ useParams: () => ({ id: "42" }) }));
vi.mock("next/dynamic", () => ({ default: () => (props: { value?: string }) => <textarea data-testid="compose-editor" value={props.value} readOnly /> }));
vi.mock("@/lib/api", () => ({
  ...mocks,
}));

import SealConfigurePage from "./page";

describe("SealConfigurePage", () => {
  let container: HTMLDivElement;
  let root: Root;

  beforeEach(() => {
    mocks.getSealConfiguration.mockResolvedValue({
      authority: "generated",
      services: [{ id: 9, seal_id: 42, repository_id: 7, name: "web", build_context: ".", internal_port: 3000, dependencies: [] }],
      facts: [{ repository_id: 7, detected: true, preconfigured: true }],
      build_permitted: true,
    });
    mocks.listSealRepositories.mockResolvedValue([{ id: 7, seal_id: 42, display_name: "site", remote_url: "https://github.com/MaxLeiter/maxleiter.com.git", branch: "main", workspace_path: "repositories/site", status: "ready" }]);
    mocks.listSealServiceRoutes.mockResolvedValue([]);
    mocks.createSealService.mockResolvedValue({ id: 10 });
    container = document.createElement("div");
    document.body.appendChild(container);
    root = createRoot(container);
  });

  afterEach(async () => {
    await act(async () => root.unmount());
    container.remove();
    vi.clearAllMocks();
  });

  it("makes detected Next.js one-click configuration prominent while Compose stays opt-in", async () => {
    await act(async () => { root.render(<SealConfigurePage />); });
    await act(async () => {});

    expect(mocks.getSealConfiguration).toHaveBeenCalledWith(42);
    expect(container.textContent).toContain("Next.js detected · preconfigured");
    expect(container.textContent).toContain("Use one-click Next.js configuration");
    expect(container.textContent).toContain("Edit Compose in advanced mode");
    expect(container.querySelector("[data-testid=compose-editor]")).toBeNull();

    const advanced = [...container.querySelectorAll("button")].find((button) => button.textContent === "Edit Compose in advanced mode");
    await act(async () => advanced?.click());
    expect(container.querySelector("[data-testid=compose-editor]")).not.toBeNull();
  });

  it("communicates private-by-default domain status and deployment readiness", async () => {
    await act(async () => { root.render(<SealConfigurePage />); });
    await act(async () => {});

    expect(container.textContent).toContain("Services are private by default.");
    expect(container.textContent).toContain("No public domains. This service remains private.");
    expect(container.textContent).toContain("Configuration is ready for the Seal build and deploy lifecycle.");
    expect([...container.querySelectorAll("button")].find((button) => button.textContent === "Deploy Seal")?.disabled).toBe(false);
  });

  it("provides repository-backed service creation before configuration actions", async () => {
    await act(async () => { root.render(<SealConfigurePage />); });
    await act(async () => {});

    expect(container.textContent).toContain("Define the repository-backed services");
    expect(container.querySelector<HTMLInputElement>("#service-name")).not.toBeNull();
    expect([...container.querySelectorAll("button")].find((button) => button.textContent === "Create service")?.disabled).toBe(true);
  });

  it("creates a service against the selected repository", async () => {
    await act(async () => { root.render(<SealConfigurePage />); });
    await act(async () => {});
    const serviceName = container.querySelector<HTMLInputElement>("#service-name")!;
    await act(async () => {
      const setter = Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, "value")?.set;
      setter?.call(serviceName, "web");
      serviceName.dispatchEvent(new Event("input", { bubbles: true }));
      serviceName.dispatchEvent(new Event("change", { bubbles: true }));
    });

    await act(async () => [...container.querySelectorAll("button")].find((button) => button.textContent === "Create service")?.click());
    expect(mocks.createSealService).toHaveBeenCalledWith(42, { repository_id: 7, name: "web", build_context: ".", internal_port: 3000 });
  });
});
