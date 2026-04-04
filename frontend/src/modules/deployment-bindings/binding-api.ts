import { apiPost, apiGet } from '@/lib/api/api-client';
import type { ApiResponse } from '@/lib/types';
import type {
  CreateDeploymentBindingFormData,
  DeploymentBindingSummary,
  DeploymentBindingListResponse,
} from '@/modules/deployment-bindings/binding-types';

export async function createDeploymentBinding(
  projectId: string,
  data: CreateDeploymentBindingFormData,
): Promise<ApiResponse<DeploymentBindingSummary>> {
  const body = {
    name: data.name,
    target_ref: data.target_ref || undefined,
    runtime_mode: data.runtime_mode,
    target_kind: data.target_kind,
    target_id: data.target_id,
    placement_policy: data.placement_policy ? JSON.parse(data.placement_policy) : {},
    domain_policy: data.domain_policy ? JSON.parse(data.domain_policy) : {},
    compatibility_policy: data.compatibility_policy ? JSON.parse(data.compatibility_policy) : {},
    scale_to_zero_policy: { enabled: data.scale_to_zero },
  };

  return apiPost<DeploymentBindingSummary>(`/projects/${projectId}/deployment-bindings`, body);
}

export async function listDeploymentBindings(
  projectId: string,
): Promise<ApiResponse<DeploymentBindingListResponse>> {
  return apiGet<DeploymentBindingListResponse>(`/projects/${projectId}/deployment-bindings`);
}
