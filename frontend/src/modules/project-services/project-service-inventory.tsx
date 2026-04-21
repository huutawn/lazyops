'use client';

import { useMemo, useState, type ReactNode } from 'react';
import Link from 'next/link';
import { Boxes, Database, Globe, Lock, Network, Plus, Server } from 'lucide-react';
import { Drawer } from '@/components/primitives/drawer';
import { ErrorState } from '@/components/primitives/error-state';
import { LoadingBlock } from '@/components/primitives/loading';
import { SectionCard } from '@/components/primitives/section-card';
import { StatusBadge } from '@/components/primitives/status-badge';
import {
  useConfigureProjectServices,
  useProjectPlacementNodes,
  useProjectServiceAction,
  useProjectServices,
} from '@/modules/project-services/project-service-hooks';
import {
  POSTGRES_CONNECTION_TEMPLATE_SLOTS,
  formatPostgresConnectionTemplatePreview,
  normalizePostgresConnectionTemplate,
} from '@/modules/project-services/postgres-connection-template';
import { INTERNAL_SERVICE_OPTIONS, type InternalServiceKind } from '@/modules/internal-services/internal-service-types';
import { useProjectRuntime } from '@/modules/project-runtime/project-runtime-hooks';
import type { ProjectRuntimeService } from '@/modules/project-runtime/project-runtime-types';
import type {
  ConfigureProjectServicesRequest,
  PlacementNode,
  ProjectServiceAction,
  ProjectServiceActionResponse,
  ProjectService,
  ProjectServiceDraft,
} from '@/modules/project-services/project-service-types';

type ProjectServiceInventoryProps = {
  projectId: string;
  title?: string;
  description?: string;
  compact?: boolean;
  sourceFilter?: 'all' | 'repo' | 'internal';
};

type DrawerMode =
  | 'closed'
  | 'choose'
  | 'create-repo'
  | 'create-internal'
  | 'edit-repo'
  | 'edit-internal'
  | 'legacy-internal';

type RepoFormState = {
  name: string;
  path: string;
  kind: RepoServiceKind;
  public: boolean;
  placement_mode: string;
  placement_node_id: string;
  connection_target_service: string;
};

type InternalFormState = {
  kind: InternalServiceKind;
  service_name: string;
  connection_template: Record<string, string>;
};

type RepoServiceKind = 'app' | 'api' | 'web' | 'worker';

const REPO_SERVICE_KIND_OPTIONS: Array<{ value: RepoServiceKind; label: string; description: string }> = [
  { value: 'app', label: 'App tổng quát', description: 'Giữ lựa chọn này nếu bạn chưa chắc. Detector/build sẽ tự suy ra thêm.' },
  { value: 'api', label: 'API / backend', description: 'Service HTTP cho backend hoặc API server.' },
  { value: 'web', label: 'Frontend web', description: 'Service giao diện web public hoặc admin panel.' },
  { value: 'worker', label: 'Worker nền', description: 'Job runner, queue consumer hoặc cron worker.' },
];

