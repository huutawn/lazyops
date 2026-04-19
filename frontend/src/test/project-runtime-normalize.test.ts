import { describe, expect, it } from 'vitest';
import { normalizeProjectRuntimeSummary } from '@/modules/project-runtime/project-runtime-normalize';

describe('project runtime normalization', () => {
  it('fills missing runtime arrays so runtime workspace does not crash on sparse payloads', () => {
    const result = normalizeProjectRuntimeSummary('prj_123', {
      project_id: 'prj_123',
      sync_state: 'queued',
      services: [
        {
          service_id: 'svc_1',
          name: 'api',
          public: true,
          runtime_status: 'queued',
        },
      ],
    });

    expect(result.public_urls).toEqual([]);
    expect(result.nodes).toEqual([]);
    expect(result.services[0]?.effective_node_ids).toEqual([]);
    expect(result.services[0]?.public_urls).toEqual([]);
    expect(result.services[0]?.internal_endpoints).toEqual([]);
    expect(result.services[0]?.dependencies).toEqual([]);
    expect(result.services[0]?.recent_logs).toEqual([]);
  });
});
