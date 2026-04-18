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
import { useProjectRuntime } from '@/modules/project-runtime/project-runtime-hooks';
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
};

const POSTGRES_TEMPLATE_PREVIEW = ['DB_URL=', 'DB_NAME=', 'DB_HOST=', 'DB_PORT=', 'DB_USERNAME=', 'DB_PASSWORD='].join('\n');

export function ProjectServiceInventory({
  projectId,
  title = 'Service inventory',
  description = 'Project nay chi la namespace; services ben duoi moi la don vi deploy va van hanh.',
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
    setDrawerMode('choose');
  };

  const openEditDrawer = (service: ProjectService) => {
    configureServices.reset();
    setRepoFormError(null);
    setSelectedService(service);
    if (service.source_type === 'internal' && service.kind === 'postgres') {
      setPostgresForm(defaultPostgresForm(service));
      setDrawerMode(isLegacyInternalService(service) ? 'legacy-internal' : 'edit-postgres');
      return;
    }
    setRepoForm(defaultRepoForm(service));
    setDrawerMode('edit-repo');
  };

  const closeDrawer = () => {
    if (configureServices.isPending) {
      return;
    }
    configureServices.reset();
    setRepoFormError(null);
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
        <div className="mb-4 rounded-2xl border border-[#1e293b] bg-[#020617]/50 p-4 text-sm text-[#94a3b8]">
          <div className="font-semibold text-white">Placement nodes</div>
          <p className="mt-1">
            {placementHint}
          </p>
        </div>

        {filteredItems.length === 0 ? (
          <div className="rounded-2xl border border-dashed border-[#334155] bg-[#0B1120]/30 p-8 text-sm text-[#94a3b8]">
            {sourceFilter === 'internal'
              ? 'Chua co internal service nao trong inventory hop nhat cua project nay.'
              : sourceFilter === 'repo'
                ? 'Chua co repo service nao trong inventory hop nhat cua project nay.'
                : 'Chua co service nao duoc khai bao cho project nay. Inventory nay la source of truth duy nhat cho repo service va internal service.'}
          </div>
        ) : (
          <div className={`grid gap-4 ${compact ? 'xl:grid-cols-2' : 'xl:grid-cols-2 2xl:grid-cols-3'}`}>
            {filteredItems.map((service) => {
              const exposure = service.public ? 'public' : 'private';
              const exposureIcon = service.public ? <Globe className="size-4 text-[#38bdf8]" /> : <Lock className="size-4 text-[#94a3b8]" />;
              const sourceType = service.source_type || 'repo';
              const placementMode = service.placement_mode || 'shared_cluster';
              const placementLabel = formatPlacementValue(service, placementNodesByID);
              const placementNode = service.placement_node_id
                ? placementNodesByID.get(service.placement_node_id)
                : null;
              const runtimeRecord = runtimeByServiceID.get(service.id);
              const connectionTemplate = service.connection_template_key || 'chua gan';
              const servicePort = service.service_port || service.target_port || 0;
              const actionPending = serviceAction.isPending && activeAction?.serviceId === service.id;
              const actionResult = lastActionResult?.service_id === service.id ? lastActionResult : null;
              const actionError = lastActionError?.serviceId === service.id ? lastActionError.message : null;

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
                          <p className="text-sm text-[#94a3b8]">{service.kind || 'app'} service</p>
                        </div>
                      </div>
                      <div className="flex flex-wrap gap-2">
                        <StatusBadge label={sourceType} variant={sourceType === 'internal' ? 'warning' : 'info'} size="sm" />
                        <StatusBadge label={exposure} variant={service.public ? 'success' : 'neutral'} size="sm" />
                        <StatusBadge label={placementMode} variant="default" size="sm" />
                        {runtimeRecord ? (
                          <StatusBadge label={runtimeRecord.runtime_status} variant={runtimeStatusVariant(runtimeRecord.runtime_status)} size="sm" />
                        ) : null}
                        {service.managed_by_lazyops ? <StatusBadge label="managed" variant="warning" size="sm" /> : null}
                      </div>
                    </div>
                    <div className="flex flex-col items-end gap-3">
                      <div className="flex items-center gap-2 rounded-full border border-[#1e293b] bg-[#0B1120]/80 px-3 py-1.5 text-xs font-semibold text-[#cbd5e1]">
                        {exposureIcon}
                        {service.public ? 'Cong khai' : 'Noi bo'}
                      </div>
                      <button
                        type="button"
                        onClick={() => openEditDrawer(service)}
                        className="rounded-xl border border-[#334155] px-3 py-2 text-xs font-semibold text-[#e2e8f0] transition-colors hover:bg-[#111827]"
                      >
                        {service.source_type === 'internal' ? 'Chi tiet' : 'Sua service'}
                      </button>
                      <Link
                        href={`/projects/${projectId}/observability?service=${encodeURIComponent(service.name)}`}
                        className="rounded-xl border border-[#0EA5E9] bg-[#0EA5E9]/10 px-3 py-2 text-xs font-semibold text-[#38bdf8] transition-colors hover:bg-[#0EA5E9]/20"
                      >
                        Runtime
                      </Link>
                    </div>
                  </div>

                  <div className="mt-5 grid gap-4 sm:grid-cols-2">
                    <MetricLine icon={<Network className="size-4 text-[#38bdf8]" />} label="Path / identity" value={service.path || 'service noi bo'} />
                    <MetricLine icon={<Server className="size-4 text-[#38bdf8]" />} label="Placement" value={placementLabel} />
                    <MetricLine icon={<Database className="size-4 text-[#38bdf8]" />} label="Service port" value={servicePort > 0 ? String(servicePort) : 'chua co'} />
                    <MetricLine icon={<Boxes className="size-4 text-[#38bdf8]" />} label="Connection template" value={connectionTemplate} />
                    <MetricLine
                      icon={<Database className="size-4 text-[#38bdf8]" />}
                      label="Postgres target"
                      value={service.connection_target_service || 'chua gan'}
                    />
                  </div>

                  {placementNode ? (
                    <div className="mt-4 rounded-2xl border border-[#1e293b] bg-[#0B1120]/60 px-4 py-3 text-sm text-[#cbd5e1]">
                      Node <span className="font-semibold text-white">{placementNode.name}</span> · {placementNode.is_ready ? 'Ready' : formatPlacementNodeStatus(placementNode.status)}
                    </div>
                  ) : null}

                  {runtimeRecord ? (
                    <div className="mt-4 rounded-2xl border border-[#1e293b] bg-[#020617]/80 px-4 py-3 text-sm text-[#cbd5e1]">
                      Runtime: <span className="font-semibold text-white">{runtimeRecord.runtime_reason || runtimeRecord.runtime_status}</span>
                    </div>
                  ) : null}

                  <div className="mt-5 rounded-2xl border border-[#1e293b] bg-[#0B1120]/70 p-4">
                    <div className="grid gap-3 sm:grid-cols-3">
                      <SummaryBlock label="Runtime profile" value={service.runtime_profile || 'chua gan'} />
                      <SummaryBlock label="Replicas" value={String(service.replicas || 1)} />
                      <SummaryBlock label="Target port" value={service.target_port ? String(service.target_port) : 'auto'} />
                    </div>
                  </div>

                  <div className="mt-5 flex flex-wrap gap-2">
                    <ActionButton
                      label="Deploy"
                      pending={actionPending && activeAction?.action === 'deploy'}
                      onClick={() => void runServiceAction(service, 'deploy')}
                    />
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
              title="Repo service"
              description="Service lay code tu repo/path cua user. Day la app, worker, frontend hoac backend ma ban muon deploy."
              onClick={() => {
                setRepoForm(defaultRepoForm());
                setDrawerMode('create-repo');
              }}
            />
            <ServiceChoiceCard
              title="Internal Postgres"
              description="LazyOps tao service Postgres noi bo trong cung project/namespace. Repo service khac co the gan DB template vao service nay."
              onClick={() => {
                setPostgresForm(defaultPostgresForm());
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
            <FieldLabel label="Service name">
              <input
                value={repoForm.name}
                onChange={(event) => setRepoForm((current) => ({ ...current, name: event.target.value }))}
                className={fieldClassName}
                placeholder="api"
                required
              />
            </FieldLabel>
            <FieldLabel label="Path">
              <input
                value={repoForm.path}
                onChange={(event) => setRepoForm((current) => ({ ...current, path: event.target.value }))}
                className={fieldClassName}
                placeholder="apps/api"
                required
              />
            </FieldLabel>
            <div className="grid gap-5 md:grid-cols-2">
              <FieldLabel label="Kind">
                <input
                  value={repoForm.kind}
                  onChange={(event) => setRepoForm((current) => ({ ...current, kind: event.target.value }))}
                  className={fieldClassName}
                  placeholder="app"
                />
              </FieldLabel>
              <FieldLabel label="Placement">
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
                  <option value="shared_cluster">shared_cluster</option>
                  <option
                    value="pinned_node"
                    disabled={readyPlacementNodes.length === 0 && repoForm.placement_mode !== 'pinned_node'}
                  >
                    pinned_node
                  </option>
                </select>
              </FieldLabel>
            </div>
            {repoForm.placement_mode === 'pinned_node' ? (
              <FieldLabel label="Node">
                <select
                  value={repoForm.placement_node_id}
                  onChange={(event) => {
                    setRepoFormError(null);
                    setRepoForm((current) => ({ ...current, placement_node_id: event.target.value }));
                  }}
                  className={fieldClassName}
                  disabled={readyPlacementNodes.length === 0}
                >
                  <option value="">{readyPlacementNodes.length > 0 ? 'Chon ready node' : 'Chua co ready node'}</option>
                  {readyPlacementNodes.map((node) => (
                    <option key={node.instance_id} value={node.instance_id}>
                      {node.name} ({node.instance_id})
                    </option>
                  ))}
                </select>
                <p className="text-xs text-[#94a3b8]">{placementHint}</p>
              </FieldLabel>
            ) : null}
            <FieldLabel label="Postgres connection">
              <select
                value={repoForm.connection_target_service}
                onChange={(event) => setRepoForm((current) => ({ ...current, connection_target_service: event.target.value }))}
                className={fieldClassName}
              >
                <option value="">Khong gan Postgres</option>
                {internalPostgresTargets.map((service) => (
                  <option key={service.id} value={service.name}>
                    {service.name}
                  </option>
                ))}
              </select>
            </FieldLabel>
            <label className="flex items-center gap-3 rounded-2xl border border-[#1e293b] bg-[#0B1120]/60 px-4 py-3 text-sm text-[#cbd5e1]">
              <input
                type="checkbox"
                checked={repoForm.public}
                onChange={(event) => setRepoForm((current) => ({ ...current, public: event.target.checked }))}
              />
              Service nay cong khai qua Ingress / public route
            </label>

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
              primaryLabel={drawerMode === 'edit-repo' ? 'Luu service' : 'Tao repo service'}
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
            <FieldLabel label="Service name">
              <input
                value={postgresForm.service_name}
                onChange={(event) => setPostgresForm({ service_name: event.target.value })}
                className={fieldClassName}
                placeholder="db"
                required
              />
            </FieldLabel>
            <div className="rounded-2xl border border-[#1e293b] bg-[#0B1120]/60 p-4 text-sm text-[#cbd5e1]">
              <div className="text-xs font-semibold uppercase tracking-[0.12em] text-[#64748b]">Path noi bo</div>
              <div className="mt-2 font-semibold text-white">
                .lazyops/internal/postgres/{postgresForm.service_name.trim() || 'db'}
              </div>
            </div>
            <div className="rounded-2xl border border-[#1e293b] bg-[#0B1120]/60 p-4">
              <div className="mb-3 text-sm font-semibold text-white">Template inject theo service</div>
              <p className="mb-3 text-sm text-[#94a3b8]">
                LazyOps se tu fill cac field nay vao repo service co gan <code>postgres.basic</code>. Day la K3s internal DNS flow, khong phai localhost helper.
              </p>
              <pre className="overflow-x-auto rounded-2xl border border-[#1e293b] bg-[#020617] p-4 text-sm text-[#e2e8f0]">
                {POSTGRES_TEMPLATE_PREVIEW}
              </pre>
            </div>

            {configureServices.isError ? (
              <div className="rounded-2xl border border-red-500/30 bg-red-500/10 px-4 py-3 text-sm text-red-200">
                {configureServices.error instanceof Error ? configureServices.error.message : 'Khong luu duoc internal Postgres.'}
              </div>
            ) : null}

            <DrawerActions
              pending={configureServices.isPending}
              primaryLabel={drawerMode === 'edit-postgres' ? 'Luu Postgres' : 'Tao Internal Postgres'}
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
                {POSTGRES_TEMPLATE_PREVIEW}
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
      return 'Them service';
    case 'create-repo':
      return 'Them repo service';
    case 'create-postgres':
      return 'Them Internal Postgres';
    case 'edit-repo':
      return `Sua service ${service?.name || ''}`.trim();
    case 'edit-postgres':
      return `Chi tiet Postgres ${service?.name || ''}`.trim();
    case 'legacy-internal':
      return `Legacy internal ${service?.name || ''}`.trim();
    default:
      return 'Service';
  }
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
        Dong
      </button>
      {hidePrimary ? null : (
        <button
          type="submit"
          disabled={pending}
          className="rounded-xl bg-[#0EA5E9] px-4 py-2 text-sm font-semibold text-[#020617] transition-opacity hover:opacity-90 disabled:cursor-not-allowed disabled:opacity-60"
        >
          {pending ? 'Dang luu...' : primaryLabel}
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

function SummaryBlock({ label, value }: { label: string; value: string }) {
  return (
    <div className="space-y-1">
      <div className="text-xs font-semibold uppercase tracking-[0.12em] text-[#64748b]">{label}</div>
      <div className="text-base font-semibold text-white">{value}</div>
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
      {pending ? 'Dang chay...' : label}
    </button>
  );
}

function resolvePlacementHint({
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
    return 'Dang tai placement nodes tu cluster hien tai...';
  }
  if (placementNodesError) {
    return `Khong tai duoc placement nodes: ${placementNodesError}`;
  }
  if (!clusterID) {
    return 'Project chua co deployment binding/cluster san sang, nen pinned_node tam thoi bi khoa.';
  }
  if (readyCount === 0) {
    return 'Cluster da lien ket nhung chua co ready node nao de pin placement.';
  }
  return `${readyCount} ready node co san cho pinned_node. Deploy project van tiep tuc chay shared_cluster neu service khong pin rieng.`;
}

function formatPlacementValue(service: ProjectService, placementNodesByID: Map<string, PlacementNode>) {
  const placementMode = service.placement_mode || 'shared_cluster';
  if (placementMode !== 'pinned_node') {
    return 'Shared cluster';
  }

  const node = service.placement_node_id ? placementNodesByID.get(service.placement_node_id) : null;
  if (!node) {
    return `Pinned node ${service.placement_node_id || 'dang cho'}`;
  }
  return `${node.name} · ${node.is_ready ? 'Ready' : formatPlacementNodeStatus(node.status)}`;
}

function formatPlacementNodeStatus(status: string) {
  return status.replace(/_/g, ' ').replace(/\b\w/g, (char) => char.toUpperCase());
}

function formatActionResult(result: ProjectServiceActionResponse) {
  if (result.message) {
    return result.message;
  }
  if (result.action === 'restart') {
    return `Restart da duoc gui cho service ${result.service_name}.`;
  }
  if (result.deployment_id) {
    return `${result.action} da tao deployment ${result.deployment_id}.`;
  }
  return `${result.action} da duoc kich hoat cho ${result.service_name}.`;
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
