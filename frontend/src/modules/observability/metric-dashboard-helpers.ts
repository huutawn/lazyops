import type { MetricDashboardPoint, MetricDashboardRecord } from '@/modules/observability/observability-types';

export const OBSERVABILITY_WINDOWS = ['1h', '6h', '24h'] as const;
export type ObservabilityWindow = (typeof OBSERVABILITY_WINDOWS)[number];

export function resolveMetricDashboardStep(window: string): '1m' | '5m' | '15m' {
  switch (window) {
    case '24h':
      return '15m';
    case '6h':
      return '5m';
    case '1h':
    default:
      return '1m';
  }
}

export function hasMetricDashboardData(dashboard?: MetricDashboardRecord | null): boolean {
  if (!dashboard) {
    return false;
  }
  if (dashboard.summary.request_total > 0) {
    return true;
  }
  return dashboard.series.some((point) =>
    point.request_count > 0 ||
    point.latency_p95_ms > 0 ||
    point.cpu_p95 > 0 ||
    point.ram_p95_mb > 0,
  );
}

export function buildMetricLinePath(
  series: MetricDashboardPoint[],
  selectValue: (point: MetricDashboardPoint) => number,
  width: number,
  height: number,
) {
  if (series.length === 0) {
    return { linePath: '', areaPath: '' };
  }

  const values = series.map(selectValue);
  const max = Math.max(...values, 0);
  const min = Math.min(...values, 0);
  const range = max - min || 1;

  const points = series.map((point, index) => {
    const x = series.length === 1 ? width / 2 : (index / (series.length - 1)) * width;
    const normalized = (selectValue(point) - min) / range;
    const y = height - normalized * height;
    return { x, y };
  });

  const linePath = points
    .map((point, index) => `${index === 0 ? 'M' : 'L'} ${point.x.toFixed(2)} ${point.y.toFixed(2)}`)
    .join(' ');

  const areaPath = points.length > 0
    ? `${linePath} L ${points[points.length - 1]?.x.toFixed(2)} ${height.toFixed(2)} L ${points[0]?.x.toFixed(2)} ${height.toFixed(2)} Z`
    : '';

  return { linePath, areaPath };
}

export function formatMetricTimestampLabel(timestamp: string, window: string): string {
  const date = new Date(timestamp);
  return window === '24h'
    ? date.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
    : date.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
}
