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

export const connectClusterNodeSchema = z
  .object({
    instance_name: z
      .string()
      .min(1, 'Node name is required')
      .max(255, 'Node name must be less than 255 characters'),
    public_ip: z.string().min(1, 'Public IP is required'),
    private_ip: z.string(),
    ssh_host: z.string().min(1, 'SSH host is required'),
    ssh_port: z.number().int().min(1).max(65535),
    ssh_username: z.string().min(1, 'SSH username is required'),
    ssh_password: z.string(),
    ssh_private_key: z.string(),
    ssh_host_key_fingerprint: z.string(),
    control_plane_url: z.string(),
    agent_image: z.string(),
    container_name: z.string(),
  })
  .superRefine((value, ctx) => {
    if (!value.ssh_password.trim() && !value.ssh_private_key.trim()) {
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        message: 'Enter an SSH password or private key',
        path: ['ssh_password'],
      });
    }
  });

export type ConnectClusterNodeFormData = z.infer<typeof connectClusterNodeSchema>;

export type ClusterStatus = 'validating' | 'ready' | 'degraded' | 'unreachable' | 'revoked';

export type ClusterSummary = {
  id: string;
  target_kind: string;
  name: string;
  provider: string;
  status: ClusterStatus;
  public_ip?: string | null;
  instance_id?: string | null;
  created_at: string;
  updated_at: string;
};

export type ClusterListResponse = {
  items: ClusterSummary[];
};

export type ClusterNode = {
  cluster_id: string;
  instance_id: string;
  name: string;
  status: string;
  k8s_node_name?: string;
  labels: Record<string, string>;
  last_seen_at?: string;
  is_ready: boolean;
};

export type ClusterNodeListResponse = {
  items: ClusterNode[];
};

export type ConnectClusterNodeSSHRequest = ConnectClusterNodeFormData & {
  labels?: Record<string, string>;
};

export type ClusterNodeJoinStage = {
  key: string;
  label: string;
  status: string;
  detail?: string;
};

export type ConnectClusterNodeSSHResponse = {
  cluster_id: string;
  instance: {
    id: string;
    target_kind: string;
    name: string;
    public_ip?: string | null;
    private_ip?: string | null;
    agent_id?: string | null;
    status: string;
    labels: Record<string, string>;
    runtime_capabilities: Record<string, unknown>;
    created_at: string;
    updated_at: string;
  };
  join: {
    cluster_id: string;
    instance_id: string;
    started_at: string;
    host_key_fingerprint?: string;
    node_name?: string;
    join_server_url?: string;
    labeled_by_control: boolean;
    placement_label_key?: string;
    placement_label_value?: string;
    stages?: ClusterNodeJoinStage[];
  };
};
