import { useQuery } from '@tanstack/react-query';
import { mockFetchTopology } from '@/modules/topology/topology-mocks';
import { fetchLiveTopology } from '@/modules/topology/topology-api';
import { adaptTopology } from '@/modules/topology/topology-adapter';
import type { TopologyAdapterResult } from '@/modules/topology/topology-adapter';
import type { TopologyDisplayConfig } from '@/modules/topology/topology-types';
import { RUNTIME_MODE_DISPLAY_RULES, DEFAULT_DISPLAY_CONFIG } from '@/modules/topology/topology-types';

export function topologyQueryKey(projectId?: string) {
  return ['topology', projectId ?? 'all'] as const;
}

const USE_MOCK = process.env.NEXT_PUBLIC_MOCK_MODE === 'true';

export function useTopology(projectId?: string) {
  return useQuery({
    queryKey: topologyQueryKey(projectId),
    queryFn: async (): Promise<TopologyAdapterResult> => {
      if (USE_MOCK) {
        const raw = await mockFetchTopology(projectId);
        return adaptTopology(raw, 'mock');
      }

      if (!projectId) {
        const raw = await mockFetchTopology();
        return adaptTopology(raw, 'mock');
      }

      const raw = await fetchLiveTopology(projectId);
      if (!raw) {
        const fallback = await mockFetchTopology(projectId);
        return adaptTopology(fallback, 'mock');
      }
      return adaptTopology(raw, 'live');
    },
    staleTime: 60 * 1000,
  });
}

export function useTopologyDisplayConfig(runtimeMode: string): TopologyDisplayConfig {
  const modeRules = RUNTIME_MODE_DISPLAY_RULES[runtimeMode] ?? {};
  return { ...DEFAULT_DISPLAY_CONFIG, ...modeRules } as TopologyDisplayConfig;
}
