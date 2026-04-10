export const INTERNAL_SERVICE_OPTIONS = [
  { kind: 'postgres', label: 'PostgreSQL', localhost: 'localhost:5432' },
  { kind: 'mysql', label: 'MySQL', localhost: 'localhost:3306' },
  { kind: 'redis', label: 'Redis', localhost: 'localhost:6379' },
  { kind: 'rabbitmq', label: 'RabbitMQ', localhost: 'localhost:5672' },
] as const;

export type InternalServiceKind = (typeof INTERNAL_SERVICE_OPTIONS)[number]['kind'];

export type ProjectInternalService = {
  id: string;
  project_id: string;
  kind: InternalServiceKind;
  alias: string;
  protocol: string;
  port: number;
  local_endpoint: string;
  created_at: string;
  updated_at: string;
};

export type ProjectInternalServiceListResponse = {
  items: ProjectInternalService[];
};

export type ConfigureProjectInternalServicesRequest = {
  kinds: InternalServiceKind[];
};
