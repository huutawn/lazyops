import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { createMeshNetwork, listMeshNetworks } from '@/modules/mesh-networks/mesh-network-api';
import type { CreateMeshNetworkFormData, MeshNetworkSummary, MeshNetworkListResponse } from '@/modules/mesh-networks/mesh-network-types';

const MESH_NETWORKS_KEY = ['mesh-networks', 'list'];

export function useMeshNetworks() {
  return useQuery({
    queryKey: MESH_NETWORKS_KEY,
    queryFn: async () => {
      const result = await listMeshNetworks();
      if (result.error) throw new Error(result.error.message);
      return result.data as MeshNetworkListResponse;
    },
    staleTime: 30 * 1000,
  });
}

export function useCreateMeshNetwork() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (data: CreateMeshNetworkFormData) => createMeshNetwork(data),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: MESH_NETWORKS_KEY });
    },
  });
}
