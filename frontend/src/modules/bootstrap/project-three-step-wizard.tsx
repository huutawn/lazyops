'use client';

import { useMemo, useState } from 'react';
import Link from 'next/link';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { apiFetch } from '@/lib/api/api-client';
import { ErrorState } from '@/components/primitives/error-state';
import { LoadingBlock } from '@/components/primitives/loading';
import { SectionCard } from '@/components/primitives/section-card';
import { StatusBadge, type StatusBadgeProps } from '@/components/primitives/status-badge';
import { Modal } from '@/components/primitives/modal';
import { FormButton, FormField, FormInput } from '@/components/forms/form-fields';
import { bootstrapStatusQueryKey, useAutoBootstrapProject, useConnectProjectInfraSSH, useOneClickDeploy, useProjectBootstrapStatus } from '@/modules/bootstrap/bootstrap-hooks';
import type { BootstrapOneClickDeployResult, BootstrapPipelineEvent, BootstrapStep, BootstrapStepAction } from '@/modules/bootstrap/bootstrap-types';
import { cn } from '@/lib/utils';
import { getProjectDeployment } from '@/modules/deployments/deployment-api';
import type { DeploymentDetail, DeploymentTimelineEvent } from '@/modules/deployments/deployment-types';
import { useSession } from '@/lib/auth/auth-hooks';

type ProjectThreeStepWizardProps = {
  projectId: string;
  compact?: boolean;
};

const STEP_ORDER = ['connect_code', 'connect_infra', 'deploy'] as const;

const STEP_TITLE: Record<string, string> = {
  connect_code: 'Kết nối mã nguồn',
  connect_infra: 'Kết nối máy chủ',
  deploy: 'Triển khai',
};

const STEP_NUMBER: Record<string, string> = {
  connect_code: '1',
  connect_infra: '2',
  deploy: '3',
};

const STEP_BADGE: Record<string, StatusBadgeProps['variant']> = {
  healthy: 'success',
  ready: 'success',
  linked: 'info',
  deploying: 'warning',
  installing: 'warning',
  degraded: 'warning',
  blocked: 'neutral',
  missing: 'neutral',
  error: 'danger',
  rolled_back: 'danger',
};

const OVERALL_BADGE: Record<string, StatusBadgeProps['variant']> = {
  running: 'success',
  ready_to_deploy: 'info',
  deploying: 'warning',
  partially_ready: 'warning',
  not_ready: 'neutral',
  attention_required: 'danger',
};

const TIMELINE_BADGE: Record<string, StatusBadgeProps['variant']> = {
  completed: 'success',
  success: 'success',
  pending: 'warning',
  running: 'warning',
  deploying: 'warning',
  failed: 'danger',
  error: 'danger',
  rolled_back: 'danger',
  started: 'info',
  queued: 'neutral',
  promoted: 'success',
};

function formatStateLabel(value: string): string {
  return value.replace(/_/g, ' ').replace(/\b\w/g, (match) => match.toUpperCase());
}

function formatStateLabelVN(value: string): string {
  const normalized = value.toLowerCase();
  const map: Record<string, string> = {
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
  };
  if (map[normalized]) {
    return map[normalized];
  }
  return formatStateLabel(value);
}

function translatedActionLabel(action: BootstrapStepAction): string {
  const mapByID: Record<string, string> = {
    reconnect_github: 'Kết nối GitHub',
    add_server: 'Kết nối máy chủ',
    deploy_now: 'Triển khai ngay',
    view_deployments: 'Xem lịch sử triển khai',
  };
  return mapByID[action.id] ?? action.label;
}

function normalizeActionEndpoint(endpoint: string): string {
  if (endpoint.startsWith('/api/v1/')) {
    return endpoint.slice('/api/v1'.length);
  }
  if (endpoint === '/api/v1') {
    return '/';
  }
  return endpoint;
}