export function ProjectServiceInventory({
  projectId,
  title = 'Dịch vụ',
  description = 'Mỗi service là một phần chạy độc lập của project. Hãy cấu hình đúng source, database và cách truy cập.',
  compact = false,
  sourceFilter = 'all',
}: ProjectServiceInventoryProps) {
  const services = useProjectServices(projectId);
  const placementNodes = useProjectPlacementNodes(projectId);
  const runtime = useProjectRuntime(projectId);
  const configureServices = useConfigureProjectServices(projectId);
  const serviceAction = useProjectServiceAction(projectId);
  const [drawerMode, setDrawerMode] = useState<DrawerMode>('closed');
  const [selectedService, setSelectedService] = useState<ProjectService | null>(null);
  const [repoForm, setRepoForm] = useState<RepoFormState>(defaultRepoForm());
  const [internalForm, setInternalForm] = useState<InternalFormState>(defaultInternalForm());
  const [showRepoAdvanced, setShowRepoAdvanced] = useState(false);
  const [repoFormError, setRepoFormError] = useState<string | null>(null);
  const [activeAction, setActiveAction] = useState<{ serviceId: string; action: ProjectServiceAction } | null>(null);
  const [lastActionResult, setLastActionResult] = useState<ProjectServiceActionResponse | null>(null);
  const [lastActionError, setLastActionError] = useState<{ serviceId: string; message: string } | null>(null);
  const items = services.data?.items ?? [];
  const filteredItems = items.filter((item) => {
    if (sourceFilter === 'repo') {
      return item.source_type !== 'internal';
    }
    if (sourceFilter === 'internal') {
      return item.source_type === 'internal';
    }
    return true;
  });
  const internalPostgresTargets = items.filter(
    (item) => item.source_type === 'internal' && item.kind === 'postgres' && !isLegacyInternalService(item),
  );
  const placementNodeItems = placementNodes.data?.items ?? [];
  const readyPlacementNodes = placementNodeItems.filter((item) => item.is_ready);
  const placementNodesByID = useMemo(
    () => new Map(placementNodeItems.map((item) => [item.instance_id, item])),
    [placementNodeItems],
  );
  const placementHint = resolvePlacementHint({
    placementNodesError: placementNodes.error instanceof Error ? placementNodes.error.message : null,
    isLoading: placementNodes.isLoading,
    clusterID: placementNodes.data?.cluster_id,
    readyCount: readyPlacementNodes.length,
  });
  const runtimeByServiceID = useMemo(
    () => new Map((runtime.data?.services ?? []).map((item) => [item.service_id, item])),
    [runtime.data?.services],
  );

  if (services.isLoading) {
    return (
      <SectionCard title={title} description={description}>
        <LoadingBlock label="Dang tai service inventory..." className="py-10" />
      </SectionCard>
    );
  }

  if (services.isError) {
    return (
      <ErrorState
        title="Khong the tai service inventory"
        message={services.error instanceof Error ? services.error.message : 'Khong tai duoc danh sach services.'}
      />
    );
  }

  const openChooseDrawer = () => {
    configureServices.reset();
    setRepoFormError(null);
    setSelectedService(null);
    setRepoForm(defaultRepoForm());
    setInternalForm(defaultInternalForm());
    setShowRepoAdvanced(false);
    setDrawerMode('choose');
  };

  const openEditDrawer = (service: ProjectService) => {
    configureServices.reset();
    setRepoFormError(null);
    setSelectedService(service);
    if (service.source_type === 'internal') {
      setInternalForm(defaultInternalForm(service));
      setShowRepoAdvanced(false);
      setDrawerMode(isLegacyInternalService(service) ? 'legacy-internal' : 'edit-internal');
      return;
    }
    setRepoForm(defaultRepoForm(service));
    setShowRepoAdvanced(service.placement_mode === 'pinned_node');
    setDrawerMode('edit-repo');
  };

  const closeDrawer = () => {
    if (configureServices.isPending) {
      return;
    }
    configureServices.reset();
    setRepoFormError(null);
    setShowRepoAdvanced(false);
    setDrawerMode('closed');
    setSelectedService(null);
  };

  const submitRepoService = async () => {
    if (repoForm.placement_mode === 'pinned_node' && !repoForm.placement_node_id.trim()) {
      setRepoFormError('Chon mot ready node truoc khi luu service pinned_node.');
      return;
    }
    setRepoFormError(null);
    const draft: ProjectServiceDraft = {
      name: repoForm.name.trim(),
      path: repoForm.path.trim(),
      kind: repoForm.kind.trim() || 'app',
      source_type: 'repo',
      public: repoForm.public,
      placement_mode: repoForm.placement_mode || 'shared_cluster',
      placement_node_id:
        repoForm.placement_mode === 'pinned_node' ? repoForm.placement_node_id.trim() : '',
      connection_template_key: repoForm.connection_target_service ? 'postgres.basic' : '',
      connection_target_service: repoForm.connection_target_service.trim(),
      managed_by_lazyops: false,
      start_hint: selectedService?.start_hint || '',
      image_ref: selectedService?.image_ref || '',
      image_digest: selectedService?.image_digest || '',
      target_port: selectedService?.target_port || 0,
      service_port: selectedService?.service_port || 0,
      replicas: selectedService?.replicas || 1,
      env_bundle: selectedService?.env_bundle || {},
      pvc_spec: selectedService?.pvc_spec || {},
      deploy_strategy: selectedService?.deploy_strategy || {},
      healthcheck: selectedService?.healthcheck || {},
    };

    await configureServices.mutateAsync(
      buildCatalogMutation(items, draft, drawerMode === 'edit-repo' ? selectedService?.id : undefined),
    );
    closeDrawer();
  };

  const submitInternalService = async () => {
    const kind = internalForm.kind;
    const serviceName = internalForm.service_name.trim() || defaultInternalServiceName(kind);
    const existingEnv = selectedService?.env_bundle || {};
    const existingPVC = selectedService?.pvc_spec || { size: '5Gi' };
    const defaultPort = defaultInternalServicePort(kind);
    const existingHealthcheck = selectedService?.healthcheck || { protocol: 'tcp', port: defaultPort };
    const draft: ProjectServiceDraft = {
      name: serviceName,
      path: `.lazyops/internal/${kind}/${serviceName}`,
      kind,
      source_type: 'internal',
      public: false,
      runtime_profile: 'internal-db',
      placement_mode: 'shared_cluster',
      connection_template_key: '',
      connection_target_service: '',
      managed_by_lazyops: true,
      start_hint: 'managed-internal-service',
      image_ref: selectedService?.image_ref || defaultInternalServiceImage(kind),
      image_digest: selectedService?.image_digest || '',
      target_port: selectedService?.target_port || defaultPort,
      service_port: selectedService?.service_port || defaultPort,
      replicas: selectedService?.replicas || 1,
      env_bundle: existingEnv,
      pvc_spec: existingPVC,
      connection_template: kind === 'postgres' ? normalizePostgresConnectionTemplate(internalForm.connection_template) : {},
      deploy_strategy: selectedService?.deploy_strategy || {},
      healthcheck: existingHealthcheck,
    };

    await configureServices.mutateAsync(
      buildCatalogMutation(items, draft, drawerMode === 'edit-internal' ? selectedService?.id : undefined),
    );
    closeDrawer();
  };

  const runServiceAction = async (service: ProjectService, action: ProjectServiceAction) => {
    setActiveAction({ serviceId: service.id, action });
    setLastActionResult(null);
    setLastActionError(null);
    try {
      const result = await serviceAction.mutateAsync({ serviceId: service.id, action });
      setLastActionResult(result);
    } catch (error) {
      setLastActionError({
        serviceId: service.id,
        message: error instanceof Error ? error.message : 'Khong the thuc hien service action.',
      });
    } finally {
      setActiveAction(null);
    }
  };

  return (
    <>
      <SectionCard
        title={title}
        description={description}
        actions={
          <>
            <button
              type="button"
              onClick={openChooseDrawer}
              className="inline-flex items-center gap-2 rounded-xl border border-[#0EA5E9] bg-[#0EA5E9]/10 px-6 py-2 text-base font-semibold text-[#38bdf8] transition-colors hover:bg-[#0EA5E9]/20"
            >
              <Plus className="size-4" />
              Them service
            </button>
            <Link
              href={`/projects/${projectId}/deployments`}
              className="rounded-xl border border-[#334155] px-6 py-2 text-base font-semibold text-[#e2e8f0] transition-colors hover:bg-[#111827]"
            >
              Xem deploy
            </Link>
          </>
        }
      >
        {filteredItems.length === 0 ? (
          <div className="rounded-2xl border border-dashed border-[#334155] bg-[#0B1120]/30 p-8 text-base text-[#94a3b8]">
            {sourceFilter === 'internal'
              ? 'Chưa có dịch vụ nội bộ nào trong project này.'
              : sourceFilter === 'repo'
                ? 'Chưa có dịch vụ nào lấy code từ repository.'
                : 'Project này chưa có service nào. Hãy tạo service đầu tiên để bắt đầu deploy.'}
          </div>
        ) : (
          <div className={`grid gap-4 ${compact ? 'xl:grid-cols-2' : 'xl:grid-cols-2 2xl:grid-cols-3'}`}>
            {filteredItems.map((service) => {
              const exposureIcon = service.public ? <Globe className="size-4 text-[#38bdf8]" /> : <Lock className="size-4 text-[#94a3b8]" />;
              const placementLabel = formatPlacementValue(service, placementNodesByID);
              const placementNode = service.placement_node_id
                ? placementNodesByID.get(service.placement_node_id)
                : null;
              const runtimeRecord = runtimeByServiceID.get(service.id);
              const servicePort = service.service_port || service.target_port || 0;
              const actionPending = serviceAction.isPending && activeAction?.serviceId === service.id;
              const actionResult = lastActionResult?.service_id === service.id ? lastActionResult : null;
              const actionError = lastActionError?.serviceId === service.id ? lastActionError.message : null;
              const displayStatus = resolveServiceDisplayStatus(service, runtimeRecord);
              const primaryAction = resolvePrimaryServiceAction(service, runtimeRecord);
              const advancedSummary = buildAdvancedSummary(service, runtimeRecord, placementLabel, servicePort);

              return (
                <article
                  key={service.id}
                  className="rounded-2xl border border-[#1e293b] bg-[#020617]/70 p-6 shadow-[0_20px_60px_rgba(2,6,23,0.35)]"
                >
                  <div className="flex items-start justify-between gap-4">
                    <div className="space-y-2">
                      <div className="flex items-center gap-3">
                        <div className="rounded-xl border border-[#1e293b] bg-[#0F172A] p-2">
                          <Boxes className="size-5 text-[#38bdf8]" />
                        </div>
                        <div>
                          <h3 className="text-xl font-bold tracking-tight text-white">{service.name}</h3>
                          <p className="text-base text-[#94a3b8]">{formatServiceKindLabel(service)}</p>
                        </div>
                      </div>
                      <div className="flex flex-wrap gap-2">
                        <StatusBadge label={displayStatus} variant={resolveServiceStatusVariant(service, runtimeRecord)} size="sm" />
                        <StatusBadge label={service.source_type === 'internal' ? 'Nội bộ' : 'Repository'} variant={service.source_type === 'internal' ? 'warning' : 'info'} size="sm" />
                        <StatusBadge label={service.public ? 'Công khai' : 'Nội bộ'} variant={service.public ? 'success' : 'neutral'} size="sm" />
                      </div>
                    </div>
                    <div className="flex items-center gap-2 rounded-full border border-[#1e293b] bg-[#0B1120]/80 px-3 py-1.5 text-sm font-semibold text-[#cbd5e1]">
                      {exposureIcon}
                      {service.public ? 'Public route' : 'Chỉ nội bộ'}
                    </div>
                  </div>

                  <div className="mt-5 grid gap-4 sm:grid-cols-2">
                    <MetricLine icon={<Boxes className="size-4 text-[#38bdf8]" />} label="Loại service" value={formatShortKind(service)} />
                    <MetricLine icon={<Network className="size-4 text-[#38bdf8]" />} label="Thư mục trong repo" value={service.source_type === 'internal' ? 'Dịch vụ nội bộ' : service.path || 'Chưa có'} />
                    <MetricLine icon={<Globe className="size-4 text-[#38bdf8]" />} label="Truy cập" value={service.public ? 'Có thể truy cập từ Internet' : 'Chỉ dùng nội bộ'} />
                    <MetricLine
                      icon={<Database className="size-4 text-[#38bdf8]" />}
                      label="Database"
                      value={formatDatabaseTarget(service)}
                    />
                  </div>

                  {runtimeRecord?.runtime_reason ? (
                    <div className="mt-4 rounded-2xl border border-[#1e293b] bg-[#0B1120]/60 px-6 py-3 text-base text-[#cbd5e1]">
                      Trạng thái chạy: <span className="font-semibold text-white">{runtimeRecord.runtime_reason}</span>
                    </div>
                  ) : null}

                  <div className="mt-5 flex flex-wrap gap-2">
                    <ActionButton
                      label={primaryAction.label}
                      pending={actionPending && activeAction?.action === 'deploy'}
                      onClick={() => void runServiceAction(service, primaryAction.action)}
                    />
                    <ActionButton
                      label={service.source_type === 'internal' ? 'Chi tiết' : 'Sửa'}
                      pending={false}
                      onClick={() => openEditDrawer(service)}
                      variant="secondary"
                    />
                    <Link
                      href={`/projects/${projectId}/observability?service=${encodeURIComponent(service.name)}`}
                      className="rounded-xl border border-[#334155] px-3 py-2 text-sm font-semibold text-[#e2e8f0] transition-colors hover:bg-[#111827]"
                    >
                      Nhật ký
                    </Link>
                  </div>

                  <details className="mt-4 rounded-2xl border border-[#1e293b] bg-[#0B1120]/40 p-6">
                    <summary className="cursor-pointer list-none text-base font-semibold text-white">
                      Nâng cao
                    </summary>
                    <div className="mt-4 grid gap-4 sm:grid-cols-2">
                      {advancedSummary.map((item) => (
                        <MetricLine
                          key={`${service.id}-${item.label}`}
                          icon={item.icon}
                          label={item.label}
                          value={item.value}
                        />
                      ))}
                    </div>
                    <div className="mt-4 flex flex-wrap gap-2">
                      {service.source_type !== 'internal' ? (
                        <ActionButton
                          label="Rebuild"
                          pending={actionPending && activeAction?.action === 'rebuild'}
                          onClick={() => void runServiceAction(service, 'rebuild')}
                          variant="secondary"
                        />
                      ) : null}
                      <ActionButton
                        label="Restart"
                        pending={actionPending && activeAction?.action === 'restart'}
                        onClick={() => void runServiceAction(service, 'restart')}
                        variant="secondary"
                      />
                    </div>
                    {placementNode ? (
                      <div className="mt-4 rounded-2xl border border-[#1e293b] bg-[#020617]/80 px-6 py-3 text-base text-[#cbd5e1]">
                        Máy chủ đã ghim: <span className="font-semibold text-white">{placementNode.name}</span> · {placementNode.is_ready ? 'Sẵn sàng' : formatPlacementNodeStatus(placementNode.status)}
                      </div>
                    ) : null}
                  </details>

                  {actionResult ? (
                    <div className="mt-3 rounded-2xl border border-[#0EA5E9]/30 bg-[#0EA5E9]/10 px-6 py-3 text-base text-[#bae6fd]">
                      {formatActionResult(actionResult)}
                    </div>
                  ) : null}

                  {actionError ? (
                    <div className="mt-3 rounded-2xl border border-[#ef4444]/30 bg-[#ef4444]/10 px-6 py-3 text-base text-[#fecaca]">
                      {actionError}
                    </div>
                  ) : null}
                </article>
              );
            })}
          </div>
        )}
      </SectionCard>

      <Drawer
        open={drawerMode !== 'closed'}
        onClose={closeDrawer}
        title={resolveDrawerTitle(drawerMode, selectedService)}
        size="lg"
      >
        {drawerMode === 'choose' ? (
          <div className="grid gap-4">
            <ServiceChoiceCard
              title="App từ repository"
              description="Dùng cho web, API, worker hoặc bất kỳ service nào lấy code từ repository của bạn."
              onClick={() => {
                setRepoForm(defaultRepoForm());
                setShowRepoAdvanced(false);
                setDrawerMode('create-repo');
              }}
            />
            <ServiceChoiceCard
              title="Managed internal service"
              description="Chọn PostgreSQL, MySQL, Redis hoặc RabbitMQ từ catalog nội bộ do LazyOps quản lý."
              onClick={() => {
                setInternalForm(defaultInternalForm());
                setShowRepoAdvanced(false);
                setDrawerMode('create-internal');
              }}
            />
          </div>
        ) : null}

        {drawerMode === 'create-repo' || drawerMode === 'edit-repo' ? (
          <form
            className="grid gap-5"
            onSubmit={(event) => {
              event.preventDefault();
              void submitRepoService();
            }}
          >
            <div className="rounded-2xl border border-[#1e293b] bg-[#0B1120]/60 p-6 text-base text-[#cbd5e1]">
              <div className="font-semibold text-white">Service từ repository là gì?</div>
              <p className="mt-2 text-[#94a3b8]">
                Đây là code nằm trong repo GitHub của bạn. Nếu repo chỉ có một app duy nhất ở root thì thư mục là <code className="rounded bg-[#020617] px-1.5 py-0.5 text-[#e2e8f0]">.</code>.
                Nếu là monorepo, hãy điền đường dẫn tương đối như <code className="rounded bg-[#020617] px-1.5 py-0.5 text-[#e2e8f0]">apps/api</code> hoặc <code className="rounded bg-[#020617] px-1.5 py-0.5 text-[#e2e8f0]">apps/web</code>.
              </p>
            </div>
            <FieldLabel label="Tên dịch vụ" help="Tên runtime của service. Nó được dùng cho DNS nội bộ, log, và public subdomain nếu service được mở Internet, ví dụ: api, web, auth, worker.">
              <input
                value={repoForm.name}
                onChange={(event) => setRepoForm((current) => ({ ...current, name: event.target.value }))}
                className={fieldClassName}
                placeholder="api"
                required
              />
            </FieldLabel>
              <FieldLabel
                label="Thư mục trong repo"
                help="Đường dẫn thư mục dùng để build service này, tính từ root repo GitHub. Dùng . nếu app nằm ngay ở root repo."
              >
                <input
                  value={repoForm.path}
                  onChange={(event) => setRepoForm((current) => ({ ...current, path: event.target.value }))}
                className={fieldClassName}
                placeholder="., apps/api, services/worker"
                required
              />
            </FieldLabel>
            <div className="grid gap-5 md:grid-cols-2">
              <FieldLabel label="Loại service" help="Giữ App tổng quát nếu bạn chưa chắc. Hệ thống vẫn sẽ tự detect thêm khi build/deploy.">
                <select
                  value={repoForm.kind}
                  onChange={(event) => setRepoForm((current) => ({ ...current, kind: event.target.value as RepoServiceKind }))}
                  className={fieldClassName}
                >
                  {REPO_SERVICE_KIND_OPTIONS.map((option) => (
                    <option key={option.value} value={option.value}>
                      {option.label}
                    </option>
                  ))}
                </select>
                <p className="text-sm text-[#94a3b8]">
                  {REPO_SERVICE_KIND_OPTIONS.find((option) => option.value === repoForm.kind)?.description}
                </p>
              </FieldLabel>
              <FieldLabel label="Database kết nối" help="Tùy chọn. Chỉ chọn khi service này cần được inject biến môi trường kết nối tới database nội bộ đã tạo trước đó.">
                <select
                  value={repoForm.connection_target_service}
                  onChange={(event) => setRepoForm((current) => ({ ...current, connection_target_service: event.target.value }))}
                  className={fieldClassName}
                >
                  <option value="">Chưa dùng database</option>
                  {internalPostgresTargets.map((service) => (
                    <option key={service.id} value={service.name}>
                      {service.name}
                    </option>
                  ))}
                </select>
              </FieldLabel>
            </div>
            <label className="flex items-center gap-3 rounded-2xl border border-[#1e293b] bg-[#0B1120]/60 px-6 py-3 text-base text-[#cbd5e1]">
              <input
                type="checkbox"
                checked={repoForm.public}
                onChange={(event) => setRepoForm((current) => ({ ...current, public: event.target.checked }))}
              />
              Cho phép truy cập từ Internet
            </label>
            {repoForm.public ? (
              <div className="rounded-2xl border border-[#1e293b] bg-[#0B1120]/60 p-6 text-base text-[#cbd5e1]">
                Service public sẽ được đưa vào shared domain của project dưới <code className="rounded bg-[#020617] px-1.5 py-0.5 text-[#e2e8f0]">lazyops.cloud</code>, ví dụ <code className="rounded bg-[#020617] px-1.5 py-0.5 text-[#e2e8f0]">myapp-ab12.lazyops.cloud/api</code>.
                <div className="mt-2 text-[#94a3b8]">`service.name` vẫn quan trọng để gợi ý route mặc định như <code className="rounded bg-[#020617] px-1.5 py-0.5 text-[#e2e8f0]">/</code>, <code className="rounded bg-[#020617] px-1.5 py-0.5 text-[#e2e8f0]">/api</code> hoặc <code className="rounded bg-[#020617] px-1.5 py-0.5 text-[#e2e8f0]">/ws</code>. `service.path` chỉ là thư mục build trong repo.</div>
              </div>
            ) : null}

            <div className="rounded-2xl border border-[#1e293b] bg-[#0B1120]/50 p-6">
              <button
                type="button"
                className="text-base font-semibold text-white"
                onClick={() => setShowRepoAdvanced((current) => !current)}
              >
                {showRepoAdvanced ? 'Ẩn tùy chọn nâng cao' : 'Mở tùy chọn nâng cao'}
              </button>
              {showRepoAdvanced ? (
                <div className="mt-4 grid gap-5">
                  <FieldLabel label="Vị trí chạy">
                    <select
                      value={repoForm.placement_mode}
                      onChange={(event) => {
                        const nextMode = event.target.value;
                        setRepoFormError(null);
                        setRepoForm((current) => ({
                          ...current,
                          placement_mode: nextMode,
                          placement_node_id: nextMode === 'pinned_node' ? current.placement_node_id : '',
                        }));
                      }}
                      className={fieldClassName}
                    >
                      <option value="shared_cluster">Cụm dùng chung</option>
                      <option
                        value="pinned_node"
                        disabled={readyPlacementNodes.length === 0 && repoForm.placement_mode !== 'pinned_node'}
                      >
                        Ghim vào máy chủ cụ thể
                      </option>
                    </select>
                  </FieldLabel>

                  {repoForm.placement_mode === 'pinned_node' ? (
                    <FieldLabel label="Chọn máy chủ">
                      <select
                        value={repoForm.placement_node_id}
                        onChange={(event) => {
                          setRepoFormError(null);
                          setRepoForm((current) => ({ ...current, placement_node_id: event.target.value }));
                        }}
                        className={fieldClassName}
                        disabled={readyPlacementNodes.length === 0}
                      >
                        <option value="">{readyPlacementNodes.length > 0 ? 'Chọn máy chủ sẵn sàng' : 'Chưa có máy chủ sẵn sàng'}</option>
                        {readyPlacementNodes.map((node) => (
                          <option key={node.instance_id} value={node.instance_id}>
                            {node.name} ({node.instance_id})
                          </option>
                        ))}
                      </select>
                      <p className="text-sm text-[#94a3b8]">{placementHint}</p>
                    </FieldLabel>
                  ) : null}
                </div>
              ) : null}
            </div>

            {configureServices.isError ? (
              <div className="rounded-2xl border border-red-500/30 bg-red-500/10 px-6 py-3 text-base text-red-200">
                {configureServices.error instanceof Error ? configureServices.error.message : 'Khong luu duoc service.'}
              </div>
            ) : null}

            {repoFormError ? (
              <div className="rounded-2xl border border-red-500/30 bg-red-500/10 px-6 py-3 text-base text-red-200">
                {repoFormError}
              </div>
            ) : null}

            <DrawerActions
              pending={configureServices.isPending}
              primaryLabel={drawerMode === 'edit-repo' ? 'Lưu dịch vụ' : 'Tạo dịch vụ'}
              onCancel={closeDrawer}
            />
          </form>
        ) : null}

        {drawerMode === 'create-internal' || drawerMode === 'edit-internal' ? (
          <form
            className="grid gap-5"
            onSubmit={(event) => {
              event.preventDefault();
              void submitInternalService();
            }}
          >
            <div className="rounded-2xl border border-[#1e293b] bg-[#0B1120]/60 p-6 text-base text-[#cbd5e1]">
              <div className="font-semibold text-white">Internal service là gì?</div>
              <p className="mt-2 text-[#94a3b8]">
                Đây là service hạ tầng do LazyOps quản lý cho project của bạn, ví dụ PostgreSQL, MySQL, Redis hoặc RabbitMQ. Bạn không cần có thư mục code cho loại này trong repo.
              </p>
            </div>
            <FieldLabel label="Loại internal service" help="Chọn loại hạ tầng mà app của bạn sẽ dùng nội bộ.">
              <select
                value={internalForm.kind}
                onChange={(event) =>
                  setInternalForm((current) => ({
                    ...current,
                    kind: event.target.value as InternalServiceKind,
                    service_name:
                      current.service_name.trim() === '' || current.service_name === defaultInternalServiceName(current.kind)
                        ? defaultInternalServiceName(event.target.value as InternalServiceKind)
                        : current.service_name,
                  }))
                }
                className={fieldClassName}
                disabled={drawerMode === 'edit-internal'}
              >
                {INTERNAL_SERVICE_OPTIONS.map((option) => (
                  <option key={option.kind} value={option.kind}>
                    {option.label}
                  </option>
                ))}
              </select>
            </FieldLabel>
            <FieldLabel label="Tên dịch vụ" help="Tên DNS nội bộ của service trong cluster. App khác sẽ kết nối tới DB này qua tên này, ví dụ: db, redis, rabbitmq.">
              <input
                value={internalForm.service_name}
                onChange={(event) =>
                  setInternalForm((current) => ({
                    ...current,
                    service_name: event.target.value,
                  }))
                }
                className={fieldClassName}
                placeholder={defaultInternalServiceName(internalForm.kind)}
                required
              />
              <p className="text-sm text-[#94a3b8]">
                Tên này sẽ trở thành DNS nội bộ trong cluster, ví dụ <code className="rounded bg-[#020617] px-1.5 py-0.5 text-[#e2e8f0]">{internalForm.service_name.trim() || defaultInternalServiceName(internalForm.kind)}</code>.
              </p>
            </FieldLabel>
            <div className="rounded-2xl border border-[#1e293b] bg-[#0B1120]/60 p-6 text-base text-[#cbd5e1]">
              <div className="text-sm font-semibold uppercase tracking-[0.12em] text-[#64748b]">Đường dẫn nội bộ</div>
              <div className="mt-2 font-semibold text-white">
                .lazyops/internal/{internalForm.kind}/{internalForm.service_name.trim() || defaultInternalServiceName(internalForm.kind)}
              </div>
            </div>
            {internalForm.kind === 'postgres' ? (
              <div className="rounded-2xl border border-[#1e293b] bg-[#0B1120]/60 p-6">
                <div className="mb-3 text-base font-semibold text-white">Biến môi trường sẽ được inject</div>
                <p className="mb-3 text-base text-[#94a3b8]">
                  LazyOps sẽ tự điền các biến này vào service dùng database này. Bạn không cần tự nối thủ công bằng localhost.
                </p>
                <div className="grid gap-4">
                  {POSTGRES_CONNECTION_TEMPLATE_SLOTS.map((slot) => (
                    <FieldLabel key={slot} label={`${slot} env name`}>
                      <input
                        value={internalForm.connection_template[slot] || ''}
                        onChange={(event) =>
                          setInternalForm((current) => ({
                            ...current,
                            connection_template: {
                              ...current.connection_template,
                              [slot]: event.target.value,
                            },
                          }))
                        }
                        className={fieldClassName}
                        placeholder={slot}
                        required
                      />
                    </FieldLabel>
                  ))}
                </div>
                <div className="mb-3 mt-5 text-sm font-semibold uppercase tracking-[0.12em] text-[#64748b]">
                  Xem trước biến môi trường
                </div>
                <pre className="overflow-x-auto rounded-2xl border border-[#1e293b] bg-[#020617] p-6 text-base text-[#e2e8f0]">
                  {formatPostgresConnectionTemplatePreview(internalForm.connection_template)}
                </pre>
              </div>
            ) : (
              <div className="rounded-2xl border border-[#1e293b] bg-[#0B1120]/60 p-6 text-base text-[#cbd5e1]">
                <div className="text-base font-semibold text-white">{internalServiceLabel(internalForm.kind)}</div>
                <p className="mt-2 text-base text-[#94a3b8]">
                  LazyOps sẽ provision service này theo catalog managed sẵn. Endpoint mặc định: {internalServiceEndpointHint(internalForm.kind)}.
                </p>
              </div>
            )}

            {configureServices.isError ? (
              <div className="rounded-2xl border border-red-500/30 bg-red-500/10 px-6 py-3 text-base text-red-200">
                {configureServices.error instanceof Error ? configureServices.error.message : 'Khong luu duoc internal service.'}
              </div>
            ) : null}

            <DrawerActions
              pending={configureServices.isPending}
              primaryLabel={drawerMode === 'edit-internal' ? `Lưu ${internalServiceLabel(internalForm.kind)}` : `Tạo ${internalServiceLabel(internalForm.kind)}`}
              onCancel={closeDrawer}
            />
          </form>
        ) : null}

        {drawerMode === 'legacy-internal' ? (
          <div className="grid gap-5">
            <div className="rounded-2xl border border-amber-500/30 bg-amber-500/10 p-6 text-base text-amber-100">
              Internal service này đến từ compatibility lane cũ. Service này hiện ở chế độ read-only cho đến khi được đưa về unified service catalog.
            </div>
            <div className="rounded-2xl border border-[#1e293b] bg-[#0B1120]/60 p-6">
              <div className="mb-2 text-sm font-semibold uppercase tracking-[0.12em] text-[#64748b]">Service</div>
              <div className="text-lg font-semibold text-white">{selectedService?.name || 'internal-postgres'}</div>
              <div className="mt-3 text-base text-[#94a3b8]">{selectedService?.path || '.lazyops/internal/postgres'}</div>
            </div>
            {selectedService?.kind === 'postgres' ? (
              <div className="rounded-2xl border border-[#1e293b] bg-[#0B1120]/60 p-6">
                <div className="mb-3 text-base font-semibold text-white">Template inject theo service</div>
                <pre className="overflow-x-auto rounded-2xl border border-[#1e293b] bg-[#020617] p-6 text-base text-[#e2e8f0]">
                  {formatPostgresConnectionTemplatePreview(selectedService?.connection_template)}
                </pre>
              </div>
            ) : null}
            <DrawerActions pending={false} primaryLabel="" onCancel={closeDrawer} hidePrimary />
          </div>
        ) : null}
      </Drawer>
    </>
  );
}

