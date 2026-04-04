export type NavItem = {
  label: string;
  href: string;
  icon?: string;
  children?: NavItem[];
};

export const NAV_ITEMS: NavItem[] = [
  { label: 'Onboarding', href: '/onboarding' },
  { label: 'Projects', href: '/projects' },
  { label: 'Targets', href: '/targets' },
  { label: 'Integrations', href: '/integrations' },
  { label: 'Topology', href: '/topology' },
  { label: 'Deployments', href: '/deployments' },
  { label: 'Observability', href: '/observability' },
  { label: 'FinOps', href: '/finops' },
];
