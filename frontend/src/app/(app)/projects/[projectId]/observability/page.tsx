import { ObservabilityConsole } from '@/modules/observability/observability-console';

type ProjectObservabilityPageProps = {
  params: Promise<{ projectId: string }>;
};

export default async function ProjectObservabilityPage({ params }: ProjectObservabilityPageProps) {
  const { projectId } = await params;

  return (
    <ObservabilityConsole
      fixedProjectId={projectId}
      title="Giám sát"
      subtitle="Theo dõi logs, traces, incidents và metric trong phạm vi project này."
    />
  );
}
