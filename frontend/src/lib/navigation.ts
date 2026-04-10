import {
  Rocket, FolderGit2, Target, Blocks,
  Send, type LucideIcon
} from 'lucide-react';

export type NavItem = {
  label: string;
  href: string;
  icon?: LucideIcon;
  children?: NavItem[];
};

export const NAV_ITEMS: NavItem[] = [
  { label: 'Bắt đầu', href: '/onboarding', icon: Rocket },
  { label: 'Dự án', href: '/projects', icon: FolderGit2 },
  { label: 'Máy chủ', href: '/targets', icon: Target },
  { label: 'GitHub', href: '/integrations/github', icon: Blocks },
  { label: 'Triển khai', href: '/deployments', icon: Send },
];
