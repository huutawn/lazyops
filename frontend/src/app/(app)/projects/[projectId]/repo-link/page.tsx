'use client';

import { useState, useMemo, useEffect } from 'react';
import { useParams, useRouter } from 'next/navigation';
import { zodResolver } from '@hookform/resolvers/zod';
import { useForm } from 'react-hook-form';
import { useLinkProjectRepo, repoLinkQueryKey } from '@/modules/repo-link/repo-link-hooks';
import { linkRepoSchema, type LinkRepoFormData, type ProjectRepoLink, type GitHubRepoOption } from '@/modules/repo-link/repo-link-types';
import { useGitHubInstallations } from '@/modules/github-sync/github-hooks';
import { PageHeader } from '@/components/primitives/page-header';
import { SectionCard } from '@/components/primitives/section-card';
import { EmptyState } from '@/components/primitives/empty-state';
import { ErrorState } from '@/components/primitives/error-state';
import { SkeletonPage } from '@/components/primitives/skeleton';
import { StatusBadge } from '@/components/primitives/status-badge';
import { Modal } from '@/components/primitives/modal';
import { FormField, FormInput, FormButton } from '@/components/forms/form-fields';
import { isFeatureEnabled } from '@/lib/flags/feature-flags';

export default function RepoLinkPage() {
  const params = useParams();
  const router = useRouter();
  const projectId = params?.projectId as string;
  const threeStepFlowEnabled = isFeatureEnabled('ux_three_step_flow');

  useEffect(() => {
    if (threeStepFlowEnabled && projectId) {
      router.replace(`/projects/${projectId}`);
    }
  }, [threeStepFlowEnabled, projectId, router]);

  const { data: reposData, isLoading: reposLoading, isError: reposError } = useGitHubInstallations();
  const [showLinkModal, setShowLinkModal] = useState(false);
  const [linkedRepo, setLinkedRepo] = useState<ProjectRepoLink | null>(null);

  if (threeStepFlowEnabled) {
    return <SkeletonPage title cards={2} />;
  }

  if (reposLoading) {
    return <SkeletonPage title cards={2} />;
  }

  if (reposError) {
    return (
      <div className="flex flex-col gap-6">
        <PageHeader title="Repository Link" subtitle="Connect a GitHub repository to this project." />
        <ErrorState
          title="Failed to load GitHub repositories"
          message="Could not fetch your GitHub repositories. Make sure you have synced your GitHub App installations."
        />
      </div>
    );
  }

  const repos = reposData?.items ?? [];

  if (repos.length === 0) {
    return (
      <div className="flex flex-col gap-6">
        <PageHeader title="Repository Link" subtitle="Connect a GitHub repository to this project." />
        <SectionCard title="No repositories available" description="You need to sync your GitHub installations first.">
          <EmptyState
            title="No GitHub repositories found"
            description="Sync your GitHub App installations to see available repositories."
            action={
              <a
                href="/integrations/github"
                className="inline-block rounded-lg bg-primary px-4 py-2 text-sm font-semibold text-lazyops-bg transition-colors hover:bg-primary/90"
              >
                Go to GitHub App
              </a>
            }
          />
        </SectionCard>
      </div>
    );
  }

  return (
    <div className="flex flex-col gap-6">
      <PageHeader
        title="Repository Link"
        subtitle="Connect this project to a GitHub repository for automated deployments."
        actions={
          <button
            type="button"
            className="rounded-lg bg-primary px-4 py-2 text-sm font-semibold text-lazyops-bg transition-colors hover:bg-primary/90"
            onClick={() => setShowLinkModal(true)}
          >
            Link repository
          </button>
        }
      />

      {linkedRepo ? (
        <LinkedRepoCard repo={linkedRepo} />
      ) : (
        <SectionCard title="No repository linked" description="Link a GitHub repository to enable automated deployments.">
          <EmptyState
            title="No repository linked yet"
            description="Select a GitHub repository to link to this project. Once linked, pushes to the tracked branch will trigger deployments."
            action={
              <button
                type="button"
                className="rounded-lg bg-primary px-4 py-2 text-sm font-semibold text-lazyops-bg transition-colors hover:bg-primary/90"
                onClick={() => setShowLinkModal(true)}
              >
                Link repository
              </button>
            }
          />
        </SectionCard>
      )}

      <LinkRepoModal
        projectId={projectId}
        repos={repos}
        open={showLinkModal}
        onClose={() => setShowLinkModal(false)}
        onSuccess={(repo) => {
          setLinkedRepo(repo);
          setShowLinkModal(false);
        }}
      />
    </div>
  );
}

