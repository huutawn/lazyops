'use client';

import Link from 'next/link';
import { useMemo, useState, type ReactNode } from 'react';
import { Boxes, ExternalLink, FileCode2, Logs, Rocket, Server } from 'lucide-react';
import { ErrorState } from '@/components/primitives/error-state';
import { LoadingBlock } from '@/components/primitives/loading';
import { SectionCard } from '@/components/primitives/section-card';
import { useProjectBootstrapStatus, useOneClickDeploy } from '@/modules/bootstrap/bootstrap-hooks';
import { buildPublicURLDisplay, formatBootstrapStateLabelVN, resolveProjectNextAction, summarizeProjectSetup } from '@/modules/bootstrap/bootstrap-ui';
import { ProjectConnectInfraModal } from '@/modules/bootstrap/project-connect-infra-modal';
import { ProjectThreeStepWizard } from '@/modules/bootstrap/project-three-step-wizard';
import { useDeployments } from '@/modules/deployments/deployment-hooks';
import { useGitHubInstallations } from '@/modules/github-sync/github-hooks';
import { useProjectRepoLink } from '@/modules/repo-link/repo-link-hooks';
import { ProjectRepoLinkModal } from '@/modules/repo-link/repo-link-modal';
import { useProjectRuntime } from '@/modules/project-runtime/project-runtime-hooks';
import { useProjectServices } from '@/modules/project-services/project-service-hooks';

type ProjectOverviewDashboardProps = {
  projectId: string;
};

