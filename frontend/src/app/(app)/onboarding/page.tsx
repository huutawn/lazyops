'use client';

import Link from 'next/link';
import { useState } from 'react';
import { LoadingBlock } from '@/components/primitives/loading';
import { EmptyState } from '@/components/primitives/empty-state';
import { PageHeader } from '@/components/primitives/page-header';
import { SectionCard } from '@/components/primitives/section-card';
import { SkeletonPage } from '@/components/primitives/skeleton';
import { StatusBadge } from '@/components/primitives/status-badge';
import { isFeatureEnabled } from '@/lib/flags/feature-flags';
import { ProjectThreeStepWizard } from '@/modules/bootstrap/project-three-step-wizard';
import { CreateProjectForm } from '@/modules/onboarding/create-project-form';
import { RuntimeModeCard } from '@/modules/onboarding/runtime-mode-card';
import { RUNTIME_MODES } from '@/modules/onboarding/runtime-modes';
import { useProjects } from '@/modules/projects/project-hooks';

const ONBOARDING_STEPS = [
  { step: 1, title: 'Create a project', description: 'Give your application a home in LazyOps.' },
  { step: 2, title: 'Connect GitHub', description: 'Link your repository for automated deployments.' },
  { step: 3, title: 'Add targets', description: 'Register machines or clusters to deploy onto.' },
  { step: 4, title: 'Deploy', description: 'LazyOps handles the rest — no YAML required.' },
];

export default function OnboardingPage() {
  if (!isFeatureEnabled('ux_three_step_flow')) {
    return <LegacyOnboardingPage />;
  }
  return <ThreeStepOnboardingPage />;
}

function ThreeStepOnboardingPage() {
  const { data: projects, isLoading } = useProjects();
  const [showCreateForm, setShowCreateForm] = useState(false);
  const [selectedProjectID, setSelectedProjectID] = useState<string | null>(null);

  const projectItems = projects?.items ?? [];
  const effectiveProjectID =
    selectedProjectID && projectItems.some((project) => project.id === selectedProjectID)
      ? selectedProjectID
      : (projectItems[0]?.id ?? null);

  const selectedProject = projectItems.find((project) => project.id === effectiveProjectID) ?? null;

  if (isLoading) {
    return <SkeletonPage title cards={2} />;
  }

  return (
    <div className="flex flex-col gap-6">
      <PageHeader
        title="LazyOps 3-step onboarding"
        subtitle="Connect code, connect infrastructure, then deploy. No binding JSON needed."
      />

      {projectItems.length === 0 ? (
        <>
          {!showCreateForm ? (
            <SectionCard
              title="Step 0: Create your first project"
              description="Once the project exists, the 3-step wizard becomes available immediately."
              actions={
                <button
                  type="button"
                  className="rounded-lg bg-primary px-4 py-1.5 text-sm font-semibold text-lazyops-bg transition-colors hover:bg-primary/90"
                  onClick={() => setShowCreateForm(true)}
                >
                  New project
                </button>
              }
            >
              <EmptyState
                title="No projects yet"
                description="Create one project to unlock the simplified flow."
                action={
                  <button
                    type="button"
                    className="rounded-lg bg-primary px-4 py-2 text-sm font-semibold text-lazyops-bg transition-colors hover:bg-primary/90"
                    onClick={() => setShowCreateForm(true)}
                  >
                    Create project
                  </button>
                }
              />
            </SectionCard>
          ) : (
            <CreateProjectForm onSuccess={() => setShowCreateForm(false)} />
          )}
        </>
      ) : (
        <>
          <SectionCard
            title="Choose project"
            description="The wizard below will run setup for the selected project."
            actions={
              selectedProject ? (
                <Link
                  href={`/projects/${selectedProject.id}`}
                  className="rounded-lg border border-lazyops-border px-3 py-1.5 text-xs font-semibold text-lazyops-text transition-colors hover:bg-lazyops-border/10"
                >
                  Open project page
                </Link>
              ) : null
            }
          >
            <div className="grid gap-2 sm:grid-cols-2">
              {projectItems.map((project) => {
                const selected = project.id === effectiveProjectID;
                return (
                  <button
                    key={project.id}
                    type="button"
                    className={`rounded-lg border px-3 py-2 text-left transition-colors ${
                      selected
                        ? 'border-primary/50 bg-primary/10'
                        : 'border-lazyops-border hover:bg-lazyops-border/10'
                    }`}
                    onClick={() => setSelectedProjectID(project.id)}
                  >
                    <div className="mb-1 flex items-center justify-between">
                      <span className="text-sm font-medium text-lazyops-text">{project.name}</span>
                      <StatusBadge label={project.default_branch} variant="neutral" size="sm" dot={false} />
                    </div>
                    <p className="text-xs text-lazyops-muted">/{project.slug}</p>
                  </button>
                );
              })}
            </div>
          </SectionCard>

          {selectedProject ? <ProjectThreeStepWizard projectId={selectedProject.id} /> : <LoadingBlock label="Loading project..." className="py-6" />}
        </>
      )}
    </div>
  );
}

