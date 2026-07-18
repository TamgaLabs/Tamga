import React, { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

const mocks = vi.hoisted(() => ({
  replace: vi.fn(),
  createSeal: vi.fn(),
  createSealRepository: vi.fn(),
  addSeal: vi.fn(),
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

vi.mock("@/contexts/workspace-context", () => ({
  useWorkspace: () => ({ addSeal: mocks.addSeal }),
}));

vi.mock("@/lib/api", () => ({
  createSeal: mocks.createSeal,
  createSealRepository: mocks.createSealRepository,
}));

import NewSealPage from "./page";

describe("NewSealPage", () => {
  let container: HTMLDivElement;
  let root: Root;

  beforeEach(() => {
    mocks.replace.mockReset();
    mocks.useAuth.mockReset();
    mocks.createSeal.mockReset();
    mocks.createSealRepository.mockReset();
    mocks.addSeal.mockReset();
    mocks.createSeal.mockResolvedValue({ id: 42 });
    mocks.createSealRepository.mockResolvedValue({ id: 99 });
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

    await act(async () => root.render(<NewSealPage />));

    expect(mocks.replace).toHaveBeenCalledWith("/login");
    expect(container.childElementCount).toBe(0);
  });

  it("renders the Seal form after authentication is resolved", async () => {
    mocks.useAuth.mockReturnValue({ user: { user_id: 1 }, loading: false });

    await act(async () => root.render(<NewSealPage />));

    expect(container.textContent).toContain("New Seal");
    expect(container.querySelector<HTMLInputElement>("#name")?.required).toBe(true);
    expect(container.querySelector<HTMLInputElement>("#repository-url")).toBeNull();
  });

  it("reveals repository fields and validates them before submission", async () => {
    mocks.useAuth.mockReturnValue({ user: { user_id: 1 }, loading: false });
    await act(async () => root.render(<NewSealPage />));

    const repositoryMode = container.querySelector<HTMLInputElement>("#repository")!;
    await act(async () => repositoryMode.click());
    expect(container.querySelector<HTMLInputElement>("#repository-url")?.required).toBe(true);
    expect(container.querySelector<HTMLInputElement>("#branch")?.value).toBe("main");

    await act(async () => container.querySelector<HTMLFormElement>("form")!.dispatchEvent(new Event("submit", { bubbles: true, cancelable: true })));
    expect(container.textContent).toContain("Seal name is required.");
    expect(container.textContent).toContain("Repository URL is required.");
    expect(mocks.createSeal).not.toHaveBeenCalled();
  });

  it("submits the repository-backed Seal and repository payloads in order", async () => {
    mocks.useAuth.mockReturnValue({ user: { user_id: 1 }, loading: false });
    await act(async () => root.render(<NewSealPage />));
    await act(async () => container.querySelector<HTMLInputElement>("#repository")!.click());
    const input = async (selector: string, value: string) => {
      const element = container.querySelector<HTMLInputElement>(selector)!;
      await act(async () => {
        const setter = Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, "value")?.set;
        setter?.call(element, value);
        element.dispatchEvent(new Event("input", { bubbles: true }));
        element.dispatchEvent(new Event("change", { bubbles: true }));
      });
    };
    await input("#name", "web");
    await input("#repository-url", "https://github.com/tamga/web.git");
    await input("#branch", "release");

    await act(async () => container.querySelector<HTMLFormElement>("form")!.dispatchEvent(new Event("submit", { bubbles: true, cancelable: true })));
    expect(mocks.createSeal).toHaveBeenCalledWith({ name: "web" });
    expect(mocks.addSeal).toHaveBeenCalledWith({ id: 42 });
    expect(mocks.createSealRepository).toHaveBeenCalledWith(42, { display_name: "web", remote_url: "https://github.com/tamga/web.git", branch: "release" });
    expect(container.textContent).toContain("Next, select services");
    expect(container.textContent).toContain("Configure Seal");
    expect(container.textContent).toContain("Add another Seal");

    await act(async () => (Array.from(container.querySelectorAll("button")).find((button) => button.textContent?.includes("Add another Seal")) as HTMLButtonElement).click());
    expect(container.querySelector<HTMLInputElement>("#name")?.value).toBe("");
    expect(container.textContent).not.toContain("Configure Seal");
  });
});
