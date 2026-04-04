import type { ReactNode } from 'react';
import { EmptyState } from '@/components/primitives/empty-state';
import { SectionCard } from '@/components/primitives/section-card';

type OnboardingEmptyProps = {
  type: 'no-projects' | 'no-github' | 'no-targets';
  action?: ReactNode;
};

const EMPTY_CONFIG: Record<OnboardingEmptyProps['type'], { title: string; description: string }> = {
  'no-projects': {
    title: 'No projects yet',
    description: 'Create your first project to start managing targets, integrations, and deployments.',
  },
  'no-github': {
    title: 'No GitHub installation',
    description: 'Connect your GitHub App to enable automated deployments from your repositories.',
  },
  'no-targets': {
    title: 'No targets configured',
    description: 'Register machines or clusters so LazyOps knows where to deploy your services.',
  },
};

export function OnboardingEmpty({ type, action }: OnboardingEmptyProps) {
  const config = EMPTY_CONFIG[type];
  return (
    <SectionCard title={config.title} description={config.description}>
      <EmptyState title={config.title} description={config.description} action={action} />
    </SectionCard>
  );
}
