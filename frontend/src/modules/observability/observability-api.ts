import { apiGet } from '@/lib/api/api-client';
import type { ApiResponse } from '@/lib/types';
import type { Incident, LogEntry, MetricRecord, TraceDetail } from '@/modules/observability/observability-types';

export type ObservabilityLogListResponse = {
  items: LogEntry[];
};

export type ObservabilityIncidentListResponse = {
  items: Incident[];
};

export type ObservabilityMetricListResponse = {
  items: MetricRecord[];
};

export async function listProjectLogs(
  projectID: string,
  filters: { service?: string; level?: string; contains?: string; limit?: number } = {},
): Promise<ApiResponse<ObservabilityLogListResponse>> {
  const params: Record<string, string> = {};
  if (filters.service) params.service = filters.service;
  if (filters.level) params.level = filters.level;
  if (filters.contains) params.contains = filters.contains;
  if (filters.limit && filters.limit > 0) params.limit = String(filters.limit);

  return apiGet<ObservabilityLogListResponse>(`/projects/${projectID}/observability/logs`, { params });
}

export async function listProjectIncidents(projectID: string): Promise<ApiResponse<ObservabilityIncidentListResponse>> {
  return apiGet<ObservabilityIncidentListResponse>(`/projects/${projectID}/observability/incidents`);
}

export async function listProjectMetrics(
  projectID: string,
  limit?: number,
): Promise<ApiResponse<ObservabilityMetricListResponse>> {
  const params: Record<string, string> = {};
  if (limit && limit > 0) {
    params.limit = String(limit);
  }
  return apiGet<ObservabilityMetricListResponse>(`/projects/${projectID}/observability/metrics`, { params });
}

export async function getTraceByCorrelationID(correlationID: string): Promise<ApiResponse<TraceDetail>> {
  return apiGet<TraceDetail>(`/traces/${correlationID}`);
}
