export type ValidateLazyopsProject = {
  id: string;
  name: string;
  slug: string;
  default_branch: string;
  created_at: string;
  updated_at: string;
};

export type ValidateLazyopsBinding = {
  id: string;
  project_id: string;
  name: string;
  target_ref: string;
  runtime_mode: string;
  target_kind: string;
  target_id: string;
  created_at: string;
  updated_at: string;
};

export type ValidateLazyopsTarget = {
  id: string;
  name: string;
  kind: string;
  status: string;
  runtime_mode: string;
};

export type ValidateLazyopsSchema = {
  allowed_dependency_protocols: string[];
  allowed_magic_domain_providers: string[];
  forbidden_field_names: string[];
};

export type ValidateRoutingGuidanceRoute = {
  path: string;
  service: string;
  audience?: string;
  source?: string;
  websocket?: boolean;
};

export type ValidateMigrationFinding = {
  category: string;
  severity: string;
  service_name?: string;
  current_value?: string;
  recommended_value?: string;
  message: string;
};

export type ValidateLazyopsResponse = {
  project: ValidateLazyopsProject;
  deployment_binding: ValidateLazyopsBinding;
  target_summary: ValidateLazyopsTarget;
  schema: ValidateLazyopsSchema;
  suggested_routes: ValidateRoutingGuidanceRoute[];
  effective_public_paths: ValidateRoutingGuidanceRoute[];
  migration_findings: ValidateMigrationFinding[];
};

export type LazyopsYAMLService = {
  name: string;
  path: string;
  start_hint?: string;
  public?: boolean;
  healthcheck?: {
    path: string;
    port: number;
  };
};

export type LazyopsYAMLDependencyBinding = {
  service: string;
  alias: string;
  target_service: string;
  protocol: string;
  local_endpoint: string;
};

export type LazyopsYAMLCompatibilityPolicy = {
  env_injection: boolean;
  managed_credentials: boolean;
  localhost_rescue: boolean;
};

export type LazyopsYAMLMagicDomainPolicy = {
  enabled: boolean;
  provider: string;
};

export type LazyopsYAMLPreviewPolicy = {
  enabled: boolean;
};

export type LazyopsYAMLScaleToZeroPolicy = {
  enabled: boolean;
};

export type LazyopsYAMLDraft = {
  project_slug: string;
  runtime_mode: string;
  deployment_binding: {
    target_ref: string;
  };
  services: LazyopsYAMLService[];
  dependency_bindings: LazyopsYAMLDependencyBinding[];
  compatibility_policy: LazyopsYAMLCompatibilityPolicy;
  magic_domain_policy: LazyopsYAMLMagicDomainPolicy;
  preview_policy: LazyopsYAMLPreviewPolicy;
  scale_to_zero_policy: LazyopsYAMLScaleToZeroPolicy;
  routing_policy?: {
    shared_domain?: string;
    routes: Array<{
      path: string;
      service: string;
      port?: number;
      websocket?: boolean;
      strip_prefix?: boolean;
    }>;
  };
};
