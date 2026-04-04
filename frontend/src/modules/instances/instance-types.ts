import { z } from 'zod';

export const createInstanceSchema = z.object({
  name: z
    .string()
    .min(1, 'Instance name is required')
    .max(255, 'Instance name must be less than 255 characters'),
  public_ip: z
    .string()
    .regex(/^(\d{1,3}\.){3}\d{1,3}$/, 'Invalid public IP address')
    .optional()
    .or(z.literal('')),
  private_ip: z
    .string()
    .regex(/^(\d{1,3}\.){3}\d{1,3}$/, 'Invalid private IP address')
    .optional()
    .or(z.literal('')),
  labels: z
    .string()
    .optional(),
}).refine((data) => data.public_ip || data.private_ip, {
  message: 'At least one IP address (public or private) is required',
  path: ['public_ip'],
});

export type CreateInstanceFormData = z.infer<typeof createInstanceSchema>;

export type InstanceStatus = 'pending_enrollment' | 'online' | 'offline' | 'degraded' | 'revoked';

export type InstanceSummary = {
  id: string;
  target_kind: string;
  name: string;
  public_ip: string | null;
  private_ip: string | null;
  agent_id: string | null;
  status: InstanceStatus;
  labels: Record<string, string>;
  runtime_capabilities: Record<string, unknown>;
  created_at: string;
  updated_at: string;
};

export type BootstrapToken = {
  token: string;
  token_id: string;
  expires_at: string;
  single_use: boolean;
};

export type CreateInstanceResponse = {
  instance: InstanceSummary;
  bootstrap: BootstrapToken;
};

export type InstanceListResponse = {
  items: InstanceSummary[];
};
