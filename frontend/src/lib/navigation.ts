import {
  LayoutGrid, FolderGit2, Rocket, Settings, type LucideIcon
} from 'lucide-react';

export type NavItem = {
  label: string;
  href: string;
  icon?: LucideIcon;
  children?: NavItem[];
};

export const NAV_ITEMS: NavItem[] = [
  { label: 'Tổng quan', href: '/dashboard', icon: LayoutGrid },
  { label: 'Dự án', href: '/projects', icon: FolderGit2 },
  { label: 'Triển khai', href: '/deployments', icon: Rocket },
  { label: 'Tích hợp', href: '/integrations', icon: Settings },
];