function buildCatalogMutation(
  currentItems: ProjectService[],
  nextDraft: ProjectServiceDraft,
  editingId?: string,
): ConfigureProjectServicesRequest {
  const nextItems = currentItems
    .filter((item) => item.id !== editingId)
    .map(toProjectServiceDraft);
  nextItems.push(nextDraft);
  return { items: nextItems };
}

function toProjectServiceDraft(item: ProjectService): ProjectServiceDraft {
  return {
    name: item.name,
    path: item.path,
    kind: item.kind,
    source_type: item.source_type,
    public: item.public,
    runtime_profile: item.runtime_profile,
    placement_mode: item.placement_mode,
    placement_node_id: item.placement_node_id,
    connection_template_key: item.connection_template_key,
    connection_template: item.connection_template,
    connection_target_service: item.connection_target_service,
    managed_by_lazyops: item.managed_by_lazyops,
    start_hint: item.start_hint,
    image_ref: item.image_ref,
    image_digest: item.image_digest,
    target_port: item.target_port,
    service_port: item.service_port,
    replicas: item.replicas,
    env_bundle: item.env_bundle,
    pvc_spec: item.pvc_spec,
    deploy_strategy: item.deploy_strategy,
    healthcheck: item.healthcheck,
  };
}

function defaultRepoForm(service?: ProjectService): RepoFormState {
  return {
    name: service?.name || '',
    path: service?.path || '',
    kind: normalizeRepoServiceKind(service?.kind),
    public: service?.public ?? false,
    placement_mode: service?.placement_mode || 'shared_cluster',
    placement_node_id: service?.placement_node_id || '',
    connection_target_service: service?.connection_target_service || '',
  };
}

