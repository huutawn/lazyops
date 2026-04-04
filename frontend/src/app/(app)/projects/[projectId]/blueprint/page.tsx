'use client';

import { useState } from 'react';
import { useParams } from 'next/navigation';
import { compileBlueprint } from '@/modules/blueprint/blueprint-api';
import type { CompileBlueprintResponse, BlueprintService, PlacementAssignment } from '@/modules/blueprint/blueprint-types';
import { PageHeader } from '@/components/primitives/page-header';
import { SectionCard } from '@/components/primitives/section-card';
import { StatusBadge } from '@/components/primitives/status-badge';
import { HealthChip } from '@/components/primitives/health-chip';
import { LoadingPage } from '@/components/primitives/loading';
import { ErrorState } from '@/components/primitives/error-state';
import { FormField, FormInput, FormButton } from '@/components/forms/form-fields';

const EXPLANATION = {
  title: 'What is a blueprint?',
  description:
    'A blueprint is LazyOps\'s compiled deployment plan. It takes your lazyops.yaml contract and resolves it into concrete service definitions, placement assignments, and policies — ready for rollout.',
};

export default function BlueprintReviewPage() {
  const params = useParams();
  const projectId = params?.projectId as string;

  const [result, setResult] = useState<CompileBlueprintResponse | null>(null);
  const [isCompiling, setIsCompiling] = useState(false);
  const [compileError, setCompileError] = useState<string | null>(null);
  const [activeTab, setActiveTab] = useState<'services' | 'policies' | 'revision'>('services');

  const [formData, setFormData] = useState({
    source_ref: '',
    commit_sha: '',
    artifact_ref: '',
    image_ref: '',
    trigger_kind: 'api_blueprint_compile',
  });

  const handleCompile = async () => {
    if (!formData.commit_sha.trim()) {
      setCompileError('Commit SHA is required.');
      return;
    }

    setIsCompiling(true);
    setCompileError(null);

    try {
      const lazyopsYaml = {};
      const reqData = {
        source_ref: formData.source_ref || undefined,
        trigger_kind: formData.trigger_kind || undefined,
        artifact_metadata: {
          commit_sha: formData.commit_sha,
          artifact_ref: formData.artifact_ref || undefined,
          image_ref: formData.image_ref || undefined,
        },
        lazyops_yaml: lazyopsYaml,
      };

      const res = await compileBlueprint(projectId, reqData);
      if (res.error) {
        setCompileError(res.error.message);
      } else if (res.data) {
        setResult(res.data);
      }
    } catch (err) {
      setCompileError(err instanceof Error ? err.message : 'Compilation failed');
    } finally {
      setIsCompiling(false);
    }
  };

  return (
    <div className="flex flex-col gap-6">
      <PageHeader
        title="Blueprint Review"
        subtitle="Compile and review the deployment blueprint before rollout."
      />

      <SectionCard
        title={EXPLANATION.title}
        description={EXPLANATION.description}
      >
        <p className="text-sm text-lazyops-muted">
          The blueprint resolves your deployment contract into a concrete plan with service definitions,
          placement assignments, and policy configurations. Review it carefully before proceeding to rollout.
        </p>
      </SectionCard>

      <SectionCard title="Compile blueprint" description="Provide source and artifact information.">
        <div className="flex flex-col gap-4">
          <FormField label="Source reference (owner/repo@branch)">
            <FormInput
              type="text"
              placeholder="myorg/myrepo@main"
              value={formData.source_ref}
              onChange={(e) => setFormData({ ...formData, source_ref: e.target.value })}
            />
          </FormField>

          <div className="grid gap-4 sm:grid-cols-2">
            <FormField label="Commit SHA" >
              <FormInput
                type="text"
                placeholder="abc123def456"
                value={formData.commit_sha}
                onChange={(e) => setFormData({ ...formData, commit_sha: e.target.value })}
              />
            </FormField>

            <FormField label="Trigger kind">
              <FormInput
                type="text"
                placeholder="api_blueprint_compile"
                value={formData.trigger_kind}
                onChange={(e) => setFormData({ ...formData, trigger_kind: e.target.value })}
              />
            </FormField>
          </div>

          <div className="grid gap-4 sm:grid-cols-2">
            <FormField label="Artifact reference">
              <FormInput
                type="text"
                placeholder="v1.2.3"
                value={formData.artifact_ref}
                onChange={(e) => setFormData({ ...formData, artifact_ref: e.target.value })}
              />
            </FormField>

            <FormField label="Image reference">
              <FormInput
                type="text"
                placeholder="ghcr.io/myorg/myapp:latest"
                value={formData.image_ref}
                onChange={(e) => setFormData({ ...formData, image_ref: e.target.value })}
              />
            </FormField>
          </div>

          {compileError && (
            <div className="rounded-lg border border-health-unhealthy/30 bg-health-unhealthy/10 px-3 py-2 text-xs text-health-unhealthy">
              {compileError}
            </div>
          )}

          <FormButton onClick={handleCompile} loading={isCompiling}>
            Compile blueprint
          </FormButton>
        </div>
      </SectionCard>

      {isCompiling && <LoadingPage label="Compiling blueprint…" />}

      {compileError && !isCompiling && (
        <ErrorState title="Blueprint compilation failed" message={compileError} />
      )}

      {result && (
        <div className="flex flex-col gap-4">
          <div className="flex gap-2 border-b border-lazyops-border">
            {(['services', 'policies', 'revision'] as const).map((tab) => (
              <button
                key={tab}
                type="button"
                className={`rounded-t-lg px-4 py-2 text-sm font-medium transition-colors ${
                  activeTab === tab
                    ? 'border-b-2 border-primary text-primary'
                    : 'text-lazyops-muted hover:text-lazyops-text'
                }`}
                onClick={() => setActiveTab(tab)}
              >
                {tab.charAt(0).toUpperCase() + tab.slice(1)}
              </button>
            ))}
          </div>

          {activeTab === 'services' && <ServicesTab services={result.services} blueprint={result.blueprint} />}
          {activeTab === 'policies' && <PoliciesTab blueprint={result.blueprint} />}
          {activeTab === 'revision' && <RevisionTab draft={result.desired_revision_draft} />}
        </div>
      )}
    </div>
  );
}

