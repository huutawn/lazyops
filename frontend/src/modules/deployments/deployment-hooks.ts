import { useQuery } from '@tanstack/react-query';
import { mockListDeployments, mockGetDeployment } from '@/modules/deployments/deployment-mocks';
import type { DeploymentRecord, DeploymentDetail } from '@/modules/deployments/deployment-types';

export function deploymentsQueryKey(projectId?: string) {
  return ['deployments', projectId ?? 'all'] as const;
}

export function deploymentDetailQueryKey(id: string) {
  return ['deployment', id] as const;
}

export function useDeployments(projectId?: string) {
  return useQuery({
    queryKey: deploymentsQueryKey(projectId),
    queryFn: () => mockListDeployments(projectId),
    staleTime: 30 * 1000,
  });
}

export function useDeployment(id: string) {
  return useQuery({
    queryKey: deploymentDetailQueryKey(id),
    queryFn: () => mockGetDeployment(id),
    enabled: !!id,
    staleTime: 30 * 1000,
  });
}