function normalizeRepoServiceKind(kind?: string): RepoServiceKind {
  const value = (kind || '').trim().toLowerCase();
  if (REPO_SERVICE_KIND_OPTIONS.some((option) => option.value === value)) {
    return value as RepoServiceKind;
  }
  return 'app';
}

function defaultInternalForm(service?: ProjectService): InternalFormState {
  const kind = normalizeInternalKind(service?.kind);
  return {
    kind,
    service_name: service?.name || defaultInternalServiceName(kind),
    connection_template: normalizePostgresConnectionTemplate(service?.connection_template),
  };
}

function normalizeInternalKind(kind?: string): InternalServiceKind {
  const value = (kind || '').trim().toLowerCase();
  if (INTERNAL_SERVICE_OPTIONS.some((option) => option.kind === value)) {
    return value as InternalServiceKind;
  }
  return 'postgres';
}

function defaultInternalServiceName(kind: InternalServiceKind) {
  switch (kind) {
    case 'postgres':
      return 'db';
    case 'mysql':
      return 'mysql';
    case 'redis':
      return 'redis';
    case 'rabbitmq':
      return 'rabbitmq';
  }
}

function defaultInternalServicePort(kind: InternalServiceKind) {
  switch (kind) {
    case 'postgres':
      return 5432;
    case 'mysql':
      return 3306;
    case 'redis':
      return 6379;
    case 'rabbitmq':
      return 5672;
  }
}

