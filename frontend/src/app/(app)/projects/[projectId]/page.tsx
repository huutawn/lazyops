import { redirect } from 'next/navigation';
import { PageHeader } from '@/components/primitives/page-header';
import { isFeatureEnabled } from '@/lib/flags/feature-flags';
import { ProjectThreeStepWizard } from '@/modules/bootstrap/project-three-step-wizard';

type ProjectRootPageProps = {
  params: Promise<{ projectId: string }>;
};

export default async function ProjectRootPage({ params }: ProjectRootPageProps) {
  const { projectId } = await params;
  if (!isFeatureEnabled('ux_three_step_flow')) {
    redirect(`/projects/${projectId}/integrations`);
  }

  return (
    <div className="flex flex-col gap-6">
      <PageHeader
        title="Thiết lập dự án"
        subtitle="3 bước: Kết nối mã nguồn, kết nối máy chủ, triển khai."
      />
      <ProjectThreeStepWizard projectId={projectId} />
    </div>
  );
}
