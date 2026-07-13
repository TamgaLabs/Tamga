import { describe, expect, it } from "vitest";
import { mergeTerminalTabs, removeTerminalTab } from "./terminal-tabs";

describe("terminal tabs", () => {
  it("does not resurrect a successfully terminated tab from a stale session snapshot", () => {
    const merged = mergeTerminalTabs(
      [],
      [{ id: "terminated", created_at: "2026-07-13T10:00:00Z" }],
      new Set(["terminated"])
    );

    expect(merged).toEqual([]);
  });

  it("keeps a concurrently resolved real tab when an older snapshot omits it", () => {
    const merged = mergeTerminalTabs(
      [
        { id: "existing", pending: false },
        { id: "resolved-during-request", pending: false },
      ],
      [{ id: "existing", created_at: "2026-07-13T10:00:00Z" }],
      new Set()
    );

    expect(merged).toEqual([
      { id: "existing", pending: false },
      { id: "resolved-during-request", pending: false },
    ]);
  });

  it("removes the active tab and selects the first remaining tab", () => {
    expect(
      removeTerminalTab(
        [
          { id: "first", pending: false },
          { id: "active", pending: false },
        ],
        "active",
        "active"
      )
    ).toEqual({ tabs: [{ id: "first", pending: false }], activeTabId: "first" });
  });

  it("leaves the active tab unchanged when a background tab is terminated", () => {
    expect(
      removeTerminalTab(
        [
          { id: "active", pending: false },
          { id: "other", pending: false },
        ],
        "active",
        "other"
      )
    ).toEqual({ tabs: [{ id: "active", pending: false }], activeTabId: "active" });
  });
});
