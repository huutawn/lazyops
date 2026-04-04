'use client';

import { useState } from 'react';
import { zodResolver } from '@hookform/resolvers/zod';
import { useForm } from 'react-hook-form';
import { useInstances, useCreateInstance } from '@/modules/instances/instance-hooks';
import { createInstanceSchema, type CreateInstanceFormData } from '@/modules/instances/instance-types';
import { getStatusVariant, formatStatus } from '@/modules/instances/instance-utils';
import { PageHeader } from '@/components/primitives/page-header';
import { SectionCard } from '@/components/primitives/section-card';
import { EmptyState } from '@/components/primitives/empty-state';
import { ErrorState } from '@/components/primitives/error-state';
import { SkeletonPage } from '@/components/primitives/skeleton';
import { StatusBadge } from '@/components/primitives/status-badge';
import { Modal } from '@/components/primitives/modal';
import { FormField, FormInput, FormButton } from '@/components/forms/form-fields';

export default function InstancesPage() {
  const { data, isLoading, isError } = useInstances();
  const [showCreateModal, setShowCreateModal] = useState(false);
  const [showBootstrapModal, setShowBootstrapModal] = useState(false);
  const [bootstrapToken, setBootstrapToken] = useState<{ token: string; expires_at: string } | null>(null);

  if (isLoading) {
    return <SkeletonPage title cards={1} />;
  }

  if (isError) {
    return (
      <div className="flex flex-col gap-6">
        <PageHeader title="Instances" subtitle="Manage your target machines" />
        <ErrorState title="Failed to load instances" message="Could not fetch instance data. Please try again." />
      </div>
    );
  }

  const instances = data?.items ?? [];

  return (
    <div className="flex flex-col gap-6">
      <PageHeader
        title="Instances"
        subtitle="Register machines for LazyOps to manage and deploy onto."
        actions={
          <button
            type="button"
            className="rounded-lg bg-primary px-4 py-2 text-sm font-semibold text-lazyops-bg transition-colors hover:bg-primary/90"
            onClick={() => setShowCreateModal(true)}
          >
            Add instance
          </button>
        }
      />

      {instances.length === 0 ? (
        <SectionCard title="No instances" description="Add your first machine to get started.">
          <EmptyState
            title="No instances registered"
            description="Create an instance to register a target machine. You'll get a bootstrap command to install the LazyOps agent."
            action={
              <button
                type="button"
                className="rounded-lg bg-primary px-4 py-2 text-sm font-semibold text-lazyops-bg transition-colors hover:bg-primary/90"
                onClick={() => setShowCreateModal(true)}
              >
                Add instance
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
                  <th className="px-4 py-3 text-left font-medium text-lazyops-muted">Status</th>
                  <th className="px-4 py-3 text-left font-medium text-lazyops-muted">Public IP</th>
                  <th className="px-4 py-3 text-left font-medium text-lazyops-muted">Private IP</th>
                  <th className="px-4 py-3 text-left font-medium text-lazyops-muted">Labels</th>
                  <th className="px-4 py-3 text-left font-medium text-lazyops-muted">Agent</th>
                  <th className="px-4 py-3 text-left font-medium text-lazyops-muted">Actions</th>
                </tr>
              </thead>
              <tbody>
                {instances.map((instance) => (
                  <tr
                    key={instance.id}
                    className="border-b border-lazyops-border/50 transition-colors hover:bg-lazyops-border/10"
                  >
                    <td className="px-4 py-3 font-medium text-lazyops-text">{instance.name}</td>
                    <td className="px-4 py-3">
                      <StatusBadge
                        label={formatStatus(instance.status)}
                        variant={getStatusVariant(instance.status)}
                        size="sm"
                      />
                    </td>
                    <td className="px-4 py-3 font-mono text-xs text-lazyops-muted">
                      {instance.public_ip ?? '—'}
                    </td>
                    <td className="px-4 py-3 font-mono text-xs text-lazyops-muted">
                      {instance.private_ip ?? '—'}
                    </td>
                    <td className="px-4 py-3">
                      <div className="flex flex-wrap gap-1">
                        {Object.entries(instance.labels).map(([key, value]) => (
                          <span
                            key={key}
                            className="rounded bg-lazyops-border/20 px-1.5 py-0.5 text-[10px] text-lazyops-muted"
                          >
                            {key}{value ? `:${value}` : ''}
                          </span>
                        ))}
                        {Object.keys(instance.labels).length === 0 && (
                          <span className="text-xs text-lazyops-muted/50">—</span>
                        )}
                      </div>
                    </td>
                    <td className="px-4 py-3 text-xs text-lazyops-muted">
                      {instance.agent_id ? (
                        <span className="font-mono">{instance.agent_id.slice(0, 12)}…</span>
                      ) : (
                        <span className="text-lazyops-muted/50">Not enrolled</span>
                      )}
                    </td>
                    <td className="px-4 py-3">
                      {instance.status === 'pending_enrollment' && (
                        <button
                          type="button"
                          className="text-xs text-primary hover:underline"
                          onClick={() => {
                            setBootstrapToken(null);
                            setShowBootstrapModal(true);
                          }}
                        >
                          View bootstrap
                        </button>
                      )}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </SectionCard>
      )}

      <CreateInstanceModal
        open={showCreateModal}
        onClose={() => setShowCreateModal(false)}
        onSuccess={() => {
          setShowCreateModal(false);
        }}
      />

      <BootstrapModal
        open={showBootstrapModal}
        onClose={() => {
          setShowBootstrapModal(false);
          setBootstrapToken(null);
        }}
        token={bootstrapToken}
      />
    </div>
  );
}

type CreateInstanceModalProps = {
  open: boolean;
  onClose: () => void;
  onSuccess: () => void;
};

function CreateInstanceModal({ open, onClose, onSuccess }: CreateInstanceModalProps) {
  const {
    register,
    handleSubmit,
    formState: { errors, isSubmitting },
  } = useForm<CreateInstanceFormData>({
    resolver: zodResolver(createInstanceSchema),
    defaultValues: { name: '', public_ip: '', private_ip: '', labels: '' },
  });

  const createInstance = useCreateInstance();
  const serverError = createInstance.error?.message ?? null;

  const onSubmit = (data: CreateInstanceFormData) => {
    return createInstance.mutateAsync(data).then(() => {
      onSuccess();
    });
  };

  return (
    <Modal open={open} onClose={onClose} title="Add instance" size="md">
      <form onSubmit={handleSubmit(onSubmit)} className="flex flex-col gap-4" noValidate>
        <FormField label="Instance name" error={errors.name?.message}>
          <FormInput
            type="text"
            placeholder="prod-web-01"
            error={!!errors.name}
            {...register('name')}
          />
        </FormField>

        <div className="grid gap-4 sm:grid-cols-2">
          <FormField label="Public IP" error={errors.public_ip?.message}>
            <FormInput
              type="text"
              placeholder="203.0.113.10"
              error={!!errors.public_ip}
              {...register('public_ip')}
            />
          </FormField>

          <FormField label="Private IP" error={errors.private_ip?.message}>
            <FormInput
              type="text"
              placeholder="10.0.1.10"
              error={!!errors.private_ip}
              {...register('private_ip')}
            />
          </FormField>
        </div>

        <FormField label="Labels" error={errors.labels?.message}>
          <FormInput
            type="text"
            placeholder="env:prod, role:web"
            error={!!errors.labels}
            {...register('labels')}
          />
          <p className="mt-1 text-[10px] text-lazyops-muted/60">
            Comma-separated key:value pairs. Keys are lowercased automatically.
          </p>
        </FormField>

        {serverError && (
          <div className="rounded-lg border border-health-unhealthy/30 bg-health-unhealthy/10 px-3 py-2 text-xs text-health-unhealthy">
            {serverError}
          </div>
        )}

        <FormButton type="submit" loading={isSubmitting || createInstance.isPending}>
          Create instance
        </FormButton>
      </form>
    </Modal>
  );
}

type BootstrapModalProps = {
  open: boolean;
  onClose: () => void;
  token: { token: string; expires_at: string } | null;
};

function BootstrapModal({ open, onClose, token }: BootstrapModalProps) {
  const [copied, setCopied] = useState(false);

  const command = token
    ? `lazyops agent enroll --token ${token.token}`
    : 'Run the bootstrap command on your target machine after creating an instance.';

  const handleCopy = async () => {
    await navigator.clipboard.writeText(command);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };

  return (
    <Modal open={open} onClose={onClose} title="Bootstrap command" size="lg">
      <div className="flex flex-col gap-4">
        <p className="text-sm text-lazyops-muted">
          Run this command on your target machine to install and enroll the LazyOps agent.
        </p>

        <div className="relative rounded-lg border border-lazyops-border bg-lazyops-bg p-4">
          <code className="block break-all text-sm text-lazyops-text">{command}</code>
          <button
            type="button"
            className="absolute right-3 top-3 rounded-md border border-lazyops-border bg-lazyops-bg-accent px-2.5 py-1.5 text-xs text-lazyops-muted transition-colors hover:text-lazyops-text"
            onClick={handleCopy}
          >
            {copied ? 'Copied!' : 'Copy'}
          </button>
        </div>

        {token && (
          <div className="rounded-lg border border-lazyops-border/50 bg-lazyops-bg-accent/50 px-3 py-2 text-xs text-lazyops-muted">
            Token expires at: {new Date(token.expires_at).toLocaleString()}
          </div>
        )}

        <div className="rounded-lg border border-lazyops-border/50 bg-lazyops-bg-accent/50 px-3 py-3 text-xs text-lazyops-muted">
          <p className="mb-1 font-medium text-lazyops-text">What happens next?</p>
          <ol className="list-decimal space-y-1 pl-4">
            <li>The agent installs on the target machine</li>
            <li>It enrolls using the bootstrap token</li>
            <li>Instance status changes to <span className="text-health-healthy">online</span></li>
            <li>LazyOps can now deploy services to this target</li>
          </ol>
        </div>
      </div>
    </Modal>
  );
}
