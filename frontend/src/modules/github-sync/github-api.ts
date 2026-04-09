import { apiPost, apiGet } from '@/lib/api/api-client';
import type { ApiResponse } from '@/lib/types';
import type {
  SyncGitHubInstallationsFormData,
  GitHubInstallationSyncResponse,
  GitHubRepositoryListResponse,
  GitHubAppConfig,
} from '@/modules/github-sync/github-types';

export async function syncGitHubInstallations(
  data: SyncGitHubInstallationsFormData,
): Promise<ApiResponse<GitHubInstallationSyncResponse>> {
  return apiPost<GitHubInstallationSyncResponse>('/github/app/installations/sync', data);
}

export async function listGitHubRepos(): Promise<ApiResponse<GitHubRepositoryListResponse>> {
  return apiGet<GitHubRepositoryListResponse>('/github/repos');
}

export async function getGitHubAppConfig(): Promise<ApiResponse<GitHubAppConfig>> {
  return apiGet<GitHubAppConfig>('/github/app/config');
}
