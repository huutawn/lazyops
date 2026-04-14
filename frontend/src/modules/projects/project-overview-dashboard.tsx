'use client';

import Link from 'next/link';
import type { ReactNode } from 'react';
import { useMemo } from 'react';
import {
  Activity,
  Boxes,
  ExternalLink,
  FileCode2,
  Globe,
  PackageCheck,
  Server,
  Settings2,
  TerminalSquare,
} from 'lucide-react';
import { ErrorState } from '@/components/primitives/error-state';
import { LoadingBlock } from '@/components/primitives/loading';
import { SectionCard } from '@/components/primitives/section-card';
import { StatusBadge } from '@/components/primitives/status-badge';
import { ProjectThreeStepWizard } from '@/modules/bootstrap/project-three-step-wizard';
import { useProjectBootstrapStatus } from '@/modules/bootstrap/bootstrap-hooks';
import { useDeployments } from '@/modules/deployments/deployment-hooks';
import { useProjectEnv } from '@/modules/project-env/project-env-hooks';
import { useProjectInternalServices } from '@/modules/internal-services/internal-service-hooks';

const RUNTIME_STATUS_VARIANT: Record<string, 'success' | 'warning' | 'danger' | 'info' | 'neutral'> = {
  synced: 'success',
  live: 'success',
  running: 'success',
  progressing: 'warning',
  starting: 'warning',
  queued: 'info',
  candidate_ready: 'info',
  configured: 'neutral',
  disabled: 'neutral',
  unavailable: 'neutral',
  missing: 'neutral',
  stale: 'danger',
  inactive: 'danger',
};

type ProjectOverviewDashboardProps = {
  projectId: string;
};