function defaultInternalServiceImage(kind: InternalServiceKind) {
  switch (kind) {
    case 'postgres':
      return 'postgres:16-alpine';
    case 'mysql':
      return 'mysql:8';
    case 'redis':
      return 'redis:7-alpine';
    case 'rabbitmq':
      return 'rabbitmq:3-management-alpine';
  }
}

function internalServiceLabel(kind: InternalServiceKind) {
  return INTERNAL_SERVICE_OPTIONS.find((option) => option.kind === kind)?.label || kind;
}

function internalServiceEndpointHint(kind: InternalServiceKind) {
  return INTERNAL_SERVICE_OPTIONS.find((option) => option.kind === kind)?.endpoint_hint || '';
}

function isLegacyInternalService(service: ProjectService) {
  if (service.source_type !== 'internal') {
    return false;
  }
  return service.path.split('/').length < 4;
}

function resolveDrawerTitle(mode: DrawerMode, service: ProjectService | null) {
  switch (mode) {
    case 'choose':
      return 'Thêm dịch vụ';
    case 'create-repo':
      return 'Thêm dịch vụ từ repository';
    case 'create-internal':
      return 'Thêm internal service';
    case 'edit-repo':
      return `Sửa dịch vụ ${service?.name || ''}`.trim();
    case 'edit-internal':
      return `Chi tiết internal service ${service?.name || ''}`.trim();
    case 'legacy-internal':
      return `Dịch vụ cũ ${service?.name || ''}`.trim();
    default:
      return 'Dịch vụ';
  }
}

