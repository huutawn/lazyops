export type BlueprintService = {
  id: string;
  project_id: string;
  name: string;
  path: string;
  public: boolean;
  runtime_profile: string;
  start_hint: string;
  healthcheck: {
    path: string;
    port: number;
  } | null;
  created_at: string;
  updated_at: string;
};

export type BlueprintRepoState = {
  project_repo_link_id: string;
  repo_owner: string;
  repo_name: string;
  repo_full_name: string;
  tracked_branch: string;
  preview_enabled: boolean;
};

export type BlueprintCompiled = {
  project_id: string;
  runtime_mode: string;
  repo: BlueprintRepoState;
  binding: Record<string, unknown>;
  services: BlueprintService[];
  dependency_bindings: Record<string, unknown>[];
  compatibility_policy: Record<string, unknown>;
  magic_domain_policy: Record<string, unknown>;
  scale_to_zero_policy: Record<string, unknown>;
  artifact_metadata: {
    commit_sha: string;
    artifact_ref: string;
    image_ref: string;
  };
};

export type BlueprintRecord = {
  id: string;
  project_id: string;
  source_kind: string;
  source_ref: string;
  compiled: BlueprintCompiled;
  created_at: string;
};

export type PlacementAssignment = {
  service_name: string;
  target_id: string;
  target_kind: string;
  labels: Record<string, string>;
};

export type DesiredRevisionDraft = {
  revision_id: string;
  project_id: string;
  blueprint_id: string;
  deployment_binding_id: string;
  commit_sha: string;
  artifact_ref: string;
  image_ref: string;
  trigger_kind: string;
  runtime_mode: string;
  services: BlueprintService[];
  dependency_bindings: Record<string, unknown>[];
  compatibility_policy: Record<string, unknown>;
  magic_domain_policy: Record<string, unknown>;
  scale_to_zero_policy: Record<string, unknown>;
  placement_assignments: PlacementAssignment[];
};

export type CompileBlueprintResponse = {
  services: BlueprintService[];
  blueprint: BlueprintRecord;
  desired_revision_draft: DesiredRevisionDraft;
};

export type CompileBlueprintRequest = {
  source_ref?: string;
  trigger_kind?: string;
  artifact_metadata: {
    commit_sha: string;
    artifact_ref?: string;
    image_ref?: string;
  };
  lazyops_yaml: unknown;
};
