import { apiPost, apiGet } from '@/lib/api/api-client';
import type { ApiResponse } from '@/lib/types';
import type {
  CreateMeshNetworkFormData,
  MeshNetworkSummary,
  MeshNetworkListResponse,
} from '@/modules/mesh-networks/mesh-network-types';

export async function createMeshNetwork(data: CreateMeshNetworkFormData): Promise<ApiResponse<MeshNetworkSummary>> {
  return apiPost<MeshNetworkSummary>('/mesh-networks', data);
}

export async function listMeshNetworks(): Promise<ApiResponse<MeshNetworkListResponse>> {
  return apiGet<MeshNetworkListResponse>('/mesh-networks');
}
