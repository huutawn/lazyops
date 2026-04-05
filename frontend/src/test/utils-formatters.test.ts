import { describe, expect, it } from 'vitest';
import { getStatusVariant, formatStatus } from '@/modules/instances/instance-utils';
import { getMeshStatusVariant, formatMeshStatus, getProviderLabel } from '@/modules/mesh-networks/mesh-network-utils';
import { getClusterStatusVariant, formatClusterStatus } from '@/modules/clusters/cluster-utils';

describe('instance utils', () => {
  it('maps online to success', () => {
    expect(getStatusVariant('online')).toBe('success');
  });

  it('maps offline to danger', () => {
    expect(getStatusVariant('offline')).toBe('danger');
  });

  it('maps degraded to warning', () => {
    expect(getStatusVariant('degraded')).toBe('warning');
  });

  it('formats status with capitalization', () => {
    expect(formatStatus('pending_enrollment')).toBe('Pending Enrollment');
  });
});

describe('mesh network utils', () => {
  it('maps active to success', () => {
    expect(getMeshStatusVariant('active')).toBe('success');
  });

  it('maps provisioning to info', () => {
    expect(getMeshStatusVariant('provisioning')).toBe('info');
  });

  it('formats mesh status', () => {
    expect(formatMeshStatus('degraded')).toBe('Degraded');
  });

  it('labels providers correctly', () => {
    expect(getProviderLabel('tailscale')).toBe('Tailscale');
    expect(getProviderLabel('wireguard')).toBe('WireGuard');
  });
});

describe('cluster utils', () => {
  it('maps ready to success', () => {
    expect(getClusterStatusVariant('ready')).toBe('success');
  });

  it('maps unreachable to danger', () => {
    expect(getClusterStatusVariant('unreachable')).toBe('danger');
  });

  it('formats cluster status', () => {
    expect(formatClusterStatus('validating')).toBe('Validating');
  });
});
