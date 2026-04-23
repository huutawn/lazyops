import { z } from 'zod';

export const INTERNAL_SERVICE_KINDS = ['postgres', 'mysql', 'mongodb', 'redis', 'rabbitmq', 'kafka', 'eureka-server'] as const;

export const projectServiceDependencySchema = z.object({
  target_service: z.string().min(1, 'Dependency target is required'),
  connection_template_key: z.string().optional(),
  connection_template: z.record(z.string(), z.string()).optional(),
});

export const projectServiceSchema = z.object({
  name: z.string().min(1, 'Service name is required'),
  path: z.string().min(1, 'Service path is required'),
  kind: z.string().optional(),
  source_type: z.string().optional(),
  public: z.boolean(),
  runtime_profile: z.string().optional(),
  placement_mode: z.string().optional(),
  placement_node_id: z.string().optional(),
  dependencies: z.array(projectServiceDependencySchema).optional(),
  connection_template_key: z.string().optional(),
  connection_template: z.record(z.string(), z.string()).optional(),
  connection_target_service: z.string().optional(),
  managed_by_lazyops: z.boolean().optional(),
  start_hint: z.string().optional(),
  image_ref: z.string().optional(),
  image_digest: z.string().optional(),
  target_port: z.number().optional(),
  service_port: z.number().optional(),
  replicas: z.number().optional(),
  env_bundle: z.record(z.string(), z.string()).optional(),
  pvc_spec: z.record(z.string(), z.unknown()).optional(),
  deploy_strategy: z.record(z.string(), z.unknown()).optional(),
  healthcheck: z.record(z.string(), z.unknown()).optional(),
});

export const createProjectSchema = z.object({
  name: z
    .string()
    .min(1, 'Project name is required')
    .max(100, 'Project name must be less than 100 characters'),
  slug: z
    .string()
    .min(1, 'Slug is required')
    .max(60, 'Slug must be less than 60 characters')
    .regex(/^[a-z0-9]+(-[a-z0-9]+)*$/, 'Slug must be lowercase alphanumeric with hyphens'),
  default_branch: z
    .string()
    .min(1, 'Branch name is required')
    .max(100, 'Branch name must be less than 100 characters'),
  services: z.array(projectServiceSchema).default([]),
  internal_services: z.array(z.enum(INTERNAL_SERVICE_KINDS)).default([]),
});

export type CreateProjectFormData = z.input<typeof createProjectSchema>;
export type CreateProjectService = z.input<typeof projectServiceSchema>;

export type ProjectSummary = {
  id: string;
  name: string;
  slug: string;
  namespace_slug: string;
  cluster_id?: string;
  runtime_mode: string;
  default_branch: string;
  created_at: string;
  updated_at: string;
};

export type ProjectListResponse = {
  items: ProjectSummary[];
};
