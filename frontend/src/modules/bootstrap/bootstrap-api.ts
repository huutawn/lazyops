import { apiGet, apiPost } from '@/lib/api/api-client';
import type { ApiResponse } from '@/lib/types';
import type {
  BootstrapAutoAccepted,
  BootstrapAutoRequest,
  BootstrapOneClickDeployRequest,
  BootstrapOneClickDeployResult,
  ProjectBootstrapStatus,
} from '@/modules/bootstrap/bootstrap-types';

export async function getProjectBootstrapStatus(projectId: string): Promise<ApiResponse<ProjectBootstrapStatus>> {
  return apiGet<ProjectBootstrapStatus>(`/projects/${projectId}/bootstrap/status`);
}

export async function autoBootstrapProject(data: BootstrapAutoRequest): Promise<ApiResponse<BootstrapAutoAccepted>> {
  return apiPost<BootstrapAutoAccepted>('/projects/bootstrap/auto', data);
}

export async function deployProjectOneClick(
  projectId: string,
  data: BootstrapOneClickDeployRequest = {},
): Promise<ApiResponse<BootstrapOneClickDeployResult>> {
  return apiPost<BootstrapOneClickDeployResult>(`/projects/${projectId}/deploy/one-click`, data);
}
