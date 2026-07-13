import { afterEach, describe, expect, it, vi } from "vitest";
import { api } from "./api";

describe("api", () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("adds the stored token and surfaces a plain-text API failure", async () => {
    localStorage.setItem("token", "test-token");
    const fetchMock = vi.fn().mockResolvedValue(
      new Response("Permission denied", { status: 403 })
    );
    vi.stubGlobal("fetch", fetchMock);

    await expect(api("/projects")).rejects.toThrow("Permission denied");
    expect(fetchMock).toHaveBeenCalledWith("/api/projects", {
      headers: {
        "Content-Type": "application/json",
        Authorization: "Bearer test-token",
      },
    });
  });

  it("returns undefined for successful empty responses", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(new Response(null, { status: 204 })));

    await expect(api<void>("/projects/1", { method: "DELETE" })).resolves.toBeUndefined();
  });
});
