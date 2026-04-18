import { ProjectRuntimeWorkspace } from '@/modules/project-runtime/project-runtime-workspace';

type ProjectObservabilityPageProps = {
  params: Promise<{ projectId: string }>;
};

export default async function ProjectObservabilityPage({ params }: ProjectObservabilityPageProps) {
  const { projectId } = await params;

  return <ProjectRuntimeWorkspace projectId={projectId} />;
}
