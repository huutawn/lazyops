'use client';

import { useState } from 'react';
import { zodResolver } from '@hookform/resolvers/zod';
import { useForm } from 'react-hook-form';
import { useMeshNetworks, useCreateMeshNetwork } from '@/modules/mesh-networks/mesh-network-hooks';
import { createMeshNetworkSchema, type CreateMeshNetworkFormData } from '@/modules/mesh-networks/mesh-network-types';
import { getMeshStatusVariant, formatMeshStatus, getProviderLabel } from '@/modules/mesh-networks/mesh-network-utils';
import { PageHeader } from '@/components/primitives/page-header';
import { SectionCard } from '@/components/primitives/section-card';
import { EmptyState } from '@/components/primitives/empty-state';
import { ErrorState } from '@/components/primitives/error-state';
import { SkeletonPage } from '@/components/primitives/skeleton';
import { StatusBadge } from '@/components/primitives/status-badge';
import { Modal } from '@/components/primitives/modal';
import { FormField, FormInput, FormButton } from '@/components/forms/form-fields';

const MESH_EXPLAINER = {
  title: 'What is a distributed mesh?',
  description:
    'A mesh network connects your target machines into a private, encrypted overlay. Services deployed across the mesh can communicate securely regardless of physical location — no VPN or port forwarding required.',
  benefits: [
    'Encrypted peer-to-peer communication between all targets',
    'Services discover each other automatically via magic DNS',
    'No need to manage firewall rules or NAT traversal',
    'Works across cloud providers, data centers, and home labs',
  ],
};

