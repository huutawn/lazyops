import { z } from 'zod';

export const syncGitHubInstallationsSchema = z.object({
  github_access_token: z.string().min(1, 'GitHub access token is required'),
});

export type SyncGitHubInstallationsFormData = z.infer<typeof syncGitHubInstallationsSchema>;

export type GitHubInstallationRepository = {
  id: number;
  name: string;
  full_name: string;
  owner_login: string;
  private: boolean;
};

export type GitHubInstallationScope = {
  repository_selection: string;
  permissions: Record<string, string>;
  repositories: GitHubInstallationRepository[];
};

export type GitHubInstallation = {
  id: string;
  github_installation_id: number;
  account_login: string;
  account_type: string;
  installed_at: string;
  revoked_at: string | null;
  status: 'active' | 'revoked';
  scope: GitHubInstallationScope;
};

export type GitHubInstallationSyncResponse = {
  items: GitHubInstallation[];
};

export type GitHubRepository = {
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

export type GitHubRepositoryListResponse = {
  items: GitHubRepository[];
};

export type GitHubAppConfig = {
  name: string;
  install_url: string;
  webhook_url: string;
  callback_url: string;
  enabled: boolean;
};
