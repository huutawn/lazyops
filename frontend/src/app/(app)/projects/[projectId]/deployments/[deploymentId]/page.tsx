'use client';

import { useParams } from 'next/navigation';
import DeploymentDetailPage from '@/app/(app)/deployments/[deploymentId]/page';

export default function ProjectDeploymentDetailPage() {
  return <DeploymentDetailPage />;
}
