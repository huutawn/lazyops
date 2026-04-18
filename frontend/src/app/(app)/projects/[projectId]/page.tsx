import { PageHeader } from '@/components/primitives/page-header';
import { ProjectOverviewDashboard } from '@/modules/projects/project-overview-dashboard';

type ProjectRootPageProps = {
  params: Promise<{ projectId: string }>;
};

export default async function ProjectRootPage({ params }: ProjectRootPageProps) {
  const { projectId } = await params;

  return (
    <div className="flex flex-col gap-6">
      <PageHeader
        title="Project namespace"
        subtitle="Project nay chi dong vai tro namespace va folder logic. Service moi la trung tam cua deploy, runtime, va ket noi."
      />
      <ProjectOverviewDashboard projectId={projectId} />
    </div>
  );
}
