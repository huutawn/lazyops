import { apiPost, apiGet } from '@/lib/api/api-client';
import type { ApiResponse } from '@/lib/types';
import type {
  CreateInstanceFormData,
  CreateInstanceResponse,
  InstanceListResponse,
  InstanceSummary,
} from '@/modules/instances/instance-types';

export async function createInstance(data: CreateInstanceFormData): Promise<ApiResponse<CreateInstanceResponse>> {
  const labels: Record<string, string> = {};
  if (data.labels?.trim()) {
    data.labels.split(',').forEach((pair) => {
      const [key, ...rest] = pair.split(':');
      if (key?.trim()) {
        labels[key.trim().toLowerCase()] = rest.join(':').trim();
      }
    });
  }

  const body = {
    name: data.name,
    public_ip: data.public_ip || undefined,
    private_ip: data.private_ip || undefined,
    labels,
  };

  return apiPost<CreateInstanceResponse>('/instances', body);
}

export async function listInstances(): Promise<ApiResponse<InstanceListResponse>> {
  return apiGet<InstanceListResponse>('/instances');
}
