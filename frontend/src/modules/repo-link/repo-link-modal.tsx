'use client';

import { useMemo, useState } from 'react';
import { zodResolver } from '@hookform/resolvers/zod';
import { useForm } from 'react-hook-form';
import { Modal } from '@/components/primitives/modal';
import { StatusBadge } from '@/components/primitives/status-badge';
import { FormButton, FormField, FormInput } from '@/components/forms/form-fields';
import { useLinkProjectRepo } from '@/modules/repo-link/repo-link-hooks';
import { linkRepoSchema, type GitHubRepoOption, type LinkRepoFormData, type ProjectRepoLink } from '@/modules/repo-link/repo-link-types';

type ProjectRepoLinkModalProps = {
  projectId: string;
  repos: GitHubRepoOption[];
  open: boolean;
  onClose: () => void;
  onSuccess?: (repo: ProjectRepoLink) => void;
};

export function ProjectRepoLinkModal({
  projectId,
  repos,
  open,
  onClose,
  onSuccess,
}: ProjectRepoLinkModalProps) {
  const [selectedRepoId, setSelectedRepoId] = useState<number | null>(null);
  const {
    register,
    handleSubmit,
    setValue,
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
    () => repos.find((repo) => repo.github_repo_id === selectedRepoId),
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
        onSuccess?.(result.data);
        onClose();
      }
    });
  };

  return (
    <Modal open={open} onClose={onClose} title="Kết nối mã nguồn" size="lg">
      <form onSubmit={handleSubmit(onSubmit)} className="flex flex-col gap-5" noValidate>
        <div>
          <label className="mb-2 block text-base font-medium text-lazyops-text">Chọn repository</label>
          <div className="max-h-64 overflow-y-auto rounded-lg border border-lazyops-border">
            {repos.map((repo) => {
              const isSelected = selectedRepoId === repo.github_repo_id;
              return (
                <button
                  key={repo.github_repo_id}
                  type="button"
                  className={`flex w-full items-center justify-between border-b border-lazyops-border/50 px-6 py-3 text-left transition-colors last:border-b-0 ${
                    isSelected ? 'bg-primary/10' : 'hover:bg-lazyops-border/10'
                  }`}
                  onClick={() => handleRepoSelect(repo)}
                >
                  <div>
                    <span className="text-base font-medium text-lazyops-text">{repo.full_name}</span>
                    <span className="ml-2 text-sm text-lazyops-muted">({repo.installation_account_login})</span>
                  </div>
                  <StatusBadge
                    label={repo.private ? 'Riêng tư' : 'Công khai'}
                    variant={repo.private ? 'warning' : 'neutral'}
                    size="sm"
                    dot={false}
                  />
                </button>
              );
            })}
          </div>
          {errors.github_repo_id ? (
            <p className="mt-1 text-sm text-health-unhealthy">{errors.github_repo_id.message}</p>
          ) : null}
        </div>

        {selectedRepo ? (
          <div className="rounded-lg border border-lazyops-border/50 bg-lazyops-bg-accent/30 p-6">
            <h4 className="mb-3 text-base font-medium text-lazyops-text">Thiết lập theo dõi</h4>
            <div className="flex flex-col gap-4">
              <FormField label="Nhánh theo dõi" error={errors.tracked_branch?.message}>
                <FormInput
                  type="text"
                  placeholder="main"
                  error={!!errors.tracked_branch}
                  {...register('tracked_branch')}
                />
                <p className="mt-1 text-[10px] text-lazyops-muted/60">
                  Mỗi lần push vào nhánh này sẽ kích hoạt build/deploy.
                </p>
              </FormField>

              <label className="flex items-center gap-2 text-base text-lazyops-text">
                <input type="checkbox" className="accent-primary" {...register('preview_enabled')} />
                Bật preview deploy cho pull request
              </label>
            </div>
          </div>
        ) : null}

        {serverError ? (
          <div className="rounded-lg border border-health-unhealthy/30 bg-health-unhealthy/10 px-3 py-2 text-sm text-health-unhealthy">
            {serverError}
          </div>
        ) : null}

        <FormButton type="submit" loading={isSubmitting || linkRepo.isPending}>
          Kết nối repository
        </FormButton>
      </form>
    </Modal>
  );
}
