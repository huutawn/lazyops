import { z } from 'zod';

export const createClusterSchema = z.object({
  name: z
    .string()
    .min(1, 'Cluster name is required')
    .max(255, 'Cluster name must be less than 255 characters'),
  provider: z.literal('k3s'),
  kubeconfig_secret_ref: z
    .string()
    .min(1, 'Kubeconfig reference is required')
    .refine((val) => !/[\r\n\t]/.test(val), {
      message: 'Reference must not contain line breaks or tabs',
    }),
});

export type CreateClusterFormData = z.infer<typeof createClusterSchema>;

export type ClusterStatus = 'validating' | 'ready' | 'degraded' | 'unreachable' | 'revoked';

export type ClusterSummary = {
  id: string;
  target_kind: string;
  name: string;
  provider: string;
  status: ClusterStatus;
  created_at: string;
  updated_at: string;
};

export type ClusterListResponse = {
  items: ClusterSummary[];
};
