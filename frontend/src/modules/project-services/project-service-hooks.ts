import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { actOnProjectService, configureProjectServices, listProjectPlacementNodes, listProjectServices } from '@/modules/project-services/project-service-api';
import type {
  ConfigureProjectServicesRequest,
  PlacementNodeListResponse,
  ProjectServiceAction,
  ProjectServiceActionResponse,
  ProjectServiceListResponse,
} from '@/modules/project-services/project-service-types';

export function projectServicesQueryKey(projectId: string) {
  return ['project-services', projectId] as const;
}

export function projectPlacementNodesQueryKey(projectId: string) {
  return ['project-placement-nodes', projectId] as const;
}

export function useProjectServices(projectId: string) {
  return useQuery({
    queryKey: projectServicesQueryKey(projectId),
    queryFn: async (): Promise<ProjectServiceListResponse> => {
      const result = await listProjectServices(projectId);
      if (result.error) {
        throw new Error(result.error.message);
      }
      return result.data ?? { items: [] };
    },
    enabled: !!projectId,
    staleTime: 15 * 1000,
  });
}

export function useConfigureProjectServices(projectId: string) {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async (payload: ConfigureProjectServicesRequest): Promise<ProjectServiceListResponse> => {
      const result = await configureProjectServices(projectId, payload);
      if (result.error) {
        throw new Error(result.error.message);
      }
      if (!result.data) {
        throw new Error('Failed to save project services');
      }
      return result.data;
    },
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: projectServicesQueryKey(projectId) });
      void queryClient.invalidateQueries({ queryKey: ['project-env', projectId] });
      void queryClient.invalidateQueries({ queryKey: ['bootstrap-status', projectId] });
    },
  });
}

export function useProjectPlacementNodes(projectId: string) {
  return useQuery({
    queryKey: projectPlacementNodesQueryKey(projectId),
    queryFn: async (): Promise<PlacementNodeListResponse> => {
      const result = await listProjectPlacementNodes(projectId);
      if (result.error) {
        throw new Error(result.error.message);
      }
      return result.data ?? { items: [] };
    },
    enabled: !!projectId,
    staleTime: 15 * 1000,
  });
}

export function useProjectServiceAction(projectId: string) {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async ({ serviceId, action }: { serviceId: string; action: ProjectServiceAction }): Promise<ProjectServiceActionResponse> => {
      const result = await actOnProjectService(projectId, serviceId, action);
      if (result.error) {
        throw new Error(result.error.message);
      }
      if (!result.data) {
        throw new Error('Failed to execute service action');
      }
      return result.data;
    },
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: projectServicesQueryKey(projectId) });
      void queryClient.invalidateQueries({ queryKey: projectPlacementNodesQueryKey(projectId) });
      void queryClient.invalidateQueries({ queryKey: ['deployments', projectId] });
      void queryClient.invalidateQueries({ queryKey: ['bootstrap-status', projectId] });
    },
  });
}
