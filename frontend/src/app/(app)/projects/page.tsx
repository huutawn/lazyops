'use client';

import Link from 'next/link';
import { useProjects } from '@/modules/projects/project-hooks';
import { PageHeader } from '@/components/primitives/page-header';
import { SectionCard } from '@/components/primitives/section-card';
import { EmptyState } from '@/components/primitives/empty-state';
import { ErrorState } from '@/components/primitives/error-state';
import { SkeletonPage } from '@/components/primitives/skeleton';
import { StatusBadge } from '@/components/primitives/status-badge';

export default function ProjectsPage() {
  const { data, isLoading, isError } = useProjects();

  if (isLoading) {
    return <SkeletonPage title cards={1} />;
  }

  if (isError) {
    return (
      <div className="flex flex-col gap-6">
        <PageHeader title="Projects" subtitle="Manage your application projects." />
        <ErrorState title="Failed to load projects" message="Could not fetch projects from API." />
      </div>
    );
  }

  const projects = data?.items ?? [];

  return (
    <div className="flex flex-col gap-6">
      <PageHeader
        title="Projects"
        subtitle="Manage repositories, infrastructure readiness, and deployments per project."
        actions={
          <Link
            href="/onboarding"
            className="rounded-lg bg-primary px-4 py-2 text-sm font-semibold text-lazyops-bg transition-colors hover:bg-primary/90"
          >
            New project
          </Link>
        }
      />

      {projects.length === 0 ? (
        <SectionCard title="No projects yet" description="Create your first project to get started.">
          <EmptyState
            title="No projects found"
            description="Create a project from the onboarding flow, then connect repo and targets."
            action={
              <Link
                href="/onboarding"
                className="rounded-lg bg-primary px-4 py-2 text-sm font-semibold text-lazyops-bg transition-colors hover:bg-primary/90"
              >
                Open onboarding
              </Link>
            }
          />
        </SectionCard>
      ) : (
        <SectionCard>
          <div className="flex flex-col gap-2">
            {projects.map((project) => (
              <Link
                key={project.id}
                href={`/projects/${project.id}`}
                className="flex items-center justify-between rounded-lg border border-lazyops-border/50 px-3 py-3 transition-colors hover:bg-lazyops-border/10"
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
    </div>
  );
}
