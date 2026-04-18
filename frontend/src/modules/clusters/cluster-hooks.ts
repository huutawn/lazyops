import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { connectClusterNodeSSH, createCluster, listClusterNodes, listClusters } from '@/modules/clusters/cluster-api';
import type {
  ClusterNodeListResponse,
  ConnectClusterNodeSSHRequest,
  ConnectClusterNodeSSHResponse,
  CreateClusterFormData,
  ClusterSummary,
  ClusterListResponse,
} from '@/modules/clusters/cluster-types';

const CLUSTERS_KEY = ['clusters', 'list'];

export function clusterNodesQueryKey(clusterId: string) {
  return ['clusters', clusterId, 'nodes'] as const;
}

export function useClusters() {
  return useQuery({
    queryKey: CLUSTERS_KEY,
    queryFn: async () => {
      const result = await listClusters();
      if (result.error) throw new Error(result.error.message);
      return result.data as ClusterListResponse;
    },
    staleTime: 30 * 1000,
  });
}

export function useCreateCluster() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (data: CreateClusterFormData) => createCluster(data),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: CLUSTERS_KEY });
    },
  });
}

export function useClusterNodes(clusterId: string) {
  return useQuery({
    queryKey: clusterNodesQueryKey(clusterId),
    queryFn: async (): Promise<ClusterNodeListResponse> => {
      const result = await listClusterNodes(clusterId);
      if (result.error) throw new Error(result.error.message);
      return result.data ?? { items: [] };
    },
    enabled: !!clusterId,
    staleTime: 15 * 1000,
    refetchInterval: 20 * 1000,
  });
}

export function useConnectClusterNode(clusterId: string) {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async (data: ConnectClusterNodeSSHRequest): Promise<ConnectClusterNodeSSHResponse> => {
      const result = await connectClusterNodeSSH(clusterId, data);
      if (result.error) {
        throw new Error(result.error.message);
      }
      if (!result.data) {
        throw new Error('Failed to connect node to cluster');
      }
      return result.data;
    },
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: CLUSTERS_KEY });
      void queryClient.invalidateQueries({ queryKey: clusterNodesQueryKey(clusterId) });
    },
  });
}
