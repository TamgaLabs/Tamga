const SHOW_SYSTEM_KEY = "tamga_show_system";

export function getShowSystem(): boolean {
  if (typeof window === "undefined") return true;
  return localStorage.getItem(SHOW_SYSTEM_KEY) !== "false";
}

export function setShowSystem(v: boolean): void {
  if (typeof window === "undefined") return;
  localStorage.setItem(SHOW_SYSTEM_KEY, String(v));
}
