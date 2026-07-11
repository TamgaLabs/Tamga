"use client";

import { useEffect, useState } from "react";
import { getSystemMetrics, getProjectMetrics, type MetricsPanels, type MetricsQueryParams } from "@/lib/api";

interface UseMetricsOptions {
  enabled?: boolean;
  refetchInterval?: number;
}

export function useSystemMetrics(params?: MetricsQueryParams, options?: UseMetricsOptions) {
  const [data, setData] = useState<MetricsPanels | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<Error | null>(null);

  const enabled = options?.enabled ?? true;

  useEffect(() => {
    if (!enabled) return;

    let isMounted = true;
    const fetchData = async () => {
      try {
        setLoading(true);
        const result = await getSystemMetrics(params);
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
  }, [enabled, params?.from, params?.to, params?.resolution, options?.refetchInterval]);

  return { data, loading, error };
}

export function useProjectMetrics(
  projectId: number,
  params?: MetricsQueryParams,
  options?: UseMetricsOptions
) {
  const [data, setData] = useState<MetricsPanels | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<Error | null>(null);

  const enabled = options?.enabled ?? true;

  useEffect(() => {
    if (!enabled) return;

    let isMounted = true;
    const fetchData = async () => {
      try {
        setLoading(true);
        const result = await getProjectMetrics(projectId, params);
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
  }, [projectId, enabled, params?.from, params?.to, params?.resolution, options?.refetchInterval]);

  return { data, loading, error };
}
