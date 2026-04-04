'use client';

import { useState } from 'react';
import { zodResolver } from '@hookform/resolvers/zod';
import { useForm } from 'react-hook-form';
import { useGitHubInstallations, useSyncGitHubInstallations } from '@/modules/github-sync/github-hooks';
import { syncGitHubInstallationsSchema, type SyncGitHubInstallationsFormData } from '@/modules/github-sync/github-types';
import { PageHeader } from '@/components/primitives/page-header';
import { SectionCard } from '@/components/primitives/section-card';
import { EmptyState } from '@/components/primitives/empty-state';
import { ErrorState } from '@/components/primitives/error-state';
import { SkeletonPage } from '@/components/primitives/skeleton';
import { StatusBadge } from '@/components/primitives/status-badge';
import { Modal } from '@/components/primitives/modal';
import { FormField, FormInput, FormButton } from '@/components/forms/form-fields';

const GITHUB_EXPLAINER = {
  title: 'How GitHub App sync works',
  description:
    'Syncing fetches all GitHub App installations from your linked GitHub account. This lets LazyOps see which repositories are available for deployment.',
  steps: [
    { title: 'Link your GitHub account', desc: 'Sign in with GitHub OAuth to link your account.' },
    { title: 'Sync installations', desc: 'Fetch all GitHub App installations and their repository scopes.' },
    { title: 'Link repos to projects', desc: 'Connect repositories to your LazyOps projects for automated deployments.' },
  ],
};