function LinkedRepoCard({ repo }: { repo: ProjectRepoLink }) {
  return (
    <SectionCard title="Linked repository" description={repo.repo_full_name}>
      <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
        <SummaryField label="Repository" value={repo.repo_full_name} />
        <SummaryField label="Tracked branch" value={repo.tracked_branch} />
        <SummaryField label="Preview deploys" value={repo.preview_enabled ? 'Enabled' : 'Disabled'} />
        <SummaryField label="Installation" value={String(repo.github_installation_id)} />
        <SummaryField label="Linked at" value={new Date(repo.created_at).toLocaleString()} />
      </div>
      <div className="mt-3 flex items-center gap-2">
        <StatusBadge label="Linked" variant="success" size="md" />
        <span className="text-sm text-lazyops-muted">
          Pushes to <code className="rounded bg-lazyops-border/20 px-1.5 py-0.5 text-xs">{repo.tracked_branch}</code> will trigger deployments.
        </span>
      </div>
    </SectionCard>
  );
}

type LinkRepoModalProps = {
  projectId: string;
  repos: GitHubRepoOption[];
  open: boolean;
  onClose: () => void;
  onSuccess: (repo: ProjectRepoLink) => void;
};

function LinkRepoModal({ projectId, repos, open, onClose, onSuccess }: LinkRepoModalProps) {
  const [selectedRepoId, setSelectedRepoId] = useState<number | null>(null);

  const {
    register,
    handleSubmit,
    setValue,
    watch,
    formState: { errors, isSubmitting },
  } = useForm<LinkRepoFormData>({
    resolver: zodResolver(linkRepoSchema),
    defaultValues: {
      github_installation_id: 0,
      github_repo_id: 0,
      tracked_branch: 'main',
      preview_enabled: false,
    },
  });

  const linkRepo = useLinkProjectRepo(projectId);
  const serverError = linkRepo.error?.message ?? null;

  const selectedRepo = useMemo(
    () => repos.find((r) => r.github_repo_id === selectedRepoId),
    [repos, selectedRepoId],
  );

  const handleRepoSelect = (repo: GitHubRepoOption) => {
    setSelectedRepoId(repo.github_repo_id);
    setValue('github_installation_id', repo.github_installation_id);
    setValue('github_repo_id', repo.github_repo_id);
  };

  const onSubmit = (data: LinkRepoFormData) => {
    return linkRepo.mutateAsync(data).then((result) => {
      if (result.data) {
        onSuccess(result.data);
      }
    });
  };

  return (
    <Modal open={open} onClose={onClose} title="Link repository" size="lg">
      <form onSubmit={handleSubmit(onSubmit)} className="flex flex-col gap-5" noValidate>
        <div>
          <label className="mb-2 block text-sm font-medium text-lazyops-text">Select repository</label>
          <div className="max-h-64 overflow-y-auto rounded-lg border border-lazyops-border">
            {repos.map((repo) => {
              const isSelected = selectedRepoId === repo.github_repo_id;
              return (
                <button
                  key={repo.github_repo_id}
                  type="button"
                  className={`flex w-full items-center justify-between border-b border-lazyops-border/50 px-4 py-3 text-left last:border-b-0 transition-colors ${
                    isSelected
                      ? 'bg-primary/10'
                      : 'hover:bg-lazyops-border/10'
                  }`}
                  onClick={() => handleRepoSelect(repo)}
                >
                  <div>
                    <span className="text-sm font-medium text-lazyops-text">{repo.full_name}</span>
                    <span className="ml-2 text-xs text-lazyops-muted">
                      ({repo.installation_account_login})
                    </span>
                  </div>
                  <div className="flex items-center gap-2">
                    <StatusBadge
                      label={repo.private ? 'Private' : 'Public'}
                      variant={repo.private ? 'warning' : 'neutral'}
                      size="sm"
                      dot={false}
                    />
                  </div>
                </button>
              );
            })}
          </div>
          {errors.github_repo_id && (
            <p className="mt-1 text-xs text-health-unhealthy">{errors.github_repo_id.message}</p>
          )}
        </div>

        {selectedRepo && (
          <div className="rounded-lg border border-lazyops-border/50 bg-lazyops-bg-accent/30 p-4">
            <h4 className="mb-3 text-sm font-medium text-lazyops-text">Configuration</h4>
            <div className="flex flex-col gap-4">
              <FormField label="Tracked branch" error={errors.tracked_branch?.message}>
                <FormInput
                  type="text"
                  placeholder="main"
                  error={!!errors.tracked_branch}
                  {...register('tracked_branch')}
                />
                <p className="mt-1 text-[10px] text-lazyops-muted/60">
                  Pushes to this branch will trigger deployments.
                </p>
              </FormField>

              <label className="flex items-center gap-2 text-sm text-lazyops-text">
                <input
                  type="checkbox"
                  className="accent-primary"
                  {...register('preview_enabled')}
                />
                Enable preview deployments for pull requests
              </label>
            </div>
          </div>
        )}

        {serverError && (
          <div className="rounded-lg border border-health-unhealthy/30 bg-health-unhealthy/10 px-3 py-2 text-xs text-health-unhealthy">
            {serverError}
          </div>
        )}

        <FormButton type="submit" loading={isSubmitting || linkRepo.isPending}>
          Link repository
        </FormButton>
      </form>
    </Modal>
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
