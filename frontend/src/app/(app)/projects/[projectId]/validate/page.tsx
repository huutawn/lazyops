'use client';

import { useState } from 'react';
import { useParams } from 'next/navigation';
import { useDeploymentBindings } from '@/modules/deployment-bindings/binding-hooks';
import { validateLazyopsYaml } from '@/modules/validate-lazyops/validate-api';
import type { ValidateLazyopsResponse, LazyopsYAMLDraft } from '@/modules/validate-lazyops/validate-types';
import { PageHeader } from '@/components/primitives/page-header';
import { SectionCard } from '@/components/primitives/section-card';
import { StatusBadge } from '@/components/primitives/status-badge';
import { LoadingPage } from '@/components/primitives/loading';
import { ErrorState } from '@/components/primitives/error-state';

const EXPLANATION = {
  title: 'What is the deploy contract?',
  description:
    'The deploy contract defines how your services map to deployment targets. Instead of writing raw YAML or Kubernetes manifests, you describe your intent — LazyOps validates it against your project\'s bindings and policies.',
  steps: [
    { title: 'Select a binding', desc: 'Choose which deployment binding to validate against.' },
    { title: 'Review the contract', desc: 'LazyOps shows what the system intends to deploy.' },
    { title: 'Validate', desc: 'The contract is checked for forbidden fields, protocol compatibility, and policy compliance.' },
  ],
};

const FORBIDDEN_FIELDS = [
  'ssh', 'ssh_key', 'private_key', 'password', 'pat', 'token',
  'agent_token', 'github_token', 'secret', 'kubeconfig',
  'kubeconfig_secret_ref', 'public_ip', 'private_ip', 'server_ip',
  'project_id', 'deployment_binding_id', 'target_id', 'target_kind',
  'instance_id', 'mesh_network_id', 'cluster_id', 'deploy_command',
];

const ALLOWED_PROTOCOLS = ['http', 'https', 'tcp', 'grpc'];

export default function ValidateContractPage() {
  const params = useParams();
  const projectId = params?.projectId as string;

  const { data: bindingsData, isLoading: bindingsLoading } = useDeploymentBindings(projectId);
  const [validationResult, setValidationResult] = useState<ValidateLazyopsResponse | null>(null);
  const [isValidating, setIsValidating] = useState(false);
  const [validationError, setValidationError] = useState<string | null>(null);
  const [selectedBindingIdx, setSelectedBindingIdx] = useState(0);

  const bindings = bindingsData?.items ?? [];

  const handleValidate = async () => {
    if (bindings.length === 0) return;
    const binding = bindings[selectedBindingIdx];

    setIsValidating(true);
    setValidationError(null);

    const draft: LazyopsYAMLDraft = {
      project_slug: '',
      runtime_mode: binding.runtime_mode,
      deployment_binding: { target_ref: binding.target_ref },
      services: [],
      dependency_bindings: [],
      compatibility_policy: { env_injection: false, managed_credentials: false, localhost_rescue: false },
      magic_domain_policy: { enabled: false, provider: '' },
      preview_policy: { enabled: false },
      scale_to_zero_policy: { enabled: binding.scale_to_zero_policy?.enabled === true },
    };

    try {
      const result = await validateLazyopsYaml(projectId, draft);
      if (result.error) {
        setValidationError(result.error.message);
      } else if (result.data) {
        setValidationResult(result.data);
      }
    } catch (err) {
      setValidationError(err instanceof Error ? err.message : 'Validation failed');
    } finally {
      setIsValidating(false);
    }
  };

  if (bindingsLoading) {
    return <LoadingPage label="Loading bindings…" />;
  }

  if (bindings.length === 0) {
    return (
      <div className="flex flex-col gap-6">
        <PageHeader title="Deploy Contract" subtitle="Review and validate your deployment contract." />
        <SectionCard title="No bindings available" description="Create a deployment binding first to validate a contract.">
          <p className="text-sm text-lazyops-muted">
            You need at least one deployment binding before you can review the deploy contract.
          </p>
        </SectionCard>
      </div>
    );
  }

  const selectedBinding = bindings[selectedBindingIdx];

  return (
    <div className="flex flex-col gap-6">
      <PageHeader
        title="Deploy Contract"
        subtitle="Review what LazyOps intends to deploy before rollout."
      />

      <SectionCard
        title={EXPLANATION.title}
        description={EXPLANATION.description}
      >
        <div className="flex flex-col gap-3">
          {EXPLANATION.steps.map((step, i) => (
            <div key={step.title} className="flex items-start gap-3">
              <div className="flex size-6 shrink-0 items-center justify-center rounded-full bg-primary/15 text-xs font-bold text-primary">
                {i + 1}
              </div>
              <div>
                <span className="text-sm font-medium text-lazyops-text">{step.title}</span>
                <p className="text-xs text-lazyops-muted">{step.desc}</p>
              </div>
            </div>
          ))}
        </div>
      </SectionCard>

      <SectionCard title="Select binding" description="Choose which deployment binding to validate.">
        <div className="flex flex-col gap-2">
          {bindings.map((binding, i) => (
            <button
              key={binding.id}
              type="button"
              className={`flex items-center justify-between rounded-lg border px-4 py-3 text-left transition-colors ${
                i === selectedBindingIdx
                  ? 'border-primary/40 bg-primary/10'
                  : 'border-lazyops-border hover:bg-lazyops-border/10'
              }`}
              onClick={() => {
                setSelectedBindingIdx(i);
                setValidationResult(null);
                setValidationError(null);
              }}
            >
              <div>
                <span className="text-sm font-medium text-lazyops-text">{binding.name}</span>
                <span className="ml-2 text-xs text-lazyops-muted">/{binding.target_ref}</span>
              </div>
              <StatusBadge label={binding.runtime_mode} variant="info" size="sm" dot={false} />
            </button>
          ))}
        </div>

        <div className="mt-4">
          <button
            type="button"
            className="rounded-lg bg-primary px-4 py-2 text-sm font-semibold text-lazyops-bg transition-colors hover:bg-primary/90 disabled:opacity-50"
            onClick={handleValidate}
            disabled={isValidating}
          >
            {isValidating ? 'Validating…' : 'Validate contract'}
          </button>
        </div>
      </SectionCard>

      {isValidating && <LoadingPage label="Validating contract…" />}

      {validationError && (
        <ErrorState title="Validation failed" message={validationError} />
      )}

      {validationResult && (
        <ValidationSummary result={validationResult} />
      )}
    </div>
  );
}