function LegacyOnboardingPage() {
  const { data: projects, isLoading } = useProjects();
  const [showCreateForm, setShowCreateForm] = useState(false);

  if (isLoading) {
    return <SkeletonPage title cards={2} />;
  }

  const hasProjects = projects && projects.items.length > 0;

  return (
    <div className="flex flex-col gap-6">
      <PageHeader
        title="Welcome to LazyOps"
        subtitle="Infrastructure without the complexity. Let's get you set up."
      />

      {hasProjects && (
        <SectionCard title="Your projects" description="You already have projects set up.">
          <div className="flex flex-col gap-2">
            {projects.items.map((project) => (
              <Link
                key={project.id}
                href={`/projects/${project.id}`}
                className="flex items-center justify-between rounded-lg px-3 py-2 transition-colors hover:bg-lazyops-border/20"
              >
                <div className="flex flex-col">
                  <span className="text-sm font-medium text-lazyops-text">{project.name}</span>
                  <span className="text-xs text-lazyops-muted">/{project.slug}</span>
                </div>
                <StatusBadge label={project.default_branch} variant="neutral" size="sm" dot={false} />
              </Link>
            ))}
          </div>
        </SectionCard>
      )}

      {!showCreateForm ? (
        <SectionCard
          title="Get started"
          description="Create your first project to begin deploying services."
          actions={
            <button
              type="button"
              className="rounded-lg bg-primary px-4 py-1.5 text-sm font-semibold text-lazyops-bg transition-colors hover:bg-primary/90"
              onClick={() => setShowCreateForm(true)}
            >
              New project
            </button>
          }
        >
          {hasProjects ? null : (
            <EmptyState
              title="No projects yet"
              description="Create your first project to start managing targets, integrations, and deployments."
              action={
                <button
                  type="button"
                  className="rounded-lg bg-primary px-4 py-2 text-sm font-semibold text-lazyops-bg transition-colors hover:bg-primary/90"
                  onClick={() => setShowCreateForm(true)}
                >
                  Create project
                </button>
              }
            />
          )}
        </SectionCard>
      ) : (
        <CreateProjectForm onSuccess={() => setShowCreateForm(false)} />
      )}

      <SectionCard
        title="How it works"
        description="LazyOps supports three runtime modes to match your infrastructure needs."
      >
        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
          {RUNTIME_MODES.map((mode) => (
            <RuntimeModeCard key={mode.id} {...mode} />
          ))}
        </div>
      </SectionCard>

      <SectionCard title="Setup checklist" description="Follow these steps to get your first service running.">
        <div className="flex flex-col gap-3">
          {ONBOARDING_STEPS.map((item) => (
            <div key={item.step} className="flex items-start gap-4">
              <div className="flex size-7 shrink-0 items-center justify-center rounded-full bg-primary/15 text-xs font-bold text-primary">
                {item.step}
              </div>
              <div className="flex flex-col">
                <span className="text-sm font-medium text-lazyops-text">{item.title}</span>
                <span className="text-xs text-lazyops-muted">{item.description}</span>
              </div>
            </div>
          ))}
        </div>
      </SectionCard>
    </div>
  );
}
