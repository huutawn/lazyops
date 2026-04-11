import { apiGet, apiPut } from '@/lib/api/api-client';
import type { ApiResponse } from '@/lib/types';
import type {
  ProjectRoutingResponse,
  UpdateRoutingPolicyRequest,
} from '@/modules/project-routing/project-routing-types';

export async function getProjectRouting(projectId: string): Promise<ApiResponse<ProjectRoutingResponse>> {
  return apiGet<ProjectRoutingResponse>(`/projects/${projectId}/routing`);
}

export async function updateProjectRouting(
  projectId: string,
  data: UpdateRoutingPolicyRequest,
): Promise<ApiResponse<ProjectRoutingResponse>> {
  return apiPut<ProjectRoutingResponse>(`/projects/${projectId}/routing`, data);
}
