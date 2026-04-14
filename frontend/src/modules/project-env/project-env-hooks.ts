import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { deleteProjectEnv, getProjectEnv, upsertProjectEnv } from '@/modules/project-env/project-env-api';
import type { ProjectEnvBundleResponse, UpsertProjectEnvRequest } from '@/modules/project-env/project-env-types';

export function projectEnvQueryKey(projectId: string) {
  return ['project-env', projectId] as const;
}

export function useProjectEnv(projectId: string) {
  return useQuery({
    queryKey: projectEnvQueryKey(projectId),
    queryFn: async (): Promise<ProjectEnvBundleResponse> => {
      const result = await getProjectEnv(projectId);
      if (result.error) {
        throw new Error(result.error.message);
      }
      if (!result.data) {
        throw new Error('Project env unavailable');
      }
      return result.data;
    },
    enabled: !!projectId,
    staleTime: 20 * 1000,
  });
}

export function useUpsertProjectEnv(projectId: string) {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async (data: UpsertProjectEnvRequest): Promise<ProjectEnvBundleResponse> => {
      const result = await upsertProjectEnv(projectId, data);
      if (result.error) {
        throw new Error(result.error.message);
      }
      if (!result.data) {
        throw new Error('Failed to save project env');
      }
      return result.data;
    },
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: projectEnvQueryKey(projectId) });
    },
  });
}

export function useDeleteProjectEnv(projectId: string) {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async (): Promise<ProjectEnvBundleResponse> => {
      const result = await deleteProjectEnv(projectId);
      if (result.error) {
        throw new Error(result.error.message);
      }
      if (!result.data) {
        throw new Error('Failed to clear project env');
      }
      return result.data;
    },
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: projectEnvQueryKey(projectId) });
    },
  });
}