function ValidationSummary({ result }: { result: ValidateLazyopsResponse }) {
  const { project, deployment_binding, target_summary, schema } = result;

  return (
    <div className="flex flex-col gap-4">
      <SectionCard
        title="Validation result"
        description="Contract is valid — LazyOps can deploy based on this configuration."
      >
        <div className="flex items-center gap-2">
          <StatusBadge label="Valid" variant="success" size="md" />
          <span className="text-sm text-lazyops-muted">Ready for blueprint compilation.</span>
        </div>
      </SectionCard>

      <SectionCard title="Project" description={project.name}>
        <div className="grid gap-2 sm:grid-cols-2">
          <SummaryField label="Slug" value={project.slug} />
          <SummaryField label="Default branch" value={project.default_branch} />
        </div>
      </SectionCard>

      <SectionCard title="Deployment binding" description={deployment_binding.name}>
        <div className="grid gap-2 sm:grid-cols-2 lg:grid-cols-3">
          <SummaryField label="Target ref" value={deployment_binding.target_ref} />
          <SummaryField label="Runtime mode" value={deployment_binding.runtime_mode} />
          <SummaryField label="Target kind" value={deployment_binding.target_kind} />
        </div>
      </SectionCard>

      <SectionCard title="Target" description={target_summary.name}>
        <div className="grid gap-2 sm:grid-cols-2">
          <SummaryField label="Kind" value={target_summary.kind} />
          <SummaryField label="Status" value={target_summary.status} />
        </div>
      </SectionCard>

      <SectionCard title="Schema constraints" description="Rules enforced by the deploy contract.">
        <div className="flex flex-col gap-4">
          <div>
            <h4 className="mb-2 text-sm font-medium text-lazyops-text">Allowed dependency protocols</h4>
            <div className="flex flex-wrap gap-2">
              {schema.allowed_dependency_protocols.map((p) => (
                <StatusBadge key={p} label={p} variant="info" size="sm" dot={false} />
              ))}
            </div>
          </div>

          <div>
            <h4 className="mb-2 text-sm font-medium text-lazyops-text">Allowed magic domain providers</h4>
            <div className="flex flex-wrap gap-2">
              {schema.allowed_magic_domain_providers.map((p) => (
                <StatusBadge key={p} label={p} variant="neutral" size="sm" dot={false} />
              ))}
            </div>
          </div>

          <div>
            <h4 className="mb-2 text-sm font-medium text-lazyops-text">Forbidden fields</h4>
            <p className="mb-2 text-xs text-lazyops-muted">
              These fields must not appear in your lazyops.yaml. LazyOps manages them automatically.
            </p>
            <div className="flex flex-wrap gap-1.5">
              {schema.forbidden_field_names.map((f) => (
                <code key={f} className="rounded bg-health-unhealthy/10 px-1.5 py-0.5 text-[10px] text-health-unhealthy">
                  {f}
                </code>
              ))}
            </div>
          </div>
        </div>
      </SectionCard>
    </div>
  );
}

function SummaryField({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex flex-col gap-0.5">
      <span className="text-xs text-lazyops-muted">{label}</span>
      <span className="text-sm text-lazyops-text">{value}</span>
    </div>
  );
}
