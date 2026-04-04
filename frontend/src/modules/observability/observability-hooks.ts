import { useQuery } from '@tanstack/react-query';
import { mockFetchLogs, mockFetchTrace, mockFetchIncidents, mockFetchMetrics } from '@/modules/observability/observability-mocks';

export function useLogs(deploymentId?: string, level?: string) {
  return useQuery({
    queryKey: ['observability', 'logs', deploymentId, level],
    queryFn: () => mockFetchLogs(deploymentId, level),
    staleTime: 15 * 1000,
  });
}

export function useTrace(correlationId: string) {
  return useQuery({
    queryKey: ['observability', 'trace', correlationId],
    queryFn: () => mockFetchTrace(correlationId),
    enabled: !!correlationId,
    staleTime: 60 * 1000,
  });
}

export function useIncidents() {
  return useQuery({
    queryKey: ['observability', 'incidents'],
    queryFn: () => mockFetchIncidents(),
    staleTime: 30 * 1000,
  });
}

export function useMetrics() {
  return useQuery({
    queryKey: ['observability', 'metrics'],
    queryFn: () => mockFetchMetrics(),
    staleTime: 60 * 1000,
  });
}
