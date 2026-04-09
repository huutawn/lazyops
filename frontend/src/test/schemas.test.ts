import { describe, expect, it } from 'vitest';
import { loginSchema, registerSchema } from '@/lib/schemas/auth-schemas';
import { createProjectSchema } from '@/modules/projects/project-types';
import { createInstanceSchema } from '@/modules/instances/instance-types';
import { createMeshNetworkSchema } from '@/modules/mesh-networks/mesh-network-types';
import { createClusterSchema } from '@/modules/clusters/cluster-types';
import { createDeploymentBindingSchema } from '@/modules/deployment-bindings/binding-types';
import { syncGitHubInstallationsSchema } from '@/modules/github-sync/github-types';
import { linkRepoSchema } from '@/modules/repo-link/repo-link-types';

describe('loginSchema', () => {
  it('validates correct input', () => {
    const result = loginSchema.safeParse({ email: 'test@example.com', password: 'secret123' });
    expect(result.success).toBe(true);
  });

  it('rejects empty email', () => {
    const result = loginSchema.safeParse({ email: '', password: 'secret123' });
    expect(result.success).toBe(false);
  });

  it('rejects invalid email format', () => {
    const result = loginSchema.safeParse({ email: 'not-an-email', password: 'secret123' });
    expect(result.success).toBe(false);
  });

  it('rejects empty password', () => {
    const result = loginSchema.safeParse({ email: 'test@example.com', password: '' });
    expect(result.success).toBe(false);
  });
});

describe('registerSchema', () => {
  it('validates correct input', () => {
    const result = registerSchema.safeParse({
      name: 'Test User',
      email: 'test@example.com',
      password: 'SecurePass1',
    });
    expect(result.success).toBe(true);
  });

  it('rejects short password', () => {
    const result = registerSchema.safeParse({
      name: 'Test User',
      email: 'test@example.com',
      password: 'Short1',
    });
    expect(result.success).toBe(false);
  });

  it('rejects password without uppercase', () => {
    const result = registerSchema.safeParse({
      name: 'Test User',
      email: 'test@example.com',
      password: 'nouppercase1',
    });
    expect(result.success).toBe(false);
  });

  it('rejects password without lowercase', () => {
    const result = registerSchema.safeParse({
      name: 'Test User',
      email: 'test@example.com',
      password: 'NOLOWERCASE1',
    });
    expect(result.success).toBe(false);
  });

  it('rejects password without digit', () => {
    const result = registerSchema.safeParse({
      name: 'Test User',
      email: 'test@example.com',
      password: 'NoDigitHere',
    });
    expect(result.success).toBe(false);
  });

  it('rejects name over 100 characters', () => {
    const result = registerSchema.safeParse({
      name: 'a'.repeat(101),
      email: 'test@example.com',
      password: 'SecurePass1',
    });
    expect(result.success).toBe(false);
  });
});

describe('createProjectSchema', () => {
  it('validates correct input', () => {
    const result = createProjectSchema.safeParse({
      name: 'My Project',
      slug: 'my-project',
      default_branch: 'main',
    });
    expect(result.success).toBe(true);
  });

  it('rejects invalid slug format', () => {
    const result = createProjectSchema.safeParse({
      name: 'My Project',
      slug: 'INVALID_SLUG',
      default_branch: 'main',
    });
    expect(result.success).toBe(false);
  });

  it('rejects empty name', () => {
    const result = createProjectSchema.safeParse({
      name: '',
      slug: 'my-project',
      default_branch: 'main',
    });
    expect(result.success).toBe(false);
  });
});

