'use client';

import { useParams } from 'next/navigation';
import { useDeployments } from '@/modules/deployments/deployment-hooks';
import type { BuildState, RolloutState } from '@/modules/deployments/deployment-types';
import { PageHeader } from '@/components/primitives/page-header';
import { SectionCard } from '@/components/primitives/section-card';
import { EmptyState } from '@/components/primitives/empty-state';
import { ErrorState } from '@/components/primitives/error-state';
import { SkeletonPage } from '@/components/primitives/skeleton';
import { StatusBadge } from '@/components/primitives/status-badge';
import Link from 'next/link';

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

export default function DeploymentsPage() {
  const params = useParams();
  const projectId = params?.projectId as string | undefined;
  const { data, isLoading, isError } = useDeployments(projectId);

  if (isLoading) {
    return <SkeletonPage title cards={1} />;
  }

  if (isError) {
    return (
      <div className="flex flex-col gap-6">
        <PageHeader title="Deployments" subtitle="Deployment history and status." />
        <ErrorState title="Failed to load deployments" message="Could not fetch deployment data." />
      </div>
    );
  }

  const deployments = data?.items ?? [];

  return (
    <div className="flex flex-col gap-6">
      <PageHeader
        title="Deployments"
        subtitle={projectId ? 'Deployment history for this project.' : 'Deployment history across all projects.'}
      />

      {deployments.length === 0 ? (
        <SectionCard title="No deployments" description="No deployments have been created yet.">
          <EmptyState
            title="No deployments yet"
            description="Create a deployment binding and compile a blueprint to trigger your first deployment."
          />
        </SectionCard>
      ) : (
        <SectionCard>
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead>
                <tr className="border-b border-lazyops-border">
                  <th className="px-4 py-3 text-left font-medium text-lazyops-muted">Revision</th>
                  <th className="px-4 py-3 text-left font-medium text-lazyops-muted">Commit</th>
                  <th className="px-4 py-3 text-left font-medium text-lazyops-muted">Build</th>
                  <th className="px-4 py-3 text-left font-medium text-lazyops-muted">Rollout</th>
                  <th className="px-4 py-3 text-left font-medium text-lazyops-muted">Trigger</th>
                  <th className="px-4 py-3 text-left font-medium text-lazyops-muted">By</th>
                  <th className="px-4 py-3 text-left font-medium text-lazyops-muted">Completed</th>
                </tr>
              </thead>
              <tbody>
                {deployments.map((dep) => (
                  <tr
                    key={dep.id}
                    className="border-b border-lazyops-border/50 transition-colors hover:bg-lazyops-border/10"
                  >
                    <td className="px-4 py-3">
                      <Link
                        href={`/projects/${dep.project_id}/deployments/${dep.id}`}
                        className="font-medium text-primary hover:underline"
                      >
                        r{dep.revision}
                      </Link>
                    </td>
                    <td className="px-4 py-3 font-mono text-xs text-lazyops-muted">
                      {dep.commit_sha.slice(0, 7)}
                    </td>
                    <td className="px-4 py-3">
                      <StatusBadge
                        label={formatState(dep.build_state)}
                        variant={BUILD_STATE_VARIANT[dep.build_state]}
                        size="sm"
                      />
                    </td>
                    <td className="px-4 py-3">
                      <StatusBadge
                        label={formatState(dep.rollout_state)}
                        variant={ROLLOUT_STATE_VARIANT[dep.rollout_state]}
                        size="sm"
                      />
                    </td>
                    <td className="px-4 py-3 text-xs text-lazyops-muted">
                      <StatusBadge label={dep.trigger_kind} variant="neutral" size="sm" dot={false} />
                    </td>
                    <td className="px-4 py-3 text-xs text-lazyops-muted">{dep.triggered_by}</td>
                    <td className="px-4 py-3 text-xs text-lazyops-muted">
                      {dep.completed_at ? new Date(dep.completed_at).toLocaleString() : '—'}
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
