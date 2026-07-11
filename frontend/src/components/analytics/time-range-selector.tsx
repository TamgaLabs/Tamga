"use client";

import React from "react";
import { Button } from "@/components/ui/button";

export type TimeRange = "1h" | "24h" | "7d";

interface TimeRangeSelectorProps {
  value: TimeRange;
  onChange: (range: TimeRange, from: number, to: number) => void;
}

/**
 * TimeRangeSelector - Converts time range selections to unix timestamps
 * Used by Analytics and project-specific metric views
 * Reusable component for FEAT-033/FEAT-035
 */
export function TimeRangeSelector({ value, onChange }: TimeRangeSelectorProps) {
  const getRangeTimestamps = (range: TimeRange): { from: number; to: number } => {
    const now = Math.floor(Date.now() / 1000);

    switch (range) {
      case "1h":
        return { from: now - 3600, to: now };
      case "24h":
        return { from: now - 86400, to: now };
      case "7d":
        return { from: now - 604800, to: now };
      default:
        return { from: now - 3600, to: now };
    }
  };

  const handleClick = (range: TimeRange) => {
    const { from, to } = getRangeTimestamps(range);
    onChange(range, from, to);
  };

  const ranges: TimeRange[] = ["1h", "24h", "7d"];

  return (
    <div className="flex gap-2">
      {ranges.map((range) => (
        <Button
          key={range}
          variant={value === range ? "default" : "outline"}
          size="sm"
          onClick={() => handleClick(range)}
        >
          Last {range}
        </Button>
      ))}
    </div>
  );
}
