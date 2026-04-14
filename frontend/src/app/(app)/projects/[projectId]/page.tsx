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
        title="Tổng quan dự án"
        subtitle="Public domain, runtime hiện tại, env, internal services và các đường dẫn thao tác chính cho project này."
      />
      <ProjectOverviewDashboard projectId={projectId} />
    </div>
  );
}
