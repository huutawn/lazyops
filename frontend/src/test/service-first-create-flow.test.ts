import { describe, expect, it } from 'vitest';
import { createProjectSchema } from '@/modules/projects/project-types';
import { formatPostgresConnectionTemplatePreview } from '@/modules/project-services/postgres-connection-template';

describe('service-first create flow helpers', () => {
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
      services: [],
    });

    expect(result.success).toBe(true);
  });
});
