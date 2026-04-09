import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import {
  createInstance,
  installInstanceAgentViaSSH,
  issueInstanceBootstrapToken,
  listInstances,
} from '@/modules/instances/instance-api';
import type {
  BootstrapToken,
  CreateInstanceFormData,
  CreateInstanceResponse,
  InstallInstanceAgentSSHRequest,
  InstallInstanceAgentSSHResponse,
  InstanceListResponse,
} from '@/modules/instances/instance-types';

const INSTANCES_KEY = ['instances', 'list'];

export function useInstances() {
  return useQuery({
    queryKey: INSTANCES_KEY,
    queryFn: async () => {
      const result = await listInstances();
      if (result.error) throw new Error(result.error.message);
      return result.data as InstanceListResponse;
    },
    staleTime: 30 * 1000,
  });
}

export function useCreateInstance() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async (data: CreateInstanceFormData): Promise<CreateInstanceResponse> => {
      const result = await createInstance(data);
      if (result.error) {
        throw new Error(result.error.message);
      }
      if (!result.data) {
        throw new Error('Instance creation failed: missing response payload');
      }
      return result.data;
    },
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: INSTANCES_KEY });
    },
  });
}

export function useIssueInstanceBootstrapToken() {
  return useMutation({
    mutationFn: async (instanceID: string): Promise<BootstrapToken> => {
      const result = await issueInstanceBootstrapToken(instanceID);
      if (result.error) {
        throw new Error(result.error.message);
      }
      if (!result.data) {
        throw new Error('Bootstrap token issue failed: missing response payload');
      }
      return result.data;
    },
  });
}

export function useInstallInstanceAgentViaSSH() {
  return useMutation({
    mutationFn: async (payload: { instanceID: string; data: InstallInstanceAgentSSHRequest }): Promise<InstallInstanceAgentSSHResponse> => {
      const result = await installInstanceAgentViaSSH(payload.instanceID, payload.data);
      if (result.error) {
        throw new Error(result.error.message);
      }
      if (!result.data) {
        throw new Error('SSH install failed: missing response payload');
      }
      return result.data;
    },
  });
}
