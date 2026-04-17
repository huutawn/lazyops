import { describe, expect, it } from 'vitest';
import { listObservedServices, mergeObservedLogs, normalizeLiveLogEnvelope } from '@/modules/observability/observability-live';

describe('observability live helpers', () => {
  it('normalizes live log envelopes for the active project', () => {
    const items = normalizeLiveLogEnvelope({
      type: 'logs.live',
      payload: {
        project_id: 'prj_123',
        service_name: 'api',
        revision_id: 'rev_123',
        entries: [
          {
            timestamp: '2026-04-16T12:00:00Z',
            severity: 'warn',
            source: 'k3s-pod',
            message: 'slow query',
          },
        ],
      },
    }, 'prj_123');

    expect(items).toHaveLength(1);
    expect(items[0]?.service).toBe('api');
    expect(items[0]?.revision_id).toBe('rev_123');
    expect(items[0]?.level).toBe('warn');
  });

  it('ignores envelopes for other projects', () => {
    const items = normalizeLiveLogEnvelope({
      type: 'logs.live',
      payload: {
        project_id: 'prj_other',
        service_name: 'api',
        entries: [{ timestamp: '2026-04-16T12:00:00Z', message: 'ignored' }],
      },
    }, 'prj_123');

    expect(items).toEqual([]);
  });

  it('merges historical and live logs without duplicating identical records', () => {
    const merged = mergeObservedLogs(
      [
        {
          id: '1',
          service: 'api',
          revision_id: 'rev_123',
          source: 'k3s-pod',
          level: 'info',
          message: 'ready',
          timestamp: '2026-04-16T12:00:00Z',
        },
      ],
      [
        {
          id: '2',
          service: 'api',
          revision_id: 'rev_123',
          source: 'k3s-pod',
          level: 'info',
          message: 'ready',
          timestamp: '2026-04-16T12:00:00Z',
        },
        {
          id: '3',
          service: 'web',
          revision_id: 'rev_123',
          source: 'k3s-pod',
          level: 'info',
          message: 'serving traffic',
          timestamp: '2026-04-16T12:00:01Z',
        },
      ],
    );

    expect(merged).toHaveLength(2);
    expect(merged[0]?.service).toBe('api');
    expect(merged[1]?.service).toBe('web');
  });

  it('lists distinct services for service filtering', () => {
    const services = listObservedServices([
      {
        id: '1',
        service: 'web',
        level: 'info',
        message: 'ok',
        timestamp: '2026-04-16T12:00:00Z',
      },
      {
        id: '2',
        service: 'api',
        level: 'info',
        message: 'ok',
        timestamp: '2026-04-16T12:00:01Z',
      },
      {
        id: '3',
        service: 'web',
        level: 'warn',
        message: 'slow',
        timestamp: '2026-04-16T12:00:02Z',
      },
    ]);

    expect(services).toEqual(['api', 'web']);
  });
});
