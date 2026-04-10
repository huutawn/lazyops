import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { configureProjectInternalServices, getProjectInternalServices } from '@/modules/internal-services/internal-service-api';
import type {
  ConfigureProjectInternalServicesRequest,
  ProjectInternalServiceListResponse,
} from '@/modules/internal-services/internal-service-types';

export function projectInternalServicesQueryKey(projectId: string) {
  return ['project-internal-services', projectId] as const;
}

export function useProjectInternalServices(projectId: string) {
  return useQuery({
    queryKey: projectInternalServicesQueryKey(projectId),
    queryFn: async (): Promise<ProjectInternalServiceListResponse> => {
      const result = await getProjectInternalServices(projectId);
      if (result.error) {
        throw new Error(result.error.message);
      }
      if (!result.data) {
        throw new Error('Internal services unavailable');
      }
      return result.data;
    },
    enabled: !!projectId,
    staleTime: 20 * 1000,
  });
}

export function useConfigureProjectInternalServices(projectId: string) {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async (data: ConfigureProjectInternalServicesRequest): Promise<ProjectInternalServiceListResponse> => {
      const result = await configureProjectInternalServices(projectId, data);
      if (result.error) {
        throw new Error(result.error.message);
      }
      if (!result.data) {
        throw new Error('Failed to configure internal services');
      }
      return result.data;
    },
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: projectInternalServicesQueryKey(projectId) });
      void queryClient.invalidateQueries({ queryKey: ['bootstrap-status', projectId] });
    },
  });
}
