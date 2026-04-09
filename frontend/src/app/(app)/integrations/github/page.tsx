'use client';

import { useEffect, useRef, useState } from 'react';
import { useSearchParams } from 'next/navigation';
import { zodResolver } from '@hookform/resolvers/zod';
import { useForm } from 'react-hook-form';
import { useGitHubAppConfig, useGitHubInstallations, useSyncGitHubInstallations } from '@/modules/github-sync/github-hooks';
import { syncGitHubInstallationsSchema, type SyncGitHubInstallationsFormData } from '@/modules/github-sync/github-types';
import { PageHeader } from '@/components/primitives/page-header';
import { SectionCard } from '@/components/primitives/section-card';
import { EmptyState } from '@/components/primitives/empty-state';
import { ErrorState } from '@/components/primitives/error-state';
import { SkeletonPage } from '@/components/primitives/skeleton';
import { StatusBadge } from '@/components/primitives/status-badge';
import { Modal } from '@/components/primitives/modal';
import { FormField, FormInput, FormButton } from '@/components/forms/form-fields';

export default function GitHubIntegrationsPage() {
  const searchParams = useSearchParams();
  const { data: reposData, isLoading: reposLoading, isError: reposError } = useGitHubInstallations();
  const { data: appConfig } = useGitHubAppConfig();
  const quickSync = useSyncGitHubInstallations();
  const [showSyncModal, setShowSyncModal] = useState(false);
  const autoSyncTriggered = useRef(false);
  const appInstallURL = appConfig?.install_url ?? '';
  const appName = appConfig?.name?.trim() || 'LazyOps';
  const fromInstallFlow = !!searchParams.get('installation_id') || !!searchParams.get('setup_action');

  useEffect(() => {
    if (!fromInstallFlow || autoSyncTriggered.current) {
      return;
    }
    autoSyncTriggered.current = true;
    quickSync.mutate({ github_access_token: '' });
  }, [fromInstallFlow, quickSync]);

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
            <div className="flex items-center gap-2">
              <button
                type="button"
                className="rounded-lg bg-primary px-4 py-2 text-sm font-semibold text-lazyops-bg transition-colors hover:bg-primary/90 disabled:opacity-60"
                onClick={() => quickSync.mutate({ github_access_token: '' })}
                disabled={quickSync.isPending}
              >
                {quickSync.isPending ? 'Refreshing...' : 'Refresh installations'}
              </button>
              <button
                type="button"
                className="rounded-lg border border-lazyops-border px-4 py-2 text-sm font-semibold text-lazyops-text transition-colors hover:bg-lazyops-border/10"
                onClick={() => setShowSyncModal(true)}
              >
                Advanced sync
              </button>
            </div>
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
          <div className="flex items-center gap-2">
            <a
              href="/api/auth/oauth/github/start?next=/integrations/github"
              className="rounded-lg border border-lazyops-border px-4 py-2 text-sm font-semibold text-lazyops-text transition-colors hover:bg-lazyops-border/10"
            >
              Link GitHub
            </a>
            {appInstallURL && (
              <a
                href={appInstallURL}
                target="_blank"
                rel="noreferrer"
                className="rounded-lg border border-lazyops-border px-4 py-2 text-sm font-semibold text-lazyops-text transition-colors hover:bg-lazyops-border/10"
              >
                Install GitHub App
              </a>
            )}
            <button
              type="button"
              className="rounded-lg bg-primary px-4 py-2 text-sm font-semibold text-lazyops-bg transition-colors hover:bg-primary/90 disabled:opacity-60"
              onClick={() => quickSync.mutate({ github_access_token: '' })}
              disabled={quickSync.isPending}
            >
              {quickSync.isPending ? 'Refreshing...' : 'Refresh'}
            </button>
          </div>
        }
      />

      <SectionCard title="Faster flow" description="Install app -> return here -> Refresh once.">
        <p className="text-sm text-lazyops-muted">
          You do not need to enter a token every time. Use <span className="font-medium text-lazyops-text">Advanced sync</span> only when PAT-based force sync is needed.
        </p>
        {quickSync.error && (
          <div className="mt-3 rounded-lg border border-health-unhealthy/30 bg-health-unhealthy/10 px-3 py-2 text-xs text-health-unhealthy">
            {(quickSync.error as Error).message}
          </div>
        )}
      </SectionCard>

      {repos.length === 0 ? (
        <SectionCard title="No installations" description="Sync your GitHub App to see available installations and repositories.">
          <EmptyState
            title="No GitHub installations found"
            description={`If refresh stays empty, click Link GitHub once, install ${appName}, then refresh.`}
            action={
              <div className="flex items-center gap-2">
                <a
                  href="/api/auth/oauth/github/start?next=/integrations/github"
                  className="rounded-lg border border-lazyops-border px-4 py-2 text-sm font-semibold text-lazyops-text transition-colors hover:bg-lazyops-border/10"
                >
                  Link GitHub
                </a>
                {appInstallURL && (
                  <a
                    href={appInstallURL}
                    target="_blank"
                    rel="noreferrer"
                    className="rounded-lg border border-lazyops-border px-4 py-2 text-sm font-semibold text-lazyops-text transition-colors hover:bg-lazyops-border/10"
                  >
                    Install GitHub App
                  </a>
                )}
                <button
                  type="button"
                  className="rounded-lg bg-primary px-4 py-2 text-sm font-semibold text-lazyops-bg transition-colors hover:bg-primary/90 disabled:opacity-60"
                  onClick={() => quickSync.mutate({ github_access_token: '' })}
                  disabled={quickSync.isPending}
                >
                  {quickSync.isPending ? 'Refreshing...' : 'Refresh installations'}
                </button>
                <button
                  type="button"
                  className="rounded-lg border border-lazyops-border px-4 py-2 text-sm font-semibold text-lazyops-text transition-colors hover:bg-lazyops-border/10"
                  onClick={() => setShowSyncModal(true)}
                >
                  Advanced sync
                </button>
              </div>
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
          Provide a GitHub PAT only when you want to force sync from GitHub API. You can leave it empty to reload current cache.
        </p>

        <FormField label="GitHub access token" error={errors.github_access_token?.message}>
          <FormInput
            type="password"
            placeholder="(optional) ghp_xxxxxxxxxxxx"
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
