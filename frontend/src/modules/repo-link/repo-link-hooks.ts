import { useMutation, useQueryClient } from '@tanstack/react-query';
import { linkProjectRepo } from '@/modules/repo-link/repo-link-api';
import type { LinkRepoFormData, ProjectRepoLink } from '@/modules/repo-link/repo-link-types';

export function repoLinkQueryKey(projectId: string) {
  return ['repo-link', projectId] as const;
}

export function useLinkProjectRepo(projectId: string) {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (data: LinkRepoFormData) => linkProjectRepo(projectId, data),
    onSuccess: (result) => {
      if (result.data) {
        queryClient.setQueryData(repoLinkQueryKey(projectId), result.data);
      }
      void queryClient.invalidateQueries({ queryKey: ['projects', 'list'] });
    },
  });
}