function formatServiceKindLabel(service: ProjectService) {
  if (service.source_type === 'internal') {
    return `${internalServiceLabel(normalizeInternalKind(service.kind))} nội bộ`;
  }
  if (service.kind === 'web') {
    return 'Frontend web';
  }
  if (service.kind === 'api') {
    return 'API / backend';
  }
  return `${formatShortKind(service)} service`;
}

function formatShortKind(service: ProjectService) {
  switch ((service.kind || '').toLowerCase()) {
    case 'postgres':
      return 'Database';
    case 'mysql':
      return 'MySQL';
    case 'redis':
      return 'Redis';
    case 'rabbitmq':
      return 'RabbitMQ';
    case 'web':
      return 'Web';
    case 'api':
      return 'API';
    case 'worker':
      return 'Worker';
    default:
      return service.kind || 'App';
  }
}

function formatDatabaseTarget(service: ProjectService) {
  if (service.source_type === 'internal') {
    return `Tự là ${internalServiceLabel(normalizeInternalKind(service.kind))}`;
  }
  return service.connection_target_service || 'Chưa kết nối';
}

export function resolveServiceDisplayStatus(service: ProjectService, runtimeRecord?: ProjectRuntimeService) {
  if (runtimeRecord?.runtime_status === 'live') {
    return 'Đang chạy';
  }
  if (runtimeRecord?.runtime_status === 'deploying') {
    return 'Đang triển khai';
  }
  if (runtimeRecord?.runtime_status === 'degraded') {
    return 'Cần xử lý';
  }
  if (runtimeRecord?.runtime_status === 'configured') {
    return 'Sẵn sàng deploy';
  }
  if (service.source_type === 'internal') {
    return 'Chưa khởi động';
  }
  return 'Chưa deploy';
}