export function ProjectOverviewDashboard({ projectId }: ProjectOverviewDashboardProps) {
  const deployments = useDeployments(projectId);
  const services = useProjectServices(projectId);
  const runtime = useProjectRuntime(projectId);
  const bootstrap = useProjectBootstrapStatus(projectId);
  const repoLink = useProjectRepoLink(projectId);
  const repoOptions = useGitHubInstallations();
  const oneClickDeploy = useOneClickDeploy(projectId);
  const [showRepoModal, setShowRepoModal] = useState(false);
  const [showInfraModal, setShowInfraModal] = useState(false);
  const [actionMessage, setActionMessage] = useState<string | null>(null);
  const [actionError, setActionError] = useState<string | null>(null);

  const latestDeployment = useMemo(() => {
    const items = [...(deployments.data?.items ?? [])];
    items.sort((a, b) => new Date(b.created_at).getTime() - new Date(a.created_at).getTime());
    return items[0] ?? null;
  }, [deployments.data?.items]);

  if (
    deployments.isLoading ||
    services.isLoading ||
    runtime.isLoading ||
    bootstrap.isLoading ||
    repoLink.isLoading ||
    repoOptions.isLoading
  ) {
    return (
      <SectionCard title="Bạn cần làm gì tiếp theo?" description="Đang kiểm tra trạng thái hiện tại của project.">
        <LoadingBlock label="Đang tải trạng thái project..." className="py-10" />
      </SectionCard>
    );
  }

  if (deployments.isError || services.isError || runtime.isError || bootstrap.isError || repoLink.isError || !bootstrap.data) {
    return (
      <ErrorState
        title="Không thể tải project workspace"
        message="Không thể đồng bộ trạng thái mã nguồn, dịch vụ, runtime hoặc thiết lập triển khai của project này."
      />
    );
  }

  const serviceItems = services.data?.items ?? [];
  const runtimeServices = runtime.data?.services ?? [];
  const runtimeNodes = runtime.data?.nodes ?? [];
  const liveServices = runtimeServices.filter((item) => item.runtime_status === 'live').length;
  const readyNodes = runtimeNodes.filter((item) => item.is_ready).length;
  const repoItems = repoOptions.data?.items ?? [];
  const primaryPublicURL =
    bootstrap.data.public_urls?.[0] ||
    runtime.data?.public_urls?.[0] ||
    runtimeServices.flatMap((service) => service.public_urls ?? [])[0] ||
    '';
  const publicURLDisplay = buildPublicURLDisplay(
    primaryPublicURL,
    bootstrap.data.public_url_status ?? runtime.data?.public_url_status,
    bootstrap.data.public_url_reason ?? runtime.data?.public_url_reason,
  );
  const nextAction = resolveProjectNextAction({
    status: bootstrap.data,
    repoLink: repoLink.data ?? null,
    primaryPublicURL,
    logsHref: `/projects/${projectId}/observability`,
  });
  const setupCards = summarizeProjectSetup(bootstrap.data);
  const latestDeploymentLabel = latestDeployment
    ? new Date(latestDeployment.created_at).toLocaleString()
    : 'Chưa có lần deploy nào';
  const runtimeSummary = primaryPublicURL
    ? 'HTTPS công khai đã sẵn sàng'
    : publicURLDisplay.state === 'pending' || publicURLDisplay.state === 'error'
      ? publicURLDisplay.message
      : liveServices > 0
      ? `${liveServices} service đang chạy`
      : runtime.data?.sync_reason || 'Chưa có runtime';

  const runPrimaryAction = async () => {
    setActionError(null);
    setActionMessage(null);

    if (nextAction.kind === 'repo') {
      if (repoItems.length > 0) {
        setShowRepoModal(true);
      }
      return;
    }

    if (nextAction.kind === 'infra') {
      setShowInfraModal(true);
      return;
    }

    if (nextAction.kind === 'deploy') {
      try {
        const result = await oneClickDeploy.mutateAsync({});
        setActionMessage(
          result.deployment_id
            ? `Đã tạo deployment ${result.deployment_id}. Bạn có thể mở tab Triển khai hoặc Nhật ký để theo dõi tiếp.`
            : result.build_job_id
              ? `Đã xếp build job ${result.build_job_id}. LazyOps sẽ build image rồi tự tạo deployment sau khi callback thành công.`
              : 'Đã bắt đầu triển khai project.',
        );
      } catch (error) {
        setActionError(error instanceof Error ? error.message : 'Không thể bắt đầu triển khai.');
      }
    }
  };

  return (
    <div className="flex flex-col gap-6">
      <SectionCard
        title="Bạn cần làm gì tiếp theo?"
        description={nextAction.description}
        actions={
          <PrimaryActionButton
            action={nextAction}
            isPending={oneClickDeploy.isPending}
            repoOptionsAvailable={repoItems.length > 0}
            onClick={() => {
              void runPrimaryAction();
            }}
          />
        }
      >
        <div className="grid gap-4 lg:grid-cols-3">
          {setupCards.map((card) => (
            <div key={card.id} className="rounded-2xl border border-[#1e293b] bg-[#020617]/70 p-6">
              <div className="flex items-center justify-between gap-3">
                <div className="text-base font-semibold text-white">{card.title}</div>
                <div className={`text-sm font-semibold ${stateColorClass(card.state)}`}>
                  {formatBootstrapStateLabelVN(card.state)}
                </div>
              </div>
              <p className="mt-3 text-base leading-relaxed text-[#94a3b8]">{card.summary}</p>
            </div>
          ))}
        </div>

        {actionMessage ? (
          <div className="mt-4 rounded-2xl border border-[#0EA5E9]/30 bg-[#0EA5E9]/10 px-6 py-3 text-base text-[#bae6fd]">
            {actionMessage}
          </div>
        ) : null}

        {actionError ? (
          <div className="mt-4 rounded-2xl border border-[#ef4444]/30 bg-[#ef4444]/10 px-6 py-3 text-base text-[#fecaca]">
            {actionError}
          </div>
        ) : null}

        {bootstrap.data.latest_build ? (
          <div className="mt-4 rounded-2xl border border-[#38bdf8]/20 bg-[#0B1120]/80 px-6 py-3">
            <div className="flex flex-wrap items-center justify-between gap-3">
              <div>
                <div className="text-base font-semibold text-white">
                  Build gần nhất: {formatBootstrapStateLabelVN(bootstrap.data.latest_build.status)}
                </div>
                <div className="mt-1 text-base text-[#94a3b8]">
                  {bootstrap.data.latest_build.summary || `Build job ${bootstrap.data.latest_build.build_job_id}`}
                </div>
                {bootstrap.data.latest_build.details ? (
                  <div className="mt-1 text-sm text-[#cbd5e1]">{bootstrap.data.latest_build.details}</div>
                ) : null}
              </div>
              <div className="text-sm text-[#64748b]">{bootstrap.data.latest_build.build_job_id}</div>
            </div>
          </div>
        ) : null}
      </SectionCard>

      <SectionCard
        title="Tóm tắt nhanh"
        description="Những gì bạn cần nhìn thấy đầu tiên: website, deploy gần nhất, số dịch vụ và trạng thái chạy."
      >
        <div className="grid gap-4 lg:grid-cols-4">
          <QuickSummaryCard
            icon={<ExternalLink className="size-5 text-[#38bdf8]" />}
            label="Website"
            value={primaryPublicURL ? 'Đã có' : publicURLDisplay.label || 'Chưa có'}
            hint={primaryPublicURL || publicURLDisplay.message}
          />
          <QuickSummaryCard
            icon={<Rocket className="size-5 text-[#38bdf8]" />}
            label="Deploy gần nhất"
            value={latestDeployment ? `r${latestDeployment.revision}` : 'Chưa có'}
            hint={latestDeploymentLabel}
          />
          <QuickSummaryCard
            icon={<Boxes className="size-5 text-[#38bdf8]" />}
            label="Dịch vụ"
            value={String(serviceItems.length)}
            hint={`${serviceItems.filter((service) => service.public).length} public · ${serviceItems.filter((service) => service.source_type === 'internal').length} nội bộ`}
          />
          <QuickSummaryCard
            icon={<Server className="size-5 text-[#38bdf8]" />}
            label="Trạng thái chạy"
            value={liveServices > 0 ? 'Đang chạy' : 'Chưa chạy'}
            hint={runtimeSummary}
          />
        </div>

        <div className="mt-5 grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
          <QuickLink href={`/projects/${projectId}/repo-link`} icon={<FileCode2 className="size-4" />} label="Mở mã nguồn" />
          <QuickLink href={`/projects/${projectId}/services`} icon={<Boxes className="size-4" />} label="Mở dịch vụ" />
          <QuickLink href={`/projects/${projectId}/deployments`} icon={<Rocket className="size-4" />} label="Mở triển khai" />
          <QuickLink href={`/projects/${projectId}/observability`} icon={<Logs className="size-4" />} label="Mở nhật ký" />
        </div>
      </SectionCard>

      <details className="rounded-[28px] border border-[#1e293b] bg-[radial-gradient(circle_at_top_left,_rgba(14,165,233,0.08),_transparent_32%),linear-gradient(180deg,rgba(15,23,42,0.88),rgba(2,6,23,0.92))] p-6 shadow-[0_24px_80px_rgba(2,6,23,0.4)]">
        <summary className="cursor-pointer list-none text-lg font-black tracking-tight text-white">
          Chi tiết thiết lập và thông tin nâng cao
        </summary>
        <div className="mt-5 grid gap-6 xl:grid-cols-[0.85fr_1.15fr]">
          <div className="rounded-2xl border border-[#1e293b] bg-[#020617]/60 p-6">
            <div className="text-base font-semibold text-white">Thông tin nâng cao</div>
            <div className="mt-4 grid gap-4 sm:grid-cols-2">
              <AdvancedField label="Repo đang nối" value={repoLink.data?.repo_full_name || 'Chưa kết nối'} />
              <AdvancedField label="Nhánh theo dõi" value={repoLink.data?.tracked_branch || 'Chưa có'} />
              <AdvancedField label="Đồng bộ runtime" value={formatBootstrapStateLabelVN(bootstrap.data.runtime_inventory.sync_state || 'missing')} />
              <AdvancedField label="Node sẵn sàng" value={`${readyNodes}/${runtimeNodes.length || 0}`} />
            </div>
          </div>

          <ProjectThreeStepWizard projectId={projectId} compact showSummary={false} />
        </div>
      </details>

      {repoItems.length > 0 ? (
        <ProjectRepoLinkModal
          projectId={projectId}
          repos={repoItems}
          open={showRepoModal}
          onClose={() => setShowRepoModal(false)}
        />
      ) : null}

      <ProjectConnectInfraModal
        projectId={projectId}
        open={showInfraModal}
        onClose={() => setShowInfraModal(false)}
      />
    </div>
  );
}

