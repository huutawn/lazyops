import {
  Rocket, FolderGit2, Target, Blocks,
  Send, ServerIcon, type LucideIcon
} from 'lucide-react';

export type NavItem = {
  label: string;
  href: string;
  icon?: LucideIcon;
  children?: NavItem[];
};

export const NAV_ITEMS: NavItem[] = [
  { label: 'Tổng quan', href: '/onboarding', icon: Rocket },
  { label: 'Dự án', href: '/projects', icon: FolderGit2 },
  { label: 'Lịch sử Triển khai', href: '/deployments', icon: Send },
  // Optional/Advanced nested settings for Targets & Github integrations
  { label: 'Hệ thống & Tích hợp', href: '/integrations/github', icon: ServerIcon },
];
