import type {
  ProjectFixture,
  InstanceFixture,
  MeshNetworkFixture,
  ClusterFixture,
  DeploymentBindingFixture,
  DeploymentFixture,
  LogEntryFixture,
  TraceFixture,
  MetricFixture,
  TopologyNodeFixture,
  TopologyEdgeFixture,
} from '@/lib/mock-fixtures';

export const projects: ProjectFixture[] = [
  {
    id: 'proj_01',
    name: 'E-Commerce Platform',
    slug: 'ecommerce-platform',
    default_branch: 'main',
    created_at: '2026-03-15T10:00:00Z',
    updated_at: '2026-03-20T14:30:00Z',
  },
  {
    id: 'proj_02',
    name: 'Analytics Dashboard',
    slug: 'analytics-dashboard',
    default_branch: 'main',
    created_at: '2026-03-18T08:00:00Z',
    updated_at: '2026-03-18T08:00:00Z',
  },
];

export const instances: InstanceFixture[] = [
  {
    id: 'inst_01',
    name: 'prod-web-01',
    status: 'online',
    public_ip: '203.0.113.10',
    private_ip: '10.0.1.10',
    labels: ['env:prod', 'role:web'],
    agent_version: '1.2.0',
    last_heartbeat: '2026-04-04T12:00:00Z',
  },
  {
    id: 'inst_02',
    name: 'prod-api-01',
    status: 'online',
    public_ip: '203.0.113.11',
    private_ip: '10.0.1.11',
    labels: ['env:prod', 'role:api'],
    agent_version: '1.2.0',
    last_heartbeat: '2026-04-04T11:59:30Z',
  },
  {
    id: 'inst_03',
    name: 'staging-web-01',
    status: 'offline',
    public_ip: '203.0.113.20',
    private_ip: '10.0.2.10',
    labels: ['env:staging', 'role:web'],
    agent_version: '1.1.0',
    last_heartbeat: '2026-04-03T18:00:00Z',
  },
];

export const meshNetworks: MeshNetworkFixture[] = [
  {
    id: 'mesh_01',
    name: 'prod-mesh',
    provider: 'tailscale',
    cidr: '100.64.0.0/16',
    status: 'active',
    node_count: 5,
    created_at: '2026-03-10T09:00:00Z',
  },
  {
    id: 'mesh_02',
    name: 'staging-mesh',
    provider: 'tailscale',
    cidr: '100.65.0.0/16',
    status: 'active',
    node_count: 3,
    created_at: '2026-03-12T11:00:00Z',
  },
];

export const clusters: ClusterFixture[] = [
  {
    id: 'clust_01',
    name: 'prod-k3s',
    provider: 'k3s',
    status: 'ready',
    target_count: 3,
    created_at: '2026-03-20T10:00:00Z',
  },
];

export const deploymentBindings: DeploymentBindingFixture[] = [
  {
    id: 'bind_01',
    project_id: 'proj_01',
    project_slug: 'ecommerce-platform',
    target_kind: 'instance',
    target_id: 'inst_01',
    target_name: 'prod-web-01',
    runtime_mode: 'standalone',
    placement_policy: 'single-target',
    domain_policy: 'auto',
    scale_to_zero: false,
    created_at: '2026-03-16T10:00:00Z',
  },
  {
    id: 'bind_02',
    project_id: 'proj_01',
    project_slug: 'ecommerce-platform',
    target_kind: 'mesh',
    target_id: 'mesh_01',
    target_name: 'prod-mesh',
    runtime_mode: 'distributed-mesh',
    placement_policy: 'spread',
    domain_policy: 'magic',
    scale_to_zero: true,
    created_at: '2026-03-17T10:00:00Z',
  },
];

export const deployments: DeploymentFixture[] = [
  {
    id: 'dep_01',
    project_id: 'proj_01',
    binding_id: 'bind_01',
    revision: 12,
    build_state: 'built',
    rollout_state: 'completed',
    promoted: true,
    triggered_by: 'alice@example.com',
    created_at: '2026-04-03T09:00:00Z',
    completed_at: '2026-04-03T09:05:30Z',
  },
  {
    id: 'dep_02',
    project_id: 'proj_01',
    binding_id: 'bind_01',
    revision: 13,
    build_state: 'built',
    rollout_state: 'rolling',
    promoted: false,
    triggered_by: 'bob@example.com',
    created_at: '2026-04-04T11:00:00Z',
    completed_at: null,
  },
  {
    id: 'dep_03',
    project_id: 'proj_02',
    binding_id: 'bind_02',
    revision: 3,
    build_state: 'failed',
    rollout_state: 'failed',
    promoted: false,
    triggered_by: 'alice@example.com',
    created_at: '2026-04-04T08:00:00Z',
    completed_at: '2026-04-04T08:02:15Z',
  },
];

