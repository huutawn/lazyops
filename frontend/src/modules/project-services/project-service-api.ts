import { apiGet, apiPost, apiPut } from '@/lib/api/api-client';
import type { ApiResponse } from '@/lib/types';
import type {
  ConfigureProjectServicesRequest,
  PlacementNodeListResponse,
  ProjectServiceAction,
  ProjectServiceActionResponse,
  ProjectServiceListResponse,
} from '@/modules/project-services/project-service-types';

export async function listProjectServices(projectId: string): Promise<ApiResponse<ProjectServiceListResponse>> {
  return apiGet<ProjectServiceListResponse>(`/projects/${projectId}/services`);
}

export async function configureProjectServices(
  projectId: string,
  payload: ConfigureProjectServicesRequest,
): Promise<ApiResponse<ProjectServiceListResponse>> {
  return apiPut<ProjectServiceListResponse>(`/projects/${projectId}/services`, payload);
}

export async function listProjectPlacementNodes(projectId: string): Promise<ApiResponse<PlacementNodeListResponse>> {
  return apiGet<PlacementNodeListResponse>(`/projects/${projectId}/placement-nodes`);
}

export async function actOnProjectService(
  projectId: string,
  serviceId: string,
  action: ProjectServiceAction,
): Promise<ApiResponse<ProjectServiceActionResponse>> {
  return apiPost<ProjectServiceActionResponse>(`/projects/${projectId}/services/${serviceId}/actions`, { action });
}
