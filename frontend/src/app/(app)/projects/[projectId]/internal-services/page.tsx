import { redirect } from 'next/navigation';

type LegacyProjectInternalServicesPageProps = {
  params: Promise<{ projectId: string }>;
};

export default async function LegacyProjectInternalServicesPage({
  params,
}: LegacyProjectInternalServicesPageProps) {
  const { projectId } = await params;
  redirect(`/projects/${projectId}/services?source=internal`);
}