export default function MeshNetworksPage() {
  const { data, isLoading, isError } = useMeshNetworks();
  const [showCreateModal, setShowCreateModal] = useState(false);

  if (isLoading) {
    return <SkeletonPage title cards={2} />;
  }

  if (isError) {
    return (
      <div className="flex flex-col gap-6">
        <PageHeader title="Mesh Networks" subtitle="Manage your distributed mesh networks" />
        <ErrorState title="Failed to load mesh networks" message="Could not fetch mesh network data. Please try again." />
      </div>
    );
  }

  const networks = data?.items ?? [];

  return (
    <div className="flex flex-col gap-6">
      <PageHeader
        title="Mesh Networks"
        subtitle="Create private overlay networks for distributed-mesh deployments."
        actions={
          <button
            type="button"
            className="rounded-lg bg-primary px-4 py-2 text-sm font-semibold text-lazyops-bg transition-colors hover:bg-primary/90"
            onClick={() => setShowCreateModal(true)}
          >
            Create mesh
          </button>
        }
      />

      <SectionCard
        title={MESH_EXPLAINER.title}
        description={MESH_EXPLAINER.description}
      >
        <ul className="grid gap-2 sm:grid-cols-2">
          {MESH_EXPLAINER.benefits.map((benefit) => (
            <li key={benefit} className="flex items-start gap-2 text-sm text-lazyops-muted">
              <svg className="mt-0.5 shrink-0 size-4 text-health-healthy" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                <polyline points="20 6 9 17 4 12" />
              </svg>
              {benefit}
            </li>
          ))}
        </ul>
      </SectionCard>

      {networks.length === 0 ? (
        <SectionCard title="No mesh networks" description="Create your first mesh network to enable distributed deployments.">
          <EmptyState
            title="No mesh networks"
            description="Create a mesh network to connect your targets into a private overlay."
            action={
              <button
                type="button"
                className="rounded-lg bg-primary px-4 py-2 text-sm font-semibold text-lazyops-bg transition-colors hover:bg-primary/90"
                onClick={() => setShowCreateModal(true)}
              >
                Create mesh
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
                  <th className="px-4 py-3 text-left font-medium text-lazyops-muted">CIDR</th>
                  <th className="px-4 py-3 text-left font-medium text-lazyops-muted">Status</th>
                  <th className="px-4 py-3 text-left font-medium text-lazyops-muted">Created</th>
                </tr>
              </thead>
              <tbody>
                {networks.map((network) => (
                  <tr
                    key={network.id}
                    className="border-b border-lazyops-border/50 transition-colors hover:bg-lazyops-border/10"
                  >
                    <td className="px-4 py-3 font-medium text-lazyops-text">{network.name}</td>
                    <td className="px-4 py-3">
                      <StatusBadge
                        label={getProviderLabel(network.provider)}
                        variant="info"
                        size="sm"
                        dot={false}
                      />
                    </td>
                    <td className="px-4 py-3 font-mono text-xs text-lazyops-muted">{network.cidr}</td>
                    <td className="px-4 py-3">
                      <StatusBadge
                        label={formatMeshStatus(network.status)}
                        variant={getMeshStatusVariant(network.status)}
                        size="sm"
                      />
                    </td>
                    <td className="px-4 py-3 text-xs text-lazyops-muted">
                      {new Date(network.created_at).toLocaleDateString()}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </SectionCard>
      )}

      <CreateMeshNetworkModal
        open={showCreateModal}
        onClose={() => setShowCreateModal(false)}
      />
    </div>
  );
}

type CreateMeshNetworkModalProps = {
  open: boolean;
  onClose: () => void;
};

function CreateMeshNetworkModal({ open, onClose }: CreateMeshNetworkModalProps) {
  const {
    register,
    handleSubmit,
    formState: { errors, isSubmitting },
  } = useForm<CreateMeshNetworkFormData>({
    resolver: zodResolver(createMeshNetworkSchema),
    defaultValues: { name: '', provider: 'tailscale', cidr: '' },
  });

  const createMeshNetwork = useCreateMeshNetwork();
  const serverError = createMeshNetwork.error?.message ?? null;

  const onSubmit = (data: CreateMeshNetworkFormData) => {
    return createMeshNetwork.mutateAsync(data).then(() => {
      onClose();
    });
  };

  return (
    <Modal open={open} onClose={onClose} title="Create mesh network" size="md">
      <form onSubmit={handleSubmit(onSubmit)} className="flex flex-col gap-4" noValidate>
        <FormField label="Network name" error={errors.name?.message}>
          <FormInput
            type="text"
            placeholder="prod-mesh"
            error={!!errors.name}
            {...register('name')}
          />
        </FormField>

        <FormField label="Provider" error={errors.provider?.message}>
          <div className="grid grid-cols-2 gap-3">
            {(['tailscale', 'wireguard'] as const).map((provider) => (
              <label
                key={provider}
                className={cn(
                  'flex cursor-pointer items-center gap-3 rounded-lg border px-4 py-3 text-sm transition-colors',
                  'hover:border-lazyops-border/80',
                )}
              >
                <input
                  type="radio"
                  value={provider}
                  {...register('provider')}
                  className="accent-primary"
                />
                <div className="flex flex-col">
                  <span className="font-medium text-lazyops-text">{getProviderLabel(provider)}</span>
                  <span className="text-xs text-lazyops-muted">
                    {provider === 'tailscale'
                      ? 'Zero-config, managed control plane'
                      : 'Open-source, self-hosted'}
                  </span>
                </div>
              </label>
            ))}
          </div>
        </FormField>

        <FormField label="CIDR" error={errors.cidr?.message}>
          <FormInput
            type="text"
            placeholder="100.64.0.0/16"
            error={!!errors.cidr}
            {...register('cidr')}
          />
          <p className="mt-1 text-[10px] text-lazyops-muted/60">
            The IP range for the mesh. Use a /16 or /24 for most setups.
          </p>
        </FormField>

        {serverError && (
          <div className="rounded-lg border border-health-unhealthy/30 bg-health-unhealthy/10 px-3 py-2 text-xs text-health-unhealthy">
            {serverError}
          </div>
        )}

        <FormButton type="submit" loading={isSubmitting || createMeshNetwork.isPending}>
          Create mesh network
        </FormButton>
      </form>
    </Modal>
  );
}

function cn(...classes: (string | false | undefined | null)[]) {
  return classes.filter(Boolean).join(' ');
}
