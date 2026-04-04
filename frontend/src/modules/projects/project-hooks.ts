import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useRouter } from 'next/navigation';
import { createProject, listProjects } from '@/modules/projects/project-api';
import type { CreateProjectFormData, ProjectSummary, ProjectListResponse } from '@/modules/projects/project-types';

const PROJECTS_KEY = ['projects', 'list'];

export function useProjects() {
  return useQuery({
    queryKey: PROJECTS_KEY,
    queryFn: async () => {
      const result = await listProjects();
      if (result.error) throw new Error(result.error.message);
      return result.data as ProjectListResponse;
    },
    staleTime: 30 * 1000,
  });
}

export function useCreateProject() {
  const router = useRouter();
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (data: CreateProjectFormData) => {
      return createProject(data);
    },
    onSuccess: (result) => {
      void queryClient.invalidateQueries({ queryKey: PROJECTS_KEY });
      if (result.data) {
        const project = result.data as ProjectSummary;
        router.push(`/projects/${project.id}`);
      }
    },
  });
}
