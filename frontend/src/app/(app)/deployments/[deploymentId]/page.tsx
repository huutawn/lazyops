'use client';

import { useParams } from 'next/navigation';
import { useDeployment, useDeploymentAction } from '@/modules/deployments/deployment-hooks';
import { PageHeader } from '@/components/primitives/page-header';
import { SectionCard } from '@/components/primitives/section-card';
import { ErrorState } from '@/components/primitives/error-state';
import { SkeletonPage } from '@/components/primitives/skeleton';
import { StatusBadge } from '@/components/primitives/status-badge';
import { HealthChip } from '@/components/primitives/health-chip';
import type { BuildState, RolloutState } from '@/modules/deployments/deployment-types';

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

function formatState(state: string): string {
  return state.replace(/_/g, ' ').replace(/\b\w/g, (c) => c.toUpperCase());
}

export default function DeploymentDetailPage() {
  const params = useParams();
  const projectId = params?.projectId as string | undefined;
  const deploymentId = params?.deploymentId as string;
  const { data, isLoading, isError } = useDeployment(projectId, deploymentId);
  const deploymentAction = useDeploymentAction(projectId, deploymentId);

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
