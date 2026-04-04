export type MetricPoint = {
  timestamp: string;
  value: number;
};

export type MetricAggregate = {
  service: string;
  cpu_p95: number;
  cpu_max: number;
  cpu_min: number;
  cpu_avg: number;
  cpu_count: number;
  ram_p95: number;
  ram_max: number;
  ram_min: number;
  ram_avg: number;
  ram_count: number;
  period: string;
};

export type IdleCandidate = {
  service: string;
  reason: string;
  cpu_avg: number;
  ram_avg: number;
  request_count: number;
  recommendation: 'scale_to_zero' | 'downsize' | 'investigate';
};

export type ScaleToZeroCandidate = {
  service: string;
  current_instances: number;
  min_idle_seconds: number;
  estimated_savings_percent: number;
};

export type CostEstimate = {
  service: string;
  current_cost: number;
  optimized_cost: number;
  currency: string;
  period: string;
  caveats: string[];
};
