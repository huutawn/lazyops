'use client';

import Link from 'next/link';
import { useEffect, useMemo, useState, type ReactNode } from 'react';
import { useQuery } from '@tanstack/react-query';
import { useSearchParams } from 'next/navigation';
import { Activity, Boxes, Cpu, Network, Server, Terminal } from 'lucide-react';
import { PageHeader } from '@/components/primitives/page-header';
import { SectionCard } from '@/components/primitives/section-card';
import { EmptyState } from '@/components/primitives/empty-state';
import { ErrorState } from '@/components/primitives/error-state';
import { SkeletonPage } from '@/components/primitives/skeleton';
import { StatusBadge } from '@/components/primitives/status-badge';
import { listProjectLogs } from '@/modules/observability/observability-api';
import { useIncidents, useMetrics } from '@/modules/observability/observability-hooks';
import { useProjectRuntime } from '@/modules/project-runtime/project-runtime-hooks';
import type {
  ProjectRuntimeNode,
  ProjectRuntimeService,
  ProjectRuntimeSummary,
} from '@/modules/project-runtime/project-runtime-types';
import { useTopology } from '@/modules/topology/topology-hooks';

type ProjectRuntimeWorkspaceProps = {
  projectId: string;
};

export function ProjectRuntimeWorkspace({ projectId }: ProjectRuntimeWorkspaceProps) {
  const searchParams = useSearchParams();
  const runtime = useProjectRuntime(projectId);
  const metrics = useMetrics(projectId);
  const incidents = useIncidents(projectId);
  const topology = useTopology(projectId);

  const [serviceFilter, setServiceFilter] = useState('all');
  const [levelFilter, setLevelFilter] = useState('all');
  const [nodeFilter, setNodeFilter] = useState('all');

  const serviceOptions = useMemo(
    () => (runtime.data?.services ?? []).map((item) => item.name),
    [runtime.data?.services],
  );

  useEffect(() => {
    const requestedService = searchParams.get('service');
    if (!requestedService) {
      return;
    }
    if (serviceOptions.includes(requestedService)) {
      setServiceFilter(requestedService);
    }
  }, [searchParams, serviceOptions]);

  useEffect(() => {
    if (serviceFilter !== 'all' && !serviceOptions.includes(serviceFilter)) {
      setServiceFilter('all');
    }
  }, [serviceFilter, serviceOptions]);

  const nodeMap = useMemo(
    () => new Map((runtime.data?.nodes ?? []).map((node) => [node.instance_id, node])),
    [runtime.data?.nodes],
  );
  const metricMap = useMemo(
    () => new Map((metrics.data ?? []).map((metric) => [metric.service, metric])),
    [metrics.data],
  );

  const filteredServices = useMemo(() => {
    const items = runtime.data?.services ?? [];
    return items.filter((item) => {
      if (serviceFilter !== 'all' && item.name !== serviceFilter) {
        return false;
      }
      if (levelFilter !== 'all' && normalizeRuntimeLevel(item.runtime_status) !== levelFilter) {
        return false;
      }
      if (nodeFilter !== 'all') {
        const effective = item.effective_node_ids ?? [];
        if (!effective.includes(nodeFilter) && item.requested_node_id !== nodeFilter) {
          return false;
        }
      }
      return true;
    });
  }, [runtime.data?.services, serviceFilter, levelFilter, nodeFilter]);

  const deepLogs = useQuery({
    queryKey: ['runtime-deep-logs', projectId, serviceFilter],
    queryFn: async () => {
      const result = await listProjectLogs(projectId, {
        service: serviceFilter === 'all' ? undefined : serviceFilter,
        limit: 30,
      });
      if (result.error) {
        throw new Error(result.error.message);
      }
      return result.data?.items ?? [];
    },
    enabled: !!projectId,
    staleTime: 10 * 1000,
  });

  if (runtime.isLoading) {
    return <SkeletonPage title cards={4} />;
  }

  if (runtime.isError || !runtime.data) {
    return (
      <div className="flex flex-col gap-6">
        <PageHeader title="Logs / Runtime" subtitle="Service-first runtime workspace for this project." />
        <ErrorState
          title="Failed to load runtime workspace"
          message={runtime.error instanceof Error ? runtime.error.message : 'Could not load runtime summary.'}
        />
      </div>
    );
  }

  const data = runtime.data;
  const readyNodes = data.nodes.filter((item) => item.is_ready).length;
  const liveServices = data.services.filter((item) => item.runtime_status === 'live').length;
  const publicServices = data.services.filter((item) => item.public).length;
  const activeIncidents = (incidents.data ?? []).filter((item) => item.status === 'open' || item.status === 'investigating').length;

  return (
    <div className="mx-auto flex w-full max-w-[1500px] flex-col gap-8 py-6 lg:px-8">
      <PageHeader
        title="Logs / Runtime"
        subtitle="Service-first runtime workspace. Track rollout state, node placement, internal connectivity, and recent logs without leaving the project shell."
        actions={
          <div className="flex flex-wrap items-center gap-2">
            <Link
              href={`/projects/${projectId}/services`}
              className="rounded-xl border border-[#334155] bg-[#0B1120]/70 px-6 py-2 text-base font-semibold text-white transition-colors hover:bg-[#111827]"
            >
              Edit services
            </Link>
            <Link
              href={`/projects/${projectId}/deployments`}
              className="rounded-xl bg-[#0EA5E9] px-6 py-2 text-base font-semibold text-[#020617] transition-opacity hover:opacity-90"
            >
              Open deployments
            </Link>
          </div>
        }
      />

      <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-4">
        <RuntimeHeroCard icon={<Boxes className="size-5 text-[#38bdf8]" />} label="Services" value={String(data.services.length)} hint={`${liveServices} live · ${publicServices} public`} />
        <RuntimeHeroCard icon={<Server className="size-5 text-[#38bdf8]" />} label="Cluster nodes" value={String(data.nodes.length)} hint={`${readyNodes} ready`} />
        <RuntimeHeroCard icon={<Activity className="size-5 text-[#38bdf8]" />} label="Live revision" value={data.live_revision ? `r${data.live_revision}` : 'None yet'} hint={data.sync_state === 'synced' ? data.runtime_mode || 'runtime active' : data.sync_reason || 'Waiting for first deploy'} />
        <RuntimeHeroCard icon={<Cpu className="size-5 text-[#38bdf8]" />} label="Incidents" value={String(activeIncidents)} hint={activeIncidents > 0 ? 'Needs attention' : 'All clear'} />
      </div>

      <SectionCard
        title="Runtime focus"
        description="Filter theo service trước, rồi mới drill into level hoặc node để điều tra sâu."
      >
        <div className="grid gap-4 lg:grid-cols-[1.2fr_0.8fr_0.8fr]">
          <FilterField label="Service">
            <select value={serviceFilter} onChange={(event) => setServiceFilter(event.target.value)} className={filterClassName}>
              <option value="all">All services</option>
              {serviceOptions.map((item) => (
                <option key={item} value={item}>
                  {item}
                </option>
              ))}
            </select>
          </FilterField>
          <FilterField label="Runtime state">
            <select value={levelFilter} onChange={(event) => setLevelFilter(event.target.value)} className={filterClassName}>
              <option value="all">All states</option>
              <option value="healthy">Healthy</option>
              <option value="pending">Pending</option>
              <option value="degraded">Degraded</option>
            </select>
          </FilterField>
          <FilterField label="Node">
            <select value={nodeFilter} onChange={(event) => setNodeFilter(event.target.value)} className={filterClassName}>
              <option value="all">All nodes</option>
              {data.nodes.map((item) => (
                <option key={item.instance_id} value={item.instance_id}>
                  {item.name}
                </option>
              ))}
            </select>
          </FilterField>
        </div>
      </SectionCard>

      <div className="grid gap-6 xl:grid-cols-[1.2fr_0.8fr]">
        <SectionCard
          title="Service runtime"
          description={resolveRuntimeSummaryText(data)}
          actions={
            <div className="flex flex-wrap gap-2">
              <StatusBadge label={data.sync_state} variant={data.sync_state === 'synced' ? 'success' : 'warning'} size="sm" dot={false} />
              {data.runtime_mode ? <StatusBadge label={data.runtime_mode} variant="info" size="sm" dot={false} /> : null}
            </div>
          }
        >
          {filteredServices.length === 0 ? (
            <EmptyState
              title="No services match this runtime filter"
              description="Try resetting the service or node filter. If this is your first rollout, deploy the project once to populate runtime data."
            />
          ) : (
            <div className="grid gap-4">
              {filteredServices.map((service) => (
                <RuntimeServiceCard
                  key={service.service_id}
                  projectId={projectId}
                  runtime={data}
                  service={service}
                  nodeMap={nodeMap}
                  hasMetric={metricMap.has(service.name)}
                />
              ))}
            </div>
          )}
        </SectionCard>

        <div className="grid gap-6">
          <RuntimeTopologyPanel runtime={data} />
          <RuntimeNodePanel nodes={data.nodes} />
          <RuntimeDeepLogPanel
            serviceFilter={serviceFilter}
            logs={deepLogs.data ?? []}
            isLoading={deepLogs.isLoading}
            isError={deepLogs.isError}
            errorMessage={deepLogs.error instanceof Error ? deepLogs.error.message : 'Could not load deep log preview.'}
          />
        </div>
      </div>
    </div>
  );
}

