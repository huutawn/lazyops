import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { createDeploymentBinding, listDeploymentBindings } from '@/modules/deployment-bindings/binding-api';
import type { CreateDeploymentBindingFormData, DeploymentBindingSummary, DeploymentBindingListResponse } from '@/modules/deployment-bindings/binding-types';

export function bindingsQueryKey(projectId: string) {
  return ['deployment-bindings', projectId] as const;
}

export function useDeploymentBindings(projectId: string) {
  return useQuery({
    queryKey: bindingsQueryKey(projectId),
    queryFn: async () => {
      const result = await listDeploymentBindings(projectId);
      if (result.error) throw new Error(result.error.message);
      return result.data as DeploymentBindingListResponse;
    },
    enabled: !!projectId,
    staleTime: 30 * 1000,
  });
}

export function useCreateDeploymentBinding(projectId: string) {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (data: CreateDeploymentBindingFormData) => createDeploymentBinding(projectId, data),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: bindingsQueryKey(projectId) });
    },
  });
}
