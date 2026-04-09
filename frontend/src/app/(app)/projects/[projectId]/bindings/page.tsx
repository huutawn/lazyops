'use client';

import { useState, useMemo, useEffect } from 'react';
import { useParams, useRouter } from 'next/navigation';
import { zodResolver } from '@hookform/resolvers/zod';
import { useForm, useWatch } from 'react-hook-form';
import { useDeploymentBindings, useCreateDeploymentBinding } from '@/modules/deployment-bindings/binding-hooks';
import {
  createDeploymentBindingSchema,
  RUNTIME_MODES,
  TARGET_KINDS,
  COMPATIBILITY_MATRIX,
  type CreateDeploymentBindingFormData,
  type RuntimeMode,
  type TargetKind,
} from '@/modules/deployment-bindings/binding-types';
import { PageHeader } from '@/components/primitives/page-header';
import { SectionCard } from '@/components/primitives/section-card';
import { EmptyState } from '@/components/primitives/empty-state';
import { ErrorState } from '@/components/primitives/error-state';
import { SkeletonPage } from '@/components/primitives/skeleton';
import { StatusBadge } from '@/components/primitives/status-badge';
import { Modal } from '@/components/primitives/modal';
import { FormField, FormInput, FormButton } from '@/components/forms/form-fields';
import { isFeatureEnabled } from '@/lib/flags/feature-flags';

const RUNTIME_MODE_LABELS: Record<RuntimeMode, string> = {
  standalone: 'Standalone',
  'distributed-mesh': 'Distributed Mesh',
  'distributed-k3s': 'Distributed K3s',
};

const TARGET_KIND_LABELS: Record<TargetKind, string> = {
  instance: 'Instance',
  mesh: 'Mesh Network',
  cluster: 'K3s Cluster',
};

export default function DeploymentBindingsPage() {
  const params = useParams();
  const router = useRouter();
  const projectId = params?.projectId as string;
  const threeStepFlowEnabled = isFeatureEnabled('ux_three_step_flow');

  useEffect(() => {
    if (threeStepFlowEnabled && projectId) {
      router.replace(`/projects/${projectId}`);
    }
  }, [threeStepFlowEnabled, projectId, router]);

  const { data, isLoading, isError } = useDeploymentBindings(projectId);
  const [showCreateModal, setShowCreateModal] = useState(false);

  if (threeStepFlowEnabled) {
    return <SkeletonPage title cards={1} />;
  }

  if (isLoading) {
    return <SkeletonPage title cards={1} />;
  }

  if (isError) {
    return (
      <div className="flex flex-col gap-6">
        <PageHeader title="Deployment Bindings" subtitle="Manage deployment targets for this project" />
        <ErrorState title="Failed to load bindings" message="Could not fetch deployment binding data. Please try again." />
      </div>
    );
  }

  const bindings = data?.items ?? [];

  return (
    <div className="flex flex-col gap-6">
      <PageHeader
        title="Deployment Bindings"
        subtitle="Connect this project to deployment targets. Each binding defines where and how services run."
        actions={
          <button
            type="button"
            className="rounded-lg bg-primary px-4 py-2 text-sm font-semibold text-lazyops-bg transition-colors hover:bg-primary/90"
            onClick={() => setShowCreateModal(true)}
          >
            Create binding
          </button>
        }
      />

      {bindings.length === 0 ? (
        <SectionCard
          title="No bindings"
          description="Create a binding to connect this project to a deployment target."
        >
          <EmptyState
            title="No deployment bindings"
            description="A binding links this project to a target (instance, mesh, or cluster) and defines how services are deployed."
            action={
              <button
                type="button"
                className="rounded-lg bg-primary px-4 py-2 text-sm font-semibold text-lazyops-bg transition-colors hover:bg-primary/90"
                onClick={() => setShowCreateModal(true)}
              >
                Create binding
              </button>
            }
          />
        </SectionCard>
      ) : (
        <div className="flex flex-col gap-4">
          {bindings.map((binding) => (
            <SectionCard key={binding.id} title={binding.name} description={`Target: ${binding.target_ref}`}>
              <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
                <PolicyField label="Runtime Mode" value={RUNTIME_MODE_LABELS[binding.runtime_mode]} />
                <PolicyField label="Target Type" value={TARGET_KIND_LABELS[binding.target_kind]} />
                <PolicyField label="Placement" value={policySummary(binding.placement_policy)} />
                <PolicyField label="Scale to Zero" value={binding.scale_to_zero_policy?.enabled ? 'Enabled' : 'Disabled'} />
              </div>
              <div className="mt-3 grid gap-3 sm:grid-cols-2">
                <PolicyField label="Domain Policy" value={policySummary(binding.domain_policy)} />
                <PolicyField label="Compatibility" value={policySummary(binding.compatibility_policy)} />
              </div>
            </SectionCard>
          ))}
        </div>
      )}

      <CreateBindingModal
        projectId={projectId}
        open={showCreateModal}
        onClose={() => setShowCreateModal(false)}
      />
    </div>
  );
}

function PolicyField({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex flex-col gap-0.5">
      <span className="text-xs text-lazyops-muted">{label}</span>
      <span className="text-sm text-lazyops-text">{value}</span>
    </div>
  );
}

function policySummary(policy: Record<string, unknown>): string {
  if (!policy || Object.keys(policy).length === 0) return 'Default';
  const keys = Object.keys(policy);
  if (keys.length === 1 && keys[0] === 'strategy') return String(policy.strategy);
  return `${keys.length} rule${keys.length > 1 ? 's' : ''} configured`;
}

