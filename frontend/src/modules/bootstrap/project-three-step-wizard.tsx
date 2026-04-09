'use client';

import { useMemo, useState } from 'react';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { apiFetch } from '@/lib/api/api-client';
import { ErrorState } from '@/components/primitives/error-state';
import { LoadingBlock } from '@/components/primitives/loading';
import { SectionCard } from '@/components/primitives/section-card';
import { StatusBadge, type StatusBadgeProps } from '@/components/primitives/status-badge';
import { bootstrapStatusQueryKey, useAutoBootstrapProject, useOneClickDeploy, useProjectBootstrapStatus } from '@/modules/bootstrap/bootstrap-hooks';
import type { BootstrapOneClickDeployResult, BootstrapPipelineEvent, BootstrapStep, BootstrapStepAction } from '@/modules/bootstrap/bootstrap-types';
import { cn } from '@/lib/utils';
import { getProjectDeployment } from '@/modules/deployments/deployment-api';
import type { DeploymentDetail, DeploymentTimelineEvent } from '@/modules/deployments/deployment-types';

type ProjectThreeStepWizardProps = {
  projectId: string;
  compact?: boolean;
};

const STEP_ORDER = ['connect_code', 'connect_infra', 'deploy'] as const;

const STEP_TITLE: Record<string, string> = {
  connect_code: 'Connect Code',
  connect_infra: 'Connect Infra',
  deploy: 'Deploy',
};

const STEP_NUMBER: Record<string, string> = {
  connect_code: '1',
  connect_infra: '2',
  deploy: '3',
};

const STEP_BADGE: Record<string, StatusBadgeProps['variant']> = {
  healthy: 'success',
  ready: 'success',
  linked: 'info',
  deploying: 'warning',
  installing: 'warning',
  degraded: 'warning',
  blocked: 'neutral',
  missing: 'neutral',
  error: 'danger',
  rolled_back: 'danger',
};

const OVERALL_BADGE: Record<string, StatusBadgeProps['variant']> = {
  running: 'success',
  ready_to_deploy: 'info',
  deploying: 'warning',
  partially_ready: 'warning',
  not_ready: 'neutral',
  attention_required: 'danger',
};

const TIMELINE_BADGE: Record<string, StatusBadgeProps['variant']> = {
  completed: 'success',
  success: 'success',
  pending: 'warning',
  running: 'warning',
  deploying: 'warning',
  failed: 'danger',
  error: 'danger',
  rolled_back: 'danger',
  started: 'info',
  queued: 'neutral',
  promoted: 'success',
};

function formatStateLabel(value: string): string {
  return value.replace(/_/g, ' ').replace(/\b\w/g, (match) => match.toUpperCase());
}

function normalizeActionEndpoint(endpoint: string): string {
  if (endpoint.startsWith('/api/v1/')) {
    return endpoint.slice('/api/v1'.length);
  }
  if (endpoint === '/api/v1') {
    return '/';
  }
  return endpoint;
}

