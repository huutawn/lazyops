import { apiGet, apiPost } from '@/lib/api/api-client';
import type { ApiResponse } from '@/lib/types';
import type { DeploymentDetail, DeploymentListResponse } from '@/modules/deployments/deployment-types';

export type DeploymentAction = 'promote' | 'rollback' | 'cancel';

export async function listProjectDeployments(projectID: string): Promise<ApiResponse<DeploymentListResponse>> {
  return apiGet<DeploymentListResponse>(`/projects/${projectID}/deployments`);
}

export async function getProjectDeployment(projectID: string, deploymentID: string): Promise<ApiResponse<DeploymentDetail>> {
  return apiGet<DeploymentDetail>(`/projects/${projectID}/deployments/${deploymentID}`);
}

export async function actProjectDeployment(
  projectID: string,
  deploymentID: string,
  action: DeploymentAction,
): Promise<ApiResponse<DeploymentDetail>> {
  return apiPost<DeploymentDetail>(`/projects/${projectID}/deployments/${deploymentID}/actions`, { action });
}
