'use client';

import { useState } from 'react';
import { useParams } from 'next/navigation';
import { useDeploymentBindings } from '@/modules/deployment-bindings/binding-hooks';
import { useProjectRouting } from '@/modules/project-routing/project-routing-hooks';
import { useProjectServices } from '@/modules/project-services/project-service-hooks';
import { validateLazyopsYaml } from '@/modules/validate-lazyops/validate-api';
import type { ValidateLazyopsResponse, LazyopsYAMLDraft } from '@/modules/validate-lazyops/validate-types';
import { PageHeader } from '@/components/primitives/page-header';
import { SectionCard } from '@/components/primitives/section-card';
import { StatusBadge } from '@/components/primitives/status-badge';
import { LoadingPage } from '@/components/primitives/loading';
import { ErrorState } from '@/components/primitives/error-state';
import { useProjectExpertRouteGuard } from '@/modules/projects/project-flow-hooks';

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

export default function ValidateContractPage() {
  const params = useParams();
  const projectId = params?.projectId as string;
  const { shouldBlock } = useProjectExpertRouteGuard(projectId);

  const { data: bindingsData, isLoading: bindingsLoading } = useDeploymentBindings(projectId);
  const services = useProjectServices(projectId);
  const routing = useProjectRouting(projectId);
  const [validationResult, setValidationResult] = useState<ValidateLazyopsResponse | null>(null);
  const [isValidating, setIsValidating] = useState(false);
  const [validationError, setValidationError] = useState<string | null>(null);
  const [selectedBindingIdx, setSelectedBindingIdx] = useState(0);

  if (shouldBlock) {
    return <LoadingPage label="Redirecting to 3-step setup…" />;
  }

  const bindings = bindingsData?.items ?? [];

  const handleValidate = async () => {
    if (bindings.length === 0) return;
    const binding = bindings[selectedBindingIdx];

    setIsValidating(true);
    setValidationError(null);

    const servicesDraft = (services.data?.items ?? []).map((service) => {
      const healthPath = typeof service.healthcheck?.['path'] === 'string' ? service.healthcheck['path'] : '';
      const healthPort = typeof service.healthcheck?.['port'] === 'number' ? service.healthcheck['port'] : 0;
      return {
        name: service.name,
        path: service.path,
        start_hint: service.start_hint || undefined,
        public: service.public,
        healthcheck: healthPath && healthPort > 0 ? {
          path: healthPath,
          port: healthPort,
        } : undefined,
      };
    });

    const draft: LazyopsYAMLDraft = {
      project_slug: '',
      runtime_mode: binding.runtime_mode,
      deployment_binding: { target_ref: binding.target_ref },
      services: servicesDraft,
      dependency_bindings: [],
      compatibility_policy: { env_injection: false, managed_credentials: false, localhost_rescue: false },
      magic_domain_policy: { enabled: false, provider: '' },
      preview_policy: { enabled: false },
      scale_to_zero_policy: { enabled: binding.scale_to_zero_policy?.enabled === true },
      routing_policy: {
        shared_domain: routing.data?.routing_policy?.shared_domain,
        routes: routing.data?.routing_policy?.routes ?? [],
      },
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

  if (bindingsLoading || services.isLoading || routing.isLoading) {
    return <LoadingPage label="Loading bindings…" />;
  }

  if (bindings.length === 0) {
    return (
      <div className="flex flex-col gap-6">
      <PageHeader
        title="Deploy Contract"
        subtitle="Expert/debug route. Normal service-first deployments da tu validate deploy plan ngầm trước khi rollout."
      />
        <SectionCard title="No bindings available" description="Create a deployment binding first to validate a contract.">
          <p className="text-base text-lazyops-muted">
            You need at least one deployment binding before you can review the deploy contract.
          </p>
        </SectionCard>
      </div>
    );
  }

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
              <div className="flex size-6 shrink-0 items-center justify-center rounded-full bg-primary/15 text-sm font-bold text-primary">
                {i + 1}
              </div>
              <div>
                <span className="text-base font-medium text-lazyops-text">{step.title}</span>
                <p className="text-sm text-lazyops-muted">{step.desc}</p>
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
              className={`flex items-center justify-between rounded-lg border px-6 py-3 text-left transition-colors ${
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
                <span className="text-base font-medium text-lazyops-text">{binding.name}</span>
                <span className="ml-2 text-sm text-lazyops-muted">/{binding.target_ref}</span>
              </div>
              <StatusBadge label={binding.runtime_mode} variant="info" size="sm" dot={false} />
            </button>
          ))}
        </div>

        <div className="mt-4">
          <button
            type="button"
            className="rounded-lg bg-primary px-6 py-2 text-base font-semibold text-lazyops-bg transition-colors hover:bg-primary/90 disabled:opacity-50"
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
          <span className="text-base text-lazyops-muted">Ready for the internal deploy-plan compiler used by the service-first flow.</span>
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
            <h4 className="mb-2 text-base font-medium text-lazyops-text">Allowed dependency protocols</h4>
            <div className="flex flex-wrap gap-2">
              {schema.allowed_dependency_protocols.map((p) => (
                <StatusBadge key={p} label={p} variant="info" size="sm" dot={false} />
              ))}
            </div>
          </div>

          <div>
            <h4 className="mb-2 text-base font-medium text-lazyops-text">Allowed magic domain providers</h4>
            <div className="flex flex-wrap gap-2">
              {schema.allowed_magic_domain_providers.map((p) => (
                <StatusBadge key={p} label={p} variant="neutral" size="sm" dot={false} />
              ))}
            </div>
          </div>

          <div>
            <h4 className="mb-2 text-base font-medium text-lazyops-text">Forbidden fields</h4>
            <p className="mb-2 text-sm text-lazyops-muted">
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

      <SectionCard title="Suggested public paths" description="Convention-first preview based on current services and routing policy.">
        <div className="space-y-3">
          {result.suggested_routes.length > 0 ? result.suggested_routes.map((route) => (
            <div key={`suggested:${route.service}:${route.path}`} className="rounded-lg border border-[#1e293b] bg-[#0B1120]/30 px-4 py-3">
              <p className="text-base font-medium text-lazyops-text">{route.service}</p>
              <p className="text-sm text-lazyops-muted">Suggested path: <code className="rounded bg-[#0B1120]/60 px-1.5 py-0.5 text-[#38BDF8]">{route.path}</code>{route.websocket ? ' · WebSocket' : ''}</p>
            </div>
          )) : (
            <p className="text-sm text-lazyops-muted">No public services were detected in the current draft.</p>
          )}
        </div>
      </SectionCard>

      <SectionCard title="Effective public paths" description="These are the public paths clients should actually use.">
        <div className="space-y-3">
          {result.effective_public_paths.length > 0 ? result.effective_public_paths.map((route) => (
            <div key={`effective:${route.service}:${route.path}`} className="rounded-lg border border-[#1e293b] bg-[#0B1120]/30 px-4 py-3">
              <p className="text-base font-medium text-lazyops-text">{route.service}</p>
              <p className="text-sm text-lazyops-muted">Effective path: <code className="rounded bg-[#0B1120]/60 px-1.5 py-0.5 text-[#38BDF8]">{route.path}</code>{route.websocket ? ' · WebSocket' : ''}</p>
            </div>
          )) : (
            <p className="text-sm text-lazyops-muted">No effective public path is configured for the current draft.</p>
          )}
        </div>
      </SectionCard>

      <SectionCard title="Migration findings" description="Warnings when the draft still looks local-only or when custom public paths require client updates.">
        <div className="space-y-3">
          {result.migration_findings.length > 0 ? result.migration_findings.map((finding) => (
            <div key={`${finding.category}:${finding.service_name}:${finding.current_value}`} className="rounded-lg border border-amber-500/20 bg-amber-500/5 px-4 py-3">
              <p className="text-base font-medium text-amber-200">{finding.message}</p>
              {finding.current_value ? (
                <p className="mt-1 text-sm text-amber-100/80">Current: <code className="rounded bg-[#0B1120]/60 px-1.5 py-0.5">{finding.current_value}</code></p>
              ) : null}
              {finding.recommended_value ? (
                <p className="mt-1 text-sm text-amber-100/80">Recommended: <code className="rounded bg-[#0B1120]/60 px-1.5 py-0.5">{finding.recommended_value}</code></p>
              ) : null}
            </div>
          )) : (
            <p className="text-sm text-lazyops-muted">No localhost or custom-path migration findings were detected in the current draft.</p>
          )}
        </div>
      </SectionCard>
    </div>
  );
}

function SummaryField({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex flex-col gap-0.5">
      <span className="text-sm text-lazyops-muted">{label}</span>
      <span className="text-base text-lazyops-text">{value}</span>
    </div>
  );
}
