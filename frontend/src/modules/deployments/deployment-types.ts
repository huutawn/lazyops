export type BuildState = 'draft' | 'queued' | 'building' | 'artifact_ready' | 'planned' | 'applying' | 'promoted' | 'failed' | 'rolled_back' | 'superseded';
export type RolloutState = 'queued' | 'running' | 'candidate_ready' | 'promoted' | 'failed' | 'rolled_back' | 'canceled';

export type DeploymentService = {
  name: string;
  path: string;
  public: boolean;
  runtime_profile: string;
};

export type PlacementAssignment = {
  service_name: string;
  target_id: string;
  target_kind: string;
  labels: Record<string, string>;
};

export type DeploymentRecord = {
  id: string;
  project_id: string;
  revision_id: string;
  revision: number;
  commit_sha: string;
  artifact_ref: string | null;
  image_ref: string | null;
  trigger_kind: string;
  build_state: BuildState;
  rollout_state: RolloutState;
  promoted: boolean;
  triggered_by: string;
  runtime_mode: string;
  services: DeploymentService[];
  placement_assignments: PlacementAssignment[];
  started_at: string | null;
  completed_at: string | null;
  created_at: string;
  updated_at: string;
};

export type DeploymentListResponse = {
  items: DeploymentRecord[];
};

export type DeploymentTimelineEvent = {
  timestamp: string;
  state: string;
  label: string;
  description: string;
};

export type DeploymentSafetyPolicy = {
  auto_rollback_enabled: boolean;
  triggers: string[];
  description: string;
};

export type DeploymentFixAction = {
  id: string;
  label: string;
  href: string;
  method: string;
};

export type DeploymentIncidentSummary = {
  state: string;
  headline: string;
  reason: string;
  recommended: string;
  incident_id?: string;
  incident_kind?: string;
  incident_level?: string;
  primary_action?: DeploymentFixAction;
};

export type DeploymentDetail = DeploymentRecord & {
  timeline: DeploymentTimelineEvent[];
  can_rollback: boolean;
  can_promote: boolean;
  can_cancel: boolean;
  safety_policy: DeploymentSafetyPolicy;
  incident_summary?: DeploymentIncidentSummary;
};