export function ProjectOverviewDashboard({ projectId }: ProjectOverviewDashboardProps) {
  const bootstrap = useProjectBootstrapStatus(projectId);
  const deployments = useDeployments(projectId);
  const internalServices = useProjectInternalServices(projectId);
  const projectEnv = useProjectEnv(projectId);

  const latestDeployment = useMemo(() => {
    const items = [...(deployments.data?.items ?? [])];
    items.sort((a, b) => new Date(b.created_at).getTime() - new Date(a.created_at).getTime());
    return items[0] ?? null;
  }, [deployments.data?.items]);

  const liveDeployment = useMemo(() => {
    const items = [...(deployments.data?.items ?? [])];
    items.sort((a, b) => new Date(b.created_at).getTime() - new Date(a.created_at).getTime());
    return items.find((deployment) => deployment.promoted || deployment.rollout_state === 'promoted') ?? items[0] ?? null;
  }, [deployments.data?.items]);

  const bootstrapSteps = useMemo(() => {
    const map = new Map<string, { state: string; summary: string }>();
    (bootstrap.data?.steps ?? []).forEach((step) => {
      map.set(step.id, { state: step.state, summary: step.summary });
    });
    return map;
  }, [bootstrap.data?.steps]);

  if (bootstrap.isLoading) {
    return (
      <SectionCard title="Tổng quan project" description="Đang đồng bộ trạng thái thiết lập và runtime.">
        <LoadingBlock label="Đang tải tổng quan project..." className="py-8" />
      </SectionCard>
    );
  }

  if (bootstrap.isError || !bootstrap.data) {
    return (
      <ErrorState
        title="Không thể tải tổng quan project"
        message={bootstrap.error instanceof Error ? bootstrap.error.message : 'Không thể lấy trạng thái project.'}
      />
    );
  }

  const bootstrapData = bootstrap.data;
  const publicURLs = bootstrapData.public_urls?.length ? bootstrapData.public_urls : liveDeployment?.public_urls ?? [];
  const primaryPublicURL = publicURLs[0] ?? '';
  const fallbackPublicURLs = publicURLs.slice(1);
  const publicURLReason = bootstrapData.public_url_reason || liveDeployment?.public_url_reason || 'Chưa có domain công khai cho project này.';
  const runtimeInventory = bootstrapData.runtime_inventory ?? {
    sync_state: 'missing',
    sync_reason: 'Chua dong bo du lieu runtime',
    runtime_mode: '',
    app_runtime: { status: 'unavailable' },
    sidecar_runtime: { enabled: false, status: 'disabled' },
    internal_services: [],
  };
  const envData = projectEnv.data;
  const envConfigured = envData?.configured ?? false;
  const internalServiceItems = internalServices.data?.items ?? [];
  const publicServiceNames =
    liveDeployment?.services.filter((service) => service.public).map((service) => service.name) ??
    (runtimeInventory.app_runtime.service_name ? [runtimeInventory.app_runtime.service_name] : []);
  const runtimeTargetRefs = runtimeInventory.app_runtime.target_ids ?? [];

  return (
    <div className="flex flex-col gap-6">
      <SectionCard
        title="Trạng thái điều phối"
        description="Màn hình này gom các thông tin cần hành động nhất: nguồn mã, hạ tầng, domain, runtime, env và các đường dẫn thao tác."
      >
        <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-4">
          <StatePanel
            label="Tổng quan"
            value={formatStateLabelVN(bootstrapData.overall_state)}
            summary={bootstrapData.auto_mode.mode_reason_human}
            icon={<Activity className="size-4 text-[#38BDF8]" />}
          />
          <StatePanel
            label="Kết nối mã nguồn"
            value={formatStateLabelVN(bootstrapSteps.get('connect_code')?.state ?? 'missing')}
            summary={bootstrapSteps.get('connect_code')?.summary ?? 'Chưa kết nối repository.'}
            icon={<FileCode2 className="size-4 text-[#38BDF8]" />}
          />
          <StatePanel
            label="Kết nối máy chủ"
            value={formatStateLabelVN(bootstrapSteps.get('connect_infra')?.state ?? 'missing')}
            summary={bootstrapSteps.get('connect_infra')?.summary ?? 'Chưa kết nối hạ tầng chạy app.'}
            icon={<Server className="size-4 text-[#38BDF8]" />}
          />
          <StatePanel
            label="Triển khai"
            value={formatStateLabelVN(bootstrapSteps.get('deploy')?.state ?? 'blocked')}
            summary={bootstrapSteps.get('deploy')?.summary ?? 'Chưa có rollout mới.'}
            icon={<PackageCheck className="size-4 text-[#38BDF8]" />}
          />
        </div>
      </SectionCard>

      <div className="grid gap-6 xl:grid-cols-2">
        <SectionCard
          title="Domain công khai"
          description="Public URL hiện tại của service public. Đây là link nên đưa cho user hoặc dùng để tạo traffic khi demo."
          actions={
            primaryPublicURL ? (
              <a
                href={primaryPublicURL}
                target="_blank"
                rel="noreferrer"
                className="inline-flex items-center gap-2 rounded-lg border border-[#334155] bg-[#0B1120]/60 px-3 py-2 text-sm font-semibold text-[#38BDF8] transition-colors hover:text-white"
              >
                Mở app <ExternalLink className="size-4" />
              </a>
            ) : null
          }
        >
          {primaryPublicURL ? (
            <div className="flex flex-col gap-3">
              <a
                href={primaryPublicURL}
                target="_blank"
                rel="noreferrer"
                className="break-all text-base font-semibold text-[#38BDF8] underline-offset-4 hover:underline"
              >
                {primaryPublicURL}
              </a>
              {fallbackPublicURLs.length > 0 ? (
                <div className="flex flex-col gap-1">
                  {fallbackPublicURLs.map((url) => (
                    <a
                      key={url}
                      href={url}
                      target="_blank"
                      rel="noreferrer"
                      className="break-all text-sm text-[#94a3b8] underline-offset-4 hover:text-white hover:underline"
                    >
                      {url}
                    </a>
                  ))}
                </div>
              ) : null}
              <p className="text-sm text-[#64748b]">
                Service public: {publicServiceNames.length > 0 ? publicServiceNames.join(', ') : 'app'}
              </p>
            </div>
          ) : (
            <p className="text-sm text-[#94a3b8]">{publicURLReason}</p>
          )}
        </SectionCard>

        <SectionCard
          title="Runtime hiện tại"
          description="Read model nhẹ cho runtime hiện tại: revision live, stable revision, app runtime, sidecar và internal services."
          actions={
            liveDeployment ? (
              <Link
                href={`/projects/${projectId}/deployments/${liveDeployment.id}`}
                className="rounded-lg border border-lazyops-border px-3 py-2 text-sm font-semibold text-lazyops-text transition-colors hover:bg-lazyops-border/10"
              >
                Xem deployment
              </Link>
            ) : null
          }
        >
          <div className="flex flex-col gap-4">
            <div className="flex flex-wrap items-center gap-2">
              <StatusBadge
                label={`Sync ${formatStateLabelVN(runtimeInventory.sync_state)}`}
                variant={runtimeBadgeVariant(runtimeInventory.sync_state)}
                size="sm"
              />
              <StatusBadge
                label={`App ${formatStateLabelVN(runtimeInventory.app_runtime.status)}`}
                variant={runtimeBadgeVariant(runtimeInventory.app_runtime.status)}
                size="sm"
              />
              <StatusBadge
                label={`Sidecar ${formatStateLabelVN(runtimeInventory.sidecar_runtime.status)}`}
                variant={runtimeBadgeVariant(runtimeInventory.sidecar_runtime.status)}
                size="sm"
              />
            </div>

            {runtimeInventory.sync_reason ? (
              <div className="rounded-xl border border-[#1e293b] bg-[#0B1120]/60 p-3 text-sm text-[#94a3b8]">
                {runtimeInventory.sync_reason}
              </div>
            ) : null}

            <div className="grid gap-3 sm:grid-cols-2">
              <SummaryLine
                label="Live revision"
                value={runtimeInventory.live_revision ? `r${runtimeInventory.live_revision}` : 'Chưa có'}
              />
              <SummaryLine
                label="Stable revision"
                value={runtimeInventory.stable_revision ? `r${runtimeInventory.stable_revision}` : 'Chưa có'}
              />
              <SummaryLine label="Runtime mode" value={runtimeInventory.runtime_mode || 'Chưa có'} />
              <SummaryLine label="App service" value={runtimeInventory.app_runtime.service_name || 'Chưa có'} />
              <SummaryLine label="App container" value={runtimeInventory.app_runtime.container_name || 'Chưa có'} />
              <SummaryLine label="Image live" value={runtimeInventory.app_runtime.image_ref || 'Chưa có'} />
              <SummaryLine
                label="Commit"
                value={runtimeInventory.app_runtime.commit_sha ? runtimeInventory.app_runtime.commit_sha.slice(0, 7) : 'Chưa có'}
              />
              <SummaryLine
                label="Mục tiêu chạy"
                value={runtimeTargetRefs.length > 0 ? runtimeTargetRefs.join(', ') : 'Chưa có target'}
              />
            </div>

            <div className="rounded-xl border border-[#1e293b] bg-[#0B1120]/60 p-4">
              <div className="mb-3 flex items-center justify-between gap-3">
                <span className="text-sm font-semibold text-white">Sidecar runtime</span>
                <StatusBadge
                  label={formatStateLabelVN(runtimeInventory.sidecar_runtime.status)}
                  variant={runtimeBadgeVariant(runtimeInventory.sidecar_runtime.status)}
                  size="sm"
                />
              </div>
              <div className="grid gap-3 sm:grid-cols-2">
                <SummaryLine label="Bật sidecar" value={runtimeInventory.sidecar_runtime.enabled ? 'Có' : 'Không'} />
                <SummaryLine label="Container" value={runtimeInventory.sidecar_runtime.container_name || 'Chưa có'} />
              </div>
              {runtimeInventory.sidecar_runtime.status_reason ? (
                <p className="mt-3 text-sm text-[#94a3b8]">{runtimeInventory.sidecar_runtime.status_reason}</p>
              ) : null}
            </div>

            <div className="rounded-xl border border-[#1e293b] bg-[#0B1120]/60 p-4">
              <div className="mb-3 flex items-center justify-between gap-3">
                <span className="text-sm font-semibold text-white">Internal services runtime</span>
                <span className="text-xs text-[#64748b]">{runtimeInventory.internal_services.length} service</span>
              </div>
              {runtimeInventory.internal_services.length > 0 ? (
                <div className="grid gap-3">
                  {runtimeInventory.internal_services.map((item) => (
                    <div key={item.id} className="rounded-xl border border-[#1e293b] bg-[#020617]/80 p-3">
                      <div className="mb-2 flex items-center justify-between gap-3">
                        <span className="text-sm font-semibold text-white">{item.alias}</span>
                        <StatusBadge label={formatStateLabelVN(item.status)} variant={runtimeBadgeVariant(item.status)} size="sm" />
                      </div>
                      <div className="grid gap-2 sm:grid-cols-2">
                        <SummaryLine label="Kind" value={item.kind} />
                        <SummaryLine label="Local endpoint" value={item.local_endpoint} />
                        <SummaryLine label="Container" value={item.container_name || 'Chưa có'} />
                        <SummaryLine label="Protocol" value={item.protocol} />
                      </div>
                      {item.status_reason ? <p className="mt-2 text-sm text-[#94a3b8]">{item.status_reason}</p> : null}
                    </div>
                  ))}
                </div>
              ) : (
                <p className="text-sm text-[#94a3b8]">Project này chưa bật internal service nào cho runtime hiện tại.</p>
              )}
            </div>
          </div>
        </SectionCard>

        <SectionCard
          title="Biến môi trường runtime"
          description="Trạng thái bundle `.env` theo project. Đây là layer env sẽ được áp dụng ở deploy kế tiếp."
          actions={
            <Link
              href={`/projects/${projectId}/env`}
              className="rounded-lg border border-lazyops-border px-3 py-2 text-sm font-semibold text-lazyops-text transition-colors hover:bg-lazyops-border/10"
            >
              Mở env
            </Link>
          }
        >
          {projectEnv.isLoading ? (
            <LoadingBlock label="Đang kiểm tra env..." className="py-6" />
          ) : projectEnv.isError ? (
            <p className="text-sm text-health-unhealthy">
              {projectEnv.error instanceof Error ? projectEnv.error.message : 'Không tải được trạng thái env.'}
            </p>
          ) : (
            <div className="flex flex-col gap-4">
              <div className="flex items-center gap-2">
                <StatusBadge label={envConfigured ? 'Đã cấu hình' : 'Chưa cấu hình'} variant={envConfigured ? 'success' : 'neutral'} size="sm" />
                <span className="text-sm text-[#94a3b8]">
                  {envConfigured
                    ? `${envData?.keys.length ?? 0} key đã lưu cho runtime.`
                    : 'Chưa có bundle env được lưu ở cấp project.'}
                </span>
              </div>
              <div className="grid gap-3 sm:grid-cols-2">
                <SummaryLine label="Cập nhật gần nhất" value={envData?.updated_at ? formatDateTime(envData.updated_at) : 'Chưa có'} />
                <SummaryLine label="Fingerprint" value={envData?.fingerprint ? shorten(envData.fingerprint) : 'Chưa có'} />
                <SummaryLine label="Số key" value={String(envData?.keys.length ?? 0)} />
                <SummaryLine label="Warnings" value={String(envData?.parse_warnings.length ?? 0)} />
              </div>
            </div>
          )}
        </SectionCard>

        <SectionCard
          title="Dịch vụ nội bộ"
          description="Các dependency do LazyOps tạo và tự nối vào runtime qua local contract."
          actions={
            <Link
              href={`/projects/${projectId}/internal-services`}
              className="rounded-lg border border-lazyops-border px-3 py-2 text-sm font-semibold text-lazyops-text transition-colors hover:bg-lazyops-border/10"
            >
              Mở dịch vụ nội bộ
            </Link>
          }
        >
          {internalServices.isLoading ? (
            <LoadingBlock label="Đang tải internal services..." className="py-6" />
          ) : internalServices.isError ? (
            <p className="text-sm text-health-unhealthy">
              {internalServices.error instanceof Error ? internalServices.error.message : 'Không tải được internal services.'}
            </p>
          ) : internalServiceItems.length > 0 ? (
            <div className="grid gap-3">
              {internalServiceItems.map((item) => (
                <div key={item.id} className="rounded-xl border border-[#1e293b] bg-[#0B1120]/60 p-4">
                  <div className="mb-1 flex items-center justify-between gap-3">
                    <span className="text-sm font-semibold text-white">{item.alias}</span>
                    <StatusBadge label={item.kind} variant="info" size="sm" dot={false} />
                  </div>
                  <div className="grid gap-2 sm:grid-cols-2">
                    <SummaryLine label="Protocol" value={item.protocol} />
                    <SummaryLine label="Local endpoint" value={item.local_endpoint} />
                  </div>
                </div>
              ))}
            </div>
          ) : (
            <p className="text-sm text-[#94a3b8]">Project này chưa bật internal service nào.</p>
          )}
        </SectionCard>
      </div>

      <SectionCard title="Đường dẫn nhanh" description="Đi thẳng tới các màn hình thao tác chính theo đúng current flow.">
        <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-5">
          <QuickLinkCard
            href={`/projects/${projectId}/repo-link`}
            title="Kho mã"
            description="Liên kết hoặc rà lại repository GitHub của project."
            icon={<FileCode2 className="size-5 text-[#38BDF8]" />}
          />
          <QuickLinkCard
            href={`/projects/${projectId}/env`}
            title="Biến môi trường"
            description="Lưu `.env` runtime và kiểm tra helper snippet."
            icon={<TerminalSquare className="size-5 text-[#38BDF8]" />}
          />
          <QuickLinkCard
            href={`/projects/${projectId}/internal-services`}
            title="Dịch vụ nội bộ"
            description="Bật Postgres, Redis hoặc các dependency local contract."
            icon={<Boxes className="size-5 text-[#38BDF8]" />}
          />
          <QuickLinkCard
            href={`/projects/${projectId}/deployments`}
            title="Triển khai"
            description="Xem lịch sử deployment, rollout và domain công khai."
            icon={<PackageCheck className="size-5 text-[#38BDF8]" />}
          />
          <QuickLinkCard
            href={`/projects/${projectId}/observability`}
            title="Giám sát"
            description="Mở logs, traces, incidents và metrics trong phạm vi project."
            icon={<Globe className="size-5 text-[#38BDF8]" />}
          />
        </div>
      </SectionCard>

      <SectionCard
        title="Luồng thao tác chính"
        description="Luồng guided setup vẫn được giữ lại để thao tác nhanh: connect code, connect infra, deploy."
        actions={
          <span className="inline-flex items-center gap-2 text-sm text-[#94a3b8]">
            <Settings2 className="size-4" />
            Tổng số máy chủ sẵn sàng: {bootstrapData.inventory.healthy_instances}
          </span>
        }
      >
        <ProjectThreeStepWizard projectId={projectId} compact showSummary={false} />
      </SectionCard>

      {latestDeployment && latestDeployment.id !== liveDeployment?.id ? (
        <SectionCard
          title="Deployment gần nhất"
          description="Revision mới nhất không nhất thiết là revision đang live, nhưng đây là deployment cuối cùng được ghi nhận."
          actions={
            <Link
              href={`/projects/${projectId}/deployments/${latestDeployment.id}`}
              className="rounded-lg border border-lazyops-border px-3 py-2 text-sm font-semibold text-lazyops-text transition-colors hover:bg-lazyops-border/10"
            >
              Mở revision mới nhất
            </Link>
          }
        >
          <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
            <SummaryLine label="Revision" value={`r${latestDeployment.revision}`} />
            <SummaryLine label="Commit" value={latestDeployment.commit_sha.slice(0, 7)} />
            <SummaryLine label="Build" value={formatStateLabel(latestDeployment.build_state)} />
            <SummaryLine label="Rollout" value={formatStateLabel(latestDeployment.rollout_state)} />
          </div>
        </SectionCard>
      ) : null}
    </div>
  );
}