function RuntimeHeroCard({
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
    <div className="rounded-[28px] border border-[#1e293b] bg-[radial-gradient(circle_at_top_left,_rgba(14,165,233,0.16),_transparent_42%),linear-gradient(180deg,rgba(15,23,42,0.96),rgba(2,6,23,0.92))] p-6 shadow-[0_24px_80px_rgba(2,6,23,0.45)]">
      <div className="mb-4 flex items-center gap-3">
        <div className="rounded-2xl border border-[#1e293b] bg-[#0B1120]/80 p-3">{icon}</div>
        <div className="text-sm font-semibold uppercase tracking-[0.16em] text-[#64748b]">{label}</div>
      </div>
      <div className="text-4xl font-black tracking-tight text-white">{value}</div>
      <div className="mt-2 text-base text-[#94a3b8]">{hint}</div>
    </div>
  );
}

function RuntimeServiceCard({
  projectId,
  runtime,
  service,
  nodeMap,
  hasMetric,
}: {
  projectId: string;
  runtime: ProjectRuntimeSummary;
  service: ProjectRuntimeService;
  nodeMap: Map<string, ProjectRuntimeNode>;
  hasMetric: boolean;
}) {
  const requestedNode = service.requested_node_id ? (nodeMap.get(service.requested_node_id) ?? null) : null;
  const effectiveNodes = (service.effective_node_ids ?? []).map((id) => nodeMap.get(id)).filter(Boolean) as ProjectRuntimeNode[];
  const dependencyState = summarizeDependencies(service);
  const runtimeVariant = runtimeStatusVariant(service.runtime_status);

  return (
    <article className="rounded-[26px] border border-[#1e293b] bg-[#020617]/85 p-6 shadow-[0_18px_60px_rgba(2,6,23,0.42)]">
      <div className="flex flex-col gap-5 lg:flex-row lg:items-start lg:justify-between">
        <div className="space-y-3">
          <div className="flex flex-wrap items-center gap-3">
            <h3 className="text-4xl font-black tracking-tight text-white">{service.name}</h3>
            <StatusBadge label={service.runtime_status} variant={runtimeVariant} size="sm" />
            {service.public ? <StatusBadge label="public" variant="info" size="sm" dot={false} /> : <StatusBadge label="private" variant="neutral" size="sm" dot={false} />}
            {service.source_type ? <StatusBadge label={service.source_type} variant="default" size="sm" dot={false} /> : null}
          </div>
          <p className="max-w-3xl text-base leading-relaxed text-[#94a3b8]">
            {service.runtime_reason || 'Runtime state will appear here after the first deployment.'}
          </p>
        </div>

        <div className="flex flex-wrap gap-2">
          <Link
            href={`/projects/${projectId}/services`}
            className="rounded-xl border border-[#334155] px-3 py-2 text-sm font-semibold text-white transition-colors hover:bg-[#111827]"
          >
            Edit service
          </Link>
          {service.deployment_id ? (
            <Link
              href={`/projects/${projectId}/deployments/${service.deployment_id}`}
              className="rounded-xl border border-[#0EA5E9] bg-[#0EA5E9]/10 px-3 py-2 text-sm font-semibold text-[#38bdf8] transition-colors hover:bg-[#0EA5E9]/20"
            >
              Open deployment
            </Link>
          ) : null}
        </div>
      </div>

      <div className="mt-5 grid gap-4 md:grid-cols-2 xl:grid-cols-4">
        <MetricTile label="Revision" value={service.revision ? `r${service.revision}` : 'Not deployed'} subvalue={service.rollout_state || service.build_state || 'No rollout yet'} />
        <MetricTile label="Image" value={service.image_ref || 'Pending image'} subvalue={service.image_digest || 'No digest yet'} mono />
        <MetricTile label="Placement" value={formatPlacementValue(service, requestedNode, effectiveNodes)} subvalue={service.placement_mode || 'shared_cluster'} />
        <MetricTile label="Signals" value={hasMetric ? 'Metrics live' : 'No metrics yet'} subvalue={service.recent_logs.length > 0 ? `${service.recent_logs.length} recent logs` : 'No logs yet'} />
      </div>

      <div className="mt-5 grid gap-4 xl:grid-cols-[0.9fr_1.1fr]">
        <div className="rounded-2xl border border-[#1e293b] bg-[#0B1120]/70 p-6">
          <div className="mb-3 flex items-center gap-2 text-base font-semibold text-white">
            <Network className="size-4 text-[#38bdf8]" />
            Connectivity
          </div>
          <div className="grid gap-3">
            <InfoRow label="Internal endpoints" value={service.internal_endpoints.length > 0 ? service.internal_endpoints.join(' · ') : 'No endpoint yet'} />
            <InfoRow label="Public URLs" value={service.public_urls.length > 0 ? service.public_urls.join(' · ') : service.public ? 'Waiting for public URL' : 'Private only'} />
            <InfoRow label="Dependencies" value={dependencyState} />
          </div>
          {service.dependencies.length > 0 ? (
            <div className="mt-4 grid gap-2">
              {service.dependencies.map((dependency) => (
                <div key={dependency.service_name} className="rounded-xl border border-[#1e293b] bg-[#020617]/70 px-3 py-2 text-base">
                  <div className="flex items-center justify-between gap-3">
                    <span className="font-semibold text-white">{dependency.service_name}</span>
                    <StatusBadge label={dependency.status} variant={dependencyStatusVariant(dependency.status)} size="sm" dot={false} />
                  </div>
                  <div className="mt-1 text-sm text-[#94a3b8]">{dependency.status_reason || dependency.internal_endpoint || 'No extra dependency detail.'}</div>
                </div>
              ))}
            </div>
          ) : null}
        </div>

        <div className="rounded-2xl border border-[#1e293b] bg-[#0B1120]/70 p-6">
          <div className="mb-3 flex items-center gap-2 text-base font-semibold text-white">
            <Terminal className="size-4 text-[#38bdf8]" />
            Recent logs
          </div>
          {service.recent_logs.length === 0 ? (
            <RuntimeEmptyCopy
              title={service.deployment_id ? 'No logs yet' : 'Service not deployed yet'}
              description={service.deployment_id ? 'The service has runtime state but no recent logs were recorded yet.' : 'Deploy this service or the whole project to start collecting logs.'}
            />
          ) : (
            <div className="grid gap-2">
              {service.recent_logs.map((line) => (
                <div key={`${service.service_id}-${line.timestamp}-${line.message}`} className="rounded-xl border border-[#1e293b] bg-[#020617]/80 px-3 py-2 font-mono text-sm">
                  <div className="mb-1 flex flex-wrap items-center gap-2 text-[#64748b]">
                    <span>{new Date(line.timestamp).toLocaleTimeString()}</span>
                    <span className={logLevelClassName(line.level)}>[{line.level.toUpperCase()}]</span>
                    {line.node ? <span>{line.node}</span> : null}
                    {line.source ? <span>{line.source}</span> : null}
                  </div>
                  <div className="break-words text-[#e2e8f0]">{line.message}</div>
                </div>
              ))}
            </div>
          )}
        </div>
      </div>

      {service.requested_node_id && !requestedNode ? (
        <div className="mt-4 rounded-2xl border border-[#ef4444]/30 bg-[#ef4444]/10 px-6 py-3 text-base text-[#fecaca]">
          Service pinning references node <code>{service.requested_node_id}</code>, but that node is currently unavailable in cluster inventory.
        </div>
      ) : null}

      {runtime.nodes.length === 0 && runtime.cluster_id ? (
        <div className="mt-4 rounded-2xl border border-[#f59e0b]/30 bg-[#f59e0b]/10 px-6 py-3 text-base text-[#fde68a]">
          The project is linked to a cluster, but runtime node inventory is still empty. This usually means the cluster has not reported any Ready node yet.
        </div>
      ) : null}
    </article>
  );
}

