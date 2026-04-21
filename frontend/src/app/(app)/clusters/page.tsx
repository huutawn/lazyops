'use client';

import { useEffect, useMemo, useState } from 'react';
import { zodResolver } from '@hookform/resolvers/zod';
import { useForm } from 'react-hook-form';
import { Boxes, Plus, Server, ShieldCheck } from 'lucide-react';
import {
  useClusterNodes,
  useClusters,
  useConnectClusterNode,
  useCreateCluster,
} from '@/modules/clusters/cluster-hooks';
import {
  connectClusterNodeSchema,
  createClusterSchema,
  type ClusterNode,
  type ClusterSummary,
  type ConnectClusterNodeFormData,
  type CreateClusterFormData,
} from '@/modules/clusters/cluster-types';
import { getClusterStatusVariant, formatClusterStatus } from '@/modules/clusters/cluster-utils';
import { PageHeader } from '@/components/primitives/page-header';
import { SectionCard } from '@/components/primitives/section-card';
import { EmptyState } from '@/components/primitives/empty-state';
import { ErrorState } from '@/components/primitives/error-state';
import { SkeletonPage } from '@/components/primitives/skeleton';
import { StatusBadge } from '@/components/primitives/status-badge';
import { Modal } from '@/components/primitives/modal';
import { Drawer } from '@/components/primitives/drawer';
import { FormField, FormInput, FormButton } from '@/components/forms/form-fields';

const CLUSTER_EXPLAINER = {
  title: 'What is a K3s cluster target?',
  description:
    'Connect an existing K3s cluster or let LazyOps manage the first control-plane node. Managed clusters can later grow by joining more VPS nodes directly from this page.',
  points: [
    'Managed clusters show node inventory and allow Add node over SSH',
    'External kubeconfig-only clusters stay read-only deployment targets',
    'Placement pinning uses LazyOps instance IDs, not raw Kubernetes node names',
    'Shared-cluster deploys remain the default when no node is pinned',
  ],
};

export default function ClustersPage() {
  const { data, isLoading, isError } = useClusters();
  const [showCreateModal, setShowCreateModal] = useState(false);
  const [joinCluster, setJoinCluster] = useState<ClusterSummary | null>(null);

  if (isLoading) {
    return <SkeletonPage title cards={3} />;
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
        subtitle="Connect K3s clusters, inspect node inventory, and grow managed clusters with Add node."
        actions={
          <button
            type="button"
            className="rounded-lg bg-primary px-6 py-2 text-base font-semibold text-lazyops-bg transition-colors hover:bg-primary/90"
            onClick={() => setShowCreateModal(true)}
          >
            Add cluster
          </button>
        }
      />

      <SectionCard title={CLUSTER_EXPLAINER.title} description={CLUSTER_EXPLAINER.description}>
        <ul className="grid gap-2 sm:grid-cols-2">
          {CLUSTER_EXPLAINER.points.map((point) => (
            <li key={point} className="flex items-start gap-2 text-base text-lazyops-muted">
              <svg className="mt-0.5 size-4 shrink-0 text-health-healthy" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
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
                className="rounded-lg bg-primary px-6 py-2 text-base font-semibold text-lazyops-bg transition-colors hover:bg-primary/90"
                onClick={() => setShowCreateModal(true)}
              >
                Add cluster
              </button>
            }
          />
        </SectionCard>
      ) : (
        <div className="grid gap-6">
          {clusters.map((cluster) => (
            <ClusterCard key={cluster.id} cluster={cluster} onAddNode={() => setJoinCluster(cluster)} />
          ))}
        </div>
      )}

      <CreateClusterModal open={showCreateModal} onClose={() => setShowCreateModal(false)} />
      <AddNodeDrawer cluster={joinCluster} onClose={() => setJoinCluster(null)} />
    </div>
  );
}

