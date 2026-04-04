import type { LogEntry, TraceDetail, Incident, MetricRecord } from '@/modules/observability/observability-types';

const MOCK_LOGS: LogEntry[] = [
  { id: 'log_01', deployment_id: 'dep_001', service: 'web', level: 'info', message: 'Service started on port 8080', timestamp: '2026-04-04T10:00:00Z' },
  { id: 'log_02', deployment_id: 'dep_001', service: 'web', level: 'info', message: 'Health check passed', timestamp: '2026-04-04T10:01:00Z' },
  { id: 'log_03', deployment_id: 'dep_002', service: 'api', level: 'warn', message: 'High memory usage detected: 85%', timestamp: '2026-04-04T11:00:00Z' },
  { id: 'log_04', deployment_id: 'dep_002', service: 'api', level: 'error', message: 'Connection timeout to database', timestamp: '2026-04-04T11:05:00Z' },
  { id: 'log_05', deployment_id: 'dep_001', service: 'web', level: 'debug', message: 'Request processed in 45ms', timestamp: '2026-04-04T10:02:00Z' },
  { id: 'log_06', deployment_id: 'dep_003', service: 'worker', level: 'info', message: 'Job completed: process-orders', timestamp: '2026-04-04T09:30:00Z' },
];

const MOCK_TRACES: Record<string, TraceDetail> = {
  'corr_abc123': {
    correlation_id: 'corr_abc123',
    service_path: ['web', 'api', 'db'],
    node_hops: ['prod-web-01', 'prod-api-01', 'postgres-primary'],
    latency_hotspot: 'api',
    total_latency_ms: 145,
  },
  'corr_def456': {
    correlation_id: 'corr_def456',
    service_path: ['web', 'api', 'payments', 'db'],
    node_hops: ['prod-web-01', 'prod-api-01', 'payments-svc', 'postgres-primary'],
    latency_hotspot: 'payments',
    total_latency_ms: 2340,
  },
};

const MOCK_INCIDENTS: Incident[] = [
  {
    id: 'inc_001',
    project_id: 'proj_01',
    deployment_id: 'dep_002',
    revision_id: 'rev_013',
    kind: 'high_latency',
    severity: 'warning',
    status: 'open',
    summary: 'API response time exceeded 2s threshold',
    details: { service: 'api', threshold_ms: 2000, actual_ms: 2340 },
    triggered_by: 'system',
    resolved_at: null,
    created_at: '2026-04-04T11:05:00Z',
  },
  {
    id: 'inc_002',
    project_id: 'proj_01',
    deployment_id: 'dep_003',
    revision_id: 'rev_003',
    kind: 'deployment_failure',
    severity: 'critical',
    status: 'investigating',
    summary: 'Deployment dep_003 failed during rollout',
    details: { error: 'port already in use', service: 'api' },
    triggered_by: 'system',
    resolved_at: null,
    created_at: '2026-04-04T08:02:15Z',
  },
  {
    id: 'inc_003',
    project_id: 'proj_01',
    deployment_id: 'dep_001',
    revision_id: 'rev_012',
    kind: 'high_memory',
    severity: 'info',
    status: 'resolved',
    summary: 'Memory usage spike resolved automatically',
    details: { service: 'web', peak_percent: 85, current_percent: 62 },
    triggered_by: 'system',
    resolved_at: '2026-04-04T10:30:00Z',
    created_at: '2026-04-04T10:15:00Z',
  },
];

const MOCK_METRICS: MetricRecord[] = [
  { service: 'web', cpu_p95: 45.2, cpu_max: 78.1, cpu_min: 5.3, cpu_avg: 28.7, ram_p95: 512, ram_max: 768, ram_min: 128, ram_avg: 384, request_count: 15420, period: '24h' },
  { service: 'api', cpu_p95: 62.8, cpu_max: 95.4, cpu_min: 12.1, cpu_avg: 41.3, ram_p95: 1024, ram_max: 1536, ram_min: 256, ram_avg: 768, request_count: 28900, period: '24h' },
  { service: 'worker', cpu_p95: 15.3, cpu_max: 30.2, cpu_min: 0.5, cpu_avg: 8.1, ram_p95: 256, ram_max: 384, ram_min: 64, ram_avg: 192, request_count: 0, period: '24h' },
];

export async function mockFetchLogs(_deploymentId?: string, _level?: string): Promise<LogEntry[]> {
  await new Promise((r) => setTimeout(r, 300));
  return MOCK_LOGS;
}

export async function mockFetchTrace(correlationId: string): Promise<TraceDetail | null> {
  await new Promise((r) => setTimeout(r, 200));
  return MOCK_TRACES[correlationId] ?? null;
}

export async function mockFetchIncidents(): Promise<Incident[]> {
  await new Promise((r) => setTimeout(r, 300));
  return MOCK_INCIDENTS;
}

export async function mockFetchMetrics(): Promise<MetricRecord[]> {
  await new Promise((r) => setTimeout(r, 400));
  return MOCK_METRICS;
}
