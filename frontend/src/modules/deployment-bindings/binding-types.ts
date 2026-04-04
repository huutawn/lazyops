import { z } from 'zod';

export const RUNTIME_MODES = ['standalone', 'distributed-mesh', 'distributed-k3s'] as const;
export const TARGET_KINDS = ['instance', 'mesh', 'cluster'] as const;

export type RuntimeMode = (typeof RUNTIME_MODES)[number];
export type TargetKind = (typeof TARGET_KINDS)[number];

export const COMPATIBILITY_MATRIX: Record<TargetKind, RuntimeMode[]> = {
  instance: ['standalone', 'distributed-mesh'],
  mesh: ['distributed-mesh'],
  cluster: ['distributed-k3s'],
};

export const createDeploymentBindingSchema = z.object({
  name: z
    .string()
    .min(1, 'Binding name is required')
    .max(255, 'Binding name must be less than 255 characters'),
  target_ref: z.string().optional(),
  runtime_mode: z.enum(RUNTIME_MODES),
  target_kind: z.enum(TARGET_KINDS),
  target_id: z.string().min(1, 'Target is required'),
  placement_policy: z.string().optional(),
  domain_policy: z.string().optional(),
  compatibility_policy: z.string().optional(),
  scale_to_zero: z.boolean(),
}).superRefine((data, ctx) => {
  const allowedModes = COMPATIBILITY_MATRIX[data.target_kind];
  if (allowedModes && !allowedModes.includes(data.runtime_mode)) {
    ctx.addIssue({
      code: z.ZodIssueCode.custom,
      message: `${data.target_kind} targets only support: ${allowedModes.join(', ')}`,
      path: ['runtime_mode'],
    });
  }
});

export type CreateDeploymentBindingFormData = z.infer<typeof createDeploymentBindingSchema> & {
  scale_to_zero: boolean;
};

export type DeploymentBindingSummary = {
  id: string;
  project_id: string;
  name: string;
  target_ref: string;
  runtime_mode: RuntimeMode;
  target_kind: TargetKind;
  target_id: string;
  placement_policy: Record<string, unknown>;
  domain_policy: Record<string, unknown>;
  compatibility_policy: Record<string, unknown>;
  scale_to_zero_policy: Record<string, unknown>;
  created_at: string;
  updated_at: string;
};

export type DeploymentBindingListResponse = {
  items: DeploymentBindingSummary[];
};
