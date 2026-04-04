import type { TopologyGraphResponse, TopologyDisplayNode, TopologyDisplayEdge } from '@/modules/topology/topology-types';

const MOCK_TOPOLOGY: Record<string, TopologyGraphResponse> = {
  default: {
    project_id: 'proj_01',
    nodes: [
      { id: 'tn_01', project_id: 'proj_01', node_kind: 'instance', node_ref: 'inst_01', name: 'prod-web-01', status: 'healthy', metadata: { ip: '10.0.1.10' }, updated_at: '2026-04-04T12:00:00Z' },
      { id: 'tn_02', project_id: 'proj_01', node_kind: 'service', node_ref: 'svc_web', name: 'web', status: 'healthy', metadata: { port: '8080' }, updated_at: '2026-04-04T12:00:00Z' },
    ],
    edges: [
      { id: 'te_01', project_id: 'proj_01', source_id: 'tn_01', target_id: 'tn_02', edge_kind: 'dependency', protocol: 'http', metadata: {} },
    ],
  },
};

export async function mockFetchTopology(projectId?: string): Promise<TopologyGraphResponse> {
  await new Promise((r) => setTimeout(r, 400));
  return MOCK_TOPOLOGY[projectId ?? 'default'] ?? MOCK_TOPOLOGY.default;
}
