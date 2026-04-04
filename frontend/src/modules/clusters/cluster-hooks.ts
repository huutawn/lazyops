import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { createCluster, listClusters } from '@/modules/clusters/cluster-api';
import type { CreateClusterFormData, ClusterSummary, ClusterListResponse } from '@/modules/clusters/cluster-types';

const CLUSTERS_KEY = ['clusters', 'list'];

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