function StatePanel({
  label,
  value,
  summary,
  icon,
}: {
  label: string;
  value: string;
  summary: string;
  icon: ReactNode;
}) {
  return (
    <div className="rounded-2xl border border-[#1e293b] bg-[#0B1120]/70 p-4">
      <div className="mb-3 flex items-center justify-between gap-3">
        <span className="inline-flex items-center gap-2 text-sm font-semibold text-white">
          {icon}
          {label}
        </span>
        <span className="text-xs font-semibold text-[#38BDF8]">{value}</span>
      </div>
      <p className="text-sm leading-relaxed text-[#94a3b8]">{summary}</p>
    </div>
  );
}

function SummaryLine({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex flex-col gap-1">
      <span className="text-xs font-medium uppercase tracking-wide text-[#64748b]">{label}</span>
      <span className="break-words text-sm font-semibold text-white">{value}</span>
    </div>
  );
}

function QuickLinkCard({
  href,
  title,
  description,
  icon,
}: {
  href: string;
  title: string;
  description: string;
  icon: ReactNode;
}) {
  return (
    <Link
      href={href}
      className="group rounded-2xl border border-[#1e293b] bg-[#0B1120]/70 p-4 transition-all hover:border-[#38BDF8]/40 hover:bg-[#0B1120]"
    >
      <div className="mb-3 inline-flex rounded-xl border border-[#1e293b] bg-[#0F172A] p-2">
        {icon}
      </div>
      <div className="flex flex-col gap-2">
        <span className="text-sm font-semibold text-white group-hover:text-[#38BDF8]">{title}</span>
        <p className="text-sm leading-relaxed text-[#94a3b8]">{description}</p>
      </div>
    </Link>
  );
}

