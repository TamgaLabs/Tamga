"use client";

import { useEffect, useState } from "react";
import { getSystemTopology, getProjectTopology, type Topology } from "@/lib/api";

interface UseTopologyOptions {
  enabled?: boolean;
  refetchInterval?: number;
}

export function useSystemTopology(options?: UseTopologyOptions) {
  const [data, setData] = useState<Topology | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<Error | null>(null);

  const enabled = options?.enabled ?? true;

  useEffect(() => {
    if (!enabled) return;

    let isMounted = true;
    const fetchData = async () => {
      try {
        setLoading(true);
        const result = await getSystemTopology();
        if (isMounted) {
          setData(result);
          setError(null);
        }
      } catch (err) {
        if (isMounted) {
          setError(err instanceof Error ? err : new Error(String(err)));
        }
      } finally {
        if (isMounted) {
          setLoading(false);
        }
      }
    };

    fetchData();

    const interval = options?.refetchInterval
      ? setInterval(fetchData, options.refetchInterval)
      : undefined;

    return () => {
      isMounted = false;
      if (interval) clearInterval(interval);
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [enabled, options?.refetchInterval]);

  return { data, loading, error };
}

export function useProjectTopology(
  projectId: number,
  options?: UseTopologyOptions
) {
  const [data, setData] = useState<Topology | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<Error | null>(null);

  const enabled = options?.enabled ?? true;

  useEffect(() => {
    if (!enabled) return;

    let isMounted = true;
    const fetchData = async () => {
      try {
        setLoading(true);
        const result = await getProjectTopology(projectId);
        if (isMounted) {
          setData(result);
          setError(null);
        }
      } catch (err) {
        if (isMounted) {
          setError(err instanceof Error ? err : new Error(String(err)));
        }
      } finally {
        if (isMounted) {
          setLoading(false);
        }
      }
    };

    fetchData();

    const interval = options?.refetchInterval
      ? setInterval(fetchData, options.refetchInterval)
      : undefined;

    return () => {
      isMounted = false;
      if (interval) clearInterval(interval);
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [projectId, enabled, options?.refetchInterval]);

  return { data, loading, error };
}
