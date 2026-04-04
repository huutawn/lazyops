import { apiGet } from '@/lib/api/api-client';
import type { ApiResponse } from '@/lib/types';
import type { TopologyGraphResponse, TopologyDisplayNode, TopologyDisplayEdge, TopologyHealth } from '@/modules/topology/topology-types';

function nodeKindToRuntimeMode(kind: string): string | undefined {
  switch (kind) {
    case 'mesh_network': return 'distributed-mesh';
    case 'cluster': return 'distributed-k3s';
    case 'instance': return 'standalone';
    default: return undefined;
  }
}

function normalizeStatus(raw: string): TopologyHealth {
  const valid: TopologyHealth[] = ['healthy', 'degraded', 'unhealthy', 'offline'];
  return valid.includes(raw as TopologyHealth) ? (raw as TopologyHealth) : 'unknown';
}

export function normalizeTopologyResponse(raw: TopologyGraphResponse): {
  nodes: TopologyDisplayNode[];
  edges: TopologyDisplayEdge[];
  runtime_mode: string;
} {
  const nodes: TopologyDisplayNode[] = raw.nodes.map((n) => ({
    id: n.id,
    kind: n.node_kind,
    label: n.name,
    status: normalizeStatus(n.status),
    runtime_mode: nodeKindToRuntimeMode(n.node_kind),
    metadata: n.metadata,
  }));

  const edges: TopologyDisplayEdge[] = raw.edges.map((e) => ({
    id: e.id,
    source: e.source_id,
    target: e.target_id,
    label: e.edge_kind,
    health: 'healthy',
    protocol: e.protocol,
    edge_kind: e.edge_kind,
  }));

  const runtime_mode = detectRuntimeMode(raw.nodes);

  return { nodes, edges, runtime_mode };
}

function detectRuntimeMode(nodes: TopologyGraphResponse['nodes']): string {
  const kinds = new Set(nodes.map((n) => n.node_kind));
  if (kinds.has('cluster')) return 'distributed-k3s';
  if (kinds.has('mesh_network')) return 'distributed-mesh';
  return 'standalone';
}

export async function fetchLiveTopology(projectId: string): Promise<TopologyGraphResponse | null> {
  const result = await apiGet<TopologyGraphResponse>(`/projects/${projectId}/topology`);
  if (result.error) return null;
  return result.data as TopologyGraphResponse;
}