type CreateBindingModalProps = {
  projectId: string;
  open: boolean;
  onClose: () => void;
};

function CreateBindingModal({ projectId, open, onClose }: CreateBindingModalProps) {
  const {
    register,
    handleSubmit,
    control,
    setValue,
    formState: { errors, isSubmitting },
  } = useForm<CreateDeploymentBindingFormData>({
    resolver: zodResolver(createDeploymentBindingSchema),
    defaultValues: {
      name: '',
      target_ref: '',
      runtime_mode: 'standalone',
      target_kind: 'instance',
      target_id: '',
      placement_policy: '',
      domain_policy: '',
      compatibility_policy: '',
      scale_to_zero: false,
    },
  });

  const createBinding = useCreateDeploymentBinding(projectId);
  const serverError = createBinding.error?.message ?? null;

  const selectedKind = useWatch({ control, name: 'target_kind' }) as TargetKind;
  const selectedMode = useWatch({ control, name: 'runtime_mode' }) as RuntimeMode;
  const allowedModes = COMPATIBILITY_MATRIX[selectedKind] ?? [];

  const handleKindChange = (kind: TargetKind) => {
    setValue('target_kind', kind);
    const allowed = COMPATIBILITY_MATRIX[kind];
    if (allowed && !allowed.includes(selectedMode)) {
      setValue('runtime_mode', allowed[0]);
    }
  };

  const onSubmit = (data: CreateDeploymentBindingFormData) => {
    return createBinding.mutateAsync(data).then(() => {
      onClose();
    });
  };

  return (
    <Modal open={open} onClose={onClose} title="Create deployment binding" size="lg">
      <form onSubmit={handleSubmit(onSubmit)} className="flex flex-col gap-5" noValidate>
        <FormField label="Binding name" error={errors.name?.message}>
          <FormInput
            type="text"
            placeholder="prod-standalone"
            error={!!errors.name}
            {...register('name')}
          />
        </FormField>

        <div>
          <label className="mb-2 block text-sm font-medium text-lazyops-text">Target type</label>
          <div className="grid grid-cols-3 gap-3">
            {TARGET_KINDS.map((kind) => {
              const isSelected = selectedKind === kind;
              return (
                <button
                  key={kind}
                  type="button"
                  className={`rounded-lg border px-3 py-3 text-sm transition-colors ${
                    isSelected
                      ? 'border-primary/40 bg-primary/10 text-primary'
                      : 'border-lazyops-border text-lazyops-muted hover:text-lazyops-text'
                  }`}
                  onClick={() => handleKindChange(kind)}
                >
                  <div className="font-medium">{TARGET_KIND_LABELS[kind]}</div>
                  <div className="mt-0.5 text-[10px] opacity-70">
                    {COMPATIBILITY_MATRIX[kind].map((m) => RUNTIME_MODE_LABELS[m]).join(', ')}
                  </div>
                </button>
              );
            })}
          </div>
        </div>

        <FormField label="Runtime mode" error={errors.runtime_mode?.message}>
          <select
            className="h-10 w-full rounded-lg border bg-lazyops-bg-accent/60 px-3 text-sm text-lazyops-text outline-none focus:border-primary/60 focus:ring-1 focus:ring-primary/30"
            {...register('runtime_mode')}
          >
            {allowedModes.map((mode) => (
              <option key={mode} value={mode}>
                {RUNTIME_MODE_LABELS[mode]}
              </option>
            ))}
          </select>
          <p className="mt-1 text-[10px] text-lazyops-muted/60">
            Only modes compatible with the selected target type are shown.
          </p>
        </FormField>

        <FormField label="Target ID" error={errors.target_id?.message}>
          <FormInput
            type="text"
            placeholder="inst_xxx or mesh_xxx or cls_xxx"
            error={!!errors.target_id}
            {...register('target_id')}
          />
        </FormField>

        <div className="rounded-lg border border-lazyops-border/50 bg-lazyops-bg-accent/30 p-4">
          <h4 className="mb-3 text-sm font-medium text-lazyops-text">Policy configuration</h4>
          <div className="flex flex-col gap-3">
            <FormField label="Placement policy (JSON)" error={errors.placement_policy?.message}>
              <FormInput
                type="text"
                placeholder='{"strategy": "spread"}'
                error={!!errors.placement_policy}
                {...register('placement_policy')}
              />
            </FormField>

            <FormField label="Domain policy (JSON)" error={errors.domain_policy?.message}>
              <FormInput
                type="text"
                placeholder='{"mode": "auto"}'
                error={!!errors.domain_policy}
                {...register('domain_policy')}
              />
            </FormField>

            <FormField label="Compatibility policy (JSON)" error={errors.compatibility_policy?.message}>
              <FormInput
                type="text"
                placeholder='{"min_version": "1.0"}'
                error={!!errors.compatibility_policy}
                {...register('compatibility_policy')}
              />
            </FormField>

            <label className="flex items-center gap-2 text-sm text-lazyops-text">
              <input
                type="checkbox"
                className="accent-primary"
                {...register('scale_to_zero')}
              />
              Enable scale-to-zero
            </label>
          </div>
        </div>

        {serverError && (
          <div className="rounded-lg border border-health-unhealthy/30 bg-health-unhealthy/10 px-3 py-2 text-xs text-health-unhealthy">
            {serverError}
          </div>
        )}

        <FormButton type="submit" loading={isSubmitting || createBinding.isPending}>
          Create binding
        </FormButton>
      </form>
    </Modal>
  );
}
