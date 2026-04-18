import { apiGet } from '@/lib/api/api-client';
import type { ApiResponse } from '@/lib/types';
import type { ProjectRuntimeSummary } from '@/modules/project-runtime/project-runtime-types';

export async function getProjectRuntime(projectId: string): Promise<ApiResponse<ProjectRuntimeSummary>> {
  return apiGet<ProjectRuntimeSummary>(`/projects/${projectId}/runtime`);
}
