import { z } from 'zod';

const MESH_PROVIDERS = ['wireguard', 'tailscale'] as const;

export const createMeshNetworkSchema = z.object({
  name: z
    .string()
    .min(1, 'Network name is required')
    .max(255, 'Network name must be less than 255 characters'),
  provider: z.enum(MESH_PROVIDERS),
  cidr: z
    .string()
    .min(1, 'CIDR is required')
    .regex(/^(\d{1,3}\.){3}\d{1,3}\/\d{1,2}$/, 'Must be a valid CIDR (e.g. 10.0.0.0/24)'),
});

export type CreateMeshNetworkFormData = z.infer<typeof createMeshNetworkSchema>;

export type MeshNetworkStatus = 'provisioning' | 'active' | 'degraded' | 'revoked';

export type MeshNetworkSummary = {
  id: string;
  target_kind: string;
  name: string;
  provider: 'wireguard' | 'tailscale';
  cidr: string;
  status: MeshNetworkStatus;
  created_at: string;
  updated_at: string;
};

export type MeshNetworkListResponse = {
  items: MeshNetworkSummary[];
};
