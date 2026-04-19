import { describe, expect, it } from 'vitest';
import { ADMIN_PROJECT_TABS, GUIDED_PROJECT_TABS } from '@/components/primitives/project-tabs';
import { formatBootstrapStateLabelVN, resolveProjectNextAction } from '@/modules/bootstrap/bootstrap-ui';
import { formatActionResult, formatPlacementValue, resolvePlacementHint, resolvePrimaryServiceAction, resolveServiceDisplayStatus } from '@/modules/project-services/project-service-inventory';
import { resolveGuidedProjectFlow, resolveProjectExpertRoute } from '@/modules/projects/project-flow-hooks';
import type { ProjectBootstrapStatus } from '@/modules/bootstrap/bootstrap-types';
import type { ProjectService } from '@/modules/project-services/project-service-types';
import type { ProjectRuntimeService } from '@/modules/project-runtime/project-runtime-types';

describe('service-first navigation', () => {
  it('keeps only the primary project tabs in both guided and admin modes', () => {
    const guidedLabels = GUIDED_PROJECT_TABS.map((tab) => tab.label);
    const adminLabels = ADMIN_PROJECT_TABS.map((tab) => tab.label);

    expect(guidedLabels).toEqual(['Bắt đầu', 'Mã nguồn', 'Dịch vụ', 'Biến môi trường', 'Triển khai', 'Nhật ký']);
    expect(adminLabels).toEqual(['Bắt đầu', 'Mã nguồn', 'Dịch vụ', 'Biến môi trường', 'Triển khai', 'Nhật ký']);
  });

  it('redirects expert routes back to the service-first project workspace when guided mode is on', () => {
    expect(resolveGuidedProjectFlow(true, 'operator')).toBe(true);
    expect(resolveProjectExpertRoute('prj_123', true)).toBe('/projects/prj_123');
    expect(resolveProjectExpertRoute('prj_123', false)).toBeNull();
  });
});

describe('service inventory helpers', () => {
  it('keeps placement messaging aligned with service-first cluster behavior', () => {
    expect(resolvePlacementHint({
      placementNodesError: null,
      isLoading: false,
      clusterID: 'clu_123',
      readyCount: 2,
    })).toContain('2 máy chủ');
  });

  it('formats deploy action feedback around service-level deployments', () => {
    expect(formatActionResult({
      action: 'deploy',
      service_id: 'svc_api',
      service_name: 'api',
      status: 'started',
      deployment_id: 'dep_123',
    })).toContain('dep_123');
  });

  it('maps the next step to repository setup first', () => {
    expect(resolveProjectNextAction({
      status: bootstrapStatusFixture({
        steps: [
          step('connect_code', 'missing', 'Chưa kết nối repository'),
          step('connect_infra', 'missing', 'Chưa kết nối máy chủ'),
          step('deploy', 'blocked', 'Chưa thể deploy'),
        ],
      }),
      repoLink: null,
      logsHref: '/projects/prj_123/observability',
    }).kind).toBe('repo');
  });

  it('maps the next step to service inventory when no service has been configured yet', () => {
    expect(resolveProjectNextAction({
      status: bootstrapStatusFixture({
        steps: [
          step('connect_code', 'healthy', 'Project đang trống'),
          step('connect_infra', 'ready', 'Máy chủ sẵn sàng'),
          {
            id: 'deploy',
            state: 'blocked',
            summary: 'Chưa có service nào được cấu hình. Hãy thêm ít nhất một service trong mục Dịch vụ',
            actions: [{ id: 'configure_services', label: 'Cấu hình dịch vụ', kind: 'screen', href: '/projects/prj_123/services' }],
          },
        ],
      }),
      repoLink: null,
      logsHref: '/projects/prj_123/observability',
    }).kind).toBe('services');
  });

  it('maps the next step to infra after repo is linked', () => {
    expect(resolveProjectNextAction({
      status: bootstrapStatusFixture({
        steps: [
          step('connect_code', 'linked', 'Đã có repo'),
          step('connect_infra', 'missing', 'Chưa kết nối máy chủ'),
          step('deploy', 'blocked', 'Chưa thể deploy'),
        ],
      }),
      repoLink: {
        id: 'prl_123',
        project_id: 'prj_123',
        github_installation_id: 100,
        github_repo_id: 42,
        repo_owner: 'lazyops',
        repo_name: 'web',
        repo_full_name: 'lazyops/web',
        tracked_branch: 'main',
        preview_enabled: false,
        created_at: '2026-04-19T00:00:00Z',
        updated_at: '2026-04-19T00:00:00Z',
      },
      logsHref: '/projects/prj_123/observability',
    }).kind).toBe('infra');
  });

  it('maps the next step to deploy when setup is ready', () => {
    expect(resolveProjectNextAction({
      status: bootstrapStatusFixture({
        steps: [
          step('connect_code', 'linked', 'Đã có repo'),
          step('connect_infra', 'ready', 'Máy chủ sẵn sàng'),
          step('deploy', 'ready', 'Có thể deploy'),
        ],
      }),
      repoLink: linkedRepoFixture(),
      logsHref: '/projects/prj_123/observability',
    }).kind).toBe('deploy');
  });

  it('maps the next step to open the website when public url exists', () => {
    expect(resolveProjectNextAction({
      status: bootstrapStatusFixture({
        steps: [
          step('connect_code', 'linked', 'Đã có repo'),
          step('connect_infra', 'ready', 'Máy chủ sẵn sàng'),
          step('deploy', 'running', 'Đang chạy'),
        ],
      }),
      repoLink: linkedRepoFixture(),
      primaryPublicURL: 'https://app.example.com',
      logsHref: '/projects/prj_123/observability',
    }).kind).toBe('open');
  });

  it('translates service status and actions into user-facing labels', () => {
    const repoService = serviceFixture();
    const liveRuntime = runtimeFixture('live');

    expect(resolveServiceDisplayStatus(repoService, liveRuntime)).toBe('Đang chạy');
    expect(resolvePrimaryServiceAction(repoService, liveRuntime).label).toBe('Deploy lại');
    expect(resolvePrimaryServiceAction({ ...repoService, source_type: 'internal', kind: 'postgres' }, undefined).label).toBe('Khởi động');
  });

  it('uses translated placement labels instead of raw cluster keys', () => {
    expect(formatPlacementValue(serviceFixture(), new Map())).toBe('Cụm dùng chung');
    expect(formatPlacementValue({ ...serviceFixture(), placement_mode: 'pinned_node', placement_node_id: 'inst_1' }, new Map())).toContain('Ghim vào máy chủ');
  });

  it('keeps bootstrap state labels in plain Vietnamese', () => {
    expect(formatBootstrapStateLabelVN('ready_to_deploy')).toBe('Sẵn sàng triển khai');
  });
});

