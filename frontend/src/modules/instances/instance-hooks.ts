import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { createInstance, listInstances } from '@/modules/instances/instance-api';
import type { CreateInstanceFormData, CreateInstanceResponse, InstanceListResponse } from '@/modules/instances/instance-types';

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
    mutationFn: (data: CreateInstanceFormData) => createInstance(data),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: INSTANCES_KEY });
    },
  });
}