function ClusterCard({ cluster, onAddNode }: { cluster: ClusterSummary; onAddNode: () => void }) {
  const nodes = useClusterNodes(cluster.id);
  const managedCluster = !!cluster.instance_id;
  const nodeItems = nodes.data?.items ?? [];
  const readyNodes = nodeItems.filter((node) => node.is_ready).length;

  return (
    <SectionCard
      title={cluster.name}
      description={managedCluster ? 'Managed cluster can accept more worker nodes through SSH join.' : 'External kubeconfig target. Add node is unavailable for read-only clusters.'}
      actions={
        managedCluster ? (
          <button
            type="button"
            onClick={onAddNode}
            className="inline-flex items-center gap-2 rounded-xl border border-[#0EA5E9] bg-[#0EA5E9]/10 px-6 py-2 text-base font-semibold text-[#38bdf8] transition-colors hover:bg-[#0EA5E9]/20"
          >
            <Plus className="size-4" />
            Add node
          </button>
        ) : null
      }
    >
      <div className="grid gap-4 lg:grid-cols-[1.1fr_0.9fr]">
        <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-4">
          <ClusterMetric label="Status" value={formatClusterStatus(cluster.status)} badgeVariant={getClusterStatusVariant(cluster.status)} />
          <ClusterMetric label="Provider" value="K3s" />
          <ClusterMetric label="Cluster instance" value={cluster.instance_id || 'External / kubeconfig only'} mono />
          <ClusterMetric label="Public IP" value={cluster.public_ip || 'Not exposed'} mono />
        </div>

        <div className="grid gap-3 rounded-2xl border border-[#1e293b] bg-[#020617]/60 p-6">
          <div className="flex items-center gap-2 text-base font-semibold text-white">
            <Boxes className="size-4 text-[#38bdf8]" />
            Node inventory
          </div>
          <div className="grid gap-3 sm:grid-cols-3">
            <SummaryChip label="Total nodes" value={String(nodeItems.length)} />
            <SummaryChip label="Ready" value={String(readyNodes)} />
            <SummaryChip label="Mode" value={managedCluster ? 'Managed' : 'Read only'} />
          </div>
          {!managedCluster ? (
            <div className="rounded-xl border border-[#334155] bg-[#0B1120]/70 px-6 py-3 text-base text-[#94a3b8]">
              This cluster was connected with kubeconfig only, so LazyOps will not expose Add node until the cluster is bootstrap-managed.
            </div>
          ) : null}
        </div>
      </div>

      <div className="mt-5 rounded-2xl border border-[#1e293b] bg-[#020617]/60">
        {nodes.isLoading ? (
          <div className="px-6 py-5 text-base text-[#94a3b8]">Loading nodes...</div>
        ) : nodes.isError ? (
          <div className="px-6 py-5 text-base text-[#fca5a5]">
            {nodes.error instanceof Error ? nodes.error.message : 'Failed to load cluster nodes.'}
          </div>
        ) : nodeItems.length === 0 ? (
          <div className="px-6 py-5 text-base text-[#94a3b8]">
            {managedCluster
              ? 'This managed cluster has no discovered nodes yet. Add a node to expand capacity.'
              : 'No node inventory is available for this external cluster target.'}
          </div>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full text-base">
              <thead>
                <tr className="border-b border-[#1e293b] text-left text-[#64748b]">
                  <th className="px-6 py-3 font-medium">Node</th>
                  <th className="px-6 py-3 font-medium">Instance ID</th>
                  <th className="px-6 py-3 font-medium">K8s node</th>
                  <th className="px-6 py-3 font-medium">State</th>
                  <th className="px-6 py-3 font-medium">Placement label</th>
                  <th className="px-6 py-3 font-medium">Last seen</th>
                </tr>
              </thead>
              <tbody>
                {nodeItems.map((node) => (
                  <ClusterNodeRow key={node.instance_id} node={node} />
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>
    </SectionCard>
  );
}

function ClusterNodeRow({ node }: { node: ClusterNode }) {
  const placementLabel = node.labels?.['lazyops.io/instance-id'] || 'missing';
  const nodeState = node.is_ready ? 'Ready' : formatNodeStatus(node.status);

  return (
    <tr className="border-b border-[#1e293b]/60 last:border-b-0">
      <td className="px-6 py-3">
        <div className="flex flex-col">
          <span className="font-semibold text-white">{node.name}</span>
          <span className="text-sm text-[#94a3b8]">{node.cluster_id}</span>
        </div>
      </td>
      <td className="px-6 py-3 font-mono text-sm text-[#cbd5e1]">{node.instance_id}</td>
      <td className="px-6 py-3 text-[#cbd5e1]">{node.k8s_node_name || 'Waiting for registration'}</td>
      <td className="px-6 py-3">
        <StatusBadge
          label={nodeState}
          variant={node.is_ready ? 'success' : getNodeStatusVariant(node.status)}
          size="sm"
        />
      </td>
      <td className="px-6 py-3 font-mono text-sm text-[#94a3b8]">{placementLabel}</td>
      <td className="px-6 py-3 text-sm text-[#94a3b8]">
        {node.last_seen_at ? new Date(node.last_seen_at).toLocaleString() : 'No heartbeat yet'}
      </td>
    </tr>
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
          <FormInput type="text" placeholder="prod-k3s" error={!!errors.name} {...register('name')} />
        </FormField>

        <FormField label="Provider">
          <div className="rounded-lg border border-primary/30 bg-primary/10 px-6 py-3 text-base text-primary">
            K3s (lightweight Kubernetes)
          </div>
          <p className="mt-1 text-[10px] text-lazyops-muted/60">Currently only K3s clusters are supported.</p>
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

        {serverError ? (
          <div className="rounded-lg border border-health-unhealthy/30 bg-health-unhealthy/10 px-3 py-2 text-sm text-health-unhealthy">
            {serverError}
          </div>
        ) : null}

        <FormButton type="submit" loading={isSubmitting || createCluster.isPending}>
          Add cluster
        </FormButton>
      </form>
    </Modal>
  );
}

function AddNodeDrawer({ cluster, onClose }: { cluster: ClusterSummary | null; onClose: () => void }) {
  const connectNode = useConnectClusterNode(cluster?.id ?? '');
  const {
    register,
    reset,
    handleSubmit,
    formState: { errors, isSubmitting },
  } = useForm<ConnectClusterNodeFormData>({
    resolver: zodResolver(connectClusterNodeSchema),
    defaultValues: {
      instance_name: '',
      public_ip: '',
      private_ip: '',
      ssh_host: '',
      ssh_port: 22,
      ssh_username: 'root',
      ssh_password: '',
      ssh_private_key: '',
      ssh_host_key_fingerprint: '',
      control_plane_url: '',
      agent_image: '',
      container_name: '',
    },
  });

  useEffect(() => {
    if (!cluster) {
      reset();
    }
  }, [cluster, reset]);

  const open = !!cluster;
  const joinSummary = useMemo(
    () => (connectNode.data?.join?.stages ?? []).filter((stage) => stage.status && stage.label),
    [connectNode.data?.join?.stages],
  );

  const onSubmit = async (data: ConnectClusterNodeFormData) => {
    if (!cluster) {
      return;
    }
    await connectNode.mutateAsync({ ...data, labels: {} });
    reset();
    onClose();
  };

  return (
    <Drawer open={open} onClose={onClose} title={cluster ? `Add node to ${cluster.name}` : 'Add node'} size="lg">
      {!cluster ? null : (
        <form onSubmit={handleSubmit(onSubmit)} className="grid gap-5" noValidate>
          <div className="rounded-2xl border border-[#1e293b] bg-[#0B1120]/70 p-6">
            <div className="flex items-center gap-2 text-base font-semibold text-white">
              <ShieldCheck className="size-4 text-[#38bdf8]" />
              Managed cluster join
            </div>
            <p className="mt-2 text-base text-[#94a3b8]">
              LazyOps will reuse the cluster join token, install a K3s agent on the VPS, and then label the Kubernetes node with its LazyOps instance ID.
            </p>
          </div>

          <div className="grid gap-4 md:grid-cols-2">
            <FormField label="Node name" error={errors.instance_name?.message}>
              <FormInput type="text" placeholder="worker-sgp-2" error={!!errors.instance_name} {...register('instance_name')} />
            </FormField>
            <FormField label="Public IP" error={errors.public_ip?.message}>
              <FormInput type="text" placeholder="203.0.113.10" error={!!errors.public_ip} {...register('public_ip')} />
            </FormField>
            <FormField label="Private IP">
              <FormInput type="text" placeholder="10.0.0.12" {...register('private_ip')} />
            </FormField>
            <FormField label="SSH host" error={errors.ssh_host?.message}>
              <FormInput type="text" placeholder="203.0.113.10" error={!!errors.ssh_host} {...register('ssh_host')} />
            </FormField>
            <FormField label="SSH port" error={errors.ssh_port?.message}>
              <FormInput
                type="number"
                min={1}
                max={65535}
                error={!!errors.ssh_port}
                {...register('ssh_port', { valueAsNumber: true })}
              />
            </FormField>
            <FormField label="SSH username" error={errors.ssh_username?.message}>
              <FormInput type="text" placeholder="root" error={!!errors.ssh_username} {...register('ssh_username')} />
            </FormField>
          </div>

          <FormField label="SSH password" error={errors.ssh_password?.message}>
            <FormInput type="password" placeholder="Optional if using private key" error={!!errors.ssh_password} {...register('ssh_password')} />
          </FormField>

          <FormField label="SSH private key">
            <textarea
              rows={7}
              className="w-full rounded-xl border border-[#1e293b] bg-[#0B1120]/40 px-6 py-3 text-base text-white outline-none transition-colors placeholder:text-[#64748b]/60 focus:border-[#0EA5E9]/50 focus:ring-4 focus:ring-[#0EA5E9]/10"
              placeholder="-----BEGIN OPENSSH PRIVATE KEY-----"
              {...register('ssh_private_key')}
            />
          </FormField>

          <details className="rounded-2xl border border-[#1e293b] bg-[#0B1120]/50 p-6">
            <summary className="cursor-pointer text-base font-semibold text-white">Advanced join options</summary>
            <div className="mt-4 grid gap-4">
              <FormField label="SSH host key fingerprint">
                <FormInput type="text" placeholder="SHA256:..." {...register('ssh_host_key_fingerprint')} />
              </FormField>
              <div className="grid gap-4 md:grid-cols-2">
                <FormField label="Control plane URL override">
                  <FormInput type="text" placeholder="https://control.example.com" {...register('control_plane_url')} />
                </FormField>
                <FormField label="Agent image override">
                  <FormInput type="text" placeholder="ghcr.io/org/agent:latest" {...register('agent_image')} />
                </FormField>
              </div>
              <FormField label="Container name override">
                <FormInput type="text" placeholder="lazyops-agent" {...register('container_name')} />
              </FormField>
            </div>
          </details>

          {connectNode.error ? (
            <div className="rounded-2xl border border-[#ef4444]/30 bg-[#ef4444]/10 px-6 py-3 text-base text-[#fecaca]">
              {connectNode.error.message}
            </div>
          ) : null}

          {joinSummary.length > 0 ? (
            <div className="rounded-2xl border border-[#1e293b] bg-[#0B1120]/60 p-6">
              <div className="mb-3 text-base font-semibold text-white">Recent join stages</div>
              <div className="grid gap-2">
                {joinSummary.map((stage) => (
                  <div key={`${stage.key}-${stage.label}`} className="flex items-center justify-between rounded-xl border border-[#1e293b] bg-[#020617]/70 px-3 py-2 text-base">
                    <span className="text-[#e2e8f0]">{stage.label}</span>
                    <span className="font-mono text-sm text-[#38bdf8]">{stage.status}</span>
                  </div>
                ))}
              </div>
            </div>
          ) : null}

          <div className="flex items-center justify-end gap-3">
            <button
              type="button"
              onClick={onClose}
              className="rounded-xl border border-[#334155] px-6 py-2 text-base font-semibold text-[#cbd5e1] transition-colors hover:bg-[#111827]"
            >
              Cancel
            </button>
            <FormButton
              type="submit"
              loading={isSubmitting || connectNode.isPending}
              className="w-auto min-w-[180px]"
            >
              Join node
            </FormButton>
          </div>
        </form>
      )}
    </Drawer>
  );
}

function ClusterMetric({
  label,
  value,
  mono = false,
  badgeVariant,
}: {
  label: string;
  value: string;
  mono?: boolean;
  badgeVariant?: 'success' | 'warning' | 'danger' | 'info' | 'neutral';
}) {
  return (
    <div className="rounded-2xl border border-[#1e293b] bg-[#020617]/70 p-6">
      <div className="mb-2 flex items-center gap-2 text-sm font-semibold uppercase tracking-[0.12em] text-[#64748b]">
        <Server className="size-3.5 text-[#38bdf8]" />
        {label}
      </div>
      {badgeVariant ? (
        <StatusBadge label={value} variant={badgeVariant} size="sm" />
      ) : (
        <div className={`text-base font-semibold text-white ${mono ? 'font-mono' : ''}`}>{value}</div>
      )}
    </div>
  );
}

function SummaryChip({ label, value }: { label: string; value: string }) {
  return (
    <div className="rounded-xl border border-[#1e293b] bg-[#0B1120]/80 px-3 py-2">
      <div className="text-[10px] font-semibold uppercase tracking-[0.12em] text-[#64748b]">{label}</div>
      <div className="mt-1 text-base font-semibold text-white">{value}</div>
    </div>
  );
}

function formatNodeStatus(status: string) {
  return status.replace(/_/g, ' ').replace(/\b\w/g, (char) => char.toUpperCase());
}

function getNodeStatusVariant(status: string): 'success' | 'warning' | 'danger' | 'info' | 'neutral' {
  switch (status) {
    case 'ready':
      return 'success';
    case 'unreachable':
    case 'failed':
      return 'danger';
    case 'provisioning':
    case 'joining':
    case 'validating':
      return 'warning';
    default:
      return 'neutral';
  }
}
