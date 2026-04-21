import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { allocateProjectDomain, getProjectDomain, renameProjectDomain } from '@/modules/project-domain/project-domain-api';
import type { AllocateProjectDomainRequest, ProjectDomain, RenameProjectDomainRequest } from '@/modules/project-domain/project-domain-types';

export function projectDomainQueryKey(projectId: string) {
  return ['project-domain', projectId] as const;
}

export function useProjectDomain(projectId: string) {
  return useQuery({
    queryKey: projectDomainQueryKey(projectId),
    queryFn: async (): Promise<ProjectDomain | null> => {
      const result = await getProjectDomain(projectId);
      if (result.error) {
        throw new Error(result.error.message);
      }
      return result.data ?? null;
    },
    enabled: !!projectId,
    staleTime: 15 * 1000,
  });
}

export function useAllocateProjectDomain(projectId: string) {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async (payload: AllocateProjectDomainRequest): Promise<ProjectDomain> => {
      const result = await allocateProjectDomain(projectId, payload);
      if (result.error) {
        throw new Error(result.error.message);
      }
      if (!result.data) {
        throw new Error('Không thể cấp domain cho project.');
      }
      return result.data;
    },
    onSuccess: (data) => {
      queryClient.setQueryData(projectDomainQueryKey(projectId), data);
      void queryClient.invalidateQueries({ queryKey: projectDomainQueryKey(projectId) });
      void queryClient.invalidateQueries({ queryKey: ['bootstrap-status', projectId] });
      void queryClient.invalidateQueries({ queryKey: ['project-runtime', projectId] });
      void queryClient.invalidateQueries({ queryKey: ['deployments', projectId] });
    },
  });
}

export function useRenameProjectDomain(projectId: string) {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async (payload: RenameProjectDomainRequest): Promise<ProjectDomain> => {
      const result = await renameProjectDomain(projectId, payload);
      if (result.error) {
        throw new Error(result.error.message);
      }
      if (!result.data) {
        throw new Error('Không thể đổi domain cho project.');
      }
      return result.data;
    },
    onSuccess: (data) => {
      queryClient.setQueryData(projectDomainQueryKey(projectId), data);
      void queryClient.invalidateQueries({ queryKey: projectDomainQueryKey(projectId) });
      void queryClient.invalidateQueries({ queryKey: ['bootstrap-status', projectId] });
      void queryClient.invalidateQueries({ queryKey: ['project-runtime', projectId] });
      void queryClient.invalidateQueries({ queryKey: ['deployments', projectId] });
    },
  });
}
