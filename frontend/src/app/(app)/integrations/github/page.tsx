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
      <div className="flex flex-col gap-6 max-w-5xl mx-auto py-4">
        <PageHeader title="Tích hợp GitHub" subtitle="Quản lý mã nguồn và ứng dụng GitHub của bạn." />
        <ErrorState
          title="Không thể tải dữ liệu"
          message="Vui lòng kiểm tra lại kết nối và chắc chắn tài khoản của bạn đã liên kết với GitHub."
          action={
            <div className="flex items-center gap-3 mt-4">
              <button
                type="button"
                className="rounded-xl bg-primary px-6 py-3 text-base font-bold text-primary-foreground shadow-lg transition-all hover:bg-primary/90 disabled:opacity-60"
                onClick={() => quickSync.mutate({ github_access_token: '' })}
                disabled={quickSync.isPending}
              >
                {quickSync.isPending ? 'Đang làm mới...' : 'Thử tải lại ngay'}
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
    <div className="flex flex-col gap-8 max-w-5xl mx-auto py-4">
      <div className="flex flex-col md:flex-row justify-between items-start md:items-center gap-4">
        <div>
          <h1 className="text-3xl font-bold tracking-tight mb-2">Tích hợp GitHub</h1>
          <p className="text-muted-foreground text-lg">Cài đặt ứng dụng GitHub để tự động đồng bộ mã nguồn của bạn vào LazyOps.</p>
        </div>
        <div className="flex items-center gap-3">
          {appInstallURL && (
            <a
              href={appInstallURL}
              target="_blank"
              rel="noreferrer"
              className="rounded-xl bg-primary px-6 py-3 text-base font-bold text-primary-foreground shadow-lg transition-all hover:bg-primary/90 hover:scale-105"
            >
              Cài đặt GitHub App
            </a>
          )}
        </div>
      </div>

      <SectionCard className="p-4 shadow-sm rounded-xl bg-primary/5 border-primary/20">
        <div className="flex flex-col md:flex-row items-center justify-between gap-4">
          <div>
            <h3 className="text-lg font-bold">Làm mới dữ liệu</h3>
            <p className="text-sm text-muted-foreground mt-1">
              Nếu bạn vừa mới cài đặt hoặc cấp quyền mới trên GitHub mà chưa thấy thay đổi, hãy nhấn ĐỒNG BỘ ngay.
            </p>
          </div>
          <div className="flex items-center gap-3">
            <button
              type="button"
              className="rounded-xl bg-foreground px-6 py-3 text-base font-bold text-background transition-all hover:bg-foreground/90 disabled:opacity-50 min-w-[120px] shadow-md"
              onClick={() => quickSync.mutate({ github_access_token: '' })}
              disabled={quickSync.isPending}
            >
              {quickSync.isPending ? 'Đang tải...' : '🔄 ĐỒNG BỘ'}
            </button>
            <button
              type="button"
              className="rounded-xl border-2 border-border px-4 py-3 text-sm font-semibold transition-colors hover:bg-muted"
              onClick={() => setShowSyncModal(true)}
            >
              Nâng cao
            </button>
          </div>
        </div>
        {quickSync.error && (
          <div className="mt-4 rounded-lg bg-destructive/10 border border-destructive/20 p-3 text-sm text-destructive font-medium">
            Có lỗi xảy ra: {(quickSync.error as Error).message}
          </div>
        )}
      </SectionCard>

      {repos.length === 0 ? (
        <SectionCard className="shadow-lg p-6 rounded-2xl border-dashed border-2">
          <EmptyState
            icon={<span className="text-5xl">🔌</span>}
            title="Chưa tìm thấy Repository nào"
            description="Bạn chưa cấp quyền truy cập repository trên GitHub cho LazyOps. Vui lòng bấm vào 'Cài đặt GitHub App' và cấp quyền cho tổ chức hoặc cá nhân của bạn, sau đó quay lại trang này và nhấn ĐỒNG BỘ."
            action={
              <div className="mt-4 flex flex-col sm:flex-row items-center gap-3">
                <a
                  href="/api/auth/oauth/github/start?next=/integrations/github"
                  className="rounded-xl border-2 border-primary text-primary px-6 py-3 font-bold hover:bg-primary/5 transition-all"
                >
                  Liên kết Tài khoản GitHub
                </a>
              </div>
            }
          />
        </SectionCard>
      ) : (
        <div className="flex flex-col gap-6">
          {Object.entries(installations).map(([key, repos]) => {
            const [accountLogin, accountType] = key.split('/');
            return (
              <div key={key} className="flex flex-col gap-3">
                <div className="flex items-center gap-3 border-b pb-2">
                  <h2 className="text-2xl font-extrabold text-foreground">{accountLogin}</h2>
                  <StatusBadge label={accountType === 'Organization' ? 'Tổ chức' : 'Cá nhân'} variant="info" size="sm" dot={false} />
                  <span className="text-muted-foreground ml-auto bg-muted px-3 py-1 rounded-full text-sm font-bold">
                    {repos.length} Kho lưu trữ
                  </span>
                </div>

                <div className="grid gap-4 sm:grid-cols-2 md:grid-cols-3 mt-2">
                  {repos.map((repo) => (
                    <div key={repo.github_repo_id} className="rounded-xl border border-border bg-card p-4 shadow-sm hover:shadow-md hover:border-primary/40 transition-all group">
                      <div className="flex items-start justify-between mb-2">
                        <span className="font-bold text-lg text-foreground group-hover:text-primary transition-colors truncate block w-[80%]" title={repo.full_name}>
                          {repo.full_name.split('/')[1] || repo.full_name}
                        </span>
                        <StatusBadge
                          label={repo.private ? 'Riêng tư' : 'Công khai'}
                          variant={repo.private ? 'warning' : 'neutral'}
                          size="sm"
                          dot={false}
                        />
                      </div>
                      <p className="text-xs text-muted-foreground uppercase font-semibold mb-2 tracking-wider">
                        Quyền truy cập
                      </p>
                      <div className="flex flex-wrap gap-1.5">
                        {Object.entries(repo.permissions).map(([perm]) => (
                          <span
                            key={perm}
                            className="rounded bg-accent px-2 py-1 text-xs font-medium text-foreground"
                          >
                            {perm === 'admin' ? 'Quản trị' : perm === 'maintain' ? 'Bảo trì' : perm === 'push' ? 'Ghi' : perm === 'pull' ? 'Đọc' : perm}
                          </span>
                        ))}
                      </div>
                    </div>
                  ))}
                </div>
              </div>
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
    <Modal open={open} onClose={onClose} title="Cài đặt Đồng bộ Nâng cao" size="md">
      <form onSubmit={handleSubmit(onSubmit)} className="flex flex-col gap-4" noValidate>
        <p className="text-base text-muted-foreground">
          Chỉ cần điền GitHub PAT nếu API của bạn bị lỗi hoặc cần ép hệ thống quét lại toàn bộ mà không thông qua Webhook. Mặc định là không cần.
        </p>

        <FormField label="GitHub Token (PAT - Tùy chọn)" error={errors.github_access_token?.message}>
          <FormInput
            type="password"
            placeholder="ghp_xxxxxxxxxxxx"
            error={!!errors.github_access_token}
            {...register('github_access_token')}
            className="p-3 text-lg"
          />
        </FormField>

        {serverError && (
          <div className="rounded-lg border border-destructive/30 bg-destructive/10 px-4 py-3 text-sm font-semibold text-destructive">
            {serverError}
          </div>
        )}

        <div className="mt-4">
          <FormButton type="submit" loading={isSubmitting || syncInstallations.isPending} className="w-full text-lg font-bold py-6">
            Bắt đầu quét
          </FormButton>
        </div>
      </form>
    </Modal>
  );
}
