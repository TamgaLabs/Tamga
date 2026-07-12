import { isOfflineMode } from "@/lib/offline-api";

export function OfflinePreviewBanner() {
  if (!isOfflineMode()) return null;

  return (
    <div className="fixed bottom-4 right-4 z-[100] rounded-full border border-amber-500/40 bg-amber-500/15 px-3 py-1.5 text-xs font-medium text-amber-700 shadow-sm dark:text-amber-300">
      Offline frontend preview — API and terminal are disabled
    </div>
  );
}
