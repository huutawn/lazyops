import { z } from 'zod';

export const createProjectSchema = z.object({
  name: z
    .string()
    .min(1, 'Project name is required')
    .max(100, 'Project name must be less than 100 characters'),
  slug: z
    .string()
    .min(1, 'Slug is required')
    .max(60, 'Slug must be less than 60 characters')
    .regex(/^[a-z0-9]+(-[a-z0-9]+)*$/, 'Slug must be lowercase alphanumeric with hyphens'),
  default_branch: z
    .string()
    .min(1, 'Branch name is required')
    .max(100, 'Branch name must be less than 100 characters'),
});

export type CreateProjectFormData = z.infer<typeof createProjectSchema> & {
  default_branch: string;
};

export type ProjectSummary = {
  id: string;
  name: string;
  slug: string;
  default_branch: string;
  created_at: string;
  updated_at: string;
};

export type ProjectListResponse = {
  items: ProjectSummary[];
};
