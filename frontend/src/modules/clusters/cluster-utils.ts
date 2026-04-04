import type { ClusterStatus } from '@/modules/clusters/cluster-types';

const STATUS_VARIANT_MAP: Record<ClusterStatus, 'success' | 'warning' | 'danger' | 'info' | 'neutral'> = {
  ready: 'success',
  validating: 'info',
  degraded: 'warning',
  unreachable: 'danger',
  revoked: 'neutral',
};

export function getClusterStatusVariant(status: ClusterStatus): 'success' | 'warning' | 'danger' | 'info' | 'neutral' {
  return STATUS_VARIANT_MAP[status] ?? 'neutral';
}

export function formatClusterStatus(status: ClusterStatus): string {
  return status.charAt(0).toUpperCase() + status.slice(1);
}
