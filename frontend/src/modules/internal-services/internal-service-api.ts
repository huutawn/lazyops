import { apiGet, apiPut } from '@/lib/api/api-client';
import type { ApiResponse } from '@/lib/types';
import type {
  ConfigureProjectInternalServicesRequest,
  ProjectInternalServiceListResponse,
} from '@/modules/internal-services/internal-service-types';

export async function getProjectInternalServices(projectId: string): Promise<ApiResponse<ProjectInternalServiceListResponse>> {
  return apiGet<ProjectInternalServiceListResponse>(`/projects/${projectId}/internal-services`);
}

export async function configureProjectInternalServices(
  projectId: string,
  data: ConfigureProjectInternalServicesRequest,
): Promise<ApiResponse<ProjectInternalServiceListResponse>> {
  return apiPut<ProjectInternalServiceListResponse>(`/projects/${projectId}/internal-services`, data);
}
