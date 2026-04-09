import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { listProjects } from '@/modules/projects/project-api';
import { actProjectDeployment, getProjectDeployment, listProjectDeployments } from '@/modules/deployments/deployment-api';
import type { DeploymentRecord, DeploymentDetail } from '@/modules/deployments/deployment-types';
import type { DeploymentAction } from '@/modules/deployments/deployment-api';

export function deploymentsQueryKey(projectId?: string) {
  return ['deployments', projectId ?? 'all'] as const;
}

export function deploymentDetailQueryKey(projectId: string | undefined, id: string) {
  return ['deployment', projectId ?? 'all', id] as const;
}

async function listDeploymentsAcrossProjects(): Promise<{ items: DeploymentRecord[] }> {
  const projectsResult = await listProjects();
  if (projectsResult.error) {
    throw new Error(projectsResult.error.message);
  }

  const projects = projectsResult.data?.items ?? [];
  if (projects.length === 0) {
    return { items: [] };
  }

  const resultSet = await Promise.all(
    projects.map(async (project) => {
      const result = await listProjectDeployments(project.id);
      if (result.error) {
        throw new Error(result.error.message);
      }
      return result.data?.items ?? [];
    }),
  );

  const items = resultSet.flat();
  items.sort((a, b) => new Date(b.created_at).getTime() - new Date(a.created_at).getTime());
  return { items };
}

async function getDeploymentAcrossProjects(id: string): Promise<DeploymentDetail> {
  const projectsResult = await listProjects();
  if (projectsResult.error) {
    throw new Error(projectsResult.error.message);
  }

  const projects = projectsResult.data?.items ?? [];
  for (const project of projects) {
    const result = await getProjectDeployment(project.id, id);
    if (!result.error && result.data) {
      return result.data;
    }
    if (result.error && result.error.code !== 'deployment_not_found') {
      throw new Error(result.error.message);
    }
  }

  throw new Error('Deployment not found');
}

export function useDeployments(projectId?: string) {
  return useQuery({
    queryKey: deploymentsQueryKey(projectId),
    queryFn: async () => {
      if (projectId) {
        const result = await listProjectDeployments(projectId);
        if (result.error) {
          throw new Error(result.error.message);
        }
        return result.data ?? { items: [] };
      }
      return listDeploymentsAcrossProjects();
    },
    staleTime: 30 * 1000,
  });
}

export function useDeployment(projectId: string | undefined, id: string) {
  return useQuery({
    queryKey: deploymentDetailQueryKey(projectId, id),
    queryFn: async () => {
      if (projectId) {
        const result = await getProjectDeployment(projectId, id);
        if (result.error) {
          throw new Error(result.error.message);
        }
        if (!result.data) {
          throw new Error('Deployment not found');
        }
        return result.data;
      }
      return getDeploymentAcrossProjects(id);
    },
    enabled: !!id,
    staleTime: 30 * 1000,
  });
}

export function useDeploymentAction(projectId: string | undefined, deploymentId: string | undefined) {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async (action: DeploymentAction) => {
      if (!projectId || !deploymentId) {
        throw new Error('Missing deployment context');
      }
      const result = await actProjectDeployment(projectId, deploymentId, action);
      if (result.error) {
        throw new Error(result.error.message);
      }
      if (!result.data) {
        throw new Error('Deployment action failed: missing response payload');
      }
      return result.data;
    },
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: deploymentsQueryKey(projectId) });
      if (deploymentId) {
        void queryClient.invalidateQueries({ queryKey: deploymentDetailQueryKey(projectId, deploymentId) });
      }
    },
  });
}
