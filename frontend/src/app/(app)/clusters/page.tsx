'use client';

import { useState } from 'react';
import { zodResolver } from '@hookform/resolvers/zod';
import { useForm } from 'react-hook-form';
import { useClusters, useCreateCluster } from '@/modules/clusters/cluster-hooks';
import { createClusterSchema, type CreateClusterFormData } from '@/modules/clusters/cluster-types';
import { getClusterStatusVariant, formatClusterStatus } from '@/modules/clusters/cluster-utils';
import { PageHeader } from '@/components/primitives/page-header';
import { SectionCard } from '@/components/primitives/section-card';
import { EmptyState } from '@/components/primitives/empty-state';
import { ErrorState } from '@/components/primitives/error-state';
import { SkeletonPage } from '@/components/primitives/skeleton';
import { StatusBadge } from '@/components/primitives/status-badge';
import { Modal } from '@/components/primitives/modal';
import { FormField, FormInput, FormButton } from '@/components/forms/form-fields';

const CLUSTER_EXPLAINER = {
  title: 'What is a K3s cluster target?',
  description:
    'Connect an existing K3s (lightweight Kubernetes) cluster to LazyOps. Once linked, you can deploy services to the cluster without managing Kubernetes manifests directly — LazyOps handles the complexity.',
  points: [
    'LazyOps validates the cluster connection and readiness',
    'Services are deployed through deployment bindings, not raw YAML',
    'You keep full control — LazyOps never modifies cluster config',
    'Works with any K3s cluster you already manage',
  ],
};

export default function ClustersPage() {
  const { data, isLoading, isError } = useClusters();
  const [showCreateModal, setShowCreateModal] = useState(false);

  if (isLoading) {
    return <SkeletonPage title cards={2} />;
  }

  if (isError) {
    return (
      <div className="flex flex-col gap-6">
        <PageHeader title="Clusters" subtitle="Manage your K3s cluster targets" />
        <ErrorState title="Failed to load clusters" message="Could not fetch cluster data. Please try again." />
      </div>
    );
  }

  const clusters = data?.items ?? [];

  return (
    <div className="flex flex-col gap-6">
      <PageHeader
        title="Clusters"
        subtitle="Connect K3s clusters for orchestrated deployments."
        actions={
          <button
            type="button"
            className="rounded-lg bg-primary px-4 py-2 text-sm font-semibold text-lazyops-bg transition-colors hover:bg-primary/90"
            onClick={() => setShowCreateModal(true)}
          >
            Add cluster
          </button>
        }
      />

      <SectionCard
        title={CLUSTER_EXPLAINER.title}
        description={CLUSTER_EXPLAINER.description}
      >
        <ul className="grid gap-2 sm:grid-cols-2">
          {CLUSTER_EXPLAINER.points.map((point) => (
            <li key={point} className="flex items-start gap-2 text-sm text-lazyops-muted">
              <svg className="mt-0.5 shrink-0 size-4 text-health-healthy" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                <polyline points="20 6 9 17 4 12" />
              </svg>
              {point}
            </li>
          ))}
        </ul>
      </SectionCard>

      {clusters.length === 0 ? (
        <SectionCard title="No clusters" description="Connect your first K3s cluster to enable orchestrated deployments.">
          <EmptyState
            title="No clusters connected"
            description="Add a K3s cluster to use it as a deployment target for distributed-k3s mode."
            action={
              <button
                type="button"
                className="rounded-lg bg-primary px-4 py-2 text-sm font-semibold text-lazyops-bg transition-colors hover:bg-primary/90"
                onClick={() => setShowCreateModal(true)}
              >
                Add cluster
              </button>
            }
          />
        </SectionCard>
      ) : (
        <SectionCard>
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead>
                <tr className="border-b border-lazyops-border">
                  <th className="px-4 py-3 text-left font-medium text-lazyops-muted">Name</th>
                  <th className="px-4 py-3 text-left font-medium text-lazyops-muted">Provider</th>
                  <th className="px-4 py-3 text-left font-medium text-lazyops-muted">Status</th>
                  <th className="px-4 py-3 text-left font-medium text-lazyops-muted">Kubeconfig Ref</th>
                  <th className="px-4 py-3 text-left font-medium text-lazyops-muted">Created</th>
                </tr>
              </thead>
              <tbody>
                {clusters.map((cluster) => (
                  <tr
                    key={cluster.id}
                    className="border-b border-lazyops-border/50 transition-colors hover:bg-lazyops-border/10"
                  >
                    <td className="px-4 py-3 font-medium text-lazyops-text">{cluster.name}</td>
                    <td className="px-4 py-3">
                      <StatusBadge label="K3s" variant="info" size="sm" dot={false} />
                    </td>
                    <td className="px-4 py-3">
                      <StatusBadge
                        label={formatClusterStatus(cluster.status)}
                        variant={getClusterStatusVariant(cluster.status)}
                        size="sm"
                      />
                    </td>
                    <td className="px-4 py-3 font-mono text-xs text-lazyops-muted">
                      {cluster.updated_at ? 'Configured' : '—'}
                    </td>
                    <td className="px-4 py-3 text-xs text-lazyops-muted">
                      {new Date(cluster.created_at).toLocaleDateString()}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </SectionCard>
      )}

      <CreateClusterModal
        open={showCreateModal}
        onClose={() => setShowCreateModal(false)}
      />
    </div>
  );
}

type CreateClusterModalProps = {
  open: boolean;
  onClose: () => void;
};

function CreateClusterModal({ open, onClose }: CreateClusterModalProps) {
  const {
    register,
    handleSubmit,
    formState: { errors, isSubmitting },
  } = useForm<CreateClusterFormData>({
    resolver: zodResolver(createClusterSchema),
    defaultValues: { name: '', provider: 'k3s', kubeconfig_secret_ref: '' },
  });

  const createCluster = useCreateCluster();
  const serverError = createCluster.error?.message ?? null;

  const onSubmit = (data: CreateClusterFormData) => {
    return createCluster.mutateAsync(data).then(() => {
      onClose();
    });
  };

  return (
    <Modal open={open} onClose={onClose} title="Add K3s cluster" size="md">
      <form onSubmit={handleSubmit(onSubmit)} className="flex flex-col gap-4" noValidate>
        <FormField label="Cluster name" error={errors.name?.message}>
          <FormInput
            type="text"
            placeholder="prod-k3s"
            error={!!errors.name}
            {...register('name')}
          />
        </FormField>

        <FormField label="Provider">
          <div className="rounded-lg border border-primary/30 bg-primary/10 px-4 py-3 text-sm text-primary">
            K3s (lightweight Kubernetes)
          </div>
          <p className="mt-1 text-[10px] text-lazyops-muted/60">
            Currently only K3s clusters are supported.
          </p>
        </FormField>

        <FormField label="Kubeconfig secret reference" error={errors.kubeconfig_secret_ref?.message}>
          <FormInput
            type="text"
            placeholder="secret-name"
            error={!!errors.kubeconfig_secret_ref}
            {...register('kubeconfig_secret_ref')}
          />
          <p className="mt-1 text-[10px] text-lazyops-muted/60">
            The name of the secret that contains your kubeconfig. This is how LazyOps authenticates with your cluster.
          </p>
        </FormField>

        {serverError && (
          <div className="rounded-lg border border-health-unhealthy/30 bg-health-unhealthy/10 px-3 py-2 text-xs text-health-unhealthy">
            {serverError}
          </div>
        )}

        <FormButton type="submit" loading={isSubmitting || createCluster.isPending}>
          Add cluster
        </FormButton>
      </form>
    </Modal>
  );
}