function RuntimeTopologyPanel({ runtime }: { runtime: ProjectRuntimeSummary }) {
  const topology = useTopology(runtime.project_id);
  const serviceNodes = topology.data?.nodes.filter((item) => item.kind === 'service') ?? [];
  const clusterNodes = topology.data?.nodes.filter((item) => item.kind === 'cluster' || item.kind === 'instance') ?? [];

  return (
    <SectionCard
      title="Topology panel"
      description="Topology stays embedded here for service-first triage. The old dedicated topology route can remain for debug-only exploration."
    >
      {topology.isLoading ? (
        <div className="text-base text-[#94a3b8]">Loading topology…</div>
      ) : topology.isError || !topology.data || topology.data.nodes.length === 0 ? (
        <RuntimeEmptyCopy
          title="No topology yet"
          description="Deploy the project once or wait for topology sync events to populate service and node relationships."
        />
      ) : (
        <div className="grid gap-4">
          <div className="grid gap-3 sm:grid-cols-3">
            <MiniStat label="Runtime mode" value={topology.data.runtime_mode || runtime.runtime_mode || 'unknown'} />
            <MiniStat label="Nodes" value={String(topology.data.nodes.length)} />
            <MiniStat label="Edges" value={String(topology.data.edges.length)} />
          </div>
          <div className="grid gap-2">
            {serviceNodes.slice(0, 4).map((node) => (
              <div key={node.id} className="rounded-xl border border-[#1e293b] bg-[#020617]/70 px-3 py-2 text-base">
                <div className="flex items-center justify-between gap-3">
                  <span className="font-semibold text-white">{node.label}</span>
                  <StatusBadge label={node.status} variant={topologyStatusVariant(node.status)} size="sm" dot={false} />
                </div>
                <div className="mt-1 text-sm text-[#94a3b8]">{node.kind} · {node.runtime_mode || 'service-first runtime'}</div>
              </div>
            ))}
          </div>
          <div className="rounded-xl border border-[#1e293b] bg-[#020617]/70 px-3 py-3 text-sm text-[#94a3b8]">
            {clusterNodes.length} infrastructure node(s) and {serviceNodes.length} service node(s) are currently visible in topology sync.
          </div>
        </div>
      )}
    </SectionCard>
  );
}

