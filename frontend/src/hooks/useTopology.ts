"use client";

import { useCallback, useEffect, useState } from "react";
import { getSystemTopology, getProjectTopology, type Topology } from "@/lib/api";

interface UseTopologyOptions {
  enabled?: boolean;
}

export function useSystemTopology(options?: UseTopologyOptions) {
  const [data, setData] = useState<Topology | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<Error | null>(null);

  const enabled = options?.enabled ?? true;

  const fetchData = useCallback(async () => {
    try {
      setLoading(true);
      const result = await getSystemTopology();
      setData(result);
      setError(null);
    } catch (err) {
      setError(err instanceof Error ? err : new Error(String(err)));
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    if (!enabled) return;
    fetchData();
  }, [enabled, fetchData]);

  return { data, loading, error, refetch: fetchData };
}

export function useProjectTopology(
  projectId: number,
  options?: UseTopologyOptions
) {
  const [data, setData] = useState<Topology | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<Error | null>(null);

  const enabled = options?.enabled ?? true;

  const fetchData = useCallback(async () => {
    try {
      setLoading(true);
      const result = await getProjectTopology(projectId);
      setData(result);
      setError(null);
    } catch (err) {
      setError(err instanceof Error ? err : new Error(String(err)));
    } finally {
      setLoading(false);
    }
  }, [projectId]);

  useEffect(() => {
    if (!enabled) return;
    fetchData();
  }, [enabled, fetchData]);

  return { data, loading, error, refetch: fetchData };
}