function formatStateLabel(value: string): string {
  return value.replace(/_/g, ' ').replace(/\b\w/g, (match) => match.toUpperCase());
}

function formatStateLabelVN(value: string): string {
  const normalized = value.toLowerCase();
  const map: Record<string, string> = {
    synced: 'Đã đồng bộ',
    missing: 'Chưa kết nối',
    linked: 'Đã liên kết',
    healthy: 'Sẵn sàng',
    installing: 'Đang cài',
    ready: 'Sẵn sàng',
    blocked: 'Bị chặn',
    deploying: 'Đang triển khai',
    degraded: 'Cảnh báo',
    rolled_back: 'Đã hoàn tác',
    error: 'Lỗi',
    running: 'Đang chạy',
    attention_required: 'Cần xử lý',
    ready_to_deploy: 'Sẵn sàng triển khai',
    partially_ready: 'Chưa hoàn tất',
    not_ready: 'Chưa sẵn sàng',
    completed: 'Hoàn tất',
    success: 'Thành công',
    pending: 'Chờ xử lý',
    failed: 'Thất bại',
    started: 'Đã bắt đầu',
    queued: 'Đang xếp hàng',
    promoted: 'Đã phát hành',
    progressing: 'Đang đồng bộ',
    live: 'Đang live',
    starting: 'Đang khởi động',
    candidate_ready: 'Sẵn sàng promote',
    unavailable: 'Chưa có dữ liệu',
    inactive: 'Không hoạt động',
    configured: 'Đã cấu hình',
    disabled: 'Đã tắt',
    stale: 'Chưa đồng bộ',
  };
  return map[normalized] ?? formatStateLabel(value);
}

function runtimeBadgeVariant(value: string): 'success' | 'warning' | 'danger' | 'info' | 'neutral' {
  return RUNTIME_STATUS_VARIANT[value.toLowerCase()] ?? 'neutral';
}

function formatDateTime(value: string): string {
  return new Date(value).toLocaleString();
}

function shorten(value: string): string {
  return value.length > 16 ? `${value.slice(0, 8)}...${value.slice(-6)}` : value;
}
