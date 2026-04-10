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

  return (
    <div className="flex flex-col gap-8 max-w-5xl mx-auto py-4">
      <div className="mb-2">
        <h1 className="text-3xl font-bold tracking-tight mb-2">Cài đặt Tích hợp</h1>
        <p className="text-muted-foreground text-lg">
          Quản lý nguồn mã tham chiếu từ kho lưu trữ để hệ thống tự động triển khai.
        </p>
      </div>

      <SectionCard
        title={
          <div className="flex items-center gap-3">
            <span className="text-2xl">🐙</span>
            <span>Kho mã nguồn GitHub</span>
          </div>
        }
        className="shadow-md rounded-2xl"
      >
        <div className="flex flex-col gap-6 mt-2">
          {/* GitHub Connection Status */}
          <div className="flex items-center justify-between border-b pb-4">
            <span className="text-base font-medium">Trạng thái kết nối App</span>
            <StatusBadge
              label={hasGitHub ? 'Đã kết nối' : 'Chưa kết nối'}
              variant={hasGitHub ? 'success' : 'neutral'}
              size="md"
            />
          </div>

          {!hasGitHub ? (
            <div className="rounded-xl border border-warning/30 bg-warning/5 p-4 text-warning-foreground mt-2">
              <p className="text-sm font-medium mb-3">
                Bạn chưa cài đặt GitHub App trên hệ thống gốc. Vui lòng vào <strong>Hệ thống & Tích hợp</strong> để cấu hình lần đầu.
              </p>
              <Link
                href="/integrations/github"
                className="rounded-lg bg-warning px-4 py-2 text-sm font-bold text-warning-foreground transition-all hover:bg-warning/80"
              >
                Cài đặt GitHub ngay &rarr;
              </Link>
            </div>
          ) : null}

          {/* Linked Repo Status */}
          <div className="flex items-center justify-between border-b pb-4">
            <span className="text-base font-medium">Kho lưu trữ đã gán</span>
            {hasRepoLink ? (
              <StatusBadge label={repoLink!.repo_full_name} variant="info" size="md" dot={false} />
            ) : (
              <StatusBadge label="Chưa gán" variant="neutral" size="md" dot={false} />
            )}
          </div>

          {hasRepoLink ? (
            <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3 bg-muted/30 p-4 rounded-xl">
              <SummaryField label="Nhánh theo dõi (Branch)" value={repoLink!.tracked_branch} />
              <SummaryField
                label="Bản nháp (Preview Deploys)"
                value={repoLink!.preview_enabled ? 'Bật' : 'Tắt'}
              />
              <SummaryField
                label="Cập nhật tự động"
                value="Sẵn sàng"
              />
            </div>
          ) : (
            <div className="text-center py-6 bg-accent/20 rounded-xl">
              <p className="text-muted-foreground mb-4">
                Dự án này chưa được kết nối với repository nào để triển khai tự động.
              </p>
              <Link
                href={`/projects/${projectId}/repo-link`}
                className="rounded-xl bg-primary px-6 py-3 text-base font-bold text-primary-foreground transition-all hover:bg-primary/90 shadow-md"
              >
                🔗 Gán mã nguồn
              </Link>
            </div>
          )}
        </div>
      </SectionCard>

      <SectionCard
        title={
          <div className="flex items-center gap-3">
            <span className="text-2xl">📡</span>
            <span>Tín hiệu Webhook</span>
          </div>
        }
        className="shadow-sm rounded-2xl"
      >
        <div className="flex items-center gap-4 mt-2 bg-health-healthy/5 p-4 rounded-xl border border-health-healthy/20">
          <HealthChip label="Hoạt động tốt" status="healthy" size="md" />
          <span className="text-base text-muted-foreground font-medium">
            Hệ thống đang sẵn sàng nhận lệnh từ GitHub.
          </span>
        </div>
        <div className="mt-4 break-all bg-card border p-3 rounded-lg text-sm text-muted-foreground font-mono">
          <span className="font-bold text-foreground">Webhook URL:</span> {webhookURL}
        </div>
      </SectionCard>
    </div>
  );
}

function SummaryField({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex flex-col gap-1 p-2 rounded-lg bg-background border shadow-sm">
      <span className="text-xs uppercase tracking-wider text-muted-foreground font-bold">{label}</span>
      <span className="text-base font-semibold text-foreground">{value}</span>
    </div>
  );
}
