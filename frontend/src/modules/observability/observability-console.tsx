'use client';

import { useEffect, useMemo, useState } from 'react';
import { buildMetricLinePath, formatMetricTimestampLabel, hasMetricDashboardData, OBSERVABILITY_WINDOWS, resolveMetricDashboardStep, type ObservabilityWindow } from '@/modules/observability/metric-dashboard-helpers';
import { listObservedServices, mergeObservedLogs } from '@/modules/observability/observability-live';
import { useIncidents, useLiveLogs, useLogs, useMetricDashboard, useMetrics, useTrace } from '@/modules/observability/observability-hooks';
import type { LogLevel, MetricDashboardPoint, MetricDashboardRecord, MetricRecord } from '@/modules/observability/observability-types';
import { useProjects } from '@/modules/projects/project-hooks';
import { PageHeader } from '@/components/primitives/page-header';
import { SectionCard } from '@/components/primitives/section-card';
import { EmptyState } from '@/components/primitives/empty-state';
import { ErrorState } from '@/components/primitives/error-state';
import { SkeletonPage } from '@/components/primitives/skeleton';
import { StatusBadge } from '@/components/primitives/status-badge';

const LOG_LEVEL_COLORS: Record<LogLevel, string> = {
  info: 'text-lazyops-muted',
  warn: 'text-health-degraded',
  error: 'text-health-unhealthy',
  debug: 'text-lazyops-muted/60',
};

const INCIDENT_SEVERITY_VARIANT: Record<string, 'danger' | 'warning' | 'info'> = {
  critical: 'danger',
  warning: 'warning',
  info: 'info',
};

const INCIDENT_STATUS_VARIANT: Record<string, 'danger' | 'warning' | 'success' | 'neutral'> = {
  open: 'danger',
  investigating: 'warning',
  resolved: 'success',
  dismissed: 'neutral',
};

type ObservabilityConsoleProps = {
  fixedProjectId?: string;
  title?: string;
  subtitle?: string;
};