function PrimaryActionButton({
  action,
  isPending,
  repoOptionsAvailable,
  onClick,
}: {
  action: ReturnType<typeof resolveProjectNextAction>;
  isPending: boolean;
  repoOptionsAvailable: boolean;
  onClick: () => void;
}) {
  if (action.kind === 'open') {
    return (
      <a
        href={action.href}
        target="_blank"
        rel="noreferrer"
        className="rounded-xl bg-[#0EA5E9] px-6 py-2 text-base font-semibold text-[#020617] transition-opacity hover:opacity-90"
      >
        {action.label}
      </a>
    );
  }

  if (action.kind === 'logs') {
    return (
      <Link
        href={action.href}
        className="rounded-xl bg-[#0EA5E9] px-6 py-2 text-base font-semibold text-[#020617] transition-opacity hover:opacity-90"
      >
        {action.label}
      </Link>
    );
  }

  if (action.kind === 'services') {
    return (
      <Link
        href={action.href}
        className="rounded-xl bg-[#0EA5E9] px-6 py-2 text-base font-semibold text-[#020617] transition-opacity hover:opacity-90"
      >
        {action.label}
      </Link>
    );
  }

  if (action.kind === 'repo' && !repoOptionsAvailable) {
    return (
      <Link
        href="/integrations/github"
        className="rounded-xl bg-[#0EA5E9] px-6 py-2 text-base font-semibold text-[#020617] transition-opacity hover:opacity-90"
      >
        Mở GitHub App
      </Link>
    );
  }

  return (
    <button
      type="button"
      onClick={onClick}
      disabled={isPending}
      className="rounded-xl bg-[#0EA5E9] px-6 py-2 text-base font-semibold text-[#020617] transition-opacity hover:opacity-90 disabled:cursor-not-allowed disabled:opacity-60"
    >
      {isPending && action.kind === 'deploy' ? 'Đang triển khai...' : action.label}
    </button>
  );
}

