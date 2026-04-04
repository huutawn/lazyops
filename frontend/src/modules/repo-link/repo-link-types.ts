import { z } from 'zod';

export const linkRepoSchema = z.object({
  github_installation_id: z.number().min(1, 'Installation is required'),
  github_repo_id: z.number().min(1, 'Repository is required'),
  tracked_branch: z
    .string()
    .min(1, 'Branch is required')
    .max(255, 'Branch name too long'),
  preview_enabled: z.boolean(),
});

export type LinkRepoFormData = z.infer<typeof linkRepoSchema> & {
  github_installation_id: number;
  github_repo_id: number;
  preview_enabled: boolean;
};

export type ProjectRepoLink = {
  id: string;
  project_id: string;
  github_installation_id: number;
  github_repo_id: number;
  repo_owner: string;
  repo_name: string;
  repo_full_name: string;
  tracked_branch: string;
  preview_enabled: boolean;
  created_at: string;
  updated_at: string;
};

export type GitHubRepoOption = {
  github_installation_id: number;
  installation_account_login: string;
  installation_account_type: string;
  github_repo_id: number;
  repo_owner: string;
  repo_name: string;
  full_name: string;
  private: boolean;
  permissions: Record<string, string>;
};