export function resolvePrimaryServiceAction(service: ProjectService, runtimeRecord?: ProjectRuntimeService) {
  if (service.source_type === 'internal') {
    return {
      label: runtimeRecord?.runtime_status === 'live' ? 'Triển khai lại' : 'Khởi động',
      action: 'deploy' as ProjectServiceAction,
    };
  }
  if (runtimeRecord?.runtime_status === 'live') {
    return {
      label: 'Deploy lại',
      action: 'deploy' as ProjectServiceAction,
    };
  }
  return {
    label: 'Deploy lần đầu',
    action: 'deploy' as ProjectServiceAction,
  };
}

function resolveServiceStatusVariant(service: ProjectService, runtimeRecord?: ProjectRuntimeService): 'success' | 'warning' | 'danger' | 'info' | 'neutral' {
  if (runtimeRecord) {
    return runtimeStatusVariant(runtimeRecord.runtime_status);
  }
  return service.source_type === 'internal' ? 'neutral' : 'info';
}

function buildAdvancedSummary(
  service: ProjectService,
  runtimeRecord: ProjectRuntimeService | undefined,
  placementLabel: string,
  servicePort: number,
) {
  return [
    {
      label: 'Vị trí chạy',
      value: placementLabel,
      icon: <Server className="size-4 text-[#38bdf8]" />,
    },
    {
      label: 'Cổng service',
      value: servicePort > 0 ? String(servicePort) : 'Chưa cấu hình',
      icon: <Database className="size-4 text-[#38bdf8]" />,
    },
    {
      label: 'Runtime profile',
      value: service.runtime_profile || 'Chưa có',
      icon: <Boxes className="size-4 text-[#38bdf8]" />,
    },
    {
      label: 'Template kết nối',
      value: service.connection_template_key || 'Chưa có',
      icon: <Network className="size-4 text-[#38bdf8]" />,
    },
    {
      label: 'Target port',
      value: service.target_port ? String(service.target_port) : 'Tự động',
      icon: <Database className="size-4 text-[#38bdf8]" />,
    },
    {
      label: 'Runtime raw',
      value: runtimeRecord?.runtime_status || 'Chưa có',
      icon: <Boxes className="size-4 text-[#38bdf8]" />,
    },
  ];
}

