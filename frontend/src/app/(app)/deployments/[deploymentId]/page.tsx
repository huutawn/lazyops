'use client';

import Link from 'next/link';
import { useParams } from 'next/navigation';
import { useQuery } from '@tanstack/react-query';
import { useDeployment, useDeploymentAction } from '@/modules/deployments/deployment-hooks';
import { PageHeader } from '@/components/primitives/page-header';
import { SectionCard } from '@/components/primitives/section-card';
import { ErrorState } from '@/components/primitives/error-state';
import { SkeletonPage } from '@/components/primitives/skeleton';
import { StatusBadge } from '@/components/primitives/status-badge';
import { HealthChip } from '@/components/primitives/health-chip';
import type { BuildState, RolloutState } from '@/modules/deployments/deployment-types';
import type { LogEntry } from '@/modules/observability/observability-types';
import { listProjectLogs } from '@/modules/observability/observability-api';
import { useTrace } from '@/modules/observability/observability-hooks';

const BUILD_STATE_VARIANT: Record<BuildState, 'success' | 'warning' | 'danger' | 'info' | 'neutral'> = {
  draft: 'neutral',
  queued: 'info',
  building: 'info',
  artifact_ready: 'success',
  planned: 'info',
  applying: 'warning',
  promoted: 'success',
  failed: 'danger',
  rolled_back: 'danger',
  superseded: 'neutral',
};

const ROLLOUT_STATE_VARIANT: Record<RolloutState, 'success' | 'warning' | 'danger' | 'info' | 'neutral'> = {
  queued: 'info',
  running: 'warning',
  candidate_ready: 'info',
  promoted: 'success',
  failed: 'danger',
  rolled_back: 'danger',
  canceled: 'neutral',
};

const INCIDENT_STATE_VARIANT: Record<string, 'success' | 'warning' | 'danger' | 'info' | 'neutral'> = {
  healthy: 'success',
  monitoring: 'info',
  attention: 'danger',
  warning: 'warning',
  info: 'info',
};

const LOG_LEVEL_COLORS: Record<string, string> = {
  info: 'text-lazyops-muted',
  warn: 'text-health-degraded',
  error: 'text-health-unhealthy',
  debug: 'text-lazyops-muted/70',
};

function formatState(state: string): string {
  return state.replace(/_/g, ' ').replace(/\b\w/g, (c) => c.toUpperCase());
}

