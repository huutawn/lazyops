import { apiFetch, apiGet, apiPost } from '@/lib/api/api-client';
import type { ApiResponse } from '@/lib/types';
import type { AllocateProjectDomainRequest, ProjectDomain, RenameProjectDomainRequest } from '@/modules/project-domain/project-domain-types';

export async function getProjectDomain(projectId: string): Promise<ApiResponse<ProjectDomain | null>> {
  const result = await apiGet<ProjectDomain>(`/projects/${projectId}/domain`);
  if (result.error?.code === 'project_domain_not_found') {
    return { data: null, error: null };
  }
  return result as ApiResponse<ProjectDomain | null>;
}

export async function allocateProjectDomain(
  projectId: string,
  payload: AllocateProjectDomainRequest = {},
): Promise<ApiResponse<ProjectDomain>> {
  return apiPost<ProjectDomain>(`/projects/${projectId}/domain`, payload);
}

export async function renameProjectDomain(
  projectId: string,
  payload: RenameProjectDomainRequest,
): Promise<ApiResponse<ProjectDomain>> {
  return apiFetch<ProjectDomain>(`/projects/${projectId}/domain`, {
    method: 'PATCH',
    body: JSON.stringify(payload),
  });
}
