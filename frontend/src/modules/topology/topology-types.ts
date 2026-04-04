export type TopologyNodeType = 'instance' | 'mesh_network' | 'cluster' | 'service';
export type TopologyHealth = 'healthy' | 'degraded' | 'unhealthy' | 'offline' | 'unknown';
export type TopologyEdgeKind = 'dependency' | 'mesh_peer' | 'routing';
export type TopologyEdgeProtocol = 'http' | 'https' | 'tcp' | 'grpc' | 'tls' | 'amqp';

export type TopologyNodeRecord = {
  id: string;
  project_id: string;
  node_kind: TopologyNodeType;
  node_ref: string;
  name: string;
  status: TopologyHealth;
  metadata: Record<string, unknown>;
  updated_at: string;
};

export type TopologyEdgeRecord = {
  id: string;
  project_id: string;
  source_id: string;
  target_id: string;
  edge_kind: TopologyEdgeKind;
  protocol: TopologyEdgeProtocol;
  metadata: Record<string, unknown>;
};

export type TopologyGraphResponse = {
  project_id: string;
  nodes: TopologyNodeRecord[];
  edges: TopologyEdgeRecord[];
};

export type TopologyDisplayNode = {
  id: string;
  kind: TopologyNodeType;
  label: string;
  status: TopologyHealth;
  runtime_mode?: string;
  metadata: Record<string, unknown>;
  position?: { x: number; y: number };
};

export type TopologyDisplayEdge = {
  id: string;
  source: string;
  target: string;
  label: string;
  latency_ms?: number;
  health: TopologyHealth;
  protocol: TopologyEdgeProtocol;
  edge_kind: TopologyEdgeKind;
};

export type TopologyDisplayConfig = {
  showLatency: boolean;
  showHealth: boolean;
  showLabels: boolean;
  layout: 'hierarchical' | 'force' | 'grid';
  nodeSize: 'sm' | 'md' | 'lg';
};

export const DEFAULT_DISPLAY_CONFIG: TopologyDisplayConfig = {
  showLatency: true,
  showHealth: true,
  showLabels: true,
  layout: 'hierarchical',
  nodeSize: 'md',
};

export const RUNTIME_MODE_DISPLAY_RULES: Record<string, Partial<TopologyDisplayConfig>> = {
  standalone: {
    layout: 'grid',
    showLatency: false,
    nodeSize: 'lg',
  },
  'distributed-mesh': {
    layout: 'force',
    showLatency: true,
    showHealth: true,
    nodeSize: 'md',
  },
  'distributed-k3s': {
    layout: 'hierarchical',
    showLatency: true,
    showHealth: true,
    nodeSize: 'sm',
  },
};
