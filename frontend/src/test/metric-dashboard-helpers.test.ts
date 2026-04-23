import { describe, expect, it } from 'vitest';
import {
  buildMetricLinePath,
  hasMetricDashboardData,
  resolveMetricDashboardStep,
} from '@/modules/observability/metric-dashboard-helpers';

describe('metric dashboard helpers', () => {
  it('resolves default polling step from window', () => {
    expect(resolveMetricDashboardStep('1h')).toBe('1m');
    expect(resolveMetricDashboardStep('6h')).toBe('5m');
    expect(resolveMetricDashboardStep('24h')).toBe('15m');
  });

  it('detects when dashboard has no usable data', () => {
    expect(hasMetricDashboardData({
      summary: {
        request_total: 0,
        latency_p95_ms: 0,
        cpu_p95: 0,
        ram_p95_mb: 0,
        open_incidents: 0,
        recent_errors: 0,
      },
      series: [
        {
          timestamp: '2026-04-23T10:00:00Z',
          request_count: 0,
          latency_p95_ms: 0,
          cpu_p95: 0,
          ram_p95_mb: 0,
        },
      ],
      services: [],
      window: '1h',
      step: '1m',
    })).toBe(false);
  });

  it('builds a chart path for non-empty series', () => {
    const result = buildMetricLinePath([
      {
        timestamp: '2026-04-23T10:00:00Z',
        request_count: 10,
        latency_p95_ms: 120,
        cpu_p95: 25,
        ram_p95_mb: 256,
      },
      {
        timestamp: '2026-04-23T10:01:00Z',
        request_count: 35,
        latency_p95_ms: 180,
        cpu_p95: 45,
        ram_p95_mb: 300,
      },
    ], (point) => point.request_count, 240, 120);

    expect(result.linePath).toContain('M');
    expect(result.linePath).toContain('L');
    expect(result.areaPath.endsWith('Z')).toBe(true);
  });
});
