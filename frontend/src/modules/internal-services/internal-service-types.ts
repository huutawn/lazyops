export const INTERNAL_SERVICE_OPTIONS = [
  { kind: 'postgres', label: 'PostgreSQL', endpoint_hint: 'K3s DNS service-name:5432' },
  { kind: 'mysql', label: 'MySQL', endpoint_hint: 'K3s DNS service-name:3306' },
  { kind: 'mongodb', label: 'MongoDB', endpoint_hint: 'K3s DNS service-name:27017' },
  { kind: 'redis', label: 'Redis', endpoint_hint: 'K3s DNS service-name:6379' },
  { kind: 'rabbitmq', label: 'RabbitMQ', endpoint_hint: 'K3s DNS service-name:5672' },
  { kind: 'kafka', label: 'Kafka', endpoint_hint: 'K3s DNS service-name:9092' },
  { kind: 'eureka-server', label: 'Eureka Server', endpoint_hint: 'K3s DNS service-name:8761/eureka' },
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
