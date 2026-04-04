import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { syncGitHubInstallations, listGitHubRepos } from '@/modules/github-sync/github-api';
import type { SyncGitHubInstallationsFormData, GitHubInstallationSyncResponse, GitHubRepositoryListResponse } from '@/modules/github-sync/github-types';

const INSTALLATIONS_KEY = ['github', 'installations'];
const REPOS_KEY = ['github', 'repos'];

export function useGitHubInstallations() {
  return useQuery({
    queryKey: INSTALLATIONS_KEY,
    queryFn: async () => {
      const result = await listGitHubRepos();
      if (result.error) throw new Error(result.error.message);
      return result.data as GitHubRepositoryListResponse;
    },
    staleTime: 60 * 1000,
  });
}

export function useSyncGitHubInstallations() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (data: SyncGitHubInstallationsFormData) => syncGitHubInstallations(data),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: INSTALLATIONS_KEY });
      void queryClient.invalidateQueries({ queryKey: REPOS_KEY });
    },
  });
}
