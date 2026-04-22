import { useQuery } from '@tanstack/react-query';
import { getProjectAIPrompt } from '@/modules/project-ai-prompt/project-ai-prompt-api';
import type { ProjectAIPromptResponse } from '@/modules/project-ai-prompt/project-ai-prompt-types';

export function projectAIPromptQueryKey(projectId: string) {
  return ['project-ai-prompt', projectId] as const;
}

export function useProjectAIPrompt(projectId: string) {
  return useQuery({
    queryKey: projectAIPromptQueryKey(projectId),
    queryFn: async (): Promise<ProjectAIPromptResponse> => {
      const result = await getProjectAIPrompt(projectId);
      if (result.error) {
        throw new Error(result.error.message);
      }
      if (!result.data) {
        throw new Error('AI migration prompt unavailable');
      }
      return result.data;
    },
    enabled: !!projectId,
    staleTime: 20 * 1000,
  });
}
