import { afterEach, describe, expect, it, vi } from "vitest";
import { api, listSeals } from "./api";

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

    await expect(api("/seals")).rejects.toThrow("Permission denied");
    expect(fetchMock).toHaveBeenCalledWith("/api/seals", {
      headers: {
        "Content-Type": "application/json",
        Authorization: "Bearer test-token",
      },
    });
  });

  it("returns undefined for successful empty responses", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(new Response(null, { status: 204 })));

    await expect(api<void>("/seals/1", { method: "DELETE" })).resolves.toBeUndefined();
  });

  it("lists Seals from the Seal API", async () => {
    const fetchMock = vi.fn().mockResolvedValue(
      new Response(JSON.stringify([{ id: 1, name: "web" }]), { status: 200 })
    );
    vi.stubGlobal("fetch", fetchMock);

    await expect(listSeals()).resolves.toEqual([{ id: 1, name: "web" }]);
    expect(fetchMock).toHaveBeenCalledWith("/api/seals", {
      headers: { "Content-Type": "application/json" },
    });
  });
});
