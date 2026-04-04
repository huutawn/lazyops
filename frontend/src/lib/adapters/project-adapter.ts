import type { Adapter } from '@/lib/adapters/adapter-factory';
import type { ProjectListResponse, ProjectSummary } from '@/modules/projects/project-types';
import { projects as mockProjects } from '@/lib/mock-data';
import { createProject as liveCreateProject, listProjects as liveListProjects } from '@/modules/projects/project-api';

function mockListProjects(): Promise<{ data: ProjectListResponse; error?: null }> {
  return Promise.resolve({ data: { items: mockProjects } });
}

function mockCreateProject(data: { name: string; slug: string; default_branch: string }): Promise<{ data: ProjectSummary; error?: null }> {
  const newProject: ProjectSummary = {
    id: `proj_${Date.now()}`,
    name: data.name,
    slug: data.slug,
    default_branch: data.default_branch,
    created_at: new Date().toISOString(),
    updated_at: new Date().toISOString(),
  };
  return Promise.resolve({ data: newProject });
}

export const listProjectsAdapter: Adapter<ProjectListResponse> = {
  mode: 'mock',
  fetch: () => mockListProjects(),
};

export const createProjectAdapter = (data: { name: string; slug: string; default_branch: string }) => ({
  mode: 'mock' as const,
  fetch: () => mockCreateProject(data),
});

export { liveListProjects, liveCreateProject };