export const logEntries: LogEntryFixture[] = [
  {
    id: 'log_01',
    deployment_id: 'dep_01',
    service: 'web',
    level: 'info',
    message: 'Service started on port 8080',
    timestamp: '2026-04-03T09:01:00Z',
  },
  {
    id: 'log_02',
    deployment_id: 'dep_01',
    service: 'web',
    level: 'info',
    message: 'Health check passed',
    timestamp: '2026-04-03T09:02:00Z',
  },
  {
    id: 'log_03',
    deployment_id: 'dep_02',
    service: 'web',
    level: 'warn',
    message: 'High memory usage detected: 85%',
    timestamp: '2026-04-04T11:03:00Z',
  },
  {
    id: 'log_04',
    deployment_id: 'dep_03',
    service: 'api',
    level: 'error',
    message: 'Failed to bind to port 3000: address already in use',
    timestamp: '2026-04-04T08:01:30Z',
  },
];

export const traces: TraceFixture[] = [
  {
    trace_id: 'trace_01',
    correlation_id: 'corr_abc123',
    service: 'web',
    operation: 'GET /api/products',
    duration_ms: 145,
    status: 'ok',
    timestamp: '2026-04-04T10:00:00Z',
    spans: [
      { service: 'web', operation: 'GET /api/products', duration_ms: 145, status: 'ok' },
      { service: 'api', operation: 'fetch_products', duration_ms: 89, status: 'ok' },
      { service: 'db', operation: 'SELECT products', duration_ms: 12, status: 'ok' },
    ],
  },
  {
    trace_id: 'trace_02',
    correlation_id: 'corr_def456',
    service: 'api',
    operation: 'POST /api/orders',
    duration_ms: 2340,
    status: 'error',
    timestamp: '2026-04-04T10:05:00Z',
    spans: [
      { service: 'web', operation: 'POST /api/orders', duration_ms: 2340, status: 'error' },
      { service: 'api', operation: 'create_order', duration_ms: 2300, status: 'error' },
      { service: 'db', operation: 'INSERT orders', duration_ms: 45, status: 'ok' },
      { service: 'payments', operation: 'charge', duration_ms: 2100, status: 'error' },
    ],
  },
];

export const metrics: MetricFixture[] = [
  {
    service: 'web',
    cpu_p95: 45.2,
    cpu_max: 78.1,
    cpu_min: 5.3,
    cpu_avg: 28.7,
    ram_p95: 512,
    ram_max: 768,
    ram_min: 128,
    ram_avg: 384,
    request_count: 15420,
    period: '24h',
  },
  {
    service: 'api',
    cpu_p95: 62.8,
    cpu_max: 95.4,
    cpu_min: 12.1,
    cpu_avg: 41.3,
    ram_p95: 1024,
    ram_max: 1536,
    ram_min: 256,
    ram_avg: 768,
    request_count: 28900,
    period: '24h',
  },
  {
    service: 'worker',
    cpu_p95: 15.3,
    cpu_max: 30.2,
    cpu_min: 0.5,
    cpu_avg: 8.1,
    ram_p95: 256,
    ram_max: 384,
    ram_min: 64,
    ram_avg: 192,
    request_count: 0,
    period: '24h',
  },
];

export const topologyNodes: TopologyNodeFixture[] = [
  { id: 'node_inst_01', kind: 'target', label: 'prod-web-01', status: 'healthy', runtime_mode: 'standalone' },
  { id: 'node_inst_02', kind: 'target', label: 'prod-api-01', status: 'healthy', runtime_mode: 'standalone' },
  { id: 'node_svc_web', kind: 'service', label: 'web', status: 'healthy' },
  { id: 'node_svc_api', kind: 'service', label: 'api', status: 'degraded' },
  { id: 'node_svc_db', kind: 'service', label: 'db', status: 'healthy' },
  { id: 'node_svc_worker', kind: 'service', label: 'worker', status: 'healthy' },
];

export const topologyEdges: TopologyEdgeFixture[] = [
  { source: 'node_svc_web', target: 'node_svc_api', label: 'HTTP', latency_ms: 12, health: 'healthy', protocol: 'http' },
  { source: 'node_svc_api', target: 'node_svc_db', label: 'TCP', latency_ms: 3, health: 'healthy', protocol: 'tcp' },
  { source: 'node_svc_web', target: 'node_svc_worker', label: 'async', latency_ms: 45, health: 'degraded', protocol: 'amqp' },
];
