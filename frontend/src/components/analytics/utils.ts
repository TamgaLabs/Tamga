/**
 * Format seconds to milliseconds with proper rounding
 */
export function secondsToMs(seconds: number): number {
  return Math.round(seconds * 1000);
}

/**
 * Format bytes with human-readable units
 */
export function formatBytes(bytes: number): string {
  if (bytes === 0) return "0 B";
  const k = 1024;
  const sizes = ["B", "KB", "MB", "GB"];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  const value = bytes / Math.pow(k, i);
  return `${value.toFixed(2)} ${sizes[i]}`;
}

/**
 * Format a number with thousands separator
 */
export function formatNumber(num: number): string {
  return num.toLocaleString();
}

/**
 * Format percentage (0-1) to display
 */
export function formatPercent(value: number): string {
  return `${(value * 100).toFixed(1)}%`;
}

/**
 * Get a color based on status or severity
 */
export function getStatusColor(statusCode2xx: number, status4xx: number, status5xx: number): string {
  const total = statusCode2xx + status4xx + status5xx;
  if (total === 0) return "text-gray-400";
  const errorRate = (status4xx + status5xx) / total;
  if (errorRate >= 0.1) return "text-red-500"; // 10%+ error rate
  if (errorRate >= 0.05) return "text-yellow-500"; // 5%+ error rate
  return "text-green-500";
}

/**
 * Parse ISO 8601 timestamp to Date
 */
export function parseTimestamp(timestamp: string): Date {
  return new Date(timestamp);
}

/**
 * Format a date for display on axis
 */
export function formatAxisTime(date: Date, resolution: "minute" | "hour" | "day"): string {
  const hours = date.getHours().toString().padStart(2, "0");
  const minutes = date.getMinutes().toString().padStart(2, "0");
  const day = date.getDate();
  const month = date.getMonth() + 1;

  if (resolution === "minute") {
    return `${hours}:${minutes}`;
  } else if (resolution === "hour") {
    return `${hours}:00`;
  } else {
    return `${month}/${day}`;
  }
}

/**
 * Clamp a number between min and max
 */
export function clamp(num: number, min: number, max: number): number {
  return Math.max(min, Math.min(max, num));
}

/**
 * Calculate the maximum value in an array, with a reasonable floor
 */
export function getMaxValue(values: number[], minFloor: number = 1): number {
  const max = Math.max(...values, minFloor);
  // Round up to a nice number
  if (max < 10) return max;
  if (max < 100) return Math.ceil(max / 10) * 10;
  if (max < 1000) return Math.ceil(max / 100) * 100;
  return Math.ceil(max / 1000) * 1000;
}