function ServicesTab({ services, blueprint }: { services: BlueprintService[]; blueprint: { compiled: { runtime_mode: string; repo: { repo_full_name: string; tracked_branch: string } } } }) {
  if (services.length === 0) {
    return (
      <SectionCard title="Services" description="No services defined in this blueprint.">
        <p className="text-sm text-lazyops-muted">Add services to your lazyops.yaml to see them here.</p>
      </SectionCard>
    );
  }

  return (
    <div className="flex flex-col gap-4">
      <SectionCard
        title="Services"
        description={`${services.length} service${services.length > 1 ? 's' : ''} compiled from blueprint.`}
      >
        <div className="flex flex-col gap-3">
          {services.map((svc) => (
            <div key={svc.id} className="rounded-lg border border-lazyops-border/50 bg-lazyops-bg-accent/30 p-4">
              <div className="mb-2 flex items-center justify-between">
                <h4 className="text-sm font-semibold text-lazyops-text">{svc.name}</h4>
                <div className="flex items-center gap-2">
                  {svc.public && <StatusBadge label="Public" variant="info" size="sm" dot={false} />}
                  <HealthChip label={svc.runtime_profile || 'default'} status="healthy" size="sm" />
                </div>
              </div>
              <div className="grid gap-1 text-xs sm:grid-cols-2">
                <span className="text-lazyops-muted">Path: <code className="text-lazyops-text">{svc.path}</code></span>
                {svc.healthcheck && (
                  <span className="text-lazyops-muted">Health: <code className="text-lazyops-text">{svc.healthcheck.path}:{svc.healthcheck.port}</code></span>
                )}
                {svc.start_hint && (
                  <span className="text-lazyops-muted">Start: <code className="text-lazyops-text">{svc.start_hint}</code></span>
                )}
              </div>
            </div>
          ))}
        </div>
      </SectionCard>

      <SectionCard title="Source" description="Repository and branch used for this blueprint.">
        <div className="grid gap-2 sm:grid-cols-2">
          <SummaryField label="Repository" value={blueprint.compiled.repo.repo_full_name} />
          <SummaryField label="Branch" value={blueprint.compiled.repo.tracked_branch} />
          <SummaryField label="Runtime mode" value={blueprint.compiled.runtime_mode} />
        </div>
      </SectionCard>
    </div>
  );
}

