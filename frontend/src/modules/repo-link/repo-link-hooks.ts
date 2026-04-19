import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { getProjectRepoLink, linkProjectRepo } from '@/modules/repo-link/repo-link-api';
import type { LinkRepoFormData, ProjectRepoLink } from '@/modules/repo-link/repo-link-types';

export function repoLinkQueryKey(projectId: string) {
  return ['repo-link', projectId] as const;
}

export function useProjectRepoLink(projectId: string) {
  return useQuery({
    queryKey: repoLinkQueryKey(projectId),
    queryFn: async (): Promise<ProjectRepoLink | null> => {
      const result = await getProjectRepoLink(projectId);
      if (result.error) {
        throw new Error(result.error.message);
      }
      return result.data ?? null;
    },
    enabled: !!projectId,
    staleTime: 15 * 1000,
  });
}

export function useLinkProjectRepo(projectId: string) {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (data: LinkRepoFormData) => linkProjectRepo(projectId, data),
    onSuccess: (result) => {
      if (result.data) {
        queryClient.setQueryData(repoLinkQueryKey(projectId), result.data);
      }
      void queryClient.invalidateQueries({ queryKey: repoLinkQueryKey(projectId) });
      void queryClient.invalidateQueries({ queryKey: ['bootstrap-status', projectId] });
      void queryClient.invalidateQueries({ queryKey: ['projects', 'list'] });
    },
  });
}
