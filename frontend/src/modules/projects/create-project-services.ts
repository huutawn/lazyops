import {
  createDefaultPostgresConnectionTemplate,
  normalizePostgresConnectionTemplate,
  type PostgresConnectionTemplate,
} from '@/modules/project-services/postgres-connection-template';

export type CreateProjectServiceInput = {
  name: string;
  path: string;
  kind?: string;
  source_type?: string;
  public: boolean;
  runtime_profile?: string;
  placement_mode?: string;
  placement_node_id?: string;
  dependencies?: Array<{
    target_service: string;
    connection_template_key?: string;
    connection_template?: Record<string, string>;
  }>;
  connection_template_key?: string;
  connection_template?: Record<string, string>;
  connection_target_service?: string;
  managed_by_lazyops?: boolean;
  start_hint?: string;
  image_ref?: string;
  image_digest?: string;
  target_port?: number;
  service_port?: number;
  replicas?: number;
  env_bundle?: Record<string, string>;
  pvc_spec?: Record<string, unknown>;
  deploy_strategy?: Record<string, unknown>;
  healthcheck?: Record<string, unknown>;
};

export type ServiceFirstScaffold = {
  backend_enabled: boolean;
  backend_name: string;
  backend_path: string;
  backend_public: boolean;
  backend_connects_to_postgres: boolean;
  frontend_enabled: boolean;
  frontend_name: string;
  frontend_path: string;
  frontend_public: boolean;
  postgres_enabled: boolean;
  postgres_service_name: string;
  postgres_connection_template: PostgresConnectionTemplate;
};

export function createDefaultServiceFirstScaffold(): ServiceFirstScaffold {
  return {
    backend_enabled: true,
    backend_name: 'api',
    backend_path: 'apps/api',
    backend_public: false,
    backend_connects_to_postgres: true,
    frontend_enabled: true,
    frontend_name: 'web',
    frontend_path: 'apps/web',
    frontend_public: true,
    postgres_enabled: true,
    postgres_service_name: 'db',
    postgres_connection_template: createDefaultPostgresConnectionTemplate(),
  };
}

export function buildCreateProjectServices(scaffold: ServiceFirstScaffold): CreateProjectServiceInput[] {
  const services: CreateProjectServiceInput[] = [];
  const postgresServiceName = scaffold.postgres_service_name.trim() || 'db';

  if (scaffold.backend_enabled) {
    services.push({
      name: scaffold.backend_name.trim() || 'api',
      path: scaffold.backend_path.trim() || 'apps/api',
      kind: 'app',
      source_type: 'repo',
      public: scaffold.backend_public,
      placement_mode: 'shared_cluster',
      dependencies:
        scaffold.postgres_enabled && scaffold.backend_connects_to_postgres
          ? [
              {
                target_service: postgresServiceName,
                connection_template_key: 'postgres.basic',
              },
            ]
          : [],
      connection_template_key:
        scaffold.postgres_enabled && scaffold.backend_connects_to_postgres ? 'postgres.basic' : '',
      connection_target_service:
        scaffold.postgres_enabled && scaffold.backend_connects_to_postgres ? postgresServiceName : '',
      managed_by_lazyops: false,
      replicas: 1,
      env_bundle: {},
      pvc_spec: {},
      deploy_strategy: {},
      healthcheck: {},
    });
  }

  if (scaffold.frontend_enabled) {
    services.push({
      name: scaffold.frontend_name.trim() || 'web',
      path: scaffold.frontend_path.trim() || 'apps/web',
      kind: 'frontend',
      source_type: 'repo',
      public: scaffold.frontend_public,
      placement_mode: 'shared_cluster',
      managed_by_lazyops: false,
      replicas: 1,
      env_bundle: {},
      pvc_spec: {},
      deploy_strategy: {},
      healthcheck: {},
    });
  }

  if (scaffold.postgres_enabled) {
    services.push({
      name: postgresServiceName,
      path: `.lazyops/internal/postgres/${postgresServiceName}`,
      kind: 'postgres',
      source_type: 'internal',
      public: false,
      runtime_profile: 'internal-db',
      placement_mode: 'shared_cluster',
      connection_template: normalizePostgresConnectionTemplate(scaffold.postgres_connection_template),
      managed_by_lazyops: true,
      start_hint: 'managed-internal-service',
      image_ref: 'postgres:16-alpine',
      target_port: 5432,
      service_port: 5432,
      replicas: 1,
      env_bundle: {},
      pvc_spec: { size: '5Gi' },
      deploy_strategy: {},
      healthcheck: { protocol: 'tcp', port: 5432 },
    });
  }

  return services;
}