export default function GitHubIntegrationsPage() {
  const { data: reposData, isLoading: reposLoading, isError: reposError } = useGitHubInstallations();
  const [showSyncModal, setShowSyncModal] = useState(false);

  if (reposLoading) {
    return <SkeletonPage title cards={2} />;
  }

  if (reposError) {
    return (
      <div className="flex flex-col gap-6">
        <PageHeader title="GitHub App" subtitle="Manage your GitHub App installations and repositories." />
        <ErrorState
          title="Failed to load GitHub data"
          message="Could not fetch GitHub repository data. Make sure your GitHub account is linked."
          action={
            <button
              type="button"
              className="rounded-lg bg-primary px-4 py-2 text-sm font-semibold text-lazyops-bg transition-colors hover:bg-primary/90"
              onClick={() => setShowSyncModal(true)}
            >
              Sync installations
            </button>
          }
        />
      </div>
    );
  }

  const repos = reposData?.items ?? [];

  const installations = repos.reduce<Record<string, typeof repos>>((acc, repo) => {
    const key = `${repo.installation_account_login}/${repo.installation_account_type}`;
    if (!acc[key]) acc[key] = [];
    acc[key].push(repo);
    return acc;
  }, {});

  return (
    <div className="flex flex-col gap-6">
      <PageHeader
        title="GitHub App"
        subtitle="Manage GitHub App installations and linked repositories."
        actions={
          <button
            type="button"
            className="rounded-lg bg-primary px-4 py-2 text-sm font-semibold text-lazyops-bg transition-colors hover:bg-primary/90"
            onClick={() => setShowSyncModal(true)}
          >
            Sync installations
          </button>
        }
      />

      <SectionCard
        title={GITHUB_EXPLAINER.title}
        description={GITHUB_EXPLAINER.description}
      >
        <div className="flex flex-col gap-3">
          {GITHUB_EXPLAINER.steps.map((step, i) => (
            <div key={step.title} className="flex items-start gap-3">
              <div className="flex size-6 shrink-0 items-center justify-center rounded-full bg-primary/15 text-xs font-bold text-primary">
                {i + 1}
              </div>
              <div>
                <span className="text-sm font-medium text-lazyops-text">{step.title}</span>
                <p className="text-xs text-lazyops-muted">{step.desc}</p>
              </div>
            </div>
          ))}
        </div>
      </SectionCard>

      {repos.length === 0 ? (
        <SectionCard title="No installations" description="Sync your GitHub App to see available installations and repositories.">
          <EmptyState
            title="No GitHub installations found"
            description="Make sure you have the LazyOps GitHub App installed, then sync to fetch your installations."
            action={
              <button
                type="button"
                className="rounded-lg bg-primary px-4 py-2 text-sm font-semibold text-lazyops-bg transition-colors hover:bg-primary/90"
                onClick={() => setShowSyncModal(true)}
              >
                Sync installations
              </button>
            }
          />
        </SectionCard>
      ) : (
        <div className="flex flex-col gap-4">
          {Object.entries(installations).map(([key, repos]) => {
            const [accountLogin, accountType] = key.split('/');
            const hasRevoked = false;
            return (
              <SectionCard
                key={key}
                title={accountLogin}
                description={`${accountType} · ${repos.length} repositor${repos.length > 1 ? 'ies' : 'y'}`}
              >
                <div className="flex items-center gap-2 mb-3">
                  <StatusBadge label={accountType === 'Organization' ? 'Org' : 'User'} variant="info" size="sm" dot={false} />
                  {hasRevoked && <StatusBadge label="Revoked" variant="danger" size="sm" />}
                </div>

                <div className="overflow-x-auto">
                  <table className="w-full text-sm">
                    <thead>
                      <tr className="border-b border-lazyops-border">
                        <th className="px-4 py-2 text-left text-xs font-medium text-lazyops-muted">Repository</th>
                        <th className="px-4 py-2 text-left text-xs font-medium text-lazyops-muted">Visibility</th>
                        <th className="px-4 py-2 text-left text-xs font-medium text-lazyops-muted">Permissions</th>
                      </tr>
                    </thead>
                    <tbody>
                      {repos.map((repo) => (
                        <tr key={repo.github_repo_id} className="border-b border-lazyops-border/50 transition-colors hover:bg-lazyops-border/10">
                          <td className="px-4 py-2">
                            <span className="font-medium text-lazyops-text">{repo.full_name}</span>
                          </td>
                          <td className="px-4 py-2">
                            <StatusBadge
                              label={repo.private ? 'Private' : 'Public'}
                              variant={repo.private ? 'warning' : 'neutral'}
                              size="sm"
                              dot={false}
                            />
                          </td>
                          <td className="px-4 py-2">
                            <div className="flex flex-wrap gap-1">
                              {Object.entries(repo.permissions).map(([perm, level]) => (
                                <span
                                  key={perm}
                                  className="rounded bg-lazyops-border/20 px-1.5 py-0.5 text-[10px] text-lazyops-muted"
                                >
                                  {perm}: {level}
                                </span>
                              ))}
                            </div>
                          </td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              </SectionCard>
            );
          })}
        </div>
      )}

      <SyncInstallationsModal
        open={showSyncModal}
        onClose={() => setShowSyncModal(false)}
      />
    </div>
  );
}

type SyncInstallationsModalProps = {
  open: boolean;
  onClose: () => void;
};

function SyncInstallationsModal({ open, onClose }: SyncInstallationsModalProps) {
  const {
    register,
    handleSubmit,
    formState: { errors, isSubmitting },
  } = useForm<SyncGitHubInstallationsFormData>({
    resolver: zodResolver(syncGitHubInstallationsSchema),
    defaultValues: { github_access_token: '' },
  });

  const syncInstallations = useSyncGitHubInstallations();
  const serverError = syncInstallations.error?.message ?? null;

  const onSubmit = (data: SyncGitHubInstallationsFormData) => {
    return syncInstallations.mutateAsync(data).then(() => {
      onClose();
    });
  };

  return (
    <Modal open={open} onClose={onClose} title="Sync GitHub installations" size="md">
      <form onSubmit={handleSubmit(onSubmit)} className="flex flex-col gap-4" noValidate>
        <p className="text-sm text-lazyops-muted">
          Enter your GitHub personal access token to sync installations. The token needs <code className="rounded bg-lazyops-border/20 px-1 text-xs">read:org</code> scope.
        </p>

        <FormField label="GitHub access token" error={errors.github_access_token?.message}>
          <FormInput
            type="password"
            placeholder="ghp_xxxxxxxxxxxx"
            error={!!errors.github_access_token}
            {...register('github_access_token')}
          />
        </FormField>

        {serverError && (
          <div className="rounded-lg border border-health-unhealthy/30 bg-health-unhealthy/10 px-3 py-2 text-xs text-health-unhealthy">
            {serverError}
          </div>
        )}

        <FormButton type="submit" loading={isSubmitting || syncInstallations.isPending}>
          Sync installations
        </FormButton>
      </form>
    </Modal>
  );
}
