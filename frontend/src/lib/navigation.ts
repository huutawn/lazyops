import { 
  Rocket, FolderGit2, Target, Blocks, Network, 
  Send, Activity, CircleDollarSign, type LucideIcon 
} from 'lucide-react';

export type NavItem = {
  label: string;
  href: string;
  icon?: LucideIcon;
  children?: NavItem[];
};

export const NAV_ITEMS: NavItem[] = [
  { label: 'Onboarding', href: '/onboarding', icon: Rocket },
  { label: 'Projects', href: '/projects', icon: FolderGit2 },
  { label: 'Targets', href: '/targets', icon: Target },
  { label: 'Integrations', href: '/integrations', icon: Blocks },
  { label: 'Topology', href: '/topology', icon: Network },
  { label: 'Deployments', href: '/deployments', icon: Send },
  { label: 'Observability', href: '/observability', icon: Activity },
  { label: 'FinOps', href: '/finops', icon: CircleDollarSign },
];
