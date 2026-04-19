import { describe, expect, it } from 'vitest';
import {
  buildCreateProjectServices,
  createDefaultServiceFirstScaffold,
} from '@/modules/projects/create-project-services';
import { createProjectSchema } from '@/modules/projects/project-types';
import { formatPostgresConnectionTemplatePreview } from '@/modules/project-services/postgres-connection-template';

describe('service-first create flow helpers', () => {
  it('builds backend, frontend, and internal postgres services from the scaffold', () => {
    const services = buildCreateProjectServices(createDefaultServiceFirstScaffold());

    expect(services.map((service) => `${service.source_type}:${service.name}`)).toEqual([
      'repo:api',
      'repo:web',
      'internal:db',
    ]);
    expect(services[0].connection_template_key).toBe('postgres.basic');
    expect(services[0].connection_target_service).toBe('db');
    expect(services[2].connection_template?.DB_URL).toBe('DB_URL');
  });

  it('formats the postgres preview from custom env names', () => {
    const preview = formatPostgresConnectionTemplatePreview({
      DB_URL: 'DATABASE_URL',
      DB_NAME: 'DATABASE_NAME',
      DB_HOST: 'PGHOST',
      DB_PORT: 'PGPORT',
      DB_USERNAME: 'PGUSER',
      DB_PASSWORD: 'PGPASSWORD',
    });

    expect(preview).toContain('DATABASE_URL=');
    expect(preview).toContain('PGPASSWORD=');
  });

  it('accepts unified services in the create project schema', () => {
    const result = createProjectSchema.safeParse({
      name: 'Commerce',
      slug: 'commerce',
      default_branch: 'main',
      services: buildCreateProjectServices(createDefaultServiceFirstScaffold()),
    });

    expect(result.success).toBe(true);
  });
});
