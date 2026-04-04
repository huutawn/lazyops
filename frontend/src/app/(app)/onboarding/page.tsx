'use client';

import { useState } from 'react';
import { useProjects } from '@/modules/projects/project-hooks';
import { PageHeader } from '@/components/primitives/page-header';
import { SectionCard } from '@/components/primitives/section-card';
import { EmptyState } from '@/components/primitives/empty-state';
import { SkeletonPage } from '@/components/primitives/skeleton';
import { StatusBadge } from '@/components/primitives/status-badge';
import { CreateProjectForm } from '@/modules/onboarding/create-project-form';
import { RuntimeModeCard } from '@/modules/onboarding/runtime-mode-card';
import { RUNTIME_MODES } from '@/modules/onboarding/runtime-modes';
import { LoadingBlock } from '@/components/primitives/loading';
import Link from 'next/link';

const ONBOARDING_STEPS = [
  { step: 1, title: 'Create a project', description: 'Give your application a home in LazyOps.' },
  { step: 2, title: 'Connect GitHub', description: 'Link your repository for automated deployments.' },
  { step: 3, title: 'Add targets', description: 'Register machines or clusters to deploy onto.' },
  { step: 4, title: 'Deploy', description: 'LazyOps handles the rest — no YAML required.' },
];

export default function OnboardingPage() {
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
