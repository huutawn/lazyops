export const POSTGRES_CONNECTION_TEMPLATE_SLOTS = [
  'DB_URL',
  'DB_NAME',
  'DB_HOST',
  'DB_PORT',
  'DB_USERNAME',
  'DB_PASSWORD',
] as const;

export type PostgresConnectionTemplateSlot = (typeof POSTGRES_CONNECTION_TEMPLATE_SLOTS)[number];

export type PostgresConnectionTemplate = Record<PostgresConnectionTemplateSlot, string>;

export function createDefaultPostgresConnectionTemplate(): PostgresConnectionTemplate {
  return {
    DB_URL: 'DATABASE_URL',
    DB_NAME: 'POSTGRES_DB',
    DB_HOST: 'POSTGRES_HOST',
    DB_PORT: 'POSTGRES_PORT',
    DB_USERNAME: 'POSTGRES_USER',
    DB_PASSWORD: 'POSTGRES_PASSWORD',
  };
}

export function normalizePostgresConnectionTemplate(
  template?: Record<string, string> | null,
): PostgresConnectionTemplate {
  const normalized = createDefaultPostgresConnectionTemplate();
  for (const slot of POSTGRES_CONNECTION_TEMPLATE_SLOTS) {
    const value = template?.[slot]?.trim();
    if (value) {
      normalized[slot] = value;
    }
  }
  return normalized;
}

export function formatPostgresConnectionTemplatePreview(
  template?: Record<string, string> | null,
): string {
  const normalized = normalizePostgresConnectionTemplate(template);
  return POSTGRES_CONNECTION_TEMPLATE_SLOTS.map((slot) => `${normalized[slot]}=`).join('\n');
}