function QuickSummaryCard({
  icon,
  label,
  value,
  hint,
}: {
  icon: ReactNode;
  label: string;
  value: string;
  hint: string;
}) {
  return (
    <div className="rounded-2xl border border-[#1e293b] bg-[#020617]/70 p-6">
      <div className="mb-3 flex items-center gap-3">
        <div className="rounded-xl border border-[#1e293b] bg-[#0B1120]/80 p-2">{icon}</div>
        <div className="text-sm font-semibold uppercase tracking-[0.12em] text-[#64748b]">{label}</div>
      </div>
      <div className="text-4xl font-bold tracking-tight text-white">{value}</div>
      <div className="mt-2 break-all text-base text-[#94a3b8]">{hint}</div>
    </div>
  );
}

function QuickLink({ href, icon, label }: { href: string; icon: ReactNode; label: string }) {
  return (
    <Link
      href={href}
      className="flex items-center gap-2 rounded-2xl border border-[#1e293b] bg-[#0B1120]/60 px-6 py-3 text-base font-semibold text-[#e2e8f0] transition-colors hover:bg-[#111827]"
    >
      <span className="text-[#38bdf8]">{icon}</span>
      {label}
    </Link>
  );
}

function AdvancedField({ label, value }: { label: string; value: string }) {
  return (
    <div className="rounded-2xl border border-[#1e293b] bg-[#0B1120]/50 p-6">
      <div className="text-sm font-semibold uppercase tracking-[0.12em] text-[#64748b]">{label}</div>
      <div className="mt-2 break-all text-base font-semibold text-[#e2e8f0]">{value}</div>
    </div>
  );
}

function stateColorClass(state: string) {
  if (state === 'healthy' || state === 'ready' || state === 'running' || state === 'promoted') {
    return 'text-[#10B981]';
  }
  if (state === 'error' || state === 'failed') {
    return 'text-[#F87171]';
  }
  return 'text-[#38bdf8]';
}