export function ObservabilityConsole({
  fixedProjectId,
  title = 'Observability',
  subtitle = 'Logs, traces, incidents, and metrics for your services.',
}: ObservabilityConsoleProps) {
  const [activeTab, setActiveTab] = useState<'overview' | 'logs' | 'traces' | 'incidents'>('overview');
  const [logFilter, setLogFilter] = useState<LogLevel | 'all'>('all');
  const [logServiceFilter, setLogServiceFilter] = useState('all');
  const [metricServiceFilter, setMetricServiceFilter] = useState('all');
  const [metricWindow, setMetricWindow] = useState<ObservabilityWindow>('1h');
  const [followMode, setFollowMode] = useState(false);
  const [traceQuery, setTraceQuery] = useState('');
  const [projectId, setProjectId] = useState(fixedProjectId ?? '');

  const {
    data: projectsData,
    isLoading: projectsLoading,
    isError: projectsError,
  } = useProjects();
  const projects = useMemo(() => projectsData?.items ?? [], [projectsData?.items]);
  const activeProjectId = useMemo(() => {
    if (fixedProjectId) {
      return fixedProjectId;
    }
    if (projects.length === 0) {
      return '';
    }
    if (projectId && projects.some((p) => p.id === projectId)) {
      return projectId;
    }
    return projects[0]?.id ?? '';
  }, [fixedProjectId, projectId, projects]);

  const { data: logs, isLoading: logsLoading, isError: logsError } = useLogs(activeProjectId);
  const liveLogs = useLiveLogs(activeProjectId, followMode);
  const { data: incidents, isLoading: incidentsLoading, isError: incidentsError } = useIncidents(activeProjectId);
  const { data: metrics, isLoading: metricsLoading, isError: metricsError } = useMetrics(activeProjectId);
  const metricStep = useMemo(() => resolveMetricDashboardStep(metricWindow), [metricWindow]);
  const selectedMetricService = metricServiceFilter === 'all' ? undefined : metricServiceFilter;
  const {
    data: metricDashboard,
    isLoading: metricDashboardLoading,
    isError: metricDashboardError,
  } = useMetricDashboard(activeProjectId, { service: selectedMetricService, window: metricWindow, step: metricStep });
  const { data: trace, isLoading: traceLoading } = useTrace(traceQuery);

  const mergedLogs = useMemo(() => mergeObservedLogs(logs ?? [], liveLogs), [liveLogs, logs]);
  const observedServices = useMemo(() => listObservedServices(mergedLogs), [mergedLogs]);
  const metricServiceOptions = useMemo(() => {
    const options = new Set<string>();
    (metricDashboard?.services ?? []).forEach((service) => options.add(service));
    (metrics ?? []).forEach((metric) => options.add(metric.service));
    return Array.from(options).sort((a, b) => a.localeCompare(b));
  }, [metricDashboard?.services, metrics]);
  const filteredLogs = useMemo(
    () => mergedLogs.filter((item) => {
      if (logFilter !== 'all' && item.level !== logFilter) {
        return false;
      }
      if (logServiceFilter !== 'all' && item.service !== logServiceFilter) {
        return false;
      }
      return true;
    }),
    [logFilter, logServiceFilter, mergedLogs],
  );

  useEffect(() => {
    if (logServiceFilter !== 'all' && !observedServices.includes(logServiceFilter)) {
      setLogServiceFilter('all');
    }
  }, [logServiceFilter, observedServices]);

  useEffect(() => {
    if (metricServiceFilter !== 'all' && !metricServiceOptions.includes(metricServiceFilter)) {
      setMetricServiceFilter('all');
    }
  }, [metricServiceFilter, metricServiceOptions]);

  if ((!fixedProjectId && projectsLoading) || logsLoading || incidentsLoading || metricsLoading || metricDashboardLoading) {
    return <SkeletonPage title cards={3} />;
  }

  if ((!fixedProjectId && projectsError) || logsError || incidentsError || metricsError || metricDashboardError) {
    return (
      <div className="flex flex-col gap-6">
        <PageHeader title={title} subtitle={subtitle} />
        <ErrorState title="Failed to load observability" message="Could not fetch observability data for this project." />
      </div>
    );
  }

  if (!fixedProjectId && projects.length === 0) {
    return (
      <div className="flex flex-col gap-6">
        <PageHeader title={title} subtitle={subtitle} />
        <SectionCard title="No projects" description="Create a project to start collecting observability data.">
          <EmptyState
            title="No projects available"
            description="Create your first project, then run deployments to see logs, incidents, and metrics."
          />
        </SectionCard>
      </div>
    );
  }

  const openIncidents = incidents?.filter((i) => i.status === 'open' || i.status === 'investigating') ?? [];
  const errorLogs = logs?.filter((l) => l.level === 'error') ?? [];

  return (
    <div className="flex flex-col gap-6">
      <PageHeader
        title={title}
        subtitle={subtitle}
        actions={
          !fixedProjectId ? (
            <select
              value={activeProjectId}
              onChange={(e) => setProjectId(e.target.value)}
              className="rounded-lg border border-lazyops-border bg-lazyops-bg-accent/50 px-3 py-2 text-base text-lazyops-text outline-none focus:border-primary/60 focus:ring-1 focus:ring-primary/30"
            >
              {projects.map((project) => (
                <option key={project.id} value={project.id}>
                  {project.name}
                </option>
              ))}
            </select>
          ) : null
        }
      />

      <div className="flex gap-2 border-b border-lazyops-border">
        {(['overview', 'logs', 'traces', 'incidents'] as const).map((tab) => (
          <button
            key={tab}
            type="button"
            className={`rounded-t-lg px-6 py-2 text-base font-medium transition-colors ${
              activeTab === tab
                ? 'border-b-2 border-primary text-primary'
                : 'text-lazyops-muted hover:text-lazyops-text'
            }`}
            onClick={() => setActiveTab(tab)}
          >
            {tab.charAt(0).toUpperCase() + tab.slice(1)}
            {tab === 'incidents' && openIncidents.length > 0 && (
              <span className="ml-1.5 rounded-full bg-health-unhealthy/20 px-1.5 py-0.5 text-[10px] text-health-unhealthy">
                {openIncidents.length}
              </span>
            )}
          </button>
        ))}
      </div>

      {activeTab === 'overview' && (
        <OverviewTab
          dashboard={metricDashboard}
          metricWindow={metricWindow}
          onMetricWindowChange={setMetricWindow}
          metricServiceFilter={metricServiceFilter}
          onMetricServiceFilterChange={setMetricServiceFilter}
          metricServiceOptions={metricServiceOptions}
          openIncidents={openIncidents}
          errorLogs={errorLogs}
          metrics={metrics}
        />
      )}

      {activeTab === 'logs' && (
        <LogsTab
          logs={filteredLogs}
          logFilter={logFilter}
          onFilterChange={setLogFilter}
          serviceFilter={logServiceFilter}
          serviceOptions={observedServices}
          onServiceFilterChange={setLogServiceFilter}
          followMode={followMode}
          onFollowToggle={() => setFollowMode(!followMode)}
        />
      )}

      {activeTab === 'traces' && (
        <TracesTab
          traceQuery={traceQuery}
          onQueryChange={setTraceQuery}
          trace={trace}
          isLoading={traceLoading}
        />
      )}

      {activeTab === 'incidents' && (
        <IncidentsTab incidents={incidents} />
      )}
    </div>
  );
}

