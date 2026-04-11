import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { updateProjectRouting, getProjectRouting } from '@/modules/project-routing/project-routing-api';
import type {
  ProjectRoutingResponse,
  UpdateRoutingPolicyRequest,
} from '@/modules/project-routing/project-routing-types';

export function projectRoutingQueryKey(projectId: string) {
  return ['project-routing', projectId] as const;
}

export function useProjectRouting(projectId: string) {
  return useQuery({
    queryKey: projectRoutingQueryKey(projectId),
    queryFn: async (): Promise<ProjectRoutingResponse> => {
      const result = await getProjectRouting(projectId);
      if (result.error) {
        throw new Error(result.error.message);
      }
      if (!result.data) {
        throw new Error('Routing configuration unavailable');
      }
      return result.data;
    },
    enabled: !!projectId,
    staleTime: 20 * 1000,
  });
}

export function useUpdateProjectRouting(projectId: string) {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async (data: UpdateRoutingPolicyRequest): Promise<ProjectRoutingResponse> => {
      const result = await updateProjectRouting(projectId, data);
      if (result.error) {
        throw new Error(result.error.message);
      }
      if (!result.data) {
        throw new Error('Failed to update routing configuration');
      }
      return result.data;
    },
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: projectRoutingQueryKey(projectId) });
      void queryClient.invalidateQueries({ queryKey: ['bootstrap-status', projectId] });
    },
  });
}
