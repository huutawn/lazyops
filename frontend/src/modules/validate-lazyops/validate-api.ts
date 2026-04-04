import { apiPost } from '@/lib/api/api-client';
import type { ApiResponse } from '@/lib/types';
import type { ValidateLazyopsResponse, LazyopsYAMLDraft } from '@/modules/validate-lazyops/validate-types';

export async function validateLazyopsYaml(
  projectId: string,
  draft: LazyopsYAMLDraft,
): Promise<ApiResponse<ValidateLazyopsResponse>> {
  return apiPost<ValidateLazyopsResponse>(
    `/projects/${projectId}/init/validate-lazyops-yaml`,
    draft,
  );
}
