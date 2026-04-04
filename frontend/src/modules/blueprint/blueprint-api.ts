import { apiPut } from '@/lib/api/api-client';
import type { ApiResponse } from '@/lib/types';
import type { CompileBlueprintRequest, CompileBlueprintResponse } from '@/modules/blueprint/blueprint-types';

export async function compileBlueprint(
  projectId: string,
  data: CompileBlueprintRequest,
): Promise<ApiResponse<CompileBlueprintResponse>> {
  return apiPut<CompileBlueprintResponse>(`/projects/${projectId}/blueprint`, data);
}
