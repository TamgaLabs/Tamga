export type TerminalTab = { id: string; pending: boolean };

type SessionLike = { id: string; created_at: string };

/**
 * Merge the entry-time server snapshot without resurrecting a session that
 * was successfully terminated while that request was in flight.
 */
export function mergeTerminalTabs(
  previous: TerminalTab[],
  sessions: SessionLike[],
  terminatedSessionIds: ReadonlySet<string>
): TerminalTab[] {
  // An entry-time snapshot is not authoritative for tabs that were resolved
  // locally while it was in flight. Keep every non-terminated local tab,
  // including real ids that are absent from this older response.
  const currentTabs = previous.filter((tab) => !terminatedSessionIds.has(tab.id));
  const existingRealIds = new Set(currentTabs.filter((tab) => !tab.pending).map((tab) => tab.id));
  const newRealTabs = sessions
    .filter((session) => !terminatedSessionIds.has(session.id) && !existingRealIds.has(session.id))
    .map((session) => ({ id: session.id, pending: false }));

  return [...currentTabs, ...newRealTabs];
}

/** Remove a terminated tab and select the first remaining tab only if needed. */
export function removeTerminalTab(
  tabs: TerminalTab[],
  activeTabId: string | null,
  removedId: string
): { tabs: TerminalTab[]; activeTabId: string | null } {
  const nextTabs = tabs.filter((tab) => tab.id !== removedId);
  return {
    tabs: nextTabs,
    activeTabId: activeTabId === removedId ? (nextTabs[0]?.id ?? null) : activeTabId,
  };
}
