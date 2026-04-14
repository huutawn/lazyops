'use client';

import Link from 'next/link';
import { usePathname } from 'next/navigation';
import { cn } from '@/lib/utils';
import { useProjectNavigationMode } from '@/modules/projects/project-flow-hooks';

const GUIDED_PROJECT_TABS = [
  { label: 'Tổng quan', href: (id: string) => `/projects/${id}` },
  { label: 'Liên kết kho mã', href: (id: string) => `/projects/${id}/repo-link` },
  { label: 'Biến môi trường', href: (id: string) => `/projects/${id}/env` },
  { label: 'Dịch vụ nội bộ', href: (id: string) => `/projects/${id}/internal-services` },
  { label: 'Triển khai', href: (id: string) => `/projects/${id}/deployments` },
  { label: 'Giám sát', href: (id: string) => `/projects/${id}/observability` },
];

const ADMIN_PROJECT_TABS = [
  { label: 'Tổng quan', href: (id: string) => `/projects/${id}` },
  { label: 'Tích hợp', href: (id: string) => `/projects/${id}/integrations` },
  { label: 'Liên kết kho mã', href: (id: string) => `/projects/${id}/repo-link` },
  { label: 'Biến môi trường', href: (id: string) => `/projects/${id}/env` },
  { label: 'Binding', href: (id: string) => `/projects/${id}/bindings` },
  { label: 'Blueprint', href: (id: string) => `/projects/${id}/blueprint` },
  { label: 'Dịch vụ nội bộ', href: (id: string) => `/projects/${id}/internal-services` },
  { label: 'Định tuyến', href: (id: string) => `/projects/${id}/routing` },
  { label: 'Triển khai', href: (id: string) => `/projects/${id}/deployments` },
  { label: 'Validate', href: (id: string) => `/projects/${id}/validate` },
  { label: 'Giám sát', href: (id: string) => `/projects/${id}/observability` },
];

type ProjectTabsProps = {
  projectId: string;
};

export function ProjectTabs({ projectId }: ProjectTabsProps) {
  const pathname = usePathname();
  const { guidedProjectFlow } = useProjectNavigationMode();
  const tabs = guidedProjectFlow ? GUIDED_PROJECT_TABS : ADMIN_PROJECT_TABS;

  return (
    <nav className="flex gap-1 overflow-x-auto border-b border-[#1e293b] pb-0">
      {tabs.map((tab) => {
        const href = tab.href(projectId);
        const isActive = pathname === href || pathname?.startsWith(href + '/');

        return (
          <Link
            key={tab.label}
            href={href}
            className={cn(
              'px-5 py-3 text-base font-semibold whitespace-nowrap border-b-2 transition-colors',
              isActive
                ? 'border-[#0EA5E9] text-[#0EA5E9]'
                : 'border-transparent text-[#94a3b8] hover:text-white hover:border-[#334155]',
            )}
          >
            {tab.label}
          </Link>
        );
      })}
    </nav>
  );
}
