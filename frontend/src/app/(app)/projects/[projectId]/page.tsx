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
        title="Bắt đầu"
        subtitle="Thiết lập project, nối mã nguồn và máy chủ, rồi triển khai ngay từ một nơi."
      />
      <ProjectOverviewDashboard projectId={projectId} />
    </div>
  );
}
