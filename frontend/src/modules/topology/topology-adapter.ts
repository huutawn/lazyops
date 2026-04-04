import type { TopologyGraphResponse, TopologyDisplayNode, TopologyDisplayEdge, TopologyDisplayConfig } from '@/modules/topology/topology-types';
import { RUNTIME_MODE_DISPLAY_RULES, DEFAULT_DISPLAY_CONFIG } from '@/modules/topology/topology-types';
import { normalizeTopologyResponse } from '@/modules/topology/topology-api';

export type TopologyAdapterResult = {
  nodes: TopologyDisplayNode[];
  edges: TopologyDisplayEdge[];
  runtime_mode: string;
  displayConfig: TopologyDisplayConfig;
  source: 'mock' | 'live';
};

export async function adaptTopology(raw: TopologyGraphResponse, source: 'mock' | 'live'): Promise<TopologyAdapterResult> {
  const { nodes, edges, runtime_mode } = normalizeTopologyResponse(raw);
  const displayConfig = {
    ...DEFAULT_DISPLAY_CONFIG,
    ...(RUNTIME_MODE_DISPLAY_RULES[runtime_mode] ?? {}),
  } as TopologyDisplayConfig;

  return { nodes, edges, runtime_mode, displayConfig, source };
}
