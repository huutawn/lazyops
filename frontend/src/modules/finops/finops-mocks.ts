import type { MetricAggregate, IdleCandidate, ScaleToZeroCandidate, CostEstimate, MetricPoint } from '@/modules/finops/finops-types';

const MOCK_AGGREGATES: MetricAggregate[] = [
  { service: 'web', cpu_p95: 45.2, cpu_max: 78.1, cpu_min: 5.3, cpu_avg: 28.7, cpu_count: 15420, ram_p95: 512, ram_max: 768, ram_min: 128, ram_avg: 384, ram_count: 15420, period: '24h' },
  { service: 'api', cpu_p95: 62.8, cpu_max: 95.4, cpu_min: 12.1, cpu_avg: 41.3, cpu_count: 28900, ram_p95: 1024, ram_max: 1536, ram_min: 256, ram_avg: 768, ram_count: 28900, period: '24h' },
  { service: 'worker', cpu_p95: 15.3, cpu_max: 30.2, cpu_min: 0.5, cpu_avg: 8.1, cpu_count: 0, ram_p95: 256, ram_max: 384, ram_min: 64, ram_avg: 192, ram_count: 0, period: '24h' },
];

const MOCK_IDLE_CANDIDATES: IdleCandidate[] = [
  {
    service: 'worker',
    reason: 'Zero requests in the last 24 hours with consistently low CPU usage.',
    cpu_avg: 8.1,
    ram_avg: 192,
    request_count: 0,
    recommendation: 'scale_to_zero',
  },
  {
    service: 'api',
    reason: 'High RAM usage (p95: 1024MB) with moderate CPU. Consider optimizing memory or downsizing if traffic drops.',
    cpu_avg: 41.3,
    ram_avg: 768,
    request_count: 28900,
    recommendation: 'investigate',
  },
];

const MOCK_SCALE_TO_ZERO: ScaleToZeroCandidate[] = [
  {
    service: 'worker',
    current_instances: 1,
    min_idle_seconds: 3600,
    estimated_savings_percent: 100,
  },
];

const MOCK_COST_ESTIMATES: CostEstimate[] = [
  {
    service: 'web',
    current_cost: 42.50,
    optimized_cost: 38.25,
    currency: 'USD',
    period: 'monthly',
    caveats: [
      'Estimates based on current usage patterns and may vary.',
      'Does not include network egress or storage costs.',
      'Actual costs depend on your cloud provider pricing.',
    ],
  },
  {
    service: 'api',
    current_cost: 85.00,
    optimized_cost: 72.25,
    currency: 'USD',
    period: 'monthly',
    caveats: [
      'Estimates based on current usage patterns and may vary.',
      'Memory optimization could reduce costs further.',
      'Does not include network egress or storage costs.',
    ],
  },
  {
    service: 'worker',
    current_cost: 21.00,
    optimized_cost: 0,
    currency: 'USD',
    period: 'monthly',
    caveats: [
      'Scale-to-zero could eliminate this cost entirely.',
      'Cold start latency may impact first request.',
    ],
  },
];

const MOCK_CPU_TREND: MetricPoint[] = Array.from({ length: 24 }, (_, i) => ({
  timestamp: new Date(Date.now() - (23 - i) * 3600000).toISOString(),
  value: 20 + Math.random() * 50,
}));

const MOCK_RAM_TREND: MetricPoint[] = Array.from({ length: 24 }, (_, i) => ({
  timestamp: new Date(Date.now() - (23 - i) * 3600000).toISOString(),
  value: 256 + Math.random() * 512,
}));

export async function mockFetchMetricAggregates(): Promise<MetricAggregate[]> {
  await new Promise((r) => setTimeout(r, 400));
  return MOCK_AGGREGATES;
}

export async function mockFetchIdleCandidates(): Promise<IdleCandidate[]> {
  await new Promise((r) => setTimeout(r, 300));
  return MOCK_IDLE_CANDIDATES;
}

export async function mockFetchScaleToZeroCandidates(): Promise<ScaleToZeroCandidate[]> {
  await new Promise((r) => setTimeout(r, 300));
  return MOCK_SCALE_TO_ZERO;
}

export async function mockFetchCostEstimates(): Promise<CostEstimate[]> {
  await new Promise((r) => setTimeout(r, 500));
  return MOCK_COST_ESTIMATES;
}

export async function mockFetchCpuTrend(): Promise<MetricPoint[]> {
  await new Promise((r) => setTimeout(r, 200));
  return MOCK_CPU_TREND;
}

export async function mockFetchRamTrend(): Promise<MetricPoint[]> {
  await new Promise((r) => setTimeout(r, 200));
  return MOCK_RAM_TREND;
}
