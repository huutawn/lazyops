'use client';

import Link from 'next/link';
import type { ReactNode } from 'react';
import { useMemo } from 'react';
import { Boxes, FolderOpen, Rocket, Server } from 'lucide-react';
import { ErrorState } from '@/components/primitives/error-state';
import { LoadingBlock } from '@/components/primitives/loading';
import { SectionCard } from '@/components/primitives/section-card';
import { StatusBadge } from '@/components/primitives/status-badge';
import { useDeployments } from '@/modules/deployments/deployment-hooks';
import { useProjectRuntime } from '@/modules/project-runtime/project-runtime-hooks';
import { useProjects } from '@/modules/projects/project-hooks';
import { ProjectServiceInventory } from '@/modules/project-services/project-service-inventory';
import { useProjectServices } from '@/modules/project-services/project-service-hooks';

type ProjectOverviewDashboardProps = {
  projectId: string;
};

export function ProjectOverviewDashboard({ projectId }: ProjectOverviewDashboardProps) {
  const projects = useProjects();
  const deployments = useDeployments(projectId);
  const services = useProjectServices(projectId);
  const runtime = useProjectRuntime(projectId);

  const project = useMemo(
    () => (projects.data?.items ?? []).find((item) => item.id === projectId) ?? null,
    [projectId, projects.data?.items],
  );

  const latestDeployment = useMemo(() => {
    const items = [...(deployments.data?.items ?? [])];
    items.sort((a, b) => new Date(b.created_at).getTime() - new Date(a.created_at).getTime());
    return items[0] ?? null;
  }, [deployments.data?.items]);

  const serviceItems = services.data?.items ?? [];
  const publicServices = serviceItems.filter((item) => item.public).length;
  const internalServices = serviceItems.filter((item) => item.source_type === 'internal').length;
  const runtimeServices = runtime.data?.services ?? [];
  const runtimeNodes = runtime.data?.nodes ?? [];
  const liveServices = runtimeServices.filter((item) => item.runtime_status === 'live').length;
  const readyNodes = runtimeNodes.filter((item) => item.is_ready).length;

  if (projects.isLoading || deployments.isLoading || services.isLoading || runtime.isLoading) {
    return (
      <SectionCard title="Project namespace" description="Dang dong bo namespace, services, va deploy lane hien tai.">
        <LoadingBlock label="Dang tai service-first dashboard..." className="py-10" />
      </SectionCard>
    );
  }

  if (projects.isError || deployments.isError || services.isError || runtime.isError || !project) {
    return (
      <ErrorState
        title="Khong the tai project dashboard"
        message="Khong dong bo duoc namespace, service inventory, runtime summary, hoac deployment lane cho project nay."
      />
    );
  }

  return (
    <div className="flex flex-col gap-6">
      <div className="grid gap-6 xl:grid-cols-[1.15fr_0.85fr]">
        <SectionCard
          title="Service-first namespace"
          description="Project chi giu namespace va repo context. Service, runtime, node placement, va rollout moi la trung tam thao tac."
        >
          <div className="grid gap-4 lg:grid-cols-2">
            <MetricCard
              icon={<FolderOpen className="size-5 text-[#38bdf8]" />}
              label="Namespace"
              value={project.namespace_slug || project.slug}
              hint="Namespace goc cho toan bo service cua project."
            />
            <MetricCard
              icon={<Server className="size-5 text-[#38bdf8]" />}
              label="Cluster target"
              value={project.cluster_id || 'Dang dung cluster mac dinh'}
              hint="Cluster K3s dang duoc project nay lien ket."
            />
            <MetricCard
              icon={<Boxes className="size-5 text-[#38bdf8]" />}
              label="Services"
              value={String(serviceItems.length)}
              hint={`${publicServices} public / ${internalServices} internal / ${liveServices} live`}
            />
            <MetricCard
              icon={<Rocket className="size-5 text-[#38bdf8]" />}
              label="Runtime"
              value={runtime.data?.sync_state === 'synced' ? (runtime.data?.runtime_mode || project.runtime_mode || 'distributed-k3s') : 'Awaiting deploy'}
              hint={runtime.data?.sync_state === 'synced' ? `${readyNodes}/${runtimeNodes.length} ready node` : runtime.data?.sync_reason || 'Run the first deployment to populate runtime state.'}
            />
          </div>
        </SectionCard>

        <SectionCard
          title="Runtime lane"
          description="Logs / Runtime la cua vao chinh de triage. Topology, deployment state, internal connectivity, va recent logs deu quy ve service."
          actions={
            <Link
              href={`/projects/${projectId}/observability`}
              className="rounded-xl border border-[#334155] px-4 py-2 text-sm font-semibold text-[#e2e8f0] transition-colors hover:bg-[#111827]"
            >
              Open runtime
            </Link>
          }
        >
          <div className="space-y-4">
            <div className="flex flex-wrap gap-2">
              <StatusBadge label="Project = namespace" variant="info" size="sm" />
              <StatusBadge label="Runtime = service-first" variant="success" size="sm" />
              <StatusBadge label="Topology embed in runtime" variant="warning" size="sm" />
            </div>

            <div className="rounded-2xl border border-[#1e293b] bg-[#020617]/70 p-4">
              <div className="text-xs font-semibold uppercase tracking-[0.12em] text-[#64748b]">Runtime snapshot</div>
              <div className="mt-2 text-xl font-bold text-white">
                {runtime.data?.sync_state === 'synced'
                  ? `r${runtime.data.live_revision || latestDeployment?.revision || 0}`
                  : 'Chua co runtime'}
              </div>
              <div className="mt-1 text-sm text-[#94a3b8]">
                {runtime.data?.sync_state === 'synced'
                  ? `${liveServices} live service · ${readyNodes}/${runtimeNodes.length} ready node · ${runtime.data.namespace || project.namespace_slug}`
                  : runtime.data?.sync_reason || 'Khi co rollout moi, trang nay se hien service runtime, placement, va logs preview.'}
              </div>
            </div>

            <div className="grid gap-3 sm:grid-cols-2">
              <ShortcutLink href={`/projects/${projectId}/services`} label="Mo service inventory" />
              <ShortcutLink href={`/projects/${projectId}/observability`} label="Mo logs / runtime" />
              <ShortcutLink href={`/projects/${projectId}/env`} label="Mo env bundle" />
              <ShortcutLink href={`/projects/${projectId}/deployments`} label="Xem rollout history" />
            </div>
          </div>
        </SectionCard>
      </div>

      <ProjectServiceInventory
        projectId={projectId}
        title="Service inventory"
        description="Danh sach nay la trung tam moi cua project. Moi service tu quyet dinh source, exposure, placement, va deploy lane."
      />
    </div>
  );
}

function MetricCard({ icon, label, value, hint }: { icon: ReactNode; label: string; value: string; hint: string }) {
  return (
    <div className="rounded-2xl border border-[#1e293b] bg-[#020617]/70 p-5">
      <div className="mb-3 flex items-center gap-3">
        <div className="rounded-xl border border-[#1e293b] bg-[#0B1120]/80 p-2">{icon}</div>
        <div className="text-xs font-semibold uppercase tracking-[0.12em] text-[#64748b]">{label}</div>
      </div>
      <div className="text-2xl font-bold tracking-tight text-white">{value}</div>
      <div className="mt-2 text-sm text-[#94a3b8]">{hint}</div>
    </div>
  );
}

function ShortcutLink({ href, label }: { href: string; label: string }) {
  return (
    <Link
      href={href}
      className="rounded-2xl border border-[#1e293b] bg-[#0B1120]/60 px-4 py-3 text-sm font-semibold text-[#e2e8f0] transition-colors hover:bg-[#111827]"
    >
      {label}
    </Link>
  );
}