function RuntimeNodePanel({ nodes }: { nodes: ProjectRuntimeNode[] }) {
  return (
    <SectionCard title="Cluster nodes" description="Placement and readiness snapshot for the nodes this project can target.">
      {nodes.length === 0 ? (
        <RuntimeEmptyCopy
          title="No runtime nodes available"
          description="The project either is not bound to a managed cluster yet or the cluster has not reported ready nodes."
        />
      ) : (
        <div className="grid gap-2">
          {nodes.map((node) => (
            <div key={node.instance_id} className="rounded-xl border border-[#1e293b] bg-[#020617]/70 px-3 py-3">
              <div className="flex items-center justify-between gap-3">
                <div>
                  <div className="font-semibold text-white">{node.name}</div>
                  <div className="text-sm text-[#94a3b8]">{node.k8s_node_name || node.instance_id}</div>
                </div>
                <StatusBadge label={node.is_ready ? 'ready' : node.status} variant={node.is_ready ? 'success' : topologyStatusVariant(node.status)} size="sm" dot={false} />
              </div>
            </div>
          ))}
        </div>
      )}
    </SectionCard>
  );
}

function RuntimeDeepLogPanel({
  serviceFilter,
  logs,
  isLoading,
  isError,
  errorMessage,
}: {
  serviceFilter: string;
  logs: Array<{ id: string; service: string; source?: string; level: string; message: string; timestamp: string; node?: string }>;
  isLoading: boolean;
  isError: boolean;
  errorMessage: string;
}) {
  return (
    <SectionCard title="Deep log drill-down" description="This panel still uses the existing logs API for deeper investigation once you move past the runtime preview.">
      {isLoading ? (
        <div className="text-base text-[#94a3b8]">Loading logs…</div>
      ) : isError ? (
        <div className="rounded-xl border border-[#ef4444]/30 bg-[#ef4444]/10 px-6 py-3 text-base text-[#fecaca]">{errorMessage}</div>
      ) : logs.length === 0 ? (
        <RuntimeEmptyCopy
          title={serviceFilter === 'all' ? 'No logs for current filters' : `No deep logs for ${serviceFilter}`}
          description="Recent runtime previews are shown on each service card. This drill-down stays empty until the logs pipeline records deeper history."
        />
      ) : (
        <div className="grid gap-2">
          {logs.slice(0, 12).map((line) => (
            <div key={line.id} className="rounded-xl border border-[#1e293b] bg-[#020617]/80 px-3 py-2 font-mono text-sm">
              <div className="mb-1 flex flex-wrap items-center gap-2 text-[#64748b]">
                <span>{new Date(line.timestamp).toLocaleTimeString()}</span>
                <span className={logLevelClassName(line.level)}>[{line.level.toUpperCase()}]</span>
                <span>{line.service}</span>
                {line.node ? <span>{line.node}</span> : null}
                {line.source ? <span>{line.source}</span> : null}
              </div>
              <div className="break-words text-[#e2e8f0]">{line.message}</div>
            </div>
          ))}
        </div>
      )}
    </SectionCard>
  );
}

