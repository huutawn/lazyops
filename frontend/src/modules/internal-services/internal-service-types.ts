export const INTERNAL_SERVICE_OPTIONS = [
  { kind: 'postgres', label: 'PostgreSQL', endpoint_hint: 'K3s DNS service-name:5432 or localhost:5432 on standalone' },
  { kind: 'mysql', label: 'MySQL', endpoint_hint: 'K3s DNS service-name:3306 or localhost:3306 on standalone' },
  { kind: 'redis', label: 'Redis', endpoint_hint: 'K3s DNS service-name:6379 or localhost:6379 on standalone' },
  { kind: 'rabbitmq', label: 'RabbitMQ', endpoint_hint: 'K3s DNS service-name:5672 or localhost:5672 on standalone' },
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