export default function DeploymentDetailPage() {
  const params = useParams();
  const projectId = params?.projectId as string | undefined;
  const deploymentId = params?.deploymentId as string;
  const { data, isLoading, isError } = useDeployment(projectId, deploymentId);
  const deploymentAction = useDeploymentAction(projectId, deploymentId);
  const revisionID = data?.revision_id;
  const deploymentLogs = useQuery({
    queryKey: ['deployment-runtime-logs', projectId, deploymentId, revisionID],
    queryFn: async (): Promise<LogEntry[]> => {
      if (!projectId || !revisionID) {
        return [];
      }
      const result = await listProjectLogs(projectId, { limit: 200 });
      if (result.error) {
        throw new Error(result.error.message);
      }
      const lines = result.data?.items ?? [];
      return lines.filter((line) => line.revision_id === revisionID).slice(0, 8);
    },
    enabled: !!projectId && !!revisionID,
    staleTime: 15 * 1000,
    refetchInterval: 10_000,
  });
  const traceCorrelationID = deploymentLogs.data?.find((line) => line.correlation_id)?.correlation_id ?? '';
  const trace = useTrace(traceCorrelationID);

  if (isLoading) {
    return <SkeletonPage title cards={3} />;
  }

  if (isError || !data) {
    return (
      <div className="flex flex-col gap-6">
        <PageHeader title="Deployment Detail" subtitle={`Revision details for ${deploymentId}`} />
        <ErrorState title="Deployment not found" message="Could not find the requested deployment." />
      </div>
    );
  }

  const dep = data;
  const isTerminal = ['promoted', 'failed', 'rolled_back', 'canceled'].includes(dep.rollout_state);
  const incident = dep.incident_summary;

  return (
    <div className="flex flex-col gap-6">
      <PageHeader
        title={`Revision ${dep.revision}`}
        subtitle={`${dep.commit_sha.slice(0, 7)} · ${dep.runtime_mode}`}
      />

      <SectionCard title="Status" description="Current build and rollout state.">
        <div className="flex flex-wrap items-center gap-3">
          <StatusBadge
            label={`Build: ${formatState(dep.build_state)}`}
            variant={BUILD_STATE_VARIANT[dep.build_state]}
            size="md"
          />
          <StatusBadge
            label={`Rollout: ${formatState(dep.rollout_state)}`}
            variant={ROLLOUT_STATE_VARIANT[dep.rollout_state]}
            size="md"
          />
          {dep.promoted && <HealthChip label="Promoted" status="healthy" size="md" />}
        </div>

        <div className="mt-4 grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
          <SummaryField label="Trigger" value={dep.trigger_kind} />
          <SummaryField label="Triggered by" value={dep.triggered_by} />
          <SummaryField label="Started" value={dep.started_at ? new Date(dep.started_at).toLocaleString() : '—'} />
          <SummaryField label="Completed" value={dep.completed_at ? new Date(dep.completed_at).toLocaleString() : '—'} />
        </div>
      </SectionCard>

      <SectionCard title="Safety" description="Default rollout safety and automatic rollback policy.">
        <div className="flex flex-wrap items-center gap-3">
          <StatusBadge
            label={dep.safety_policy.auto_rollback_enabled ? 'Auto rollback: enabled' : 'Auto rollback: disabled'}
            variant={dep.safety_policy.auto_rollback_enabled ? 'success' : 'warning'}
            size="md"
          />
          {dep.safety_policy.triggers.map((trigger) => (
            <StatusBadge key={trigger} label={formatState(trigger)} variant="neutral" size="sm" dot={false} />
          ))}
        </div>
        <p className="mt-3 text-sm text-lazyops-muted">{dep.safety_policy.description}</p>

        {incident ? (
          <div className="mt-4 rounded-lg border border-lazyops-border bg-lazyops-bg-accent/40 p-4">
            <div className="flex flex-wrap items-center gap-2">
              <StatusBadge
                label={formatState(incident.state)}
                variant={INCIDENT_STATE_VARIANT[incident.state] ?? 'neutral'}
                size="sm"
              />
              {incident.incident_level ? (
                <StatusBadge label={formatState(incident.incident_level)} variant="neutral" size="sm" dot={false} />
              ) : null}
              {incident.incident_kind ? (
                <StatusBadge label={formatState(incident.incident_kind)} variant="neutral" size="sm" dot={false} />
              ) : null}
            </div>
            <h4 className="mt-3 text-sm font-semibold text-lazyops-text">{incident.headline}</h4>
            <p className="mt-1 text-sm text-lazyops-muted">{incident.reason}</p>
            <p className="mt-2 text-sm text-lazyops-text">{incident.recommended}</p>
            {incident.primary_action ? (
              <div className="mt-3">
                <Link
                  href={incident.primary_action.href}
                  className="inline-flex rounded-lg bg-primary px-3 py-2 text-xs font-semibold text-lazyops-bg transition-colors hover:bg-primary/90"
                >
                  {incident.primary_action.label}
                </Link>
              </div>
            ) : null}
          </div>
        ) : null}
      </SectionCard>

      <SectionCard title="Artifact" description="Build artifact information.">
        <div className="grid gap-3 sm:grid-cols-3">
          <SummaryField label="Commit" value={dep.commit_sha} />
          <SummaryField label="Artifact" value={dep.artifact_ref ?? '—'} />
          <SummaryField label="Image" value={dep.image_ref ?? '—'} />
        </div>
      </SectionCard>

      <SectionCard title="Services" description={`${dep.services.length} service${dep.services.length > 1 ? 's' : ''} in this deployment.`}>
        <div className="flex flex-col gap-2">
          {dep.services.map((svc) => (
            <div key={svc.name} className="flex items-center justify-between rounded-lg bg-lazyops-bg-accent/50 px-4 py-3">
              <div>
                <span className="text-sm font-medium text-lazyops-text">{svc.name}</span>
                <span className="ml-2 text-xs text-lazyops-muted font-mono">{svc.path}</span>
              </div>
              <div className="flex items-center gap-2">
                {svc.public && <StatusBadge label="Public" variant="info" size="sm" dot={false} />}
                <StatusBadge label={svc.runtime_profile} variant="neutral" size="sm" dot={false} />
              </div>
            </div>
          ))}
        </div>
      </SectionCard>

      <SectionCard title="Placement" description="Service-to-target assignments.">
        <div className="overflow-x-auto">
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b border-lazyops-border">
                <th className="px-4 py-2 text-left text-xs font-medium text-lazyops-muted">Service</th>
                <th className="px-4 py-2 text-left text-xs font-medium text-lazyops-muted">Target</th>
                <th className="px-4 py-2 text-left text-xs font-medium text-lazyops-muted">Kind</th>
                <th className="px-4 py-2 text-left text-xs font-medium text-lazyops-muted">Labels</th>
              </tr>
            </thead>
            <tbody>
              {dep.placement_assignments.map((pa) => (
                <tr key={pa.service_name} className="border-b border-lazyops-border/50">
                  <td className="px-4 py-2 font-medium text-lazyops-text">{pa.service_name}</td>
                  <td className="px-4 py-2 font-mono text-xs text-lazyops-muted">{pa.target_id}</td>
                  <td className="px-4 py-2 text-xs text-lazyops-muted">{pa.target_kind}</td>
                  <td className="px-4 py-2">
                    <div className="flex flex-wrap gap-1">
                      {Object.entries(pa.labels).map(([k, v]) => (
                        <span key={k} className="rounded bg-lazyops-border/20 px-1.5 py-0.5 text-[10px] text-lazyops-muted">
                          {k}: {v}
                        </span>
                      ))}
                      {Object.keys(pa.labels).length === 0 && <span className="text-xs text-lazyops-muted/50">—</span>}
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </SectionCard>

      <SectionCard title="Timeline" description="Deployment event history.">
        <div className="relative pl-6">
          <div className="absolute left-2 top-0 bottom-0 w-px bg-lazyops-border" />
          <div className="flex flex-col gap-6">
            {dep.timeline.map((event, i) => (
              <div key={i} className="relative">
                <div className={`absolute -left-4 top-1 size-3 rounded-full border-2 ${
                  event.state === 'failed' || event.state === 'rolled_back'
                    ? 'border-health-unhealthy bg-health-unhealthy/30'
                    : event.state === 'promoted'
                    ? 'border-health-healthy bg-health-healthy/30'
                    : 'border-lazyops-border bg-lazyops-muted/30'
                }`} />
                <div className="flex flex-col gap-0.5">
                  <span className="text-sm font-medium text-lazyops-text">{event.label}</span>
                  <span className="text-xs text-lazyops-muted">{event.description}</span>
                  <span className="text-[10px] text-lazyops-muted/60">
                    {new Date(event.timestamp).toLocaleString()}
                  </span>
                </div>
              </div>
            ))}
          </div>
        </div>
      </SectionCard>

      <SectionCard title="Runtime Signals" description="Deployment status, recent logs, and trace hotspot in one view.">
        <div className="grid gap-4 lg:grid-cols-2">
          <div className="rounded-lg border border-lazyops-border bg-lazyops-bg-accent/30 p-3">
            <h4 className="text-sm font-semibold text-lazyops-text">Recent logs (this revision)</h4>
            {deploymentLogs.isLoading ? (
              <p className="mt-2 text-xs text-lazyops-muted">Loading logs...</p>
            ) : deploymentLogs.data && deploymentLogs.data.length > 0 ? (
              <div className="mt-3 flex flex-col gap-2">
                {deploymentLogs.data.map((line) => (
                  <div key={line.id} className="rounded-md border border-lazyops-border/60 bg-lazyops-bg/30 px-3 py-2">
                    <div className="flex items-center justify-between gap-2">
                      <span className={`text-[11px] font-medium ${LOG_LEVEL_COLORS[line.level] ?? 'text-lazyops-muted'}`}>
                        {line.level.toUpperCase()}
                      </span>
                      <span className="text-[10px] text-lazyops-muted/70">{new Date(line.timestamp).toLocaleString()}</span>
                    </div>
                    <p className="mt-1 text-xs text-lazyops-text">{line.message}</p>
                  </div>
                ))}
              </div>
            ) : (
              <p className="mt-2 text-xs text-lazyops-muted">No logs captured for this revision yet.</p>
            )}
          </div>

          <div className="rounded-lg border border-lazyops-border bg-lazyops-bg-accent/30 p-3">
            <h4 className="text-sm font-semibold text-lazyops-text">Trace hotspot</h4>
            {traceCorrelationID ? (
              <div className="mt-2 text-xs text-lazyops-muted">
                Correlation ID: <code>{traceCorrelationID}</code>
              </div>
            ) : (
              <p className="mt-2 text-xs text-lazyops-muted">No trace correlation found in current logs.</p>
            )}

            {traceCorrelationID ? (
              trace.isLoading ? (
                <p className="mt-3 text-xs text-lazyops-muted">Loading trace...</p>
              ) : trace.data ? (
                <div className="mt-3 space-y-2 text-xs">
                  <p className="text-lazyops-text">
                    Hotspot: <span className="font-semibold">{trace.data.latency_hotspot}</span>
                  </p>
                  <p className="text-lazyops-muted">Total latency: {trace.data.total_latency_ms} ms</p>
                  <p className="text-lazyops-muted">
                    Path: {trace.data.service_path.length > 0 ? trace.data.service_path.join(' -> ') : 'n/a'}
                  </p>
                  <Link
                    href="/observability"
                    className="inline-flex rounded-lg border border-lazyops-border px-3 py-1.5 text-xs font-medium text-lazyops-text transition-colors hover:bg-lazyops-bg-accent/60"
                  >
                    Open observability
                  </Link>
                </div>
              ) : (
                <p className="mt-3 text-xs text-lazyops-muted">Trace data is not available for this correlation yet.</p>
              )
            ) : null}
          </div>
        </div>
      </SectionCard>

      {!isTerminal && (
        <SectionCard title="Actions" description="Available actions for this deployment.">
          <div className="flex flex-wrap gap-3">
            {dep.can_cancel && (
              <button
                type="button"
                className="rounded-lg border border-health-degraded/30 bg-health-degraded/10 px-4 py-2 text-sm font-medium text-health-degraded transition-colors hover:bg-health-degraded/20"
                disabled={deploymentAction.isPending}
                onClick={() => deploymentAction.mutate('cancel')}
              >
                {deploymentAction.isPending ? 'Cancelling...' : 'Cancel deployment'}
              </button>
            )}
            {dep.can_promote && (
              <button
                type="button"
                className="rounded-lg bg-primary px-4 py-2 text-sm font-semibold text-lazyops-bg transition-colors hover:bg-primary/90"
                disabled={deploymentAction.isPending}
                onClick={() => deploymentAction.mutate('promote')}
              >
                {deploymentAction.isPending ? 'Promoting...' : 'Promote to production'}
              </button>
            )}
          </div>
        </SectionCard>
      )}

      {dep.can_rollback && (
        <SectionCard title="Actions" description="Available actions for this deployment.">
          <button
            type="button"
            className="rounded-lg border border-health-unhealthy/30 bg-health-unhealthy/10 px-4 py-2 text-sm font-medium text-health-unhealthy transition-colors hover:bg-health-unhealthy/20"
            disabled={deploymentAction.isPending}
            onClick={() => deploymentAction.mutate('rollback')}
          >
            {deploymentAction.isPending ? 'Rolling back...' : 'Rollback to previous revision'}
          </button>
        </SectionCard>
      )}
    </div>
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
