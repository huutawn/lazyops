export type ProjectFixture = {
  id: string;
  name: string;
  slug: string;
  default_branch: string;
  created_at: string;
  updated_at: string;
};

export type InstanceFixture = {
  id: string;
  name: string;
  status: 'online' | 'offline' | 'degraded';
  public_ip: string;
  private_ip: string;
  labels: string[];
  agent_version: string;
  last_heartbeat: string;
};

export type MeshNetworkFixture = {
  id: string;
  name: string;
  provider: string;
  cidr: string;
  status: 'active' | 'inactive' | 'error';
  node_count: number;
  created_at: string;
};

export type ClusterFixture = {
  id: string;
  name: string;
  provider: string;
  status: 'ready' | 'provisioning' | 'error';
  target_count: number;
  created_at: string;
};

export type DeploymentBindingFixture = {
  id: string;
  project_id: string;
  project_slug: string;
  target_kind: 'instance' | 'mesh' | 'cluster';
  target_id: string;
  target_name: string;
  runtime_mode: 'standalone' | 'distributed-mesh' | 'distributed-k3s';
  placement_policy: string;
  domain_policy: string;
  scale_to_zero: boolean;
  created_at: string;
};

export type DeploymentFixture = {
  id: string;
  project_id: string;
  binding_id: string;
  revision: number;
  build_state: 'pending' | 'building' | 'built' | 'failed';
  rollout_state: 'pending' | 'rolling' | 'completed' | 'paused' | 'failed' | 'rolled_back';
  promoted: boolean;
  triggered_by: string;
  created_at: string;
  completed_at: string | null;
};

export type LogEntryFixture = {
  id: string;
  deployment_id: string;
  service: string;
  level: 'info' | 'warn' | 'error' | 'debug';
  message: string;
  timestamp: string;
};

export type TraceFixture = {
  trace_id: string;
  correlation_id: string;
  service: string;
  operation: string;
  duration_ms: number;
  status: 'ok' | 'error';
  timestamp: string;
  spans: {
    service: string;
    operation: string;
    duration_ms: number;
    status: 'ok' | 'error';
  }[];
};

export type MetricFixture = {
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

export type TopologyNodeFixture = {
  id: string;
  kind: 'target' | 'service';
  label: string;
  status: 'healthy' | 'degraded' | 'unhealthy' | 'offline';
  runtime_mode?: string;
  metadata?: Record<string, string>;
};

export type TopologyEdgeFixture = {
  source: string;
  target: string;
  label: string;
  latency_ms: number;
  health: 'healthy' | 'degraded' | 'unhealthy';
  protocol: string;
};
