import { apiPost, apiGet } from '@/lib/api/api-client';
import type { ApiResponse } from '@/lib/types';
import type { CreateProjectFormData, ProjectSummary, ProjectListResponse } from '@/modules/projects/project-types';

export async function createProject(data: CreateProjectFormData): Promise<ApiResponse<ProjectSummary>> {
  return apiPost<ProjectSummary>('/projects', data);
}

export async function listProjects(): Promise<ApiResponse<ProjectListResponse>> {
  return apiGet<ProjectListResponse>('/projects');
}
