import type { ReactNode } from 'react';
import { ProjectTabs } from '@/components/primitives/project-tabs';

type ProjectLayoutProps = {
  children: ReactNode;
  params: Promise<{ projectId: string }>;
};

export default async function ProjectLayout({ children, params }: ProjectLayoutProps) {
  const { projectId } = await params;

  return (
    <div className="flex flex-col gap-6">
      <ProjectTabs projectId={projectId} />
      {children}
    </div>
  );
}
