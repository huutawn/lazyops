import { apiPost, apiGet } from '@/lib/api/api-client';
import type { ApiResponse } from '@/lib/types';
import type {
  CreateClusterFormData,
  ClusterNodeListResponse,
  ClusterSummary,
  ClusterListResponse,
  ConnectClusterNodeSSHRequest,
  ConnectClusterNodeSSHResponse,
} from '@/modules/clusters/cluster-types';

export async function createCluster(data: CreateClusterFormData): Promise<ApiResponse<ClusterSummary>> {
  return apiPost<ClusterSummary>('/clusters', data);
}

export async function listClusters(): Promise<ApiResponse<ClusterListResponse>> {
  return apiGet<ClusterListResponse>('/clusters');
}

export async function listClusterNodes(clusterId: string): Promise<ApiResponse<ClusterNodeListResponse>> {
  return apiGet<ClusterNodeListResponse>(`/clusters/${clusterId}/nodes`);
}

export async function connectClusterNodeSSH(
  clusterId: string,
  data: ConnectClusterNodeSSHRequest,
): Promise<ApiResponse<ConnectClusterNodeSSHResponse>> {
  return apiPost<ConnectClusterNodeSSHResponse>(`/clusters/${clusterId}/nodes/connect-ssh`, data);
}
