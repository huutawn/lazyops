import { useQuery } from '@tanstack/react-query';
import { getProjectRuntime } from '@/modules/project-runtime/project-runtime-api';
import { normalizeProjectRuntimeSummary } from '@/modules/project-runtime/project-runtime-normalize';
import type { ProjectRuntimeSummary } from '@/modules/project-runtime/project-runtime-types';

export function projectRuntimeQueryKey(projectId: string) {
  return ['project-runtime', projectId] as const;
}

export function useProjectRuntime(projectId?: string) {
  return useQuery({
    queryKey: projectRuntimeQueryKey(projectId ?? ''),
    queryFn: async (): Promise<ProjectRuntimeSummary> => {
      if (!projectId) {
        throw new Error('Missing project context');
      }
      const result = await getProjectRuntime(projectId);
      if (result.error) {
        throw new Error(result.error.message);
      }
      return normalizeProjectRuntimeSummary(projectId, result.data);
    },
    enabled: !!projectId,
    staleTime: 15 * 1000,
    refetchInterval: 20 * 1000,
  });
}
