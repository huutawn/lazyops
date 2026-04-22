export type ProjectEnvHelperSnippet = {
  language: string;
  framework: string;
  kind: string;
  title: string;
  content: string;
};

export type ProjectEnvHelperPack = {
  service_kind: string;
  alias: string;
  category: string;
  audience: string;
  source_service: string;
  related_services: string[];
  primary_key: string;
  public_path?: string;
  managed: boolean;
  runtime_injected: boolean;
  placeholder_env: Record<string, string>;
  env_example: Record<string, string>;
  local_example_env: Record<string, string>;
  runtime_keys: string[];
  provisioned_keys: string[];
  notes: string[];
  language_snippets: ProjectEnvHelperSnippet[];
};

export type ProjectEnvBundleResponse = {
  configured: boolean;
  updated_at?: string;
  fingerprint?: string;
  keys: string[];
  user_keys: string[];
  managed_keys: string[];
  provisioned_keys: string[];
  parse_warnings: string[];
  helper_packs: ProjectEnvHelperPack[];
};

export type UpsertProjectEnvRequest = {
  content: string;
};
