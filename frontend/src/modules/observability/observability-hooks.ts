import { useQuery } from '@tanstack/react-query';
import {
  getTraceByCorrelationID,
  listProjectIncidents,
  listProjectLogs,
  listProjectMetrics,
} from '@/modules/observability/observability-api';
import type { Incident, LogEntry, MetricRecord, TraceDetail } from '@/modules/observability/observability-types';

export function useLogs(projectId?: string, level?: string) {
  return useQuery({
    queryKey: ['observability', 'logs', projectId, level],
    queryFn: async (): Promise<LogEntry[]> => {
      if (!projectId) {
        return [];
      }
      const result = await listProjectLogs(projectId, { level });
      if (result.error) {
        throw new Error(result.error.message);
      }
      return result.data?.items ?? [];
    },
    enabled: !!projectId,
    staleTime: 15 * 1000,
  });
}

export function useTrace(correlationId: string) {
  return useQuery({
    queryKey: ['observability', 'trace', correlationId],
    queryFn: async (): Promise<TraceDetail | null> => {
      const result = await getTraceByCorrelationID(correlationId);
      if (result.error) {
        if (result.error.code === 'trace_not_found') {
          return null;
        }
        throw new Error(result.error.message);
      }
      return result.data ?? null;
    },
    enabled: !!correlationId,
    staleTime: 60 * 1000,
  });
}

export function useIncidents(projectId?: string) {
  return useQuery({
    queryKey: ['observability', 'incidents', projectId],
    queryFn: async (): Promise<Incident[]> => {
      if (!projectId) {
        return [];
      }
      const result = await listProjectIncidents(projectId);
      if (result.error) {
        throw new Error(result.error.message);
      }
      return result.data?.items ?? [];
    },
    enabled: !!projectId,
    staleTime: 30 * 1000,
  });
}

export function useMetrics(projectId?: string) {
  return useQuery({
    queryKey: ['observability', 'metrics', projectId],
    queryFn: async (): Promise<MetricRecord[]> => {
      if (!projectId) {
        return [];
      }
      const result = await listProjectMetrics(projectId, 200);
      if (result.error) {
        throw new Error(result.error.message);
      }
      return result.data?.items ?? [];
    },
    enabled: !!projectId,
    staleTime: 60 * 1000,
  });
}
