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
import { buildPublicURLDisplay } from '@/modules/bootstrap/bootstrap-ui';
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
        <PageHeader title="Lịch sử Deploy" subtitle="Trạng thái và lịch sử Deploy." />
        <ErrorState title="Lỗi tải danh sách" message="Không thể lấy dữ liệu Deploy từ máy chủ." />
      </div>
    );
  }

  const deployments = data?.items ?? [];

  return (
    <div className="flex flex-col gap-6">
      <PageHeader
        title="Lịch sử Deploy"
        subtitle={projectId ? 'Lịch sử Deploy của Project này.' : 'Lịch sử Deploy trên toàn bộ các Project.'}
      />

      {deployments.length === 0 ? (
        <SectionCard title="Chưa có Deploy nào" description="Bạn chưa có bất kỳ lịch sử Deploy nào.">
          <EmptyState
            title="Trống"
            description="Hãy vào Project và thiết lập các bước để bắt đầu Deploy lần đầu tiên."
          />
        </SectionCard>
      ) : (
        <SectionCard>
          <div className="overflow-x-auto">
            <table className="w-full text-base">
              <thead>
                <tr className="border-b border-lazyops-border">
                  <th className="px-6 py-3 text-left font-medium text-lazyops-muted">Bản sửa đổi (Revision)</th>
                  <th className="px-6 py-3 text-left font-medium text-lazyops-muted">Mã Commit</th>
                  <th className="px-6 py-3 text-left font-medium text-lazyops-muted">Tiến trình Build</th>
                  <th className="px-6 py-3 text-left font-medium text-lazyops-muted">Tiến trình Phân phối (Rollout)</th>
                  <th className="px-6 py-3 text-left font-medium text-lazyops-muted">Tên miền (Domain)</th>
                  <th className="px-6 py-3 text-left font-medium text-lazyops-muted">Kích hoạt (Trigger)</th>
                  <th className="px-6 py-3 text-left font-medium text-lazyops-muted">Bởi</th>
                  <th className="px-6 py-3 text-left font-medium text-lazyops-muted">Hoàn thành lúc</th>
                </tr>
              </thead>
              <tbody>
                {deployments.map((dep) => {
                  const publicURLs = dep.public_urls ?? [];
                  const primaryPublicURL = publicURLs[0] ?? '';
                  const publicURLDisplay = buildPublicURLDisplay(primaryPublicURL, dep.public_url_status, dep.public_url_reason);
                  return (
                  <tr
                    key={dep.id}
                    className="border-b border-lazyops-border/50 transition-colors hover:bg-lazyops-border/10"
                  >
                    <td className="px-6 py-3">
                      <Link
                        href={`/projects/${dep.project_id}/deployments/${dep.id}`}
                        className="font-medium text-primary hover:underline"
                      >
                        r{dep.revision}
                      </Link>
                    </td>
                    <td className="px-6 py-3 font-mono text-sm text-lazyops-muted">
                      {dep.commit_sha.slice(0, 7)}
                    </td>
                    <td className="px-6 py-3">
                      <StatusBadge
                        label={formatState(dep.build_state)}
                        variant={BUILD_STATE_VARIANT[dep.build_state]}
                        size="sm"
                      />
                    </td>
                    <td className="px-6 py-3">
                      <StatusBadge
                        label={formatState(dep.rollout_state)}
                        variant={ROLLOUT_STATE_VARIANT[dep.rollout_state]}
                        size="sm"
                      />
                    </td>
                    <td className="px-6 py-3 text-sm">
                      {primaryPublicURL ? (
                        <a
                          href={primaryPublicURL}
                          target="_blank"
                          rel="noreferrer"
                          className="text-primary hover:underline break-all"
                        >
                          {primaryPublicURL.replace(/^https?:\/\//, '')}
                        </a>
                      ) : (
                        <span className="text-lazyops-muted">{publicURLDisplay.message}</span>
                      )}
                    </td>
                    <td className="px-6 py-3 text-sm text-lazyops-muted">
                      <StatusBadge label={dep.trigger_kind} variant="neutral" size="sm" dot={false} />
                    </td>
                    <td className="px-6 py-3 text-sm text-lazyops-muted">{dep.triggered_by}</td>
                    <td className="px-6 py-3 text-sm text-lazyops-muted">
                      {dep.completed_at ? new Date(dep.completed_at).toLocaleString() : '—'}
                    </td>
                  </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
        </SectionCard>
      )}
    </div>
  );
}
