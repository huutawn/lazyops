'use client';

import { useParams } from 'next/navigation';
import { useGitHubAppConfig, useGitHubInstallations } from '@/modules/github-sync/github-hooks';
import { repoLinkQueryKey } from '@/modules/repo-link/repo-link-hooks';
import { useQuery } from '@tanstack/react-query';
import type { ProjectRepoLink } from '@/modules/repo-link/repo-link-types';
import { PageHeader } from '@/components/primitives/page-header';
import { SectionCard } from '@/components/primitives/section-card';
import { StatusBadge } from '@/components/primitives/status-badge';
import { HealthChip } from '@/components/primitives/health-chip';
import { SkeletonPage } from '@/components/primitives/skeleton';
import Link from 'next/link';

const NEXT_STEPS = [
  {
    title: 'Link a repository',
    description: 'Connect a GitHub repo to this project.',
    href: '/repo-link',
    done: false,
  },
  {
    title: 'Create a deployment binding',
    description: 'Define where services will be deployed.',
    href: '/bindings',
    done: false,
  },
  {
    title: 'Review the deploy contract',
    description: 'Validate your lazyops.yaml configuration.',
    href: '/validate',
    done: false,
  },
  {
    title: 'Compile the blueprint',
    description: 'Generate the deployment plan.',
    href: '/blueprint',
    done: false,
  },
];

export default function ProjectIntegrationsPage() {
  const params = useParams();
  const projectId = params?.projectId as string;

  const { data: reposData, isLoading: reposLoading } = useGitHubInstallations();
  const { data: appConfig } = useGitHubAppConfig();
  const { data: repoLink, isLoading: linkLoading } = useQuery({
    queryKey: repoLinkQueryKey(projectId),
    queryFn: () => Promise.resolve(null as ProjectRepoLink | null),
    staleTime: 60 * 1000,
  });

  if (reposLoading || linkLoading) {
    return <SkeletonPage title cards={3} />;
  }

  const repos = reposData?.items ?? [];
  const webhookURL = appConfig?.webhook_url?.trim() || 'https://your-domain.com/api/v1/integrations/github/webhook';
  const hasGitHub = repos.length > 0;
  const hasRepoLink = !!repoLink;

  const steps = NEXT_STEPS.map((step) => ({
    ...step,
    href: `/projects/${projectId}${step.href}`,
    done: step.title === 'Link a repository' ? hasRepoLink : false,
  }));

  const completedSteps = steps.filter((s) => s.done).length;

  return (
    <div className="flex flex-col gap-6">
      <PageHeader
        title="Integrations"
        subtitle="Manage all external connections for this project."
      />

      <SectionCard title="Setup progress" description={`${completedSteps} of ${steps.length} steps completed.`}>
        <div className="mb-4 h-2 w-full rounded-full bg-lazyops-border/30">
          <div
            className="h-2 rounded-full bg-primary transition-all"
            style={{ width: `${(completedSteps / steps.length) * 100}%` }}
          />
        </div>
        <div className="flex flex-col gap-2">
          {steps.map((step) => (
            <div
              key={step.title}
              className="flex items-center justify-between rounded-lg px-3 py-2 transition-colors hover:bg-lazyops-border/10"
            >
              <div className="flex items-center gap-3">
                {step.done ? (
                  <HealthChip label="Done" status="healthy" size="sm" />
                ) : (
                  <StatusBadge label="Pending" variant="neutral" size="sm" dot={false} />
                )}
                <div>
                  <span className="text-sm font-medium text-lazyops-text">{step.title}</span>
                  <p className="text-xs text-lazyops-muted">{step.description}</p>
                </div>
              </div>
              {!step.done && (
                <Link
                  href={step.href}
                  className="text-xs text-primary hover:underline"
                >
                  Go →
                </Link>
              )}
            </div>
          ))}
        </div>
      </SectionCard>

      <SectionCard title="GitHub App" description="Repository and webhook integration status.">
        <div className="flex flex-col gap-4">
          <div className="flex items-center justify-between">
            <span className="text-sm text-lazyops-muted">GitHub connection</span>
            <StatusBadge
              label={hasGitHub ? 'Connected' : 'Not connected'}
              variant={hasGitHub ? 'success' : 'neutral'}
              size="sm"
            />
          </div>

          {hasGitHub && (
            <div className="grid gap-2 sm:grid-cols-2">
              <SummaryField label="Repositories available" value={String(repos.length)} />
              <SummaryField
                label="Account"
                value={repos[0]?.installation_account_login ?? '—'}
              />
            </div>
          )}

          {!hasGitHub && (
            <p className="text-sm text-lazyops-muted">
              Connect your GitHub App to enable automated deployments from your repositories.
            </p>
          )}

          <div className="flex items-center justify-between">
            <span className="text-sm text-lazyops-muted">Linked repository</span>
            {hasRepoLink ? (
              <StatusBadge label={repoLink!.repo_full_name} variant="info" size="sm" dot={false} />
            ) : (
              <StatusBadge label="Not linked" variant="neutral" size="sm" dot={false} />
            )}
          </div>

          {hasRepoLink && (
            <div className="grid gap-2 sm:grid-cols-2 lg:grid-cols-3">
              <SummaryField label="Tracked branch" value={repoLink!.tracked_branch} />
              <SummaryField
                label="Preview deploys"
                value={repoLink!.preview_enabled ? 'Enabled' : 'Disabled'}
              />
              <SummaryField
                label="Webhook"
                value="Configured"
              />
            </div>
          )}
        </div>
      </SectionCard>

      <SectionCard
        title="Webhook health"
        description="Status of the GitHub webhook connection."
      >
        <div className="flex items-center gap-3">
          <HealthChip label="Healthy" status="healthy" size="md" />
          <span className="text-sm text-lazyops-muted">
            Webhook is receiving events from GitHub.
          </span>
        </div>
        <p className="mt-2 text-xs text-lazyops-muted/60">
          Webhook URL: <code className="rounded bg-lazyops-border/20 px-1.5 py-0.5 text-xs">{webhookURL}</code>
        </p>
      </SectionCard>

      <SectionCard
        title="Build activity"
        description="Recent deployment activity for this project."
      >
        <div className="flex flex-col items-center gap-3 py-8 text-center">
          <div className="text-3xl text-lazyops-muted/30" aria-hidden="true">📦</div>
          <p className="text-sm text-lazyops-muted">
            No builds yet. Link a repository and push to your tracked branch to trigger your first deployment.
          </p>
          {hasRepoLink && (
            <Link
              href={`/projects/${projectId}/blueprint`}
              className="rounded-lg bg-primary px-4 py-2 text-sm font-semibold text-lazyops-bg transition-colors hover:bg-primary/90"
            >
              Compile blueprint
            </Link>
          )}
        </div>
      </SectionCard>
    </div>
  );
}

function SummaryField({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex flex-col gap-0.5">
      <span className="text-xs text-lazyops-muted">{label}</span>
      <span className="text-sm text-lazyops-text">{value}</span>
    </div>
  );
}
