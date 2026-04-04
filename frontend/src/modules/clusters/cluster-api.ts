import { apiPost, apiGet } from '@/lib/api/api-client';
import type { ApiResponse } from '@/lib/types';
import type {
  CreateClusterFormData,
  ClusterSummary,
  ClusterListResponse,
} from '@/modules/clusters/cluster-types';

export async function createCluster(data: CreateClusterFormData): Promise<ApiResponse<ClusterSummary>> {
  return apiPost<ClusterSummary>('/clusters', data);
}

export async function listClusters(): Promise<ApiResponse<ClusterListResponse>> {
  return apiGet<ClusterListResponse>('/clusters');
}
