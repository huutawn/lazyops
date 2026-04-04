import { z } from 'zod';

export const userSchema = z.object({
  id: z.string(),
  display_name: z.string(),
  email: z.string().email(),
  role: z.string(),
  status: z.string(),
});

export const projectSchema = z.object({
  id: z.string(),
  name: z.string(),
  slug: z.string(),
  default_branch: z.string(),
  created_at: z.string().datetime(),
  updated_at: z.string().datetime(),
});

export const projectListSchema = z.object({
  items: z.array(projectSchema),
});

export const instanceSchema = z.object({
  id: z.string(),
  name: z.string(),
  status: z.enum(['online', 'offline', 'degraded']),
  public_ip: z.string(),
  private_ip: z.string(),
  labels: z.array(z.string()),
  agent_version: z.string(),
  last_heartbeat: z.string().datetime(),
});

export const instanceListSchema = z.object({
  items: z.array(instanceSchema),
});

export const meshNetworkSchema = z.object({
  id: z.string(),
  name: z.string(),
  provider: z.string(),
  cidr: z.string(),
  status: z.enum(['active', 'inactive', 'error']),
  node_count: z.number(),
  created_at: z.string().datetime(),
});

export const meshNetworkListSchema = z.object({
  items: z.array(meshNetworkSchema),
});

export const clusterSchema = z.object({
  id: z.string(),
  name: z.string(),
  provider: z.string(),
  status: z.enum(['ready', 'provisioning', 'error']),
  target_count: z.number(),
  created_at: z.string().datetime(),
});

export const clusterListSchema = z.object({
  items: z.array(clusterSchema),
});

export const deploymentBindingSchema = z.object({
  id: z.string(),
  project_id: z.string(),
  project_slug: z.string(),
  target_kind: z.enum(['instance', 'mesh', 'cluster']),
  target_id: z.string(),
  target_name: z.string(),
  runtime_mode: z.enum(['standalone', 'distributed-mesh', 'distributed-k3s']),
  placement_policy: z.string(),
  domain_policy: z.string(),
  scale_to_zero: z.boolean(),
  created_at: z.string().datetime(),
});

export const deploymentBindingListSchema = z.object({
  items: z.array(deploymentBindingSchema),
});

export const deploymentSchema = z.object({
  id: z.string(),
  project_id: z.string(),
  binding_id: z.string(),
  revision: z.number(),
  build_state: z.enum(['pending', 'building', 'built', 'failed']),
  rollout_state: z.enum(['pending', 'rolling', 'completed', 'paused', 'failed', 'rolled_back']),
  promoted: z.boolean(),
  triggered_by: z.string(),
  created_at: z.string().datetime(),
  completed_at: z.string().datetime().nullable(),
});

export const deploymentListSchema = z.object({
  items: z.array(deploymentSchema),
});

export const logEntrySchema = z.object({
  id: z.string(),
  deployment_id: z.string(),
  service: z.string(),
  level: z.enum(['info', 'warn', 'error', 'debug']),
  message: z.string(),
  timestamp: z.string().datetime(),
});

export const logEntryListSchema = z.object({
  items: z.array(logEntrySchema),
});

export const traceSpanSchema = z.object({
  service: z.string(),
  operation: z.string(),
  duration_ms: z.number(),
  status: z.enum(['ok', 'error']),
});

export const traceSchema = z.object({
  trace_id: z.string(),
  correlation_id: z.string(),
  service: z.string(),
  operation: z.string(),
  duration_ms: z.number(),
  status: z.enum(['ok', 'error']),
  timestamp: z.string().datetime(),
  spans: z.array(traceSpanSchema),
});

export const traceListSchema = z.object({
  items: z.array(traceSchema),
});

export const metricSchema = z.object({
  service: z.string(),
  cpu_p95: z.number(),
  cpu_max: z.number(),
  cpu_min: z.number(),
  cpu_avg: z.number(),
  ram_p95: z.number(),
  ram_max: z.number(),
  ram_min: z.number(),
  ram_avg: z.number(),
  request_count: z.number(),
  period: z.string(),
});

export const metricListSchema = z.object({
  items: z.array(metricSchema),
});

export const topologyNodeSchema = z.object({
  id: z.string(),
  kind: z.enum(['target', 'service']),
  label: z.string(),
  status: z.enum(['healthy', 'degraded', 'unhealthy', 'offline']),
  runtime_mode: z.string().optional(),
  metadata: z.record(z.string(), z.string()).optional(),
});

export const topologyEdgeSchema = z.object({
  source: z.string(),
  target: z.string(),
  label: z.string(),
  latency_ms: z.number(),
  health: z.enum(['healthy', 'degraded', 'unhealthy']),
  protocol: z.string(),
});

export const topologySchema = z.object({
  nodes: z.array(topologyNodeSchema),
  edges: z.array(topologyEdgeSchema),
});
