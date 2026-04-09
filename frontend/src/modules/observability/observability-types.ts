export type LogLevel = 'info' | 'warn' | 'error' | 'debug';

export type LogEntry = {
  id: string;
  deployment_id?: string;
  service: string;
  revision_id?: string;
  correlation_id?: string;
  level: LogLevel;
  message: string;
  timestamp: string;
  node?: string;
};

export type TraceSpan = {
  service: string;
  operation: string;
  duration_ms: number;
  status: 'ok' | 'error';
};

export type TraceDetail = {
  correlation_id: string;
  service_path: string[];
  node_hops: string[];
  latency_hotspot: string;
  total_latency_ms: number;
};

export type IncidentSeverity = 'critical' | 'warning' | 'info';
export type IncidentStatus = 'open' | 'investigating' | 'resolved' | 'dismissed';

export type Incident = {
  id: string;
  project_id: string;
  deployment_id: string;
  revision_id: string;
  kind: string;
  severity: IncidentSeverity;
  status: IncidentStatus;
  summary: string;
  details: Record<string, unknown>;
  triggered_by: string;
  resolved_at: string | null;
  created_at: string;
};

export type MetricRecord = {
  service: string;
  cpu_p95: number;
  cpu_max: number;
  cpu_min: number;
  cpu_avg: number;
  ram_p95: number;
  ram_max: number;
  ram_min: number;
  ram_avg: number;
  request_count: number;
  period: string;
};