function ServiceChoiceCard({
  title,
  description,
  onClick,
}: {
  title: string;
  description: string;
  onClick: () => void;
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      className="rounded-2xl border border-[#1e293b] bg-[#0B1120]/60 p-6 text-left transition-colors hover:border-[#0EA5E9] hover:bg-[#0F172A]"
    >
      <div className="text-lg font-semibold text-white">{title}</div>
      <p className="mt-2 text-base leading-relaxed text-[#94a3b8]">{description}</p>
    </button>
  );
}

function DrawerActions({
  pending,
  primaryLabel,
  onCancel,
  hidePrimary = false,
}: {
  pending: boolean;
  primaryLabel: string;
  onCancel: () => void;
  hidePrimary?: boolean;
}) {
  return (
    <div className="flex items-center justify-end gap-3">
      <button
        type="button"
        onClick={onCancel}
        className="rounded-xl border border-[#334155] px-6 py-2 text-base font-semibold text-[#cbd5e1] transition-colors hover:bg-[#111827]"
      >
        Đóng
      </button>
      {hidePrimary ? null : (
        <button
          type="submit"
          disabled={pending}
          className="rounded-xl bg-[#0EA5E9] px-6 py-2 text-base font-semibold text-[#020617] transition-opacity hover:opacity-90 disabled:cursor-not-allowed disabled:opacity-60"
        >
          {pending ? 'Đang lưu...' : primaryLabel}
        </button>
      )}
    </div>
  );
}

function FieldLabel({ label, help, children }: { label: string; help?: string; children: ReactNode }) {
  return (
    <label className="grid gap-2">
      <span className="text-base font-semibold text-[#e2e8f0]">{label}</span>
      {children}
      {help ? <span className="text-sm leading-relaxed text-[#94a3b8]">{help}</span> : null}
    </label>
  );
}

function MetricLine({ icon, label, value }: { icon: ReactNode; label: string; value: string }) {
  return (
    <div className="rounded-2xl border border-[#1e293b] bg-[#0B1120]/50 p-3">
      <div className="mb-2 flex items-center gap-2 text-sm font-semibold uppercase tracking-[0.12em] text-[#64748b]">
        {icon}
        {label}
      </div>
      <div className="break-all text-base font-semibold text-[#e2e8f0]">{value}</div>
    </div>
  );
}

function ActionButton({
  label,
  onClick,
  pending,
  variant = 'primary',
}: {
  label: string;
  onClick: () => void;
  pending: boolean;
  variant?: 'primary' | 'secondary';
}) {
  const className =
    variant === 'primary'
      ? 'border-[#0EA5E9] bg-[#0EA5E9]/10 text-[#38bdf8] hover:bg-[#0EA5E9]/20'
      : 'border-[#334155] bg-transparent text-[#e2e8f0] hover:bg-[#111827]';

  return (
    <button
      type="button"
      onClick={onClick}
      disabled={pending}
      className={`rounded-xl border px-3 py-2 text-sm font-semibold transition-colors disabled:cursor-not-allowed disabled:opacity-60 ${className}`}
    >
      {pending ? 'Đang chạy...' : label}
    </button>
  );
}

export function resolvePlacementHint({
  placementNodesError,
  isLoading,
  clusterID,
  readyCount,
}: {
  placementNodesError: string | null;
  isLoading: boolean;
  clusterID?: string;
  readyCount: number;
}) {
  if (isLoading) {
    return 'Đang tải danh sách máy chủ có thể ghim service...';
  }
  if (placementNodesError) {
    return `Không tải được danh sách máy chủ: ${placementNodesError}`;
  }
  if (!clusterID) {
    return 'Project chưa có cluster sẵn sàng, nên chưa thể ghim service vào một máy chủ cụ thể.';
  }
  if (readyCount === 0) {
    return 'Cluster đã kết nối nhưng chưa có máy chủ nào sẵn sàng để ghim service.';
  }
  return `${readyCount} máy chủ sẵn sàng để ghim service. Nếu không chọn riêng, service sẽ chạy trên cụm dùng chung.`;
}

export function formatPlacementValue(service: ProjectService, placementNodesByID: Map<string, PlacementNode>) {
  const placementMode = service.placement_mode || 'shared_cluster';
  if (placementMode !== 'pinned_node') {
    return 'Cụm dùng chung';
  }

  const node = service.placement_node_id ? placementNodesByID.get(service.placement_node_id) : null;
  if (!node) {
    return `Ghim vào máy chủ ${service.placement_node_id || 'đang chờ'}`;
  }
  return `${node.name} · ${node.is_ready ? 'Sẵn sàng' : formatPlacementNodeStatus(node.status)}`;
}

function formatPlacementNodeStatus(status: string) {
  return status.replace(/_/g, ' ').replace(/\b\w/g, (char) => char.toUpperCase());
}

export function formatActionResult(result: ProjectServiceActionResponse) {
  if (result.message) {
    return result.message;
  }
  if (result.action === 'restart') {
    return `Đã gửi lệnh restart cho service ${result.service_name}.`;
  }
  if (result.deployment_id) {
    return `${result.action} đã tạo deployment ${result.deployment_id}.`;
  }
  return `${result.action} đã được kích hoạt cho ${result.service_name}.`;
}

function runtimeStatusVariant(status: string): 'success' | 'warning' | 'danger' | 'info' | 'neutral' {
  switch (status) {
    case 'live':
      return 'success';
    case 'deploying':
    case 'waiting_for_node':
      return 'warning';
    case 'configured':
      return 'info';
    case 'degraded':
      return 'danger';
    default:
      return 'neutral';
  }
}

const fieldClassName =
  'w-full rounded-xl border border-[#334155] bg-[#020617] px-6 py-3 text-base text-[#e2e8f0] outline-none transition-colors placeholder:text-[#64748b] focus:border-[#0EA5E9]';