export function ProjectThreeStepWizard({ projectId, compact = false }: ProjectThreeStepWizardProps) {
  const queryClient = useQueryClient();
  const { data: session } = useSession();
  const { data, isLoading, isError, error, refetch } = useProjectBootstrapStatus(projectId);
  const autoBootstrap = useAutoBootstrapProject(projectId);
  const connectInfra = useConnectProjectInfraSSH(projectId);
  const oneClickDeploy = useOneClickDeploy(projectId);
  const [actionError, setActionError] = useState<string | null>(null);
  const [runningActionId, setRunningActionId] = useState<string | null>(null);
  const [latestOneClick, setLatestOneClick] = useState<BootstrapOneClickDeployResult | null>(null);
  const [activeDeploymentId, setActiveDeploymentId] = useState<string | null>(null);
  const [showConnectInfraModal, setShowConnectInfraModal] = useState(false);
  const [infraForm, setInfraForm] = useState({
    instance_name: '',
    public_ip: '',
    private_ip: '',
    ssh_host: '',
    ssh_port: '22',
    ssh_username: 'root',
    ssh_password: '',
    ssh_private_key: '',
    ssh_host_key_fingerprint: '',
  });
  const [showInfraAdvanced, setShowInfraAdvanced] = useState(false);
  const [infraFormError, setInfraFormError] = useState<string | null>(null);
  const isAdmin = session?.role === 'admin';

  const deploymentDetail = useQuery({
    queryKey: ['one-click-deployment-detail', projectId, activeDeploymentId],
    queryFn: async (): Promise<DeploymentDetail> => {
      if (!activeDeploymentId) {
        throw new Error('Missing deployment id');
      }
      const result = await getProjectDeployment(projectId, activeDeploymentId);
      if (result.error) {
        throw new Error(result.error.message);
      }
      if (!result.data) {
        throw new Error('Deployment detail missing');
      }
      return result.data;
    },
    enabled: !!activeDeploymentId,
    refetchInterval: 5000,
    staleTime: 0,
  });

  const orderedSteps = useMemo(() => {
    if (!data?.steps) {
      return [];
    }
    return [...data.steps].sort((a, b) => STEP_ORDER.indexOf(a.id as (typeof STEP_ORDER)[number]) - STEP_ORDER.indexOf(b.id as (typeof STEP_ORDER)[number]));
  }, [data?.steps]);

  const stepById = useMemo(() => {
    const map = new Map<string, BootstrapStep>();
    orderedSteps.forEach((step) => map.set(step.id, step));
    return map;
  }, [orderedSteps]);

  if (isLoading) {
    return (
      <SectionCard title="Thiết lập 3 bước" description="Đang kiểm tra trạng thái dự án.">
        <LoadingBlock label="Đang tải trạng thái..." className="py-8" />
      </SectionCard>
    );
  }

  if (isError || !data) {
    return (
      <ErrorState
        title="Không thể tải trạng thái thiết lập"
        message={error instanceof Error ? error.message : 'Không thể lấy trạng thái bootstrap.'}
        action={(
          <button
            type="button"
            className="rounded-lg bg-primary px-4 py-2 text-sm font-semibold text-lazyops-bg transition-colors hover:bg-primary/90"
            onClick={() => {
              void refetch();
            }}
          >
            Thử lại
          </button>
        )}
      />
    );
  }

  const code = stepById.get('connect_code');
  const infra = stepById.get('connect_infra');
  const deploy = stepById.get('deploy');

  const statusCards = [
    { title: 'Mã nguồn', value: code?.state ?? 'missing', summary: code?.summary ?? 'Chưa kết nối GitHub' },
    { title: 'Máy chủ', value: infra?.state ?? 'missing', summary: infra?.summary ?? 'Chưa kết nối máy chủ' },
    { title: 'Triển khai', value: deploy?.state ?? 'blocked', summary: deploy?.summary ?? 'Chưa thể triển khai' },
  ];

  const runAction = async (action: BootstrapStepAction) => {
    if (!action.endpoint) {
      return;
    }

    setRunningActionId(action.id);
    setActionError(null);
    try {
      const normalizedEndpoint = normalizeActionEndpoint(action.endpoint);
      if (normalizedEndpoint.endsWith('/deploy/one-click')) {
        const deployResult = await oneClickDeploy.mutateAsync({});
        setLatestOneClick(deployResult);
        setActiveDeploymentId(deployResult.deployment_id || null);
        await queryClient.invalidateQueries({ queryKey: bootstrapStatusQueryKey(projectId) });
        return;
      }

      const result = await apiFetch<unknown>(normalizedEndpoint, {
        method: (action.method || 'POST').toUpperCase(),
      });
      if (result.error) {
        throw new Error(result.error.message);
      }
      await queryClient.invalidateQueries({ queryKey: bootstrapStatusQueryKey(projectId) });
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Action failed';
      setActionError(message);
    } finally {
      setRunningActionId(null);
    }
  };

  const onConnectInfraSubmit = async () => {
    setInfraFormError(null);
    if (!infraForm.ssh_host.trim()) {
      setInfraFormError('Vui lòng nhập địa chỉ SSH host.');
      return;
    }
    if (!infraForm.ssh_username.trim()) {
      setInfraFormError('Vui lòng nhập SSH username.');
      return;
    }
    if (!infraForm.ssh_password.trim() && !infraForm.ssh_private_key.trim()) {
      setInfraFormError('Vui lòng nhập mật khẩu hoặc private key.');
      return;
    }

    try {
      await connectInfra.mutateAsync({
        instance_name: infraForm.instance_name.trim() || undefined,
        public_ip: infraForm.public_ip.trim() || undefined,
        private_ip: infraForm.private_ip.trim() || undefined,
        ssh_host: infraForm.ssh_host.trim(),
        ssh_port: Number.parseInt(infraForm.ssh_port, 10) || 22,
        ssh_username: infraForm.ssh_username.trim(),
        ssh_password: infraForm.ssh_password || undefined,
        ssh_private_key: infraForm.ssh_private_key || undefined,
        ssh_host_key_fingerprint: infraForm.ssh_host_key_fingerprint.trim() || undefined,
        control_plane_url: typeof window !== 'undefined' ? window.location.origin : undefined,
      });
      setShowConnectInfraModal(false);
      setInfraForm({
        instance_name: '',
        public_ip: '',
        private_ip: '',
        ssh_host: '',
        ssh_port: '22',
        ssh_username: 'root',
        ssh_password: '',
        ssh_private_key: '',
        ssh_host_key_fingerprint: '',
      });
      setShowInfraAdvanced(false);
      await queryClient.invalidateQueries({ queryKey: bootstrapStatusQueryKey(projectId) });
    } catch (err) {
      setInfraFormError(err instanceof Error ? err.message : 'Kết nối SSH thất bại');
    }
  };

  const pipelineEvents = latestOneClick?.timeline ?? [];
  const runtimeEvents = deploymentDetail.data?.timeline ?? [];

  return (
    <div className="flex flex-col gap-6">
      <div className="rounded-2xl border border-[#1e293b] bg-[#0F172A] p-6 shadow-sm">
        <div className="flex items-start justify-between mb-6">
          <div>
            <h2 className="text-[17px] font-bold text-white">Thiết lập 3 bước</h2>
            <p className="text-[14px] text-[#94a3b8] mt-1">Kết nối GitHub, kết nối máy chủ, rồi triển khai. LazyOps tự xử lý phần kỹ thuật.</p>
          </div>
          <button
            type="button"
            className="rounded-lg border border-[#334155] bg-[#0F172A] px-4 py-2 text-[13px] font-semibold text-white transition-colors hover:bg-[#1e293b] disabled:opacity-60"
            onClick={() => {
              void autoBootstrap.mutateAsync({});
            }}
            disabled={autoBootstrap.isPending}
          >
            {autoBootstrap.isPending ? 'Đang tự sửa...' : 'Tự sửa thiết lập'}
          </button>
        </div>

        <div className={cn('grid gap-4', compact ? 'grid-cols-1' : 'sm:grid-cols-3')}>
          {statusCards.map((item) => (
            <div key={item.title} className="rounded-xl border border-[#1e293b] bg-[#0B1120] p-4 flex flex-col justify-between min-h-[100px]">
              <div className="flex items-start justify-between">
                <span className="text-[14px] font-semibold text-[#94a3b8]">{item.title}</span>
                <span className={cn(
                  "text-[12px] font-medium",
                  item.value === 'healthy' || item.value === 'ready' || item.value === 'promoted' ? "text-[#10B981]" :
                  item.value === 'error' || item.value === 'failed' ? "text-[#EF4444]" :
                  "text-[#0EA5E9]"
                )}>
                  {formatStateLabelVN(item.value)}
                </span>
              </div>
              <p className="text-[13px] text-white mt-2 leading-snug line-clamp-2">{item.summary}</p>
            </div>
          ))}
        </div>

        <div className="mt-6 pt-6 border-t border-[#1e293b] flex flex-wrap items-center gap-6">
          <div className="flex items-center gap-2">
            <span className="text-[13px] text-[#64748b]">Tổng quan:</span>
            <span className={cn(
              "text-[13px] font-medium",
              data.overall_state === 'running' ? "text-[#10B981]" : "text-[#EF4444]"
            )}>
              {formatStateLabelVN(data.overall_state)}
            </span>
          </div>
          
          <div className="flex items-center gap-2">
            <span className="text-[13px] text-[#64748b]">Chế độ:</span>
            <span className="text-[13px] text-white font-medium">{data.auto_mode.selected_mode}</span>
          </div>

          <div className="text-[13px] text-[#64748b]">
            {data.auto_mode.mode_reason_human}
          </div>

          <Link
            href={`/projects/${projectId}/internal-services`}
            className="ml-auto rounded-lg border border-[#334155] bg-[#1e293b] px-4 py-1.5 text-[13px] font-bold text-white transition-colors hover:bg-[#2d3a4f]"
          >
            Dịch vụ nội bộ
          </Link>
        </div>
      </div>

      <div className="grid gap-6 px-2">
        {orderedSteps.map((step) => (
          <div key={step.id} className="animate-in fade-in slide-in-from-bottom-2 duration-500">
            <div className="flex items-center gap-3 mb-4">
              <h3 className="text-xl font-bold text-white">
                {STEP_NUMBER[step.id] ?? '-'} · {STEP_TITLE[step.id] ?? step.id}
              </h3>
            </div>
            
            <div className="rounded-2xl border border-[#1e293b] bg-[#0F172A] p-6">
              <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4">
                <div>
                  <p className="text-[15px] font-medium text-white mb-2">{step.summary}</p>
                  <div className="flex items-center gap-3">
                    <span className={cn(
                      "text-[13px] font-bold",
                      step.state === 'healthy' || step.state === 'ready' || step.state === 'promoted' ? "text-[#10B981]" :
                      step.state === 'error' || step.state === 'failed' ? "text-[#EF4444]" :
                      "text-[#0EA5E9]"
                    )}>
                      {formatStateLabelVN(step.state)}
                    </span>
                  </div>
                </div>

                <div className="flex flex-wrap items-center gap-3">
                  {step.id === 'connect_infra' && !isAdmin ? (
                    <button
                      type="button"
                      className="rounded-lg bg-[#0EA5E9] px-6 py-2.5 text-[14px] font-bold text-white transition-all hover:bg-[#0284c7] shadow-lg shadow-[#0ea5e9]/10 disabled:opacity-60"
                      onClick={() => setShowConnectInfraModal(true)}
                      disabled={connectInfra.isPending}
                    >
                      {connectInfra.isPending ? 'Đang kết nối...' : 'Kết nối máy chủ'}
                    </button>
                  ) : null}

                  {step.actions.map((action) => {
                    if (step.id === 'connect_infra' && !isAdmin && action.id === 'add_server') {
                      return null;
                    }
                    if ((action.kind === 'link' || action.kind === 'screen') && action.href) {
                      return (
                        <Link
                          key={action.id}
                          href={action.href}
                          className="rounded-lg border border-[#334155] bg-[#1e293b] px-6 py-2.5 text-[14px] font-bold text-white transition-colors hover:bg-[#2d3a4f]"
                        >
                          {translatedActionLabel(action)}
                        </Link>
                      );
                    }

                    if (action.kind === 'api' && action.endpoint) {
                      return (
                        <button
                          key={action.id}
                          type="button"
                          className="rounded-lg bg-[#0EA5E9] px-6 py-2.5 text-[14px] font-bold text-white transition-all hover:bg-[#0284c7] shadow-lg shadow-[#0ea5e9]/10 disabled:opacity-60"
                          onClick={() => {
                            void runAction(action);
                          }}
                          disabled={runningActionId !== null}
                        >
                          {runningActionId === action.id ? 'Đang chạy...' : translatedActionLabel(action)}
                        </button>
                      );
                    }

                    return null;
                  })}
                </div>
              </div>
            </div>
          </div>
        ))}
      </div>

      {(latestOneClick || deploymentDetail.data) ? (
        <SectionCard
          title="Tiến trình triển khai"
          description="Theo dõi tiến trình triển khai theo thời gian thực."
          actions={
            activeDeploymentId ? (
              <a
                href={`/projects/${projectId}/deployments/${activeDeploymentId}`}
                className="rounded-lg border border-lazyops-border px-3 py-1.5 text-xs font-semibold text-lazyops-text transition-colors hover:bg-lazyops-border/10"
              >
                Xem chi tiết
              </a>
            ) : null
          }
        >
          {latestOneClick ? (
            <div className="mb-3 flex flex-wrap items-center gap-2">
              <StatusBadge
                label={`Rollout: ${formatStateLabelVN(latestOneClick.rollout_status)}`}
                variant={TIMELINE_BADGE[latestOneClick.rollout_status] ?? 'neutral'}
                size="sm"
              />
              {latestOneClick.rollout_reason ? (
                <span className="text-xs text-lazyops-muted">{latestOneClick.rollout_reason}</span>
              ) : null}
            </div>
          ) : null}

          <div className="flex flex-col gap-2">
            {pipelineEvents.map((event) => (
              <TimelineRow
                key={`pipeline-${event.id}-${event.timestamp}`}
                label={event.label}
                description={event.message}
                state={event.state}
                timestamp={event.timestamp}
              />
            ))}
            {runtimeEvents.map((event, index) => (
              <TimelineRow
                key={`runtime-${index}-${event.timestamp}-${event.state}`}
                label={event.label}
                description={event.description}
                state={event.state}
                timestamp={event.timestamp}
              />
            ))}
            {deploymentDetail.isFetching ? (
              <p className="text-[11px] text-lazyops-muted">Đang làm mới tiến trình...</p>
            ) : null}
          </div>
        </SectionCard>
      ) : null}

      {actionError ? (
        <div className="rounded-lg border border-health-unhealthy/30 bg-health-unhealthy/10 px-3 py-2 text-xs text-health-unhealthy">
          {actionError}
        </div>
      ) : null}

      <Modal
        open={showConnectInfraModal}
        onClose={() => setShowConnectInfraModal(false)}
        title="Kết nối máy chủ qua SSH"
        size="lg"
      >
        <div className="flex flex-col gap-4">
          <p className="text-sm text-lazyops-muted">
            Nhập thông tin SSH, LazyOps sẽ tự cài agent và tự gắn máy chủ vào dự án. Bạn không cần biết Private IP.
          </p>

          <FormField label="Tên máy chủ (tuỳ chọn)">
            <FormInput
              type="text"
              placeholder="prod-app-01"
              value={infraForm.instance_name}
              onChange={(event) => setInfraForm((prev) => ({ ...prev, instance_name: event.target.value }))}
            />
          </FormField>

          <div className="grid gap-4 sm:grid-cols-2">
            <FormField label="SSH host">
              <FormInput
                type="text"
                placeholder="203.0.113.10"
                value={infraForm.ssh_host}
                onChange={(event) => setInfraForm((prev) => ({ ...prev, ssh_host: event.target.value }))}
              />
            </FormField>
            <FormField label="SSH port">
              <FormInput
                type="number"
                placeholder="22"
                value={infraForm.ssh_port}
                onChange={(event) => setInfraForm((prev) => ({ ...prev, ssh_port: event.target.value }))}
              />
            </FormField>
          </div>

          <button
            type="button"
            className="w-fit rounded-lg border border-lazyops-border px-3 py-1.5 text-xs font-semibold text-lazyops-muted transition-colors hover:bg-lazyops-border/10"
            onClick={() => setShowInfraAdvanced((prev) => !prev)}
          >
            {showInfraAdvanced ? 'Ẩn cấu hình nâng cao' : 'Mở cấu hình nâng cao'}
          </button>

          {showInfraAdvanced ? (
            <div className="grid gap-4 sm:grid-cols-2">
              <FormField label="Public IP (tuỳ chọn)">
                <FormInput
                  type="text"
                  placeholder="203.0.113.10"
                  value={infraForm.public_ip}
                  onChange={(event) => setInfraForm((prev) => ({ ...prev, public_ip: event.target.value }))}
                />
              </FormField>
              <FormField label="Private IP (tuỳ chọn)">
                <FormInput
                  type="text"
                  placeholder="10.0.1.10"
                  value={infraForm.private_ip}
                  onChange={(event) => setInfraForm((prev) => ({ ...prev, private_ip: event.target.value }))}
                />
              </FormField>
            </div>
          ) : null}

          <div className="grid gap-4 sm:grid-cols-2">
            <FormField label="SSH username">
              <FormInput
                type="text"
                placeholder="root"
                value={infraForm.ssh_username}
                onChange={(event) => setInfraForm((prev) => ({ ...prev, ssh_username: event.target.value }))}
              />
            </FormField>
            <FormField label="Host key fingerprint (tuỳ chọn)">
              <FormInput
                type="text"
                placeholder="SHA256:..."
                value={infraForm.ssh_host_key_fingerprint}
                onChange={(event) => setInfraForm((prev) => ({ ...prev, ssh_host_key_fingerprint: event.target.value }))}
              />
            </FormField>
          </div>

          <FormField label="Mật khẩu SSH (hoặc dùng private key)">
            <FormInput
              type="password"
              placeholder="••••••••"
              value={infraForm.ssh_password}
              onChange={(event) => setInfraForm((prev) => ({ ...prev, ssh_password: event.target.value }))}
            />
          </FormField>

          <FormField label="SSH private key (tuỳ chọn)">
            <textarea
              className="min-h-24 w-full rounded-lg border border-lazyops-border bg-lazyops-bg-accent/60 px-3 py-2 text-sm text-lazyops-text outline-none transition-colors placeholder:text-lazyops-muted/60 focus:border-primary/60 focus:ring-1 focus:ring-primary/30"
              placeholder="-----BEGIN OPENSSH PRIVATE KEY----- ..."
              value={infraForm.ssh_private_key}
              onChange={(event) => setInfraForm((prev) => ({ ...prev, ssh_private_key: event.target.value }))}
            />
          </FormField>

          {infraFormError ? (
            <div className="rounded-lg border border-health-unhealthy/30 bg-health-unhealthy/10 px-3 py-2 text-xs text-health-unhealthy">
              {infraFormError}
            </div>
          ) : null}

          <FormButton
            type="button"
            loading={connectInfra.isPending}
            onClick={() => {
              void onConnectInfraSubmit();
            }}
          >
            Kết nối và cài agent
          </FormButton>
        </div>
      </Modal>
    </div>
  );
}

function TimelineRow({
  label,
  description,
  state,
  timestamp,
}: {
  label: string;
  description: string;
  state: string;
  timestamp: string;
}) {
  return (
    <div className="rounded-lg border border-lazyops-border/60 bg-lazyops-bg-accent/20 px-3 py-2">
      <div className="mb-1 flex flex-wrap items-center justify-between gap-2">
        <span className="text-xs font-medium text-lazyops-text">{label}</span>
        <StatusBadge
          label={formatStateLabelVN(state)}
          variant={TIMELINE_BADGE[state] ?? 'neutral'}
          size="sm"
        />
      </div>
      <p className="text-xs text-lazyops-muted">{description}</p>
      <p className="mt-1 text-[11px] text-lazyops-muted/80">
        {new Date(timestamp).toLocaleString()}
      </p>
    </div>
  );
}
