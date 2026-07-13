import React, { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

const mocks = vi.hoisted(() => ({
  replace: vi.fn(),
  useAuth: vi.fn(),
}));

vi.mock("next/navigation", () => ({
  useRouter: () => ({
    replace: mocks.replace,
    push: vi.fn(),
    back: vi.fn(),
  }),
}));

vi.mock("@/lib/auth", () => ({
  useAuth: mocks.useAuth,
}));

vi.mock("@/lib/api", () => ({
  createProject: vi.fn(),
}));

import NewProjectPage from "./page";

describe("NewProjectPage", () => {
  let container: HTMLDivElement;
  let root: Root;

  beforeEach(() => {
    mocks.replace.mockReset();
    mocks.useAuth.mockReset();
    container = document.createElement("div");
    document.body.appendChild(container);
    root = createRoot(container);
  });

  afterEach(async () => {
    await act(async () => root.unmount());
    container.remove();
  });

  it("redirects an unauthenticated visitor to login without rendering the form", async () => {
    mocks.useAuth.mockReturnValue({ user: null, loading: false });

    await act(async () => root.render(<NewProjectPage />));

    expect(mocks.replace).toHaveBeenCalledWith("/login");
    expect(container.childElementCount).toBe(0);
  });

  it("renders the project form after authentication is resolved", async () => {
    mocks.useAuth.mockReturnValue({ user: { user_id: 1 }, loading: false });

    await act(async () => root.render(<NewProjectPage />));

    expect(container.textContent).toContain("New Project");
    expect(container.querySelector<HTMLInputElement>("#name")?.required).toBe(true);
  });
});
