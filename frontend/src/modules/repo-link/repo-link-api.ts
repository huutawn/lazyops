import { apiPost } from '@/lib/api/api-client';
import type { ApiResponse } from '@/lib/types';
import type { LinkRepoFormData, ProjectRepoLink } from '@/modules/repo-link/repo-link-types';

export async function linkProjectRepo(
  projectId: string,
  data: LinkRepoFormData,
): Promise<ApiResponse<ProjectRepoLink>> {
  return apiPost<ProjectRepoLink>(`/projects/${projectId}/repo-link`, data);
}
