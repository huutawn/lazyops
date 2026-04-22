import { apiGet } from '@/lib/api/api-client';
import type { ApiResponse } from '@/lib/types';
import type { ProjectAIPromptResponse } from '@/modules/project-ai-prompt/project-ai-prompt-types';

export async function getProjectAIPrompt(projectId: string): Promise<ApiResponse<ProjectAIPromptResponse>> {
  return apiGet<ProjectAIPromptResponse>(`/projects/${projectId}/ai-migration-prompt`);
}
