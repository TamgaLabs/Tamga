import React, { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

const mocks = vi.hoisted(() => ({
  listSeals: vi.fn(),
  push: vi.fn(),
  pathname: "/dashboard/system",
  user: { id: 1 },
}));

vi.mock("next/navigation", () => ({
  usePathname: () => mocks.pathname,
  useRouter: () => ({ push: mocks.push }),
}));
vi.mock("@/lib/auth", () => ({
  useAuth: () => ({ user: mocks.user }),
}));
vi.mock("@/lib/api", () => ({ listSeals: mocks.listSeals }));

import { useWorkspace, WorkspaceProvider } from "./workspace-context";

function WorkspaceProbe() {
  const { selectedSeal, seals, view } = useWorkspace();
  return (
    <output data-testid="workspace">
      {JSON.stringify({ selectedSeal, seals, view })}
    </output>
  );
}

describe("WorkspaceProvider", () => {
  let container: HTMLDivElement;
  let root: Root;

  beforeEach(() => {
    mocks.listSeals.mockResolvedValue([
      { id: 42, name: "web", source_type: "local" },
    ]);
    container = document.createElement("div");
    document.body.appendChild(container);
    root = createRoot(container);
  });

  afterEach(async () => {
    await act(async () => root.unmount());
    container.remove();
    vi.clearAllMocks();
  });

  it("keeps the virtual Tamga System Seal selected on its dashboard route", async () => {
    await act(async () => {
      root.render(
        <WorkspaceProvider>
          <WorkspaceProbe />
        </WorkspaceProvider>
      );
    });
    await act(async () => {});

    const state = JSON.parse(container.querySelector("output")!.textContent!);
    expect(state.view).toBe(-1);
    expect(state.selectedSeal).toMatchObject({ id: -1, name: "Tamga System" });
    expect(state.seals).toEqual(
      expect.arrayContaining([
        expect.objectContaining({ id: -1, name: "Tamga System" }),
        expect.objectContaining({ id: 42, name: "web" }),
      ])
    );
  });
});
