import { redirect } from 'next/navigation';

type ProjectRootPageProps = {
  params: Promise<{ projectId: string }>;
};

export default async function ProjectRootPage({ params }: ProjectRootPageProps) {
  const { projectId } = await params;
  redirect(`/projects/${projectId}/integrations`);
}