function PoliciesTab({ blueprint }: { blueprint: { compiled: { compatibility_policy: Record<string, unknown>; magic_domain_policy: Record<string, unknown>; scale_to_zero_policy: Record<string, unknown> } } }) {
  const { compatibility_policy, magic_domain_policy, scale_to_zero_policy } = blueprint.compiled;

  return (
    <div className="flex flex-col gap-4">
      <SectionCard title="Compatibility policy" description="How services interact with dependencies.">
        <PolicyGrid data={compatibility_policy} />
      </SectionCard>

      <SectionCard title="Magic domain policy" description="Automatic domain routing for services.">
        <PolicyGrid data={magic_domain_policy} />
      </SectionCard>

      <SectionCard title="Scale-to-zero policy" description="Whether idle services can be scaled down.">
        <PolicyGrid data={scale_to_zero_policy} />
      </SectionCard>
    </div>
  );
}

function RevisionTab({ draft }: { draft: { revision_id: string; commit_sha: string; artifact_ref: string; image_ref: string; trigger_kind: string; runtime_mode: string; placement_assignments: PlacementAssignment[] } }) {
  return (
    <div className="flex flex-col gap-4">
      <SectionCard title="Desired revision draft" description="What LazyOps intends to deploy next.">
        <div className="grid gap-2 sm:grid-cols-2 lg:grid-cols-3">
          <SummaryField label="Revision" value={draft.revision_id} />
          <SummaryField label="Commit" value={draft.commit_sha} />
          <SummaryField label="Artifact" value={draft.artifact_ref || '—'} />
          <SummaryField label="Image" value={draft.image_ref || '—'} />
          <SummaryField label="Trigger" value={draft.trigger_kind} />
          <SummaryField label="Runtime mode" value={draft.runtime_mode} />
        </div>
      </SectionCard>

      <SectionCard
        title="Placement assignments"
        description={`${draft.placement_assignments.length} service${draft.placement_assignments.length !== 1 ? 's' : ''} assigned to targets.`}
      >
        {draft.placement_assignments.length === 0 ? (
          <p className="text-sm text-lazyops-muted">No placement assignments yet.</p>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead>
                <tr className="border-b border-lazyops-border">
                  <th className="px-4 py-2 text-left text-xs font-medium text-lazyops-muted">Service</th>
                  <th className="px-4 py-2 text-left text-xs font-medium text-lazyops-muted">Target</th>
                  <th className="px-4 py-2 text-left text-xs font-medium text-lazyops-muted">Kind</th>
                  <th className="px-4 py-2 text-left text-xs font-medium text-lazyops-muted">Labels</th>
                </tr>
              </thead>
              <tbody>
                {draft.placement_assignments.map((pa) => (
                  <tr key={pa.service_name} className="border-b border-lazyops-border/50">
                    <td className="px-4 py-2 font-medium text-lazyops-text">{pa.service_name}</td>
                    <td className="px-4 py-2 font-mono text-xs text-lazyops-muted">{pa.target_id}</td>
                    <td className="px-4 py-2 text-xs text-lazyops-muted">{pa.target_kind}</td>
                    <td className="px-4 py-2">
                      <div className="flex flex-wrap gap-1">
                        {Object.entries(pa.labels).map(([k, v]) => (
                          <span key={k} className="rounded bg-lazyops-border/20 px-1.5 py-0.5 text-[10px] text-lazyops-muted">
                            {k}: {v}
                          </span>
                        ))}
                        {Object.keys(pa.labels).length === 0 && <span className="text-xs text-lazyops-muted/50">—</span>}
                      </div>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </SectionCard>
    </div>
  );
}

function SummaryField({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex flex-col gap-0.5">
      <span className="text-xs text-lazyops-muted">{label}</span>
      <span className="truncate text-sm text-lazyops-text" title={value}>{value}</span>
    </div>
  );
}

function PolicyGrid({ data }: { data: Record<string, unknown> }) {
  const entries = Object.entries(data);
  if (entries.length === 0) {
    return <p className="text-sm text-lazyops-muted">No policy rules configured.</p>;
  }
  return (
    <div className="grid gap-2 sm:grid-cols-2">
      {entries.map(([key, value]) => (
        <div key={key} className="flex items-center justify-between rounded-md bg-lazyops-bg-accent/50 px-3 py-2">
          <span className="text-xs text-lazyops-muted">{key}</span>
          <span className="text-sm text-lazyops-text">
            {typeof value === 'boolean' ? (value ? 'Yes' : 'No') : String(value)}
          </span>
        </div>
      ))}
    </div>
  );
}