export function ProjectThreeStepWizard({ projectId, compact = false }: ProjectThreeStepWizardProps) {
  const queryClient = useQueryClient();
  const { data, isLoading, isError, error, refetch } = useProjectBootstrapStatus(projectId);
  const autoBootstrap = useAutoBootstrapProject(projectId);
  const oneClickDeploy = useOneClickDeploy(projectId);
  const [actionError, setActionError] = useState<string | null>(null);
  const [runningActionId, setRunningActionId] = useState<string | null>(null);
  const [latestOneClick, setLatestOneClick] = useState<BootstrapOneClickDeployResult | null>(null);
  const [activeDeploymentId, setActiveDeploymentId] = useState<string | null>(null);

  const deploymentDetail = useQuery({
    queryKey: ['one-click-deployment-detail', projectId, activeDeploymentId],
    queryFn: async (): Promise<DeploymentDetail> => {
      if (!activeDeploymentId) {
        throw new Error('Missing deployment id');
      }
      const result = await getProjectDeployment(projectId, activeDeploymentId);
      if (result.error) {
        throw new Error(result.error.message);
      }
      if (!result.data) {
        throw new Error('Deployment detail missing');
      }
      return result.data;
    },
    enabled: !!activeDeploymentId,
    refetchInterval: 5000,
    staleTime: 0,
  });

  const orderedSteps = useMemo(() => {
    if (!data?.steps) {
      return [];
    }
    return [...data.steps].sort((a, b) => STEP_ORDER.indexOf(a.id as (typeof STEP_ORDER)[number]) - STEP_ORDER.indexOf(b.id as (typeof STEP_ORDER)[number]));
  }, [data?.steps]);

  const stepById = useMemo(() => {
    const map = new Map<string, BootstrapStep>();
    orderedSteps.forEach((step) => map.set(step.id, step));
    return map;
  }, [orderedSteps]);

  if (isLoading) {
    return (
      <SectionCard title="3-step setup" description="Checking current project readiness.">
        <LoadingBlock label="Loading setup status..." className="py-8" />
      </SectionCard>
    );
  }

  if (isError || !data) {
    return (
      <ErrorState
        title="Failed to load setup status"
        message={error instanceof Error ? error.message : 'Could not fetch bootstrap status.'}
        action={(
          <button
            type="button"
            className="rounded-lg bg-primary px-4 py-2 text-sm font-semibold text-lazyops-bg transition-colors hover:bg-primary/90"
            onClick={() => {
              void refetch();
            }}
          >
            Retry
          </button>
        )}
      />
    );
  }

  const code = stepById.get('connect_code');
  const infra = stepById.get('connect_infra');
  const deploy = stepById.get('deploy');

  const statusCards = [
    { title: 'Code', value: code?.state ?? 'missing', summary: code?.summary ?? 'Not connected yet' },
    { title: 'Infra', value: infra?.state ?? 'missing', summary: infra?.summary ?? 'No server connected yet' },
    { title: 'Deploy', value: deploy?.state ?? 'blocked', summary: deploy?.summary ?? 'Deployment blocked' },
  ];

  const runAction = async (action: BootstrapStepAction) => {
    if (!action.endpoint) {
      return;
    }

    setRunningActionId(action.id);
    setActionError(null);
    try {
      const normalizedEndpoint = normalizeActionEndpoint(action.endpoint);
      if (normalizedEndpoint.endsWith('/deploy/one-click')) {
        const deployResult = await oneClickDeploy.mutateAsync({});
        setLatestOneClick(deployResult);
        setActiveDeploymentId(deployResult.deployment_id || null);
        await queryClient.invalidateQueries({ queryKey: bootstrapStatusQueryKey(projectId) });
        return;
      }

      const result = await apiFetch<unknown>(normalizedEndpoint, {
        method: (action.method || 'POST').toUpperCase(),
      });
      if (result.error) {
        throw new Error(result.error.message);
      }
      await queryClient.invalidateQueries({ queryKey: bootstrapStatusQueryKey(projectId) });
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Action failed';
      setActionError(message);
    } finally {
      setRunningActionId(null);
    }
  };

  const pipelineEvents = latestOneClick?.timeline ?? [];
  const runtimeEvents = deploymentDetail.data?.timeline ?? [];

  return (
    <div className="flex flex-col gap-4">
      <SectionCard
        title="3-step setup"
        description="Connect code, connect infrastructure, then deploy. LazyOps fills technical policies automatically."
        actions={(
          <button
            type="button"
            className="rounded-lg border border-lazyops-border px-3 py-1.5 text-xs font-semibold text-lazyops-text transition-colors hover:bg-lazyops-border/10 disabled:opacity-60"
            onClick={() => {
              void autoBootstrap.mutateAsync({});
            }}
            disabled={autoBootstrap.isPending}
          >
            {autoBootstrap.isPending ? 'Auto-fixing...' : 'Auto-fix setup'}
          </button>
        )}
      >
        <div className={cn('grid gap-3', compact ? 'grid-cols-1' : 'sm:grid-cols-3')}>
          {statusCards.map((item) => (
            <div key={item.title} className="rounded-lg border border-lazyops-border/60 bg-lazyops-bg-accent/30 p-3">
              <div className="mb-1 flex items-center justify-between">
                <span className="text-xs text-lazyops-muted">{item.title}</span>
                <StatusBadge
                  label={formatStateLabel(item.value)}
                  variant={STEP_BADGE[item.value] ?? 'neutral'}
                  size="sm"
                />
              </div>
              <p className="text-xs text-lazyops-muted/90">{item.summary}</p>
            </div>
          ))}
        </div>
        <div className="mt-3 flex flex-wrap items-center gap-2">
          <StatusBadge
            label={`Overall: ${formatStateLabel(data.overall_state)}`}
            variant={OVERALL_BADGE[data.overall_state] ?? 'neutral'}
            size="sm"
          />
          <StatusBadge
            label={`Mode: ${data.auto_mode.selected_mode}`}
            variant="info"
            size="sm"
            dot={false}
          />
          <span className="text-xs text-lazyops-muted">{data.auto_mode.mode_reason_human}</span>
        </div>
      </SectionCard>

      <div className="grid gap-4">
        {orderedSteps.map((step) => (
          <SectionCard
            key={step.id}
            title={`${STEP_NUMBER[step.id] ?? '-'} · ${STEP_TITLE[step.id] ?? step.id}`}
            description={step.summary}
          >
            <div className="flex flex-wrap items-center justify-between gap-3">
              <StatusBadge
                label={formatStateLabel(step.state)}
                variant={STEP_BADGE[step.state] ?? 'neutral'}
                size="sm"
              />
              <div className="flex flex-wrap items-center gap-2">
                {step.actions.map((action) => {
                  if ((action.kind === 'link' || action.kind === 'screen') && action.href) {
                    return (
                      <a
                        key={action.id}
                        href={action.href}
                        className="rounded-lg border border-lazyops-border px-3 py-1.5 text-xs font-semibold text-lazyops-text transition-colors hover:bg-lazyops-border/10"
                      >
                        {action.label}
                      </a>
                    );
                  }

                  if (action.kind === 'api' && action.endpoint) {
                    return (
                      <button
                        key={action.id}
                        type="button"
                        className="rounded-lg bg-primary px-3 py-1.5 text-xs font-semibold text-lazyops-bg transition-colors hover:bg-primary/90 disabled:opacity-60"
                        onClick={() => {
                          void runAction(action);
                        }}
                        disabled={runningActionId !== null}
                      >
                        {runningActionId === action.id ? 'Running...' : action.label}
                      </button>
                    );
                  }

                  return null;
                })}
              </div>
            </div>
          </SectionCard>
        ))}
      </div>

      {(latestOneClick || deploymentDetail.data) ? (
        <SectionCard
          title="Deployment timeline"
          description="Real-time progress of one-click deploy pipeline and rollout."
          actions={
            activeDeploymentId ? (
              <a
                href={`/projects/${projectId}/deployments/${activeDeploymentId}`}
                className="rounded-lg border border-lazyops-border px-3 py-1.5 text-xs font-semibold text-lazyops-text transition-colors hover:bg-lazyops-border/10"
              >
                Open deployment
              </a>
            ) : null
          }
        >
          {latestOneClick ? (
            <div className="mb-3 flex flex-wrap items-center gap-2">
              <StatusBadge
                label={`Rollout: ${formatStateLabel(latestOneClick.rollout_status)}`}
                variant={TIMELINE_BADGE[latestOneClick.rollout_status] ?? 'neutral'}
                size="sm"
              />
              {latestOneClick.rollout_reason ? (
                <span className="text-xs text-lazyops-muted">{latestOneClick.rollout_reason}</span>
              ) : null}
            </div>
          ) : null}

          <div className="flex flex-col gap-2">
            {pipelineEvents.map((event) => (
              <TimelineRow
                key={`pipeline-${event.id}-${event.timestamp}`}
                label={event.label}
                description={event.message}
                state={event.state}
                timestamp={event.timestamp}
              />
            ))}
            {runtimeEvents.map((event, index) => (
              <TimelineRow
                key={`runtime-${index}-${event.timestamp}-${event.state}`}
                label={event.label}
                description={event.description}
                state={event.state}
                timestamp={event.timestamp}
              />
            ))}
            {deploymentDetail.isFetching ? (
              <p className="text-[11px] text-lazyops-muted">Refreshing timeline...</p>
            ) : null}
          </div>
        </SectionCard>
      ) : null}

      {actionError ? (
        <div className="rounded-lg border border-health-unhealthy/30 bg-health-unhealthy/10 px-3 py-2 text-xs text-health-unhealthy">
          {actionError}
        </div>
      ) : null}
    </div>
  );
}

function TimelineRow({
  label,
  description,
  state,
  timestamp,
}: {
  label: string;
  description: string;
  state: string;
  timestamp: string;
}) {
  return (
    <div className="rounded-lg border border-lazyops-border/60 bg-lazyops-bg-accent/20 px-3 py-2">
      <div className="mb-1 flex flex-wrap items-center justify-between gap-2">
        <span className="text-xs font-medium text-lazyops-text">{label}</span>
        <StatusBadge
          label={formatStateLabel(state)}
          variant={TIMELINE_BADGE[state] ?? 'neutral'}
          size="sm"
        />
      </div>
      <p className="text-xs text-lazyops-muted">{description}</p>
      <p className="mt-1 text-[11px] text-lazyops-muted/80">
        {new Date(timestamp).toLocaleString()}
      </p>
    </div>
  );
}