function OverviewTab({
  dashboard,
  metricWindow,
  onMetricWindowChange,
  metricServiceFilter,
  onMetricServiceFilterChange,
  metricServiceOptions,
  openIncidents,
  errorLogs,
  metrics,
}: {
  dashboard?: MetricDashboardRecord | null;
  metricWindow: ObservabilityWindow;
  onMetricWindowChange: (window: ObservabilityWindow) => void;
  metricServiceFilter: string;
  onMetricServiceFilterChange: (service: string) => void;
  metricServiceOptions: string[];
  openIncidents: { id: string; severity: string; status: string; summary: string }[];
  errorLogs: { id: string; message: string; timestamp: string }[];
  metrics?: MetricRecord[];
}) {
  const hasDashboardData = hasMetricDashboardData(dashboard);

  return (
    <div className="flex flex-col gap-4">
      <SectionCard
        title="Realtime monitoring"
        description="Prometheus-like overview powered by agent rollups and gateway access metrics."
        actions={(
          <div className="flex flex-wrap items-center gap-2">
            <select
              value={metricServiceFilter}
              onChange={(event) => onMetricServiceFilterChange(event.target.value)}
              className="rounded-md border border-lazyops-border bg-lazyops-bg-accent/50 px-3 py-2 text-sm text-lazyops-text outline-none focus:border-primary/60"
            >
              <option value="all">all services</option>
              {metricServiceOptions.map((service) => (
                <option key={service} value={service}>
                  {service}
                </option>
              ))}
            </select>
            <div className="flex overflow-hidden rounded-xl border border-lazyops-border bg-lazyops-bg-accent/40 p-1">
              {OBSERVABILITY_WINDOWS.map((window) => (
                <button
                  key={window}
                  type="button"
                  onClick={() => onMetricWindowChange(window)}
                  className={`rounded-lg px-3 py-1.5 text-sm font-medium transition-colors ${
                    metricWindow === window
                      ? 'bg-primary/15 text-primary'
                      : 'text-lazyops-muted hover:text-lazyops-text'
                  }`}
                >
                  {window}
                </button>
              ))}
            </div>
          </div>
        )}
      >
        <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-5">
          <KpiCard
            label="Requests"
            value={formatCompactNumber(dashboard?.summary.request_total ?? 0)}
            tone="sky"
            detail={`${dashboard?.summary.recent_errors ?? 0} recent errors`}
          />
          <KpiCard
            label="Latency p95"
            value={formatLatency(dashboard?.summary.latency_p95_ms ?? 0)}
            tone="amber"
            detail="Latest non-empty bucket"
          />
          <KpiCard
            label="CPU p95"
            value={formatPercent(dashboard?.summary.cpu_p95 ?? 0)}
            tone="emerald"
            detail={`${metrics?.length ?? 0} services reporting`}
          />
          <KpiCard
            label="RAM p95"
            value={formatMegabytes(dashboard?.summary.ram_p95_mb ?? 0)}
            tone="violet"
            detail="Latest memory pressure"
          />
          <KpiCard
            label="Open incidents"
            value={String(dashboard?.summary.open_incidents ?? openIncidents.length)}
            tone="rose"
            detail={openIncidents.length > 0 ? 'Needs attention' : 'All clear'}
          />
        </div>

        {hasDashboardData ? (
          <div className="mt-5 grid gap-4 xl:grid-cols-2">
            <MetricTrendChart
              title="Traffic"
              subtitle="Request count per bucket"
              series={dashboard?.series ?? []}
              window={metricWindow}
              color="#38BDF8"
              fill="rgba(56, 189, 248, 0.18)"
              formatValue={(value) => formatCompactNumber(value)}
              selectValue={(point) => point.request_count}
            />
            <MetricTrendChart
              title="Latency"
              subtitle="p95 request latency"
              series={dashboard?.series ?? []}
              window={metricWindow}
              color="#F59E0B"
              fill="rgba(245, 158, 11, 0.18)"
              formatValue={(value) => formatLatency(value)}
              selectValue={(point) => point.latency_p95_ms}
            />
            <MetricTrendChart
              title="CPU pressure"
              subtitle="p95 CPU utilization"
              series={dashboard?.series ?? []}
              window={metricWindow}
              color="#34D399"
              fill="rgba(52, 211, 153, 0.18)"
              formatValue={(value) => formatPercent(value)}
              selectValue={(point) => point.cpu_p95}
            />
            <MetricTrendChart
              title="Memory pressure"
              subtitle="p95 RAM usage"
              series={dashboard?.series ?? []}
              window={metricWindow}
              color="#A78BFA"
              fill="rgba(167, 139, 250, 0.18)"
              formatValue={(value) => formatMegabytes(value)}
              selectValue={(point) => point.ram_p95_mb}
            />
          </div>
        ) : (
          <div className="mt-5">
            <EmptyState
              title="Chua co du lieu realtime"
              description="Dashboard se hien bieu do sau khi agent gui metric rollup va gateway bat dau ghi request count."
            />
          </div>
        )}
      </SectionCard>

      <div className="grid gap-4 sm:grid-cols-2">
        <SectionCard title="Open incidents">
          <div className="text-4xl font-bold text-health-unhealthy">{openIncidents.length}</div>
          <p className="text-sm text-lazyops-muted">
            {openIncidents.length > 0 ? 'Requires attention' : 'All clear'}
          </p>
        </SectionCard>

        <SectionCard title="Recent errors">
          <div className="text-4xl font-bold text-health-unhealthy">{errorLogs.length}</div>
          <p className="text-sm text-lazyops-muted">Error log entries</p>
        </SectionCard>
      </div>

      {openIncidents.length > 0 && (
        <SectionCard title="Active incidents" description="Incidents requiring attention.">
          <div className="flex flex-col gap-2">
            {openIncidents.map((inc) => (
              <div key={inc.id} className="flex items-center justify-between rounded-lg bg-lazyops-bg-accent/50 px-6 py-3">
                <div className="flex items-center gap-3">
                  <StatusBadge
                    label={inc.severity}
                    variant={INCIDENT_SEVERITY_VARIANT[inc.severity] ?? 'neutral'}
                    size="sm"
                  />
                  <span className="text-base text-lazyops-text">{inc.summary}</span>
                </div>
                <StatusBadge
                  label={inc.status}
                  variant={INCIDENT_STATUS_VARIANT[inc.status] ?? 'neutral'}
                  size="sm"
                  dot={false}
                />
              </div>
            ))}
          </div>
        </SectionCard>
      )}

      <SectionCard title="Service metrics" description="Real rollup data from agent metrics and gateway access logs.">
        {metrics && metrics.length > 0 ? (
          <div className="overflow-x-auto">
            <table className="w-full text-base">
              <thead>
                <tr className="border-b border-lazyops-border">
                  <th className="px-6 py-2 text-left text-sm font-medium text-lazyops-muted">Service</th>
                  <th className="px-6 py-2 text-left text-sm font-medium text-lazyops-muted">CPU p95</th>
                  <th className="px-6 py-2 text-left text-sm font-medium text-lazyops-muted">RAM p95</th>
                  <th className="px-6 py-2 text-left text-sm font-medium text-lazyops-muted">Disk p95</th>
                  <th className="px-6 py-2 text-left text-sm font-medium text-lazyops-muted">Net RX</th>
                  <th className="px-6 py-2 text-left text-sm font-medium text-lazyops-muted">Net TX</th>
                  <th className="px-6 py-2 text-left text-sm font-medium text-lazyops-muted">Requests</th>
                </tr>
              </thead>
              <tbody>
                {metrics.map((m) => (
                  <tr key={m.service} className="border-b border-lazyops-border/50">
                    <td className="px-6 py-2 font-medium text-lazyops-text">{m.service}</td>
                    <td className="px-6 py-2 font-mono text-sm">
                      <MetricBar value={m.cpu_p95} max={100} unit="%" />
                    </td>
                    <td className="px-6 py-2 font-mono text-sm">
                      <MetricBar value={m.ram_p95} max={2048} unit="MB" />
                    </td>
                    <td className="px-6 py-2 font-mono text-sm text-lazyops-muted">
                      {formatBytes(m.disk_p95_bytes)}
                    </td>
                    <td className="px-6 py-2 font-mono text-sm text-lazyops-muted">
                      {formatBytes(m.network_in_total_bytes)}
                    </td>
                    <td className="px-6 py-2 font-mono text-sm text-lazyops-muted">
                      {formatBytes(m.network_out_total_bytes)}
                    </td>
                    <td className="px-6 py-2 font-mono text-sm text-lazyops-muted">
                      {m.request_count.toLocaleString()}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        ) : (
          <EmptyState
            title="Chua co du lieu metric that"
            description="Metrics se xuat hien sau khi agent gui rollup CPU, RAM, Disk, Network va request count that."
          />
        )}
      </SectionCard>
    </div>
  );
}

function KpiCard({ label, value, detail, tone }: { label: string; value: string; detail: string; tone: 'sky' | 'amber' | 'emerald' | 'violet' | 'rose' }) {
  const toneStyles = {
    sky: 'from-[#0ea5e9]/15 to-[#0f172a] text-[#38bdf8]',
    amber: 'from-[#f59e0b]/15 to-[#0f172a] text-[#fbbf24]',
    emerald: 'from-[#10b981]/15 to-[#0f172a] text-[#34d399]',
    violet: 'from-[#8b5cf6]/15 to-[#0f172a] text-[#c4b5fd]',
    rose: 'from-[#f43f5e]/15 to-[#0f172a] text-[#fb7185]',
  } as const;

  return (
    <div className={`rounded-2xl border border-white/5 bg-gradient-to-br p-5 ${toneStyles[tone]}`}>
      <div className="text-sm font-medium text-lazyops-muted">{label}</div>
      <div className="mt-3 text-3xl font-semibold">{value}</div>
      <div className="mt-2 text-sm text-lazyops-muted">{detail}</div>
    </div>
  );
}

function MetricTrendChart({
  title,
  subtitle,
  series,
  window,
  color,
  fill,
  selectValue,
  formatValue,
}: {
  title: string;
  subtitle: string;
  series: MetricDashboardPoint[];
  window: string;
  color: string;
  fill: string;
  selectValue: (point: MetricDashboardPoint) => number;
  formatValue: (value: number) => string;
}) {
  const chartWidth = 480;
  const chartHeight = 180;
  const { linePath, areaPath } = buildMetricLinePath(series, selectValue, chartWidth, chartHeight);
  const latestValue = series.length > 0 ? selectValue(series[series.length - 1] as MetricDashboardPoint) : 0;
  const peakValue = series.reduce((max, point) => Math.max(max, selectValue(point)), 0);
  const gradientId = `chart-${title.toLowerCase().replace(/[^a-z0-9]+/g, '-')}`;
  const labels = series.length > 0
    ? [series[0], series[Math.floor((series.length - 1) / 2)], series[series.length - 1]].filter(Boolean)
    : [];

  return (
    <SectionCard title={title} description={subtitle}>
      <div className="mb-4 flex items-end justify-between gap-4">
        <div>
          <div className="text-3xl font-semibold text-lazyops-text">{formatValue(latestValue)}</div>
          <div className="text-sm text-lazyops-muted">Latest bucket</div>
        </div>
        <div className="text-right">
          <div className="text-sm text-lazyops-muted">Peak</div>
          <div className="text-base font-medium text-lazyops-text">{formatValue(peakValue)}</div>
        </div>
      </div>

      <div className="rounded-2xl border border-lazyops-border/60 bg-[#020817]/70 p-3">
        {linePath ? (
          <svg viewBox={`0 0 ${chartWidth} ${chartHeight}`} className="h-48 w-full">
            <defs>
              <linearGradient id={gradientId} x1="0" y1="0" x2="0" y2="1">
                <stop offset="0%" stopColor={fill} />
                <stop offset="100%" stopColor="rgba(15, 23, 42, 0)" />
              </linearGradient>
            </defs>
            <path d={areaPath} fill={`url(#${gradientId})`} />
            <path d={linePath} fill="none" stroke={color} strokeWidth="3" strokeLinecap="round" />
          </svg>
        ) : (
          <div className="flex h-48 items-center justify-center text-sm text-lazyops-muted">
            Waiting for metrics...
          </div>
        )}

        <div className="mt-3 flex items-center justify-between text-xs text-lazyops-muted">
          {labels.map((point) => (
            <span key={`${title}-${point.timestamp}`}>
              {formatMetricTimestampLabel(point.timestamp, window)}
            </span>
          ))}
        </div>
      </div>
    </SectionCard>
  );
}

function MetricBar({ value, max, unit }: { value: number; max: number; unit: string }) {
  const pct = Math.min((value / max) * 100, 100);
  const color = pct > 80 ? 'bg-health-unhealthy' : pct > 60 ? 'bg-health-degraded' : 'bg-health-healthy';

  return (
    <div className="flex items-center gap-2">
      <div className="h-2 w-16 rounded-full bg-lazyops-border/30">
        <div className={`h-2 rounded-full ${color}`} style={{ width: `${pct}%` }} />
      </div>
      <span className="text-lazyops-text">{value}{unit}</span>
    </div>
  );
}

function formatBytes(value?: number) {
  if (!value || value <= 0) {
    return 'Chua co du lieu';
  }

  const units = ['B', 'KB', 'MB', 'GB', 'TB'];
  let size = value;
  let unitIndex = 0;
  while (size >= 1024 && unitIndex < units.length - 1) {
    size /= 1024;
    unitIndex += 1;
  }

  const precision = size >= 100 || unitIndex === 0 ? 0 : 1;
  return `${size.toFixed(precision)}${units[unitIndex]}`;
}

function formatCompactNumber(value: number) {
  return new Intl.NumberFormat(undefined, { notation: 'compact', maximumFractionDigits: 1 }).format(value);
}

function formatLatency(value: number) {
  if (value <= 0) {
    return '0ms';
  }
  return `${Math.round(value)}ms`;
}

function formatPercent(value: number) {
  return `${value.toFixed(value >= 10 ? 0 : 1)}%`;
}

function formatMegabytes(value: number) {
  if (value <= 0) {
    return '0MB';
  }
  return `${value.toFixed(value >= 100 ? 0 : 1)}MB`;
}

function LogsTab({ logs, logFilter, onFilterChange, serviceFilter, serviceOptions, onServiceFilterChange, followMode, onFollowToggle }: {
  logs?: { id: string; service: string; source?: string; revision_id?: string; level: LogLevel; message: string; timestamp: string }[];
  logFilter: LogLevel | 'all';
  onFilterChange: (f: LogLevel | 'all') => void;
  serviceFilter: string;
  serviceOptions: string[];
  onServiceFilterChange: (value: string) => void;
  followMode: boolean;
  onFollowToggle: () => void;
}) {
  return (
    <SectionCard
      title="Logs"
      description={`${logs?.length ?? 0} entries`}
      actions={
        <div className="flex items-center gap-2">
          <button
            type="button"
            className={`rounded-lg border px-2.5 py-1 text-sm transition-colors ${
              followMode ? 'border-primary/40 bg-primary/10 text-primary' : 'border-lazyops-border text-lazyops-muted'
            }`}
            onClick={onFollowToggle}
          >
            Follow
          </button>
        </div>
      }
    >
      <div className="mb-3 flex flex-wrap gap-2">
        {(['all', 'info', 'warn', 'error', 'debug'] as const).map((level) => (
          <button
            key={level}
            type="button"
            className={`rounded-md px-2.5 py-1 text-sm transition-colors ${
              logFilter === level
                ? 'bg-primary/15 text-primary'
                : 'text-lazyops-muted hover:bg-lazyops-border/20'
            }`}
            onClick={() => onFilterChange(level)}
          >
            {level}
          </button>
        ))}
        {serviceOptions.length > 0 && (
          <select
            value={serviceFilter}
            onChange={(event) => onServiceFilterChange(event.target.value)}
            className="rounded-md border border-lazyops-border bg-lazyops-bg-accent/50 px-2.5 py-1 text-sm text-lazyops-text outline-none focus:border-primary/60"
          >
            <option value="all">all services</option>
            {serviceOptions.map((service) => (
              <option key={service} value={service}>
                {service}
              </option>
            ))}
          </select>
        )}
      </div>

      <div className={`max-h-96 overflow-y-auto rounded-lg border border-lazyops-border bg-lazyops-bg font-mono text-sm ${followMode ? 'animate-pulse' : ''}`}>
        {logs?.length === 0 ? (
          <div className="p-6 text-lazyops-muted">No logs match the current filter.</div>
        ) : (
          <div className="flex flex-col">
            {logs?.map((log) => (
              <div key={log.id} className="flex gap-3 border-b border-lazyops-border/30 px-6 py-2 last:border-b-0">
                <span className="shrink-0 text-lazyops-muted/50">{new Date(log.timestamp).toLocaleTimeString()}</span>
                <span className={`shrink-0 font-medium ${LOG_LEVEL_COLORS[log.level]}`}>
                  {log.level.toUpperCase().padEnd(5)}
                </span>
                <span className="shrink-0 text-lazyops-muted/70">
                  [{log.service}{log.source ? ` / ${log.source}` : ''}{log.revision_id ? ` / ${log.revision_id}` : ''}]
                </span>
                <span className="text-lazyops-text">{log.message}</span>
              </div>
            ))}
          </div>
        )}
      </div>
    </SectionCard>
  );
}

function TracesTab({ traceQuery, onQueryChange, trace, isLoading }: {
  traceQuery: string;
  onQueryChange: (q: string) => void;
  trace?: { correlation_id: string; service_path: string[]; node_hops: string[]; latency_hotspot: string; total_latency_ms: number } | null;
  isLoading: boolean;
}) {
  return (
    <SectionCard title="Trace lookup" description="Look up a trace by correlation ID.">
      <div className="mb-4 flex gap-3">
        <input
          type="text"
          className="flex-1 rounded-lg border border-lazyops-border bg-lazyops-bg-accent/60 px-3 py-2 text-base text-lazyops-text outline-none placeholder:text-lazyops-muted/60 focus:border-primary/60 focus:ring-1 focus:ring-primary/30"
          placeholder="Enter correlation ID (e.g. corr_abc123)"
          value={traceQuery}
          onChange={(e) => onQueryChange(e.target.value)}
        />
      </div>

      {isLoading && <div className="py-8 text-center text-base text-lazyops-muted">Loading trace…</div>}

      {trace && (
        <div className="flex flex-col gap-4">
          <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
            <SummaryField label="Correlation ID" value={trace.correlation_id} />
            <SummaryField label="Total latency" value={`${trace.total_latency_ms}ms`} />
            <SummaryField label="Latency hotspot" value={trace.latency_hotspot} />
            <SummaryField label="Services" value={trace.service_path.join(' → ')} />
          </div>

          <div>
            <h4 className="mb-2 text-base font-medium text-lazyops-text">Service path</h4>
            <div className="flex flex-wrap items-center gap-2">
              {trace.service_path.map((svc, index) => (
                <span key={`${svc}-${index}`} className="flex items-center gap-2">
                  <span className={`rounded-md px-2.5 py-1 text-sm font-medium ${
                    svc === trace.latency_hotspot
                      ? 'bg-health-unhealthy/15 text-health-unhealthy'
                      : 'bg-lazyops-border/20 text-lazyops-text'
                  }`}>
                    {svc}
                  </span>
                  {index < trace.service_path.length - 1 && (
                    <span className="text-lazyops-muted">→</span>
                  )}
                </span>
              ))}
            </div>
          </div>

          <div>
            <h4 className="mb-2 text-base font-medium text-lazyops-text">Node hops</h4>
            <div className="flex flex-col gap-1">
              {trace.node_hops.map((hop) => (
                <span key={hop} className="text-sm font-mono text-lazyops-muted">{hop}</span>
              ))}
            </div>
          </div>
        </div>
      )}

      {!isLoading && traceQuery && !trace && (
        <EmptyState title="Trace not found" description={`No trace found for correlation ID: ${traceQuery}`} />
      )}
    </SectionCard>
  );
}

function IncidentsTab({ incidents }: { incidents?: { id: string; kind: string; severity: string; status: string; summary: string; created_at: string; resolved_at: string | null }[] }) {
  if (!incidents || incidents.length === 0) {
    return (
      <SectionCard title="Incidents" description="No incidents recorded.">
        <EmptyState title="No incidents" description="Your services are running without any recorded incidents." />
      </SectionCard>
    );
  }

  return (
    <SectionCard title="Incidents" description={`${incidents.length} incident${incidents.length > 1 ? 's' : ''} recorded.`}>
      <div className="flex flex-col gap-3">
        {incidents.map((inc) => (
          <div key={inc.id} className="rounded-lg border border-lazyops-border bg-lazyops-bg-accent/30 p-6">
            <div className="mb-2 flex items-center justify-between">
              <div className="flex items-center gap-2">
                <StatusBadge
                  label={inc.severity}
                  variant={INCIDENT_SEVERITY_VARIANT[inc.severity] ?? 'neutral'}
                  size="sm"
                />
                <StatusBadge
                  label={inc.status}
                  variant={INCIDENT_STATUS_VARIANT[inc.status] ?? 'neutral'}
                  size="sm"
                  dot={false}
                />
              </div>
              <span className="text-sm text-lazyops-muted">
                {new Date(inc.created_at).toLocaleString()}
              </span>
            </div>
            <p className="text-base text-lazyops-text">{inc.summary}</p>
            <div className="mt-1 text-sm text-lazyops-muted">
              Type: {inc.kind}
              {inc.resolved_at && ` · Resolved: ${new Date(inc.resolved_at).toLocaleString()}`}
            </div>
          </div>
        ))}
      </div>
    </SectionCard>
  );
}

function SummaryField({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex flex-col gap-0.5">
      <span className="text-sm text-lazyops-muted">{label}</span>
      <span className="truncate text-base text-lazyops-text" title={value}>{value}</span>
    </div>
  );
}