describe('createInstanceSchema', () => {
  it('validates with public IP', () => {
    const result = createInstanceSchema.safeParse({
      name: 'prod-web-01',
      public_ip: '203.0.113.10',
    });
    expect(result.success).toBe(true);
  });

  it('validates with private IP', () => {
    const result = createInstanceSchema.safeParse({
      name: 'prod-web-01',
      private_ip: '10.0.1.10',
    });
    expect(result.success).toBe(true);
  });

  it('rejects when no IP provided', () => {
    const result = createInstanceSchema.safeParse({
      name: 'prod-web-01',
    });
    expect(result.success).toBe(false);
  });

  it('rejects invalid IP format', () => {
    const result = createInstanceSchema.safeParse({
      name: 'prod-web-01',
      public_ip: 'not-an-ip',
    });
    expect(result.success).toBe(false);
  });
});

describe('createMeshNetworkSchema', () => {
  it('validates correct input', () => {
    const result = createMeshNetworkSchema.safeParse({
      name: 'prod-mesh',
      provider: 'tailscale',
      cidr: '100.64.0.0/16',
    });
    expect(result.success).toBe(true);
  });

  it('rejects invalid provider', () => {
    const result = createMeshNetworkSchema.safeParse({
      name: 'prod-mesh',
      provider: 'invalid',
      cidr: '100.64.0.0/16',
    });
    expect(result.success).toBe(false);
  });

  it('rejects invalid CIDR', () => {
    const result = createMeshNetworkSchema.safeParse({
      name: 'prod-mesh',
      provider: 'tailscale',
      cidr: 'not-a-cidr',
    });
    expect(result.success).toBe(false);
  });
});

describe('createClusterSchema', () => {
  it('validates correct input', () => {
    const result = createClusterSchema.safeParse({
      name: 'prod-k3s',
      provider: 'k3s',
      kubeconfig_secret_ref: 'my-secret',
    });
    expect(result.success).toBe(true);
  });

  it('rejects empty kubeconfig ref', () => {
    const result = createClusterSchema.safeParse({
      name: 'prod-k3s',
      provider: 'k3s',
      kubeconfig_secret_ref: '',
    });
    expect(result.success).toBe(false);
  });
});

describe('createDeploymentBindingSchema', () => {
  it('validates compatible instance + standalone', () => {
    const result = createDeploymentBindingSchema.safeParse({
      name: 'prod-standalone',
      target_kind: 'instance',
      runtime_mode: 'standalone',
      target_id: 'inst_01',
      scale_to_zero: false,
    });
    expect(result.success).toBe(true);
  });

  it('rejects incompatible mesh + standalone', () => {
    const result = createDeploymentBindingSchema.safeParse({
      name: 'bad-binding',
      target_kind: 'mesh',
      runtime_mode: 'standalone',
      target_id: 'mesh_01',
      scale_to_zero: false,
    });
    expect(result.success).toBe(false);
  });

  it('rejects incompatible cluster + mesh', () => {
    const result = createDeploymentBindingSchema.safeParse({
      name: 'bad-binding',
      target_kind: 'cluster',
      runtime_mode: 'distributed-mesh',
      target_id: 'clust_01',
      scale_to_zero: false,
    });
    expect(result.success).toBe(false);
  });
});

describe('syncGitHubInstallationsSchema', () => {
  it('validates with token', () => {
    const result = syncGitHubInstallationsSchema.safeParse({
      github_access_token: 'ghp_xxxx',
    });
    expect(result.success).toBe(true);
  });

  it('accepts empty token for cache refresh flow', () => {
    const result = syncGitHubInstallationsSchema.safeParse({
      github_access_token: '',
    });
    expect(result.success).toBe(true);
  });
});

describe('linkRepoSchema', () => {
  it('validates correct input', () => {
    const result = linkRepoSchema.safeParse({
      github_installation_id: 12345,
      github_repo_id: 67890,
      tracked_branch: 'main',
      preview_enabled: false,
    });
    expect(result.success).toBe(true);
  });

  it('rejects empty branch', () => {
    const result = linkRepoSchema.safeParse({
      github_installation_id: 12345,
      github_repo_id: 67890,
      tracked_branch: '',
      preview_enabled: false,
    });
    expect(result.success).toBe(false);
  });
});
