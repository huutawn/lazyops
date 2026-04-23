import { ProjectObservabilityWorkspace } from '@/modules/observability/project-observability-workspace';

type ProjectObservabilityPageProps = {
  params: Promise<{ projectId: string }>;
};

export default async function ProjectObservabilityPage({ params }: ProjectObservabilityPageProps) {
  const { projectId } = await params;

  return <ProjectObservabilityWorkspace projectId={projectId} />;
}
