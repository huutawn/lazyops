import type { DeploymentRecord, DeploymentListResponse, DeploymentDetail, DeploymentTimelineEvent, BuildState, RolloutState } from '@/modules/deployments/deployment-types';

const MOCK_DEPLOYMENTS: DeploymentRecord[] = [
  {
    id: 'dep_001',
    project_id: 'proj_01',
    revision_id: 'rev_012',
    revision: 12,
    commit_sha: 'a1b2c3d4e5f6',
    artifact_ref: 'v1.2.0',
    image_ref: 'ghcr.io/lazyops/ecommerce:v1.2.0',
    trigger_kind: 'push',
    build_state: 'promoted',
    rollout_state: 'promoted',
    promoted: true,
    triggered_by: 'alice@example.com',
    runtime_mode: 'standalone',
    services: [{ name: 'web', path: 'apps/web', public: true, runtime_profile: 'node' }],
    placement_assignments: [{ service_name: 'web', target_id: 'inst_01', target_kind: 'instance', labels: { env: 'prod' } }],
    started_at: '2026-04-03T09:00:00Z',
    completed_at: '2026-04-03T09:05:30Z',
    created_at: '2026-04-03T08:58:00Z',
    updated_at: '2026-04-03T09:05:30Z',
  },
  {
    id: 'dep_002',
    project_id: 'proj_01',
    revision_id: 'rev_013',
    revision: 13,
    commit_sha: 'f6e5d4c3b2a1',
    artifact_ref: null,
    image_ref: null,
    trigger_kind: 'api',
    build_state: 'building',
    rollout_state: 'running',
    promoted: false,
    triggered_by: 'bob@example.com',
    runtime_mode: 'standalone',
    services: [{ name: 'web', path: 'apps/web', public: true, runtime_profile: 'node' }],
    placement_assignments: [{ service_name: 'web', target_id: 'inst_01', target_kind: 'instance', labels: { env: 'prod' } }],
    started_at: '2026-04-04T11:00:00Z',
    completed_at: null,
    created_at: '2026-04-04T10:58:00Z',
    updated_at: '2026-04-04T11:00:00Z',
  },
  {
    id: 'dep_003',
    project_id: 'proj_01',
    revision_id: 'rev_003',
    revision: 3,
    commit_sha: 'deadbeef1234',
    artifact_ref: null,
    image_ref: null,
    trigger_kind: 'push',
    build_state: 'failed',
    rollout_state: 'failed',
    promoted: false,
    triggered_by: 'alice@example.com',
    runtime_mode: 'distributed-mesh',
    services: [{ name: 'api', path: 'services/api', public: false, runtime_profile: 'go' }],
    placement_assignments: [{ service_name: 'api', target_id: 'mesh_01', target_kind: 'mesh', labels: { env: 'staging' } }],
    started_at: '2026-04-04T08:00:00Z',
    completed_at: '2026-04-04T08:02:15Z',
    created_at: '2026-04-04T07:58:00Z',
    updated_at: '2026-04-04T08:02:15Z',
  },
];

function generateTimeline(deployment: DeploymentRecord): DeploymentTimelineEvent[] {
  const events: DeploymentTimelineEvent[] = [];
  const buildStates: BuildState[] = ['draft', 'queued', 'building', 'artifact_ready', 'planned', 'applying'];
  const rolloutStates: RolloutState[] = ['queued', 'running', 'candidate_ready'];

  const baseTime = new Date(deployment.created_at).getTime();

  buildStates.forEach((state, i) => {
    const time = new Date(baseTime + i * 30000).toISOString();
    events.push({
      timestamp: time,
      state,
      label: state.replace(/_/g, ' ').replace(/\b\w/g, (c) => c.toUpperCase()),
      description: `Build transitioned to ${state}`,
    });
  });

  if (deployment.build_state === 'promoted') {
    events.push({
      timestamp: deployment.completed_at ?? new Date().toISOString(),
      state: 'promoted',
      label: 'Promoted',
      description: 'Deployment promoted to production.',
    });
  } else if (deployment.build_state === 'failed') {
    events.push({
      timestamp: deployment.completed_at ?? new Date().toISOString(),
      state: 'failed',
      label: 'Failed',
      description: 'Build or rollout failed.',
    });
  } else if (deployment.build_state === 'rolled_back') {
    events.push({
      timestamp: deployment.completed_at ?? new Date().toISOString(),
      state: 'rolled_back',
      label: 'Rolled back',
      description: 'Deployment was rolled back.',
    });
  } else {
    rolloutStates.forEach((state, i) => {
      const time = new Date(baseTime + (buildStates.length + i) * 30000).toISOString();
      events.push({
        timestamp: time,
        state,
        label: state.replace(/_/g, ' ').replace(/\b\w/g, (c) => c.toUpperCase()),
        description: `Rollout transitioned to ${state}`,
      });
    });
  }

  return events;
}

export async function mockListDeployments(projectId?: string): Promise<DeploymentListResponse> {
  await new Promise((r) => setTimeout(r, 500));
  const items = projectId ? MOCK_DEPLOYMENTS.filter((d) => d.project_id === projectId) : MOCK_DEPLOYMENTS;
  return { items };
}

export async function mockGetDeployment(id: string): Promise<DeploymentDetail> {
  await new Promise((r) => setTimeout(r, 300));
  const deployment = MOCK_DEPLOYMENTS.find((d) => d.id === id);
  if (!deployment) throw new Error('Deployment not found');

  return {
    ...deployment,
    timeline: generateTimeline(deployment),
    can_rollback: deployment.promoted && deployment.build_state === 'promoted',
    can_promote: deployment.rollout_state === 'candidate_ready' && !deployment.promoted,
    can_cancel: deployment.rollout_state === 'running' || deployment.rollout_state === 'queued',
  };
}
