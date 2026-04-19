export type ProjectService = {
  id: string;
  project_id: string;
  name: string;
  path: string;
  kind: string;
  source_type: string;
  public: boolean;
  runtime_profile: string;
  placement_mode: string;
  placement_node_id?: string;
  connection_template_key?: string;
  connection_template?: Record<string, string>;
  connection_target_service?: string;
  managed_by_lazyops: boolean;
  start_hint: string;
  image_ref: string;
  image_digest: string;
  target_port: number;
  service_port: number;
  replicas: number;
  env_bundle: Record<string, string>;
  pvc_spec: Record<string, unknown>;
  deploy_strategy: Record<string, unknown>;
  healthcheck: Record<string, unknown>;
  created_at: string;
  updated_at: string;
};

export type ProjectServiceListResponse = {
  items: ProjectService[];
};

export type PlacementNode = {
  cluster_id: string;
  instance_id: string;
  name: string;
  status: string;
  k8s_node_name?: string;
  labels: Record<string, string>;
  last_seen_at?: string;
  is_ready: boolean;
};

export type PlacementNodeListResponse = {
  cluster_id?: string;
  items: PlacementNode[];
};

export type ProjectServiceDraft = {
  name: string;
  path: string;
  kind?: string;
  source_type?: string;
  public: boolean;
  runtime_profile?: string;
  placement_mode?: string;
  placement_node_id?: string;
  connection_template_key?: string;
  connection_template?: Record<string, string>;
  connection_target_service?: string;
  managed_by_lazyops?: boolean;
  start_hint?: string;
  image_ref?: string;
  image_digest?: string;
  target_port?: number;
  service_port?: number;
  replicas?: number;
  env_bundle?: Record<string, string>;
  pvc_spec?: Record<string, unknown>;
  deploy_strategy?: Record<string, unknown>;
  healthcheck?: Record<string, unknown>;
};

export type ConfigureProjectServicesRequest = {
  items: ProjectServiceDraft[];
};

export type ProjectServiceAction = 'deploy' | 'rebuild' | 'restart';

export type ProjectServiceActionResponse = {
  action: string;
  service_id: string;
  service_name: string;
  status: string;
  trigger_kind?: string;
  deployment_id?: string;
  revision_id?: string;
  message?: string;
};
