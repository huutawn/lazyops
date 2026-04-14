import { apiDelete, apiGet, apiPut } from '@/lib/api/api-client';
import type { ApiResponse } from '@/lib/types';
import type { ProjectEnvBundleResponse, UpsertProjectEnvRequest } from '@/modules/project-env/project-env-types';

export async function getProjectEnv(projectId: string): Promise<ApiResponse<ProjectEnvBundleResponse>> {
  return apiGet<ProjectEnvBundleResponse>(`/projects/${projectId}/env`);
}

export async function upsertProjectEnv(
  projectId: string,
  data: UpsertProjectEnvRequest,
): Promise<ApiResponse<ProjectEnvBundleResponse>> {
  return apiPut<ProjectEnvBundleResponse>(`/projects/${projectId}/env`, data);
}

export async function deleteProjectEnv(projectId: string): Promise<ApiResponse<ProjectEnvBundleResponse>> {
  return apiDelete<ProjectEnvBundleResponse>(`/projects/${projectId}/env`);
}
