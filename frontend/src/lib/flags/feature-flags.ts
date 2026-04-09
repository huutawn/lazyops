export type FeatureFlag =
  | 'mock_mode'
  | 'ux_three_step_flow'
  | 'module_instances'
  | 'module_mesh_networks'
  | 'module_clusters'
  | 'module_bindings'
  | 'module_deployments'
  | 'module_topology'
  | 'module_observability'
  | 'module_finops'
  | 'module_integrations';

type FeatureFlagConfig = Record<FeatureFlag, boolean>;

const DEFAULT_FLAGS: FeatureFlagConfig = {
  mock_mode: false,
  ux_three_step_flow: false,
  module_instances: false,
  module_mesh_networks: false,
  module_clusters: false,
  module_bindings: false,
  module_deployments: false,
  module_topology: false,
  module_observability: false,
  module_finops: false,
  module_integrations: false,
};

function parseEnvFlags(): Partial<FeatureFlagConfig> {
  const raw = process.env.NEXT_PUBLIC_FEATURE_FLAGS ?? '';
  const flags: Partial<FeatureFlagConfig> = {};
  if (raw) {
    raw.split(',').forEach((pair) => {
      const [key, value] = pair.split('=');
      if (key && value !== undefined) {
        flags[key.trim() as FeatureFlag] = value.trim() === 'true';
      }
    });
  }
  return flags;
}

const envFlags = parseEnvFlags();

const FLAGS: FeatureFlagConfig = { ...DEFAULT_FLAGS, ...envFlags };

export function isFeatureEnabled(flag: FeatureFlag): boolean {
  return FLAGS[flag] ?? false;
}

export function getFeatureFlags(): FeatureFlagConfig {
  return { ...FLAGS };
}
