import type { InstanceStatus } from '@/modules/instances/instance-types';

const STATUS_VARIANT_MAP: Record<InstanceStatus, 'success' | 'warning' | 'danger' | 'neutral' | 'info'> = {
  online: 'success',
  pending_enrollment: 'info',
  offline: 'danger',
  degraded: 'warning',
  revoked: 'neutral',
};

export function getStatusVariant(status: InstanceStatus): 'success' | 'warning' | 'danger' | 'neutral' | 'info' {
  return STATUS_VARIANT_MAP[status] ?? 'neutral';
}

export function formatStatus(status: InstanceStatus): string {
  return status
    .replace(/_/g, ' ')
    .replace(/\b\w/g, (c) => c.toUpperCase());
}