function FilterField({ label, children }: { label: string; children: ReactNode }) {
  return (
    <label className="grid gap-2">
      <span className="text-sm font-semibold uppercase tracking-[0.14em] text-[#64748b]">{label}</span>
      {children}
    </label>
  );
}

function MetricTile({
  label,
  value,
  subvalue,
  mono = false,
}: {
  label: string;
  value: string;
  subvalue: string;
  mono?: boolean;
}) {
  return (
    <div className="min-w-0 rounded-2xl border border-[#1e293b] bg-[#0B1120]/80 p-6">
      <div className="text-[11px] font-semibold uppercase tracking-[0.14em] text-[#64748b]">{label}</div>
      <div
        className={`mt-2 min-w-0 whitespace-normal text-base font-bold text-white ${
          mono ? 'break-all font-mono text-sm leading-6 lg:text-base' : 'break-words'
        }`}
      >
        {value}
      </div>
      <div className="mt-1 break-all text-sm text-[#94a3b8]">{subvalue}</div>
    </div>
  );
}

function InfoRow({ label, value }: { label: string; value: string }) {
  return (
    <div className="grid gap-1">
      <div className="text-[11px] font-semibold uppercase tracking-[0.14em] text-[#64748b]">{label}</div>
      <div className="break-all text-base text-[#e2e8f0]">{value}</div>
    </div>
  );
}

