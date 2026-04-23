export const PROJECT_OBSERVABILITY_TABS = [
  { key: 'monitoring', label: 'Monitoring' },
  { key: 'runtime', label: 'Runtime' },
] as const;

export type ProjectObservabilityTab = (typeof PROJECT_OBSERVABILITY_TABS)[number]['key'];

export function resolveProjectObservabilityTab(input?: {
  tab?: string | null;
  service?: string | null;
}): ProjectObservabilityTab {
  const requestedTab = (input?.tab || '').trim().toLowerCase();
  if (requestedTab === 'monitoring' || requestedTab === 'runtime') {
    return requestedTab;
  }
  if ((input?.service || '').trim() !== '') {
    return 'runtime';
  }
  return 'monitoring';
}
