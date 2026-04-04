import type { MeshNetworkStatus } from '@/modules/mesh-networks/mesh-network-types';

const STATUS_VARIANT_MAP: Record<MeshNetworkStatus, 'success' | 'warning' | 'danger' | 'info'> = {
  active: 'success',
  provisioning: 'info',
  degraded: 'warning',
  revoked: 'danger',
};

export function getMeshStatusVariant(status: MeshNetworkStatus): 'success' | 'warning' | 'danger' | 'info' {
  return STATUS_VARIANT_MAP[status] ?? 'info';
}

export function formatMeshStatus(status: MeshNetworkStatus): string {
  return status.charAt(0).toUpperCase() + status.slice(1);
}

export function getProviderLabel(provider: string): string {
  return provider === 'tailscale' ? 'Tailscale' : 'WireGuard';
}