function bootstrapStatusFixture(overrides: Partial<ProjectBootstrapStatus>): ProjectBootstrapStatus {
  return {
    project_id: 'prj_123',
    overall_state: 'not_ready',
    steps: [],
    auto_mode: {
      enabled: true,
      selected_mode: 'standalone',
      mode_source: 'user',
      mode_reason_code: 'manual',
      mode_reason_human: 'manual',
      upshift_allowed: true,
      downshift_allowed: true,
      downshift_block_reason: '',
    },
    inventory: {
      healthy_instances: 0,
      healthy_mesh_networks: 0,
      healthy_k3s_clusters: 0,
    },
    runtime_inventory: {
      sync_state: 'missing',
      app_runtime: { status: 'missing' },
      sidecar_runtime: { enabled: false, status: 'missing' },
      internal_services: [],
    },
    public_urls: [],
    updated_at: '2026-04-19T00:00:00Z',
    ...overrides,
  };
}

function step(id: 'connect_code' | 'connect_infra' | 'deploy', state: string, summary: string) {
  return { id, state, summary, actions: [] };
}

function linkedRepoFixture() {
  return {
    id: 'prl_123',
    project_id: 'prj_123',
    github_installation_id: 100,
    github_repo_id: 42,
    repo_owner: 'lazyops',
    repo_name: 'web',
    repo_full_name: 'lazyops/web',
    tracked_branch: 'main',
    preview_enabled: false,
    created_at: '2026-04-19T00:00:00Z',
    updated_at: '2026-04-19T00:00:00Z',
  };
}

function serviceFixture(): ProjectService {
  return {
    id: 'svc_123',
    project_id: 'prj_123',
    name: 'web',
    path: 'apps/web',
    kind: 'web',
    source_type: 'repo',
    public: true,
    runtime_profile: 'web',
    placement_mode: 'shared_cluster',
    connection_template_key: '',
    connection_target_service: '',
    managed_by_lazyops: false,
    start_hint: '',
    image_ref: '',
    image_digest: '',
    target_port: 0,
    service_port: 0,
    replicas: 1,
    env_bundle: {},
    pvc_spec: {},
    deploy_strategy: {},
    healthcheck: {},
    created_at: '2026-04-19T00:00:00Z',
    updated_at: '2026-04-19T00:00:00Z',
  };
}

function runtimeFixture(status: string): ProjectRuntimeService {
  return {
    service_id: 'svc_123',
    name: 'web',
    public: true,
    runtime_status: status,
    effective_node_ids: [],
    public_urls: [],
    internal_endpoints: [],
    dependencies: [],
    recent_logs: [],
  };
}
