export type ProjectRuntimeLogPreview = {
  id?: string;
  source?: string;
  level: string;
  message: string;
  timestamp: string;
  node?: string;
  correlation_id?: string;
};

export type ProjectRuntimeDependency = {
  service_id?: string;
  service_name: string;
  status: string;
  status_reason?: string;
  internal_endpoint?: string;
};

export type ProjectRuntimeNode = {
  cluster_id: string;
  instance_id: string;
  name: string;
  status: string;
  k8s_node_name?: string;
  labels: Record<string, string>;
  last_seen_at?: string;
  is_ready: boolean;
};

export type ProjectRuntimeService = {
  service_id: string;
  name: string;
  kind?: string;
  source_type?: string;
  public: boolean;
  runtime_profile?: string;
  runtime_status: string;
  runtime_reason?: string;
  build_state?: string;
  rollout_state?: string;
  placement_mode?: string;
  requested_node_id?: string;
  effective_node_ids: string[];
  image_ref?: string;
  image_digest?: string;
  revision_id?: string;
  revision?: number;
  deployment_id?: string;
  public_urls: string[];
  internal_endpoints: string[];
  dependencies: ProjectRuntimeDependency[];
  recent_logs: ProjectRuntimeLogPreview[];
};

export type ProjectRuntimeSummary = {
  project_id: string;
  runtime_mode?: string;
  cluster_id?: string;
  namespace?: string;
  live_revision_id?: string;
  live_revision?: number;
  stable_revision_id?: string;
  stable_revision?: number;
  sync_state: string;
  sync_reason?: string;
  public_urls: string[];
  nodes: ProjectRuntimeNode[];
  services: ProjectRuntimeService[];
};