function RuntimeEmptyCopy({ title, description }: { title: string; description: string }) {
  return (
    <div className="rounded-2xl border border-dashed border-[#334155] bg-[#020617]/70 p-6">
      <div className="text-base font-semibold text-white">{title}</div>
      <div className="mt-1 text-base text-[#94a3b8]">{description}</div>
    </div>
  );
}

function MiniStat({ label, value }: { label: string; value: string }) {
  return (
    <div className="rounded-xl border border-[#1e293b] bg-[#020617]/70 px-3 py-3">
      <div className="text-[10px] font-semibold uppercase tracking-[0.14em] text-[#64748b]">{label}</div>
      <div className="mt-1 text-base font-bold text-white">{value}</div>
    </div>
  );
}

function resolveRuntimeSummaryText(runtime: ProjectRuntimeSummary) {
  if (runtime.sync_state !== 'synced') {
    return runtime.sync_reason || 'Deploy the project once to start seeing runtime, logs, and placement summaries.';
  }
  return `${runtime.services.length} services · ${runtime.nodes.length} nodes · namespace ${runtime.namespace || 'unknown'}`;
}

function runtimeStatusVariant(status: string): 'success' | 'warning' | 'danger' | 'info' | 'neutral' {
  switch (status) {
    case 'live':
      return 'success';
    case 'deploying':
    case 'waiting_for_node':
      return 'warning';
    case 'degraded':
      return 'danger';
    case 'configured':
      return 'info';
    default:
      return 'neutral';
  }
}

