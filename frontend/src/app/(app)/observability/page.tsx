'use client';

import { useState, useMemo } from 'react';
import { useLogs, useIncidents, useMetrics, useTrace } from '@/modules/observability/observability-hooks';
import type { LogLevel } from '@/modules/observability/observability-types';
import { PageHeader } from '@/components/primitives/page-header';
import { SectionCard } from '@/components/primitives/section-card';
import { EmptyState } from '@/components/primitives/empty-state';
import { ErrorState } from '@/components/primitives/error-state';
import { SkeletonPage } from '@/components/primitives/skeleton';
import { StatusBadge } from '@/components/primitives/status-badge';
import { HealthChip } from '@/components/primitives/health-chip';

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

export default function ObservabilityPage() {
  const [activeTab, setActiveTab] = useState<'overview' | 'logs' | 'traces' | 'incidents'>('overview');
  const [logFilter, setLogFilter] = useState<LogLevel | 'all'>('all');
  const [followMode, setFollowMode] = useState(false);
  const [traceQuery, setTraceQuery] = useState('');

  const { data: logs, isLoading: logsLoading } = useLogs();
  const { data: incidents, isLoading: incidentsLoading } = useIncidents();
  const { data: metrics, isLoading: metricsLoading } = useMetrics();
  const { data: trace, isLoading: traceLoading } = useTrace(traceQuery);

  const filteredLogs = useMemo(
    () => logFilter === 'all' ? logs : logs?.filter((l) => l.level === logFilter),
    [logs, logFilter],
  );

  if (logsLoading || incidentsLoading || metricsLoading) {
    return <SkeletonPage title cards={3} />;
  }

  const openIncidents = incidents?.filter((i) => i.status === 'open' || i.status === 'investigating') ?? [];
  const errorLogs = logs?.filter((l) => l.level === 'error') ?? [];

  return (
    <div className="flex flex-col gap-6">
      <PageHeader
        title="Observability"
        subtitle="Logs, traces, incidents, and metrics for your services."
      />

      <div className="flex gap-2 border-b border-lazyops-border">
        {(['overview', 'logs', 'traces', 'incidents'] as const).map((tab) => (
          <button
            key={tab}
            type="button"
            className={`rounded-t-lg px-4 py-2 text-sm font-medium transition-colors ${
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

function OverviewTab({ openIncidents, errorLogs, metrics }: {
  openIncidents: { id: string; severity: string; status: string; summary: string }[];
  errorLogs: { id: string; message: string; timestamp: string }[];
  metrics?: { service: string; cpu_p95: number; ram_p95: number; request_count: number }[];
}) {
  return (
    <div className="flex flex-col gap-4">
      <div className="grid gap-4 sm:grid-cols-3">
        <SectionCard title="Open incidents">
          <div className="text-3xl font-bold text-health-unhealthy">{openIncidents.length}</div>
          <p className="text-xs text-lazyops-muted">
            {openIncidents.length > 0 ? 'Requires attention' : 'All clear'}
          </p>
        </SectionCard>

        <SectionCard title="Recent errors">
          <div className="text-3xl font-bold text-health-unhealthy">{errorLogs.length}</div>
          <p className="text-xs text-lazyops-muted">Error log entries</p>
        </SectionCard>

        <SectionCard title="Services monitored">
          <div className="text-3xl font-bold text-health-healthy">{metrics?.length ?? 0}</div>
          <p className="text-xs text-lazyops-muted">With metric data</p>
        </SectionCard>
      </div>

      {openIncidents.length > 0 && (
        <SectionCard title="Active incidents" description="Incidents requiring attention.">
          <div className="flex flex-col gap-2">
            {openIncidents.map((inc) => (
              <div key={inc.id} className="flex items-center justify-between rounded-lg bg-lazyops-bg-accent/50 px-4 py-3">
                <div className="flex items-center gap-3">
                  <StatusBadge
                    label={inc.severity}
                    variant={INCIDENT_SEVERITY_VARIANT[inc.severity] ?? 'neutral'}
                    size="sm"
                  />
                  <span className="text-sm text-lazyops-text">{inc.summary}</span>
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

      {metrics && metrics.length > 0 && (
        <SectionCard title="Service metrics" description="24-hour resource usage summary.">
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead>
                <tr className="border-b border-lazyops-border">
                  <th className="px-4 py-2 text-left text-xs font-medium text-lazyops-muted">Service</th>
                  <th className="px-4 py-2 text-left text-xs font-medium text-lazyops-muted">CPU p95</th>
                  <th className="px-4 py-2 text-left text-xs font-medium text-lazyops-muted">RAM p95</th>
                  <th className="px-4 py-2 text-left text-xs font-medium text-lazyops-muted">Requests</th>
                </tr>
              </thead>
              <tbody>
                {metrics.map((m) => (
                  <tr key={m.service} className="border-b border-lazyops-border/50">
                    <td className="px-4 py-2 font-medium text-lazyops-text">{m.service}</td>
                    <td className="px-4 py-2 font-mono text-xs">
                      <MetricBar value={m.cpu_p95} max={100} unit="%" label="CPU" />
                    </td>
                    <td className="px-4 py-2 font-mono text-xs">
                      <MetricBar value={m.ram_p95} max={2048} unit="MB" label="RAM" />
                    </td>
                    <td className="px-4 py-2 font-mono text-xs text-lazyops-muted">
                      {m.request_count.toLocaleString()}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </SectionCard>
      )}
    </div>
  );
}

function MetricBar({ value, max, unit, label }: { value: number; max: number; unit: string; label: string }) {
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

function LogsTab({ logs, logFilter, onFilterChange, followMode, onFollowToggle }: {
  logs?: { id: string; service: string; level: LogLevel; message: string; timestamp: string }[];
  logFilter: LogLevel | 'all';
  onFilterChange: (f: LogLevel | 'all') => void;
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
            className={`rounded-lg border px-2.5 py-1 text-xs transition-colors ${
              followMode ? 'border-primary/40 bg-primary/10 text-primary' : 'border-lazyops-border text-lazyops-muted'
            }`}
            onClick={onFollowToggle}
          >
            Follow
          </button>
        </div>
      }
    >
      <div className="mb-3 flex gap-2">
        {(['all', 'info', 'warn', 'error', 'debug'] as const).map((level) => (
          <button
            key={level}
            type="button"
            className={`rounded-md px-2.5 py-1 text-xs transition-colors ${
              logFilter === level
                ? 'bg-primary/15 text-primary'
                : 'text-lazyops-muted hover:bg-lazyops-border/20'
            }`}
            onClick={() => onFilterChange(level)}
          >
            {level}
          </button>
        ))}
      </div>

      <div className={`max-h-96 overflow-y-auto rounded-lg border border-lazyops-border bg-lazyops-bg font-mono text-xs ${followMode ? 'animate-pulse' : ''}`}>
        {logs?.length === 0 ? (
          <div className="p-4 text-lazyops-muted">No logs match the current filter.</div>
        ) : (
          <div className="flex flex-col">
            {logs?.map((log) => (
              <div key={log.id} className="flex gap-3 border-b border-lazyops-border/30 px-4 py-2 last:border-b-0">
                <span className="shrink-0 text-lazyops-muted/50">{new Date(log.timestamp).toLocaleTimeString()}</span>
                <span className={`shrink-0 font-medium ${LOG_LEVEL_COLORS[log.level]}`}>
                  {log.level.toUpperCase().padEnd(5)}
                </span>
                <span className="shrink-0 text-lazyops-muted/70">[{log.service}]</span>
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
      <div className="flex gap-3 mb-4">
        <input
          type="text"
          className="flex-1 rounded-lg border border-lazyops-border bg-lazyops-bg-accent/60 px-3 py-2 text-sm text-lazyops-text outline-none placeholder:text-lazyops-muted/60 focus:border-primary/60 focus:ring-1 focus:ring-primary/30"
          placeholder="Enter correlation ID (e.g. corr_abc123)"
          value={traceQuery}
          onChange={(e) => onQueryChange(e.target.value)}
        />
      </div>

      {isLoading && <div className="py-8 text-center text-sm text-lazyops-muted">Loading trace…</div>}

      {trace && (
        <div className="flex flex-col gap-4">
          <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
            <SummaryField label="Correlation ID" value={trace.correlation_id} />
            <SummaryField label="Total latency" value={`${trace.total_latency_ms}ms`} />
            <SummaryField label="Latency hotspot" value={trace.latency_hotspot} />
            <SummaryField label="Services" value={trace.service_path.join(' → ')} />
          </div>

          <div>
            <h4 className="mb-2 text-sm font-medium text-lazyops-text">Service path</h4>
            <div className="flex flex-wrap items-center gap-2">
              {trace.service_path.map((svc, i) => (
                <span key={svc} className="flex items-center gap-2">
                  <span className={`rounded-md px-2.5 py-1 text-xs font-medium ${
                    svc === trace.latency_hotspot
                      ? 'bg-health-unhealthy/15 text-health-unhealthy'
                      : 'bg-lazyops-border/20 text-lazyops-text'
                  }`}>
                    {svc}
                  </span>
                  {i < trace.service_path.length - 1 && (
                    <span className="text-lazyops-muted">→</span>
                  )}
                </span>
              ))}
            </div>
          </div>

          <div>
            <h4 className="mb-2 text-sm font-medium text-lazyops-text">Node hops</h4>
            <div className="flex flex-col gap-1">
              {trace.node_hops.map((hop) => (
                <span key={hop} className="text-xs font-mono text-lazyops-muted">{hop}</span>
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
          <div key={inc.id} className="rounded-lg border border-lazyops-border bg-lazyops-bg-accent/30 p-4">
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
              <span className="text-xs text-lazyops-muted">
                {new Date(inc.created_at).toLocaleString()}
              </span>
            </div>
            <p className="text-sm text-lazyops-text">{inc.summary}</p>
            <div className="mt-1 text-xs text-lazyops-muted">
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
      <span className="text-xs text-lazyops-muted">{label}</span>
      <span className="truncate text-sm text-lazyops-text" title={value}>{value}</span>
    </div>
  );
}
