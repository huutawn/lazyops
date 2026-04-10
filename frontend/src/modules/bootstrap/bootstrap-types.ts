export type BootstrapStepAction = {
  id: string;
  label: string;
  kind: string;
  href?: string;
  method?: string;
  endpoint?: string;
};

export type BootstrapStep = {
  id: string;
  state: string;
  summary: string;
  actions: BootstrapStepAction[];
};

export type BootstrapAutoMode = {
  enabled: boolean;
  selected_mode: string;
  mode_source: string;
  mode_reason_code: string;
  mode_reason_human: string;
  upshift_allowed: boolean;
  downshift_allowed: boolean;
  downshift_block_reason: string;
};

export type BootstrapInventory = {
  healthy_instances: number;
  healthy_mesh_networks: number;
  healthy_k3s_clusters: number;
};

export type ProjectBootstrapStatus = {
  project_id: string;
  overall_state: string;
  steps: BootstrapStep[];
  auto_mode: BootstrapAutoMode;
  inventory: BootstrapInventory;
  updated_at: string;
};

export type BootstrapAutoRequest = {
  project_id: string;
  project_name?: string;
  default_branch?: string;
  repo_full_name?: string;
  github_installation_id?: number;
  github_repo_id?: number;
  tracked_branch?: string;
  instance_id?: string;
  mesh_network_id?: string;
  cluster_id?: string;
  auto_mode_enabled?: boolean;
  locked_runtime_mode?: string;
};

export type BootstrapAutoAccepted = {
  job_id: string;
  status: string;
  project_id: string;
};

export type BootstrapPipelineEvent = {
  id: string;
  state: string;
  label: string;
  message: string;
  timestamp: string;
};

export type BootstrapOneClickDeployResult = {
  project_id: string;
  blueprint_id: string;
  revision_id: string;
  deployment_id: string;
  rollout_status: string;
  rollout_reason?: string;
  correlation_id?: string;
  agent_id?: string;
  timeline: BootstrapPipelineEvent[];
};

export type BootstrapOneClickDeployRequest = {
  source_ref?: string;
  commit_sha?: string;
  artifact_ref?: string;
  image_ref?: string;
  trigger_kind?: string;
};

export type BootstrapConnectInfraSSHRequest = {
  instance_name?: string;
  public_ip?: string;
  private_ip?: string;
  labels?: Record<string, string>;
  ssh_host: string;
  ssh_port?: number;
  ssh_username: string;
  ssh_password?: string;
  ssh_private_key?: string;
  ssh_host_key_fingerprint?: string;
  control_plane_url?: string;
  agent_image?: string;
  container_name?: string;
};

export type BootstrapConnectInfraSSHResult = {
  project_id: string;
  instance: {
    id: string;
    name: string;
    status: string;
    public_ip?: string | null;
    private_ip?: string | null;
  };
  install: {
    instance_id: string;
    started_at: string;
    host_key_fingerprint?: string;
    attached_project_id?: string;
  };
  auto_bootstrap: {
    job_id: string;
    status: string;
    project_id: string;
  };
};
