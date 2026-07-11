"use client";

import React from "react";
import { Button } from "@/components/ui/button";
import type { MetricResolution } from "@/lib/metrics-types";

export type ResolutionOption = "auto" | MetricResolution;

interface ResolutionSelectorProps {
  value: ResolutionOption;
  onChange: (resolution: ResolutionOption) => void;
}

/**
 * ResolutionSelector - Selects data aggregation resolution
 * "auto" = omit resolution param (backend auto-selects based on time range)
 * Used by Analytics and project-specific metric views
 * Reusable component for FEAT-033/FEAT-035
 */
export function ResolutionSelector({ value, onChange }: ResolutionSelectorProps) {
  const options: { label: string; value: ResolutionOption }[] = [
    { label: "Auto", value: "auto" },
    { label: "Minute", value: "minute" },
    { label: "Hour", value: "hour" },
    { label: "Day", value: "day" },
  ];

  return (
    <div className="flex gap-2">
      {options.map((option) => (
        <Button
          key={option.value}
          variant={value === option.value ? "default" : "outline"}
          size="sm"
          onClick={() => onChange(option.value)}
        >
          {option.label}
        </Button>
      ))}
    </div>
  );
}
