'use client';

import { useMemo } from 'react';
import {
  useMetricAggregates,
  useIdleCandidates,
  useScaleToZeroCandidates,
  useCostEstimates,
  useCpuTrend,
  useRamTrend,
} from '@/modules/finops/finops-hooks';
import { PageHeader } from '@/components/primitives/page-header';
import { SectionCard } from '@/components/primitives/section-card';
import { EmptyState } from '@/components/primitives/empty-state';
import { ErrorState } from '@/components/primitives/error-state';
import { SkeletonPage } from '@/components/primitives/skeleton';
import { StatusBadge } from '@/components/primitives/status-badge';
import { HealthChip } from '@/components/primitives/health-chip';

const RECOMMENDATION_LABELS: Record<string, string> = {
  scale_to_zero: 'Scale to zero',
  downsize: 'Downsize',
  investigate: 'Investigate',
};

const RECOMMENDATION_VARIANT: Record<string, 'success' | 'warning' | 'info'> = {
  scale_to_zero: 'success',
  downsize: 'warning',
  investigate: 'info',
};

export default function FinOpsPage() {
  const { data: aggregates, isLoading: aggLoading, isError: aggError } = useMetricAggregates();
  const { data: idleCandidates, isLoading: idleLoading } = useIdleCandidates();
  const { data: scaleToZero, isLoading: s2zLoading } = useScaleToZeroCandidates();
  const { data: costEstimates, isLoading: costLoading } = useCostEstimates();
  const { data: cpuTrend } = useCpuTrend();
  const { data: ramTrend } = useRamTrend();

  const totalCurrentCost = useMemo(
    () => costEstimates?.reduce((sum, c) => sum + c.current_cost, 0) ?? 0,
    [costEstimates],
  );
  const totalOptimizedCost = useMemo(
    () => costEstimates?.reduce((sum, c) => sum + c.optimized_cost, 0) ?? 0,
    [costEstimates],
  );
  const potentialSavings = totalCurrentCost - totalOptimizedCost;

  if (aggLoading || idleLoading || s2zLoading || costLoading) {
    return <SkeletonPage title cards={4} />;
  }

  if (aggError) {
    return (
      <div className="flex flex-col gap-6">
        <PageHeader title="FinOps" subtitle="Resource optimization and cost insights." />
        <ErrorState title="Failed to load FinOps data" message="Could not fetch metric data." />
      </div>
    );
  }

  return (
    <div className="flex flex-col gap-6">
      <PageHeader
        title="FinOps"
        subtitle="Resource utilization and cost optimization recommendations."
      />

      <div className="grid gap-4 sm:grid-cols-3">
        <SectionCard title="Current monthly cost">
          <div className="text-3xl font-bold text-lazyops-text">${totalCurrentCost.toFixed(2)}</div>
          <p className="text-xs text-lazyops-muted">Estimated across all services</p>
        </SectionCard>

        <SectionCard title="Optimized monthly cost">
          <div className="text-3xl font-bold text-health-healthy">${totalOptimizedCost.toFixed(2)}</div>
          <p className="text-xs text-lazyops-muted">With recommendations applied</p>
        </SectionCard>

        <SectionCard title="Potential savings">
          <div className="text-3xl font-bold text-primary">${potentialSavings.toFixed(2)}</div>
          <p className="text-xs text-lazyops-muted">
            {potentialSavings > 0 ? `${((potentialSavings / totalCurrentCost) * 100).toFixed(0)}% reduction possible` : 'No savings identified'}
          </p>
        </SectionCard>
      </div>

      <SectionCard title="CPU trend (24h)" description="Average CPU usage across all services.">
        {cpuTrend && <TrendChart data={cpuTrend} label="CPU %" max={100} color="var(--primary)" />}
      </SectionCard>

      <SectionCard title="RAM trend (24h)" description="Average RAM usage across all services.">
        {ramTrend && <TrendChart data={ramTrend} label="RAM MB" max={2048} color="var(--success)" />}
      </SectionCard>

      {aggregates && aggregates.length > 0 && (
        <SectionCard title="Metric aggregates" description="24-hour summary statistics per service.">
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead>
                <tr className="border-b border-lazyops-border">
                  <th className="px-3 py-2 text-left text-xs font-medium text-lazyops-muted">Service</th>
                  <th className="px-3 py-2 text-left text-xs font-medium text-lazyops-muted">CPU p95</th>
                  <th className="px-3 py-2 text-left text-xs font-medium text-lazyops-muted">CPU max</th>
                  <th className="px-3 py-2 text-left text-xs font-medium text-lazyops-muted">CPU min</th>
                  <th className="px-3 py-2 text-left text-xs font-medium text-lazyops-muted">CPU avg</th>
                  <th className="px-3 py-2 text-left text-xs font-medium text-lazyops-muted">RAM p95</th>
                  <th className="px-3 py-2 text-left text-xs font-medium text-lazyops-muted">RAM max</th>
                  <th className="px-3 py-2 text-left text-xs font-medium text-lazyops-muted">RAM min</th>
                  <th className="px-3 py-2 text-left text-xs font-medium text-lazyops-muted">RAM avg</th>
                  <th className="px-3 py-2 text-left text-xs font-medium text-lazyops-muted">Count</th>
                </tr>
              </thead>
              <tbody>
                {aggregates.map((a) => (
                  <tr key={a.service} className="border-b border-lazyops-border/50">
                    <td className="px-3 py-2 font-medium text-lazyops-text">{a.service}</td>
                    <td className="px-3 py-2 font-mono text-xs">{a.cpu_p95}%</td>
                    <td className="px-3 py-2 font-mono text-xs">{a.cpu_max}%</td>
                    <td className="px-3 py-2 font-mono text-xs">{a.cpu_min}%</td>
                    <td className="px-3 py-2 font-mono text-xs">{a.cpu_avg}%</td>
                    <td className="px-3 py-2 font-mono text-xs">{a.ram_p95}MB</td>
                    <td className="px-3 py-2 font-mono text-xs">{a.ram_max}MB</td>
                    <td className="px-3 py-2 font-mono text-xs">{a.ram_min}MB</td>
                    <td className="px-3 py-2 font-mono text-xs">{a.ram_avg}MB</td>
                    <td className="px-3 py-2 font-mono text-xs text-lazyops-muted">{a.cpu_count.toLocaleString()}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </SectionCard>
      )}

      {idleCandidates && idleCandidates.length > 0 && (
        <SectionCard
          title="Idle candidates & recommendations"
          description="Services that may benefit from optimization."
        >
          <div className="flex flex-col gap-3">
            {idleCandidates.map((c) => (
              <div key={c.service} className="rounded-lg border border-lazyops-border bg-lazyops-bg-accent/30 p-4">
                <div className="mb-2 flex items-center justify-between">
                  <div className="flex items-center gap-2">
                    <span className="text-sm font-medium text-lazyops-text">{c.service}</span>
                    <StatusBadge
                      label={RECOMMENDATION_LABELS[c.recommendation]}
                      variant={RECOMMENDATION_VARIANT[c.recommendation] ?? 'neutral'}
                      size="sm"
                      dot={false}
                    />
                  </div>
                </div>
                <p className="mb-2 text-xs text-lazyops-muted">{c.reason}</p>
                <div className="grid gap-2 sm:grid-cols-3 text-xs">
                  <span className="text-lazyops-muted">CPU avg: <span className="text-lazyops-text">{c.cpu_avg}%</span></span>
                  <span className="text-lazyops-muted">RAM avg: <span className="text-lazyops-text">{c.ram_avg}MB</span></span>
                  <span className="text-lazyops-muted">Requests: <span className="text-lazyops-text">{c.request_count.toLocaleString()}</span></span>
                </div>
              </div>
            ))}
          </div>
        </SectionCard>
      )}

      {scaleToZero && scaleToZero.length > 0 && (
        <SectionCard title="Scale-to-zero candidates" description="Services that could be scaled down when idle.">
          <div className="flex flex-col gap-3">
            {scaleToZero.map((c) => (
              <div key={c.service} className="rounded-lg border border-health-healthy/30 bg-health-healthy/5 p-4">
                <div className="mb-2 flex items-center justify-between">
                  <span className="text-sm font-medium text-lazyops-text">{c.service}</span>
                  <HealthChip label={`Save ${c.estimated_savings_percent}%`} status="healthy" size="sm" />
                </div>
                <div className="grid gap-2 sm:grid-cols-2 text-xs">
                  <span className="text-lazyops-muted">Current instances: <span className="text-lazyops-text">{c.current_instances}</span></span>
                  <span className="text-lazyops-muted">Min idle time: <span className="text-lazyops-text">{c.min_idle_seconds / 60} minutes</span></span>
                </div>
              </div>
            ))}
          </div>
        </SectionCard>
      )}

      {costEstimates && costEstimates.length > 0 && (
        <SectionCard
          title="Cost advisory"
          description="Estimated monthly costs per service with optimization potential."
        >
          <div className="flex flex-col gap-4">
            {costEstimates.map((c) => (
              <div key={c.service} className="rounded-lg border border-lazyops-border bg-lazyops-bg-accent/30 p-4">
                <div className="mb-2 flex items-center justify-between">
                  <span className="text-sm font-medium text-lazyops-text">{c.service}</span>
                  {c.optimized_cost < c.current_cost && (
                    <HealthChip label={`Save $${(c.current_cost - c.optimized_cost).toFixed(2)}/mo`} status="healthy" size="sm" />
                  )}
                </div>
                <div className="grid gap-2 sm:grid-cols-2 text-xs">
                  <span className="text-lazyops-muted">Current: <span className="text-lazyops-text font-medium">${c.current_cost.toFixed(2)}/{c.period}</span></span>
                  <span className="text-lazyops-muted">Optimized: <span className="text-health-healthy font-medium">${c.optimized_cost.toFixed(2)}/{c.period}</span></span>
                </div>
                <div className="mt-3 rounded-md bg-lazyops-bg/50 p-3">
                  <p className="mb-1 text-[10px] font-medium uppercase tracking-wider text-lazyops-muted/70">Caveats</p>
                  <ul className="list-disc space-y-0.5 pl-4 text-[10px] text-lazyops-muted/60">
                    {c.caveats.map((caveat, i) => (
                      <li key={i}>{caveat}</li>
                    ))}
                  </ul>
                </div>
              </div>
            ))}

            <div className="rounded-lg border border-health-degraded/30 bg-health-degraded/5 p-3 text-xs text-health-degraded">
              <p className="font-medium">Disclaimer</p>
              <p className="mt-1 text-lazyops-muted">
                These are estimates based on current usage patterns and do not reflect actual billing.
                Real costs depend on your cloud provider, region, reserved instances, and other factors.
                Use these numbers as directional guidance, not as financial commitments.
              </p>
            </div>
          </div>
        </SectionCard>
      )}
    </div>
  );
}

function TrendChart({ data, label, max, color }: { data: { timestamp: string; value: number }[]; label: string; max: number; color: string }) {
  const values = data.map((d) => d.value);
  const minVal = Math.min(...values);
  const maxVal = Math.max(...values);
  const range = maxVal - minVal || 1;
  const width = 600;
  const height = 80;
  const padding = 4;

  const points = data.map((d, i) => {
    const x = padding + (i / (data.length - 1)) * (width - padding * 2);
    const y = height - padding - ((d.value - minVal) / range) * (height - padding * 2);
    return `${x},${y}`;
  }).join(' ');

  return (
    <div className="flex flex-col gap-2">
      <svg viewBox={`0 0 ${width} ${height}`} className="w-full" preserveAspectRatio="none">
        <polyline
          fill="none"
          stroke={color}
          strokeWidth="2"
          points={points}
          vectorEffect="non-scaling-stroke"
        />
      </svg>
      <div className="flex justify-between text-[10px] text-lazyops-muted/60">
        <span>{minVal.toFixed(1)} {label}</span>
        <span>avg: {(values.reduce((a, b) => a + b, 0) / values.length).toFixed(1)} {label}</span>
        <span>{maxVal.toFixed(1)} {label}</span>
      </div>
    </div>
  );
}
