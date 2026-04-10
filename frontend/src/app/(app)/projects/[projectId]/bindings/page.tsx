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
import { useSession } from '@/lib/auth/auth-hooks';

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

const TARGET_KIND_PREFIX_RULES: Array<{ kind: TargetKind; prefixes: string[] }> = [
  { kind: 'instance', prefixes: ['inst_', 'instance_'] },
  { kind: 'mesh', prefixes: ['mesh_'] },
  { kind: 'cluster', prefixes: ['cls_', 'cluster_'] },
];

function inferTargetKindFromID(targetID: string): TargetKind | null {
  const normalized = targetID.trim().toLowerCase();
  if (!normalized) return null;
  const matched = TARGET_KIND_PREFIX_RULES.find((rule) =>
    rule.prefixes.some((prefix) => normalized.startsWith(prefix)),
  );
  return matched?.kind ?? null;
}

function defaultRuntimeModeForKind(kind: TargetKind): RuntimeMode {
  const allowedModes = COMPATIBILITY_MATRIX[kind];
  return allowedModes?.[0] ?? 'standalone';
}

function buildAutoBindingName(targetID: string, fallbackKind: TargetKind): string {
  const normalized = targetID.trim().toLowerCase();
  const inferredKind = inferTargetKindFromID(normalized) ?? fallbackKind;
  const stripped = normalized.replace(/^(inst_|instance_|mesh_|cls_|cluster_)/, '');
  const safeSuffix = stripped.replace(/[^a-z0-9-]+/g, '-').replace(/^-+|-+$/g, '');
  return safeSuffix ? `prod-${safeSuffix}` : `${inferredKind}-binding`;
}

export default function DeploymentBindingsPage() {
  const params = useParams();
  const router = useRouter();
  const projectId = params?.projectId as string;
  const threeStepFlowEnabled = isFeatureEnabled('ux_three_step_flow');
  const { data: session, isLoading: sessionLoading } = useSession();
  const isAdmin = session?.role === 'admin';

  useEffect(() => {
    if (!sessionLoading && threeStepFlowEnabled && projectId && !isAdmin) {
      router.replace(`/projects/${projectId}`);
    }
  }, [sessionLoading, threeStepFlowEnabled, projectId, router, isAdmin]);

  const { data, isLoading, isError } = useDeploymentBindings(projectId);
  const [showCreateModal, setShowCreateModal] = useState(false);

  if (sessionLoading || (threeStepFlowEnabled && !isAdmin)) {
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
    getValues,
    reset,
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
  const [showAdvanced, setShowAdvanced] = useState(false);

  const selectedKind = useWatch({ control, name: 'target_kind' }) as TargetKind;
  const selectedMode = useWatch({ control, name: 'runtime_mode' }) as RuntimeMode;
  const selectedTargetID = useWatch({ control, name: 'target_id' }) as string;
  const allowedModes = COMPATIBILITY_MATRIX[selectedKind] ?? [];

  const handleKindChange = (kind: TargetKind) => {
    setValue('target_kind', kind);
    const allowed = COMPATIBILITY_MATRIX[kind];
    if (allowed && !allowed.includes(selectedMode)) {
      setValue('runtime_mode', allowed[0]);
    }
  };

  useEffect(() => {
    const inferredKind = inferTargetKindFromID(selectedTargetID ?? '');
    if (!inferredKind) return;

    if (selectedKind !== inferredKind) {
      setValue('target_kind', inferredKind, { shouldValidate: true });
    }

    const inferredMode = defaultRuntimeModeForKind(inferredKind);
    if (selectedMode !== inferredMode) {
      setValue('runtime_mode', inferredMode, { shouldValidate: true });
    }

    const currentName = (getValues('name') ?? '').trim();
    if (!currentName || currentName.endsWith('-binding') || currentName.startsWith('prod-')) {
      setValue('name', buildAutoBindingName(selectedTargetID ?? '', inferredKind));
    }
  }, [selectedTargetID, selectedKind, selectedMode, setValue, getValues]);

  const onSubmit = (data: CreateDeploymentBindingFormData) => {
    const normalizedTargetID = data.target_id.trim();
    const resolvedKind = inferTargetKindFromID(normalizedTargetID) ?? data.target_kind;
    const payload: CreateDeploymentBindingFormData = {
      ...data,
      name: data.name.trim() || buildAutoBindingName(normalizedTargetID, resolvedKind),
      target_id: normalizedTargetID,
      target_kind: resolvedKind,
      runtime_mode: defaultRuntimeModeForKind(resolvedKind),
      target_ref: data.target_ref?.trim(),
      placement_policy: showAdvanced ? data.placement_policy : '',
      domain_policy: showAdvanced ? data.domain_policy : '',
      compatibility_policy: showAdvanced ? data.compatibility_policy : '',
      scale_to_zero: showAdvanced ? data.scale_to_zero : false,
    };

    return createBinding.mutateAsync(payload).then(() => {
      reset();
      setShowAdvanced(false);
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

        <FormField label="Target ID" error={errors.target_id?.message}>
          <FormInput
            type="text"
            placeholder="inst_xxx or mesh_xxx or cls_xxx"
            error={!!errors.target_id}
            {...register('target_id')}
          />
          <p className="mt-1 text-xs text-lazyops-muted">
            Auto-detected target type: <span className="text-lazyops-text">{TARGET_KIND_LABELS[selectedKind]}</span> · runtime mode:{' '}
            <span className="text-lazyops-text">{RUNTIME_MODE_LABELS[selectedMode]}</span>
          </p>
        </FormField>

        <div className="rounded-lg border border-lazyops-border/40 bg-lazyops-bg-accent/20 px-3 py-2">
          <button
            type="button"
            className="flex w-full items-center justify-between text-left text-sm text-lazyops-text"
            onClick={() => setShowAdvanced((prev) => !prev)}
          >
            <span>Advanced options</span>
            <span className="text-xs text-lazyops-muted">{showAdvanced ? 'Hide' : 'Show'}</span>
          </button>
        </div>

        {showAdvanced && (
          <>
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
                        {COMPATIBILITY_MATRIX[kind].map((mode) => RUNTIME_MODE_LABELS[mode]).join(', ')}
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
          </>
        )}

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
