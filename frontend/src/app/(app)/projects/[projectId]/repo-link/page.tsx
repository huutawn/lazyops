'use client';

import { useState } from 'react';
import Link from 'next/link';
import { useParams } from 'next/navigation';
import { useGitHubInstallations } from '@/modules/github-sync/github-hooks';
import { useProjectRepoLink } from '@/modules/repo-link/repo-link-hooks';
import { ProjectRepoLinkModal } from '@/modules/repo-link/repo-link-modal';
import type { ProjectRepoLink } from '@/modules/repo-link/repo-link-types';
import { PageHeader } from '@/components/primitives/page-header';
import { SectionCard } from '@/components/primitives/section-card';
import { EmptyState } from '@/components/primitives/empty-state';
import { ErrorState } from '@/components/primitives/error-state';
import { SkeletonPage } from '@/components/primitives/skeleton';
import { StatusBadge } from '@/components/primitives/status-badge';

export default function RepoLinkPage() {
  const params = useParams();
  const projectId = params?.projectId as string;
  const { data: reposData, isLoading: reposLoading, isError: reposError } = useGitHubInstallations();
  const repoLink = useProjectRepoLink(projectId);
  const [showLinkModal, setShowLinkModal] = useState(false);

  if (reposLoading || repoLink.isLoading) {
    return <SkeletonPage title cards={2} />;
  }

  if (reposError || repoLink.isError) {
    return (
      <div className="flex flex-col gap-6">
        <PageHeader title="Mã nguồn" subtitle="Kết nối repository để LazyOps biết lấy code từ đâu." />
        <ErrorState
          title="Không tải được cấu hình mã nguồn"
          message="Không thể lấy danh sách repository hoặc trạng thái kết nối hiện tại."
        />
      </div>
    );
  }

  const repos = reposData?.items ?? [];
  const linkedRepo = repoLink.data ?? null;

  if (repos.length === 0) {
    return (
      <div className="flex flex-col gap-6">
        <PageHeader title="Mã nguồn" subtitle="Kết nối repository để LazyOps biết lấy code từ đâu." />
        <SectionCard title="Chưa có repository khả dụng" description="Bạn cần kết nối GitHub App trước khi chọn repository cho project này.">
          <EmptyState
            title="Chưa thấy repository nào"
            description="Hãy kết nối GitHub App hoặc sync lại danh sách repository rồi quay lại đây."
            action={
              <Link
                href="/integrations/github"
                className="inline-block rounded-lg bg-primary px-4 py-2 text-sm font-semibold text-lazyops-bg transition-colors hover:bg-primary/90"
              >
                Mở GitHub App
              </Link>
            }
          />
        </SectionCard>
      </div>
    );
  }

  return (
    <div className="flex flex-col gap-6">
      <PageHeader
        title="Mã nguồn"
        subtitle="Chọn repository và nhánh theo dõi. Sau bước này bạn có thể nối máy chủ và deploy ngay từ tab Bắt đầu."
        actions={
          <button
            type="button"
            className="rounded-lg bg-primary px-4 py-2 text-sm font-semibold text-lazyops-bg transition-colors hover:bg-primary/90"
            onClick={() => setShowLinkModal(true)}
          >
            {linkedRepo ? 'Đổi repository' : 'Kết nối repository'}
          </button>
        }
      />

      {linkedRepo ? (
        <LinkedRepoCard repo={linkedRepo} />
      ) : (
        <SectionCard title="Chưa kết nối mã nguồn" description="Project này chưa biết phải lấy code từ repository nào.">
          <EmptyState
            title="Hãy kết nối repository đầu tiên"
            description="Chọn đúng repo và nhánh theo dõi để LazyOps có thể build và deploy cho project này."
            action={
              <button
                type="button"
                className="rounded-lg bg-primary px-4 py-2 text-sm font-semibold text-lazyops-bg transition-colors hover:bg-primary/90"
                onClick={() => setShowLinkModal(true)}
              >
                Kết nối repository
              </button>
            }
          />
        </SectionCard>
      )}

      <ProjectRepoLinkModal
        projectId={projectId}
        repos={repos}
        open={showLinkModal}
        onClose={() => setShowLinkModal(false)}
      />
    </div>
  );
}

function LinkedRepoCard({ repo }: { repo: ProjectRepoLink }) {
  return (
    <SectionCard title="Repository đang dùng" description={repo.repo_full_name}>
      <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
        <SummaryField label="Repository" value={repo.repo_full_name} />
        <SummaryField label="Nhánh theo dõi" value={repo.tracked_branch} />
        <SummaryField label="Preview deploy" value={repo.preview_enabled ? 'Bật' : 'Tắt'} />
        <SummaryField label="Kết nối lúc" value={new Date(repo.created_at).toLocaleString()} />
      </div>
      <div className="mt-4 flex items-center gap-2">
        <StatusBadge label="Đã kết nối" variant="success" size="md" />
        <span className="text-sm text-lazyops-muted">
          Mỗi lần push vào nhánh <code className="rounded bg-lazyops-border/20 px-1.5 py-0.5 text-xs">{repo.tracked_branch}</code> sẽ sẵn sàng cho build/deploy.
        </span>
      </div>
    </SectionCard>
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