function dependencyStatusVariant(status: string): 'success' | 'warning' | 'danger' | 'info' | 'neutral' {
  switch (status) {
    case 'ready':
      return 'success';
    case 'configured':
      return 'info';
    case 'degraded':
      return 'warning';
    default:
      return 'danger';
  }
}

function topologyStatusVariant(status: string): 'success' | 'warning' | 'danger' | 'info' | 'neutral' {
  switch (status) {
    case 'healthy':
    case 'ready':
      return 'success';
    case 'degraded':
    case 'running':
      return 'warning';
    case 'unhealthy':
    case 'failed':
    case 'offline':
      return 'danger';
    default:
      return 'neutral';
  }
}

function normalizeRuntimeLevel(status: string) {
  switch (status) {
    case 'live':
      return 'healthy';
    case 'deploying':
    case 'configured':
    case 'waiting_for_node':
      return 'pending';
    default:
      return 'degraded';
  }
}

function summarizeDependencies(service: ProjectRuntimeService) {
  if (service.dependencies.length === 0) {
    return 'No direct internal dependency';
  }
  return service.dependencies.map((item) => `${item.service_name}:${item.status}`).join(' · ');
}

function formatPlacementValue(
  service: ProjectRuntimeService,
  requestedNode: ProjectRuntimeNode | null,
  effectiveNodes: ProjectRuntimeNode[],
) {
  if ((service.effective_node_ids ?? []).length > 0 && effectiveNodes.length > 0) {
    return effectiveNodes.map((node) => node.name).join(', ');
  }
  if (requestedNode) {
    return `${requestedNode.name} (requested)`;
  }
  if (service.placement_mode === 'pinned_node' && service.requested_node_id) {
    return service.requested_node_id;
  }
  return 'Shared cluster';
}

function logLevelClassName(level: string) {
  switch (level) {
    case 'error':
      return 'text-[#ef4444]';
    case 'warn':
      return 'text-[#f59e0b]';
    default:
      return 'text-[#38bdf8]';
  }
}

const filterClassName =
  'w-full rounded-2xl border border-[#1e293b] bg-[#020617]/80 px-6 py-3 text-base font-medium text-white outline-none transition-colors focus:border-[#0EA5E9]';
