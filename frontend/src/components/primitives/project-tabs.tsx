'use client';

import Link from 'next/link';
import { usePathname } from 'next/navigation';
import { cn } from '@/lib/utils';
import { useProjectNavigationMode } from '@/modules/projects/project-flow-hooks';

export type ProjectTabDefinition = {
  label: string;
  href: (id: string) => string;
};

export const GUIDED_PROJECT_TABS: ProjectTabDefinition[] = [
  { label: 'Bắt đầu', href: (id: string) => `/projects/${id}` },
  { label: 'Mã nguồn', href: (id: string) => `/projects/${id}/repo-link` },
  { label: 'Dịch vụ', href: (id: string) => `/projects/${id}/services` },
  { label: 'Biến môi trường', href: (id: string) => `/projects/${id}/env` },
  { label: 'Triển khai', href: (id: string) => `/projects/${id}/deployments` },
  { label: 'Nhật ký', href: (id: string) => `/projects/${id}/observability` },
];

export const ADMIN_PROJECT_TABS: ProjectTabDefinition[] = [
  { label: 'Bắt đầu', href: (id: string) => `/projects/${id}` },
  { label: 'Mã nguồn', href: (id: string) => `/projects/${id}/repo-link` },
  { label: 'Dịch vụ', href: (id: string) => `/projects/${id}/services` },
  { label: 'Biến môi trường', href: (id: string) => `/projects/${id}/env` },
  { label: 'Triển khai', href: (id: string) => `/projects/${id}/deployments` },
  { label: 'Nhật ký', href: (id: string) => `/projects/${id}/observability` },
];

type ProjectTabsProps = {
  projectId: string;
};

export function ProjectTabs({ projectId }: ProjectTabsProps) {
  const pathname = usePathname();
  const { guidedProjectFlow } = useProjectNavigationMode();
  const tabs = guidedProjectFlow ? GUIDED_PROJECT_TABS : ADMIN_PROJECT_TABS;

  return (
    <nav className="flex gap-2 overflow-x-auto border-b border-[#1e293b] pb-2">
      {tabs.map((tab) => {
        const href = tab.href(projectId);
        const isActive = pathname === href || pathname?.startsWith(href + '/');

        return (
          <Link
            key={tab.label}
            href={href}
            className={cn(
              'whitespace-nowrap rounded-2xl px-5 py-3 text-base font-bold transition-colors',
              isActive
                ? 'bg-[#0EA5E9]/12 text-[#38bdf8] shadow-[0_10px_30px_rgba(14,165,233,0.14)]'
                : 'text-[#94a3b8] hover:bg-[#0B1120]/70 hover:text-white',
            )}
          >
            {tab.label}
          </Link>
        );
      })}
    </nav>
  );
}
