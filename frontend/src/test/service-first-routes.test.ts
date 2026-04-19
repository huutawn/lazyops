import { describe, expect, it } from 'vitest';
import { ADMIN_PROJECT_TABS, GUIDED_PROJECT_TABS } from '@/components/primitives/project-tabs';
import { formatActionResult, resolvePlacementHint } from '@/modules/project-services/project-service-inventory';
import { resolveGuidedProjectFlow, resolveProjectExpertRoute } from '@/modules/projects/project-flow-hooks';

describe('service-first navigation', () => {
  it('keeps only the primary project tabs in both guided and admin modes', () => {
    const guidedLabels = GUIDED_PROJECT_TABS.map((tab) => tab.label);
    const adminLabels = ADMIN_PROJECT_TABS.map((tab) => tab.label);

    expect(guidedLabels).toEqual(['Tổng quan', 'Services', 'Biến môi trường', 'Triển khai', 'Logs / Runtime']);
    expect(adminLabels).toEqual(['Tổng quan', 'Services', 'Biến môi trường', 'Triển khai', 'Logs / Runtime']);
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
    })).toContain('2 ready node');
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
});
