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
  | 'create-postgres'
  | 'edit-repo'
  | 'edit-postgres'
  | 'legacy-internal';

type RepoFormState = {
  name: string;
  path: string;
  kind: string;
  public: boolean;
  placement_mode: string;
  placement_node_id: string;
  connection_target_service: string;
};

type PostgresFormState = {
  service_name: string;
  connection_template: Record<string, string>;
};

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
  const [postgresForm, setPostgresForm] = useState<PostgresFormState>(defaultPostgresForm());
  const [showRepoAdvanced, setShowRepoAdvanced] = useState(false);
  const [repoFormError, setRepoFormError] = useState<string | null>(null);
  const [activeAction, setActiveAction] = useState<{ serviceId: string; action: ProjectServiceAction } | null>(null);
  const [lastActionResult, setLastActionResult] = useState<ProjectServiceActionResponse | null>(null);
  const [lastActionError, setLastActionError] = useState<{ serviceId: string; message: string } | null>(null);

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

  const openChooseDrawer = () => {
    configureServices.reset();
    setRepoFormError(null);
    setSelectedService(null);
    setRepoForm(defaultRepoForm());
    setPostgresForm(defaultPostgresForm());
    setShowRepoAdvanced(false);
    setDrawerMode('choose');
  };

  const openEditDrawer = (service: ProjectService) => {
    configureServices.reset();
    setRepoFormError(null);
    setSelectedService(service);
    if (service.source_type === 'internal' && service.kind === 'postgres') {
      setPostgresForm(defaultPostgresForm(service));
      setShowRepoAdvanced(false);
      setDrawerMode(isLegacyInternalService(service) ? 'legacy-internal' : 'edit-postgres');
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

  const submitInternalPostgres = async () => {
    const serviceName = postgresForm.service_name.trim();
    const existingEnv = selectedService?.env_bundle || {};
    const existingPVC = selectedService?.pvc_spec || { size: '5Gi' };
    const existingHealthcheck = selectedService?.healthcheck || { protocol: 'tcp', port: 5432 };
    const draft: ProjectServiceDraft = {
      name: serviceName,
      path: `.lazyops/internal/postgres/${serviceName}`,
      kind: 'postgres',
      source_type: 'internal',
      public: false,
      runtime_profile: 'internal-db',
      placement_mode: 'shared_cluster',
      connection_template_key: '',
      connection_target_service: '',
      managed_by_lazyops: true,
      start_hint: 'managed-internal-service',
      image_ref: selectedService?.image_ref || 'postgres:16-alpine',
      image_digest: selectedService?.image_digest || '',
      target_port: selectedService?.target_port || 5432,
      service_port: selectedService?.service_port || 5432,
      replicas: selectedService?.replicas || 1,
      env_bundle: existingEnv,
      pvc_spec: existingPVC,
      connection_template: normalizePostgresConnectionTemplate(postgresForm.connection_template),
      deploy_strategy: selectedService?.deploy_strategy || {},
      healthcheck: existingHealthcheck,
    };

    await configureServices.mutateAsync(
      buildCatalogMutation(items, draft, drawerMode === 'edit-postgres' ? selectedService?.id : undefined),
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
              className="inline-flex items-center gap-2 rounded-xl border border-[#0EA5E9] bg-[#0EA5E9]/10 px-4 py-2 text-sm font-semibold text-[#38bdf8] transition-colors hover:bg-[#0EA5E9]/20"
            >
              <Plus className="size-4" />
              Them service
            </button>
            <Link
              href={`/projects/${projectId}/deployments`}
              className="rounded-xl border border-[#334155] px-4 py-2 text-sm font-semibold text-[#e2e8f0] transition-colors hover:bg-[#111827]"
            >
              Xem deploy
            </Link>
          </>
        }
      >
        {filteredItems.length === 0 ? (
          <div className="rounded-2xl border border-dashed border-[#334155] bg-[#0B1120]/30 p-8 text-sm text-[#94a3b8]">
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
                  className="rounded-2xl border border-[#1e293b] bg-[#020617]/70 p-5 shadow-[0_20px_60px_rgba(2,6,23,0.35)]"
                >
                  <div className="flex items-start justify-between gap-4">
                    <div className="space-y-2">
                      <div className="flex items-center gap-3">
                        <div className="rounded-xl border border-[#1e293b] bg-[#0F172A] p-2">
                          <Boxes className="size-5 text-[#38bdf8]" />
                        </div>
                        <div>
                          <h3 className="text-xl font-bold tracking-tight text-white">{service.name}</h3>
                          <p className="text-sm text-[#94a3b8]">{formatServiceKindLabel(service)}</p>
                        </div>
                      </div>
                      <div className="flex flex-wrap gap-2">
                        <StatusBadge label={displayStatus} variant={resolveServiceStatusVariant(service, runtimeRecord)} size="sm" />
                        <StatusBadge label={service.source_type === 'internal' ? 'Nội bộ' : 'Repository'} variant={service.source_type === 'internal' ? 'warning' : 'info'} size="sm" />
                        <StatusBadge label={service.public ? 'Công khai' : 'Nội bộ'} variant={service.public ? 'success' : 'neutral'} size="sm" />
                      </div>
                    </div>
                    <div className="flex items-center gap-2 rounded-full border border-[#1e293b] bg-[#0B1120]/80 px-3 py-1.5 text-xs font-semibold text-[#cbd5e1]">
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
                    <div className="mt-4 rounded-2xl border border-[#1e293b] bg-[#0B1120]/60 px-4 py-3 text-sm text-[#cbd5e1]">
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
                      className="rounded-xl border border-[#334155] px-3 py-2 text-xs font-semibold text-[#e2e8f0] transition-colors hover:bg-[#111827]"
                    >
                      Nhật ký
                    </Link>
                  </div>

                  <details className="mt-4 rounded-2xl border border-[#1e293b] bg-[#0B1120]/40 p-4">
                    <summary className="cursor-pointer list-none text-sm font-semibold text-white">
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
                      <div className="mt-4 rounded-2xl border border-[#1e293b] bg-[#020617]/80 px-4 py-3 text-sm text-[#cbd5e1]">
                        Máy chủ đã ghim: <span className="font-semibold text-white">{placementNode.name}</span> · {placementNode.is_ready ? 'Sẵn sàng' : formatPlacementNodeStatus(placementNode.status)}
                      </div>
                    ) : null}
                  </details>

                  {actionResult ? (
                    <div className="mt-3 rounded-2xl border border-[#0EA5E9]/30 bg-[#0EA5E9]/10 px-4 py-3 text-sm text-[#bae6fd]">
                      {formatActionResult(actionResult)}
                    </div>
                  ) : null}

                  {actionError ? (
                    <div className="mt-3 rounded-2xl border border-[#ef4444]/30 bg-[#ef4444]/10 px-4 py-3 text-sm text-[#fecaca]">
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
              title="Postgres nội bộ"
              description="Tạo database dùng chung cho các service khác trong project mà không cần tự cấu hình tay."
              onClick={() => {
                setPostgresForm(defaultPostgresForm());
                setShowRepoAdvanced(false);
                setDrawerMode('create-postgres');
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
            <FieldLabel label="Tên dịch vụ">
              <input
                value={repoForm.name}
                onChange={(event) => setRepoForm((current) => ({ ...current, name: event.target.value }))}
                className={fieldClassName}
                placeholder="api"
                required
              />
            </FieldLabel>
            <FieldLabel label="Thư mục trong repo">
              <input
                value={repoForm.path}
                onChange={(event) => setRepoForm((current) => ({ ...current, path: event.target.value }))}
                className={fieldClassName}
                placeholder="apps/api"
                required
              />
            </FieldLabel>
            <div className="grid gap-5 md:grid-cols-2">
              <FieldLabel label="Loại service">
                <input
                  value={repoForm.kind}
                  onChange={(event) => setRepoForm((current) => ({ ...current, kind: event.target.value }))}
                  className={fieldClassName}
                  placeholder="app"
                />
              </FieldLabel>
              <FieldLabel label="Database kết nối">
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
            <label className="flex items-center gap-3 rounded-2xl border border-[#1e293b] bg-[#0B1120]/60 px-4 py-3 text-sm text-[#cbd5e1]">
              <input
                type="checkbox"
                checked={repoForm.public}
                onChange={(event) => setRepoForm((current) => ({ ...current, public: event.target.checked }))}
              />
              Cho phép truy cập từ Internet
            </label>

            <div className="rounded-2xl border border-[#1e293b] bg-[#0B1120]/50 p-4">
              <button
                type="button"
                className="text-sm font-semibold text-white"
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
                      <p className="text-xs text-[#94a3b8]">{placementHint}</p>
                    </FieldLabel>
                  ) : null}
                </div>
              ) : null}
            </div>

            {configureServices.isError ? (
              <div className="rounded-2xl border border-red-500/30 bg-red-500/10 px-4 py-3 text-sm text-red-200">
                {configureServices.error instanceof Error ? configureServices.error.message : 'Khong luu duoc service.'}
              </div>
            ) : null}

            {repoFormError ? (
              <div className="rounded-2xl border border-red-500/30 bg-red-500/10 px-4 py-3 text-sm text-red-200">
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

        {drawerMode === 'create-postgres' || drawerMode === 'edit-postgres' ? (
          <form
            className="grid gap-5"
            onSubmit={(event) => {
              event.preventDefault();
              void submitInternalPostgres();
            }}
          >
            <FieldLabel label="Tên dịch vụ">
              <input
                value={postgresForm.service_name}
                onChange={(event) =>
                  setPostgresForm((current) => ({
                    ...current,
                    service_name: event.target.value,
                  }))
                }
                className={fieldClassName}
                placeholder="db"
                required
              />
            </FieldLabel>
            <div className="rounded-2xl border border-[#1e293b] bg-[#0B1120]/60 p-4 text-sm text-[#cbd5e1]">
              <div className="text-xs font-semibold uppercase tracking-[0.12em] text-[#64748b]">Đường dẫn nội bộ</div>
              <div className="mt-2 font-semibold text-white">
                .lazyops/internal/postgres/{postgresForm.service_name.trim() || 'db'}
              </div>
            </div>
            <div className="rounded-2xl border border-[#1e293b] bg-[#0B1120]/60 p-4">
              <div className="mb-3 text-sm font-semibold text-white">Biến môi trường sẽ được inject</div>
              <p className="mb-3 text-sm text-[#94a3b8]">
                LazyOps sẽ tự điền các biến này vào service dùng database này. Bạn không cần tự nối thủ công bằng localhost.
              </p>
              <div className="grid gap-4">
                {POSTGRES_CONNECTION_TEMPLATE_SLOTS.map((slot) => (
                  <FieldLabel key={slot} label={`${slot} env name`}>
                    <input
                      value={postgresForm.connection_template[slot] || ''}
                      onChange={(event) =>
                        setPostgresForm((current) => ({
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
              <div className="mb-3 mt-5 text-xs font-semibold uppercase tracking-[0.12em] text-[#64748b]">
                Xem trước biến môi trường
              </div>
              <pre className="overflow-x-auto rounded-2xl border border-[#1e293b] bg-[#020617] p-4 text-sm text-[#e2e8f0]">
                {formatPostgresConnectionTemplatePreview(postgresForm.connection_template)}
              </pre>
            </div>

            {configureServices.isError ? (
              <div className="rounded-2xl border border-red-500/30 bg-red-500/10 px-4 py-3 text-sm text-red-200">
                {configureServices.error instanceof Error ? configureServices.error.message : 'Khong luu duoc internal Postgres.'}
              </div>
            ) : null}

            <DrawerActions
              pending={configureServices.isPending}
              primaryLabel={drawerMode === 'edit-postgres' ? 'Lưu Postgres' : 'Tạo Postgres nội bộ'}
              onCancel={closeDrawer}
            />
          </form>
        ) : null}

        {drawerMode === 'legacy-internal' ? (
          <div className="grid gap-5">
            <div className="rounded-2xl border border-amber-500/30 bg-amber-500/10 p-4 text-sm text-amber-100">
              Internal service nay den tu compatibility lane cu. Day 4 chi mo editor moi cho internal Postgres duoc tao trong unified service catalog, nen service nay hien tai chi o che do read-only.
            </div>
            <div className="rounded-2xl border border-[#1e293b] bg-[#0B1120]/60 p-4">
              <div className="mb-2 text-xs font-semibold uppercase tracking-[0.12em] text-[#64748b]">Service</div>
              <div className="text-lg font-semibold text-white">{selectedService?.name || 'internal-postgres'}</div>
              <div className="mt-3 text-sm text-[#94a3b8]">{selectedService?.path || '.lazyops/internal/postgres'}</div>
            </div>
            <div className="rounded-2xl border border-[#1e293b] bg-[#0B1120]/60 p-4">
              <div className="mb-3 text-sm font-semibold text-white">Template inject theo service</div>
              <pre className="overflow-x-auto rounded-2xl border border-[#1e293b] bg-[#020617] p-4 text-sm text-[#e2e8f0]">
                {formatPostgresConnectionTemplatePreview(selectedService?.connection_template)}
              </pre>
            </div>
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
    kind: service?.kind || 'app',
    public: service?.public ?? false,
    placement_mode: service?.placement_mode || 'shared_cluster',
    placement_node_id: service?.placement_node_id || '',
    connection_target_service: service?.connection_target_service || '',
  };
}

function defaultPostgresForm(service?: ProjectService): PostgresFormState {
  return {
    service_name: service?.name || 'db',
    connection_template: normalizePostgresConnectionTemplate(service?.connection_template),
  };
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
    case 'create-postgres':
      return 'Thêm Postgres nội bộ';
    case 'edit-repo':
      return `Sửa dịch vụ ${service?.name || ''}`.trim();
    case 'edit-postgres':
      return `Chi tiết Postgres ${service?.name || ''}`.trim();
    case 'legacy-internal':
      return `Dịch vụ cũ ${service?.name || ''}`.trim();
    default:
      return 'Dịch vụ';
  }
}

function formatServiceKindLabel(service: ProjectService) {
  if (service.kind === 'postgres') {
    return service.source_type === 'internal' ? 'Database Postgres nội bộ' : 'Database Postgres';
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
  if (service.kind === 'postgres') {
    return 'Tự là database';
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
      className="rounded-2xl border border-[#1e293b] bg-[#0B1120]/60 p-5 text-left transition-colors hover:border-[#0EA5E9] hover:bg-[#0F172A]"
    >
      <div className="text-lg font-semibold text-white">{title}</div>
      <p className="mt-2 text-sm leading-relaxed text-[#94a3b8]">{description}</p>
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
        className="rounded-xl border border-[#334155] px-4 py-2 text-sm font-semibold text-[#cbd5e1] transition-colors hover:bg-[#111827]"
      >
        Đóng
      </button>
      {hidePrimary ? null : (
        <button
          type="submit"
          disabled={pending}
          className="rounded-xl bg-[#0EA5E9] px-4 py-2 text-sm font-semibold text-[#020617] transition-opacity hover:opacity-90 disabled:cursor-not-allowed disabled:opacity-60"
        >
          {pending ? 'Đang lưu...' : primaryLabel}
        </button>
      )}
    </div>
  );
}

function FieldLabel({ label, children }: { label: string; children: ReactNode }) {
  return (
    <label className="grid gap-2">
      <span className="text-sm font-semibold text-[#e2e8f0]">{label}</span>
      {children}
    </label>
  );
}

function MetricLine({ icon, label, value }: { icon: ReactNode; label: string; value: string }) {
  return (
    <div className="rounded-2xl border border-[#1e293b] bg-[#0B1120]/50 p-3">
      <div className="mb-2 flex items-center gap-2 text-xs font-semibold uppercase tracking-[0.12em] text-[#64748b]">
        {icon}
        {label}
      </div>
      <div className="break-all text-sm font-semibold text-[#e2e8f0]">{value}</div>
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
      className={`rounded-xl border px-3 py-2 text-xs font-semibold transition-colors disabled:cursor-not-allowed disabled:opacity-60 ${className}`}
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
  'w-full rounded-xl border border-[#334155] bg-[#020617] px-4 py-3 text-sm text-[#e2e8f0] outline-none transition-colors placeholder:text-[#64748b] focus:border-[#0EA5E9]';
