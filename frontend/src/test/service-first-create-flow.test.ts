import { describe, expect, it } from 'vitest';
import { createProjectSchema } from '@/modules/projects/project-types';
import { formatPostgresConnectionTemplatePreview } from '@/modules/project-services/postgres-connection-template';

describe('service-first create flow helpers', () => {
  it('formats the postgres preview from custom env names', () => {
    const preview = formatPostgresConnectionTemplatePreview({
      DB_URL: 'DATABASE_URL',
      DB_NAME: 'POSTGRES_DB',
      DB_HOST: 'POSTGRES_HOST',
      DB_PORT: 'POSTGRES_PORT',
      DB_USERNAME: 'POSTGRES_USER',
      DB_PASSWORD: 'POSTGRES_PASSWORD',
    });

    expect(preview).toContain('DATABASE_URL=');
    expect(preview).toContain('POSTGRES_PASSWORD=');
  });

  it('accepts unified services in the create project schema', () => {
    const result = createProjectSchema.safeParse({
      name: 'Commerce',
      slug: 'commerce',
      default_branch: 'main',
      services: [],
    });

    expect(result.success).toBe(true);
  });
});
