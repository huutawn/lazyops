export type ProjectEnvHelperSnippet = {
  service_kind: string;
  alias: string;
  env: Record<string, string>;
};

export type ProjectEnvBundleResponse = {
  configured: boolean;
  updated_at?: string;
  fingerprint?: string;
  keys: string[];
  parse_warnings: string[];
  helper_snippets: ProjectEnvHelperSnippet[];
};

export type UpsertProjectEnvRequest = {
  content: string;
};
