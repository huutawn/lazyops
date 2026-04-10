import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { autoBootstrapProject, connectProjectInfraSSH, deployProjectOneClick, getProjectBootstrapStatus } from '@/modules/bootstrap/bootstrap-api';
import type {
  BootstrapAutoAccepted,
  BootstrapAutoRequest,
  BootstrapConnectInfraSSHRequest,
  BootstrapConnectInfraSSHResult,
  BootstrapOneClickDeployRequest,
  BootstrapOneClickDeployResult,
  ProjectBootstrapStatus,
} from '@/modules/bootstrap/bootstrap-types';

export function bootstrapStatusQueryKey(projectId: string) {
  return ['bootstrap-status', projectId] as const;
}

export function useProjectBootstrapStatus(projectId: string) {
  return useQuery({
    queryKey: bootstrapStatusQueryKey(projectId),
    queryFn: async (): Promise<ProjectBootstrapStatus> => {
      const result = await getProjectBootstrapStatus(projectId);
      if (result.error) {
        throw new Error(result.error.message);
      }
      if (!result.data) {
        throw new Error('Bootstrap status unavailable');
      }
      return result.data;
    },
    enabled: !!projectId,
    staleTime: 20 * 1000,
    refetchInterval: 20 * 1000,
  });
}

export function useAutoBootstrapProject(projectId: string) {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async (data: Partial<BootstrapAutoRequest> = {}): Promise<BootstrapAutoAccepted> => {
      const result = await autoBootstrapProject({
        project_id: projectId,
        ...data,
      });
      if (result.error) {
        throw new Error(result.error.message);
      }
      if (!result.data) {
        throw new Error('Auto bootstrap failed: missing response payload');
      }
      return result.data;
    },
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: bootstrapStatusQueryKey(projectId) });
      void queryClient.invalidateQueries({ queryKey: ['deployment-bindings', projectId] });
    },
  });
}

export function useOneClickDeploy(projectId: string) {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async (data: BootstrapOneClickDeployRequest = {}): Promise<BootstrapOneClickDeployResult> => {
      const result = await deployProjectOneClick(projectId, data);
      if (result.error) {
        throw new Error(result.error.message);
      }
      if (!result.data) {
        throw new Error('One-click deploy failed: missing response payload');
      }
      return result.data;
    },
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: bootstrapStatusQueryKey(projectId) });
      void queryClient.invalidateQueries({ queryKey: ['deployments', projectId] });
    },
  });
}

export function useConnectProjectInfraSSH(projectId: string) {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async (data: BootstrapConnectInfraSSHRequest): Promise<BootstrapConnectInfraSSHResult> => {
      const result = await connectProjectInfraSSH(projectId, data);
      if (result.error) {
        throw new Error(result.error.message);
      }
      if (!result.data) {
        throw new Error('Infra connection failed: missing response payload');
      }
      return result.data;
    },
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: bootstrapStatusQueryKey(projectId) });
      void queryClient.invalidateQueries({ queryKey: ['instances', 'list'] });
      void queryClient.invalidateQueries({ queryKey: ['deployment-bindings', projectId] });
    },
  });
}
