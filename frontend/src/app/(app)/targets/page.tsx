'use client';

import Link from 'next/link';
import { PageHeader } from '@/components/primitives/page-header';
import { SectionCard } from '@/components/primitives/section-card';
import { ErrorState } from '@/components/primitives/error-state';
import { SkeletonPage } from '@/components/primitives/skeleton';
import { StatusBadge } from '@/components/primitives/status-badge';
import { useInstances } from '@/modules/instances/instance-hooks';
import { useMeshNetworks } from '@/modules/mesh-networks/mesh-network-hooks';
import { useClusters } from '@/modules/clusters/cluster-hooks';

function TargetCard({
  title,
  description,
  href,
  count,
}: {
  title: string;
  description: string;
  href: string;
  count: number;
}) {
  return (
    <Link
      href={href}
      className="rounded-xl border border-lazyops-border bg-lazyops-card p-4 transition-colors hover:bg-lazyops-border/10"
    >
      <div className="mb-2 flex items-center justify-between">
        <h3 className="text-sm font-semibold text-lazyops-text">{title}</h3>
        <StatusBadge label={`${count}`} variant="neutral" size="sm" dot={false} />
      </div>
      <p className="text-xs text-lazyops-muted">{description}</p>
    </Link>
  );
}

export default function TargetsPage() {
  const instances = useInstances();
  const meshes = useMeshNetworks();
  const clusters = useClusters();

  const isLoading = instances.isLoading || meshes.isLoading || clusters.isLoading;
  const isError = instances.isError || meshes.isError || clusters.isError;

  if (isLoading) {
    return <SkeletonPage title cards={1} />;
  }

  if (isError) {
    return (
      <div className="flex flex-col gap-6">
        <PageHeader title="Targets" subtitle="Deployment destinations managed by LazyOps." />
        <ErrorState title="Failed to load targets" message="Could not fetch target data from API." />
      </div>
    );
  }

  const instanceCount = instances.data?.items?.length ?? 0;
  const meshCount = meshes.data?.items?.length ?? 0;
  const clusterCount = clusters.data?.items?.length ?? 0;

  return (
    <div className="flex flex-col gap-6">
      <PageHeader title="Targets" subtitle="Instances, mesh networks, and clusters used for placement." />

      <SectionCard title="Target groups" description="Open each target type to create, inspect, and manage resources.">
        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
          <TargetCard
            title="Instances"
            description="Standalone hosts where agents run and receive deployments."
            href="/instances"
            count={instanceCount}
          />
          <TargetCard
            title="Mesh Networks"
            description="Network overlays for routing and cross-instance connectivity."
            href="/mesh-networks"
            count={meshCount}
          />
          <TargetCard
            title="Clusters"
            description="K3s/Kubernetes clusters enrolled as deployment targets."
            href="/clusters"
            count={clusterCount}
          />
        </div>
      </SectionCard>
    </div>
  );
}
