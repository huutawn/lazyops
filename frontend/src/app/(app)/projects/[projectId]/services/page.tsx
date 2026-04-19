import Link from 'next/link';
import { PageHeader } from '@/components/primitives/page-header';
import { ProjectServiceInventory } from '@/modules/project-services/project-service-inventory';

type ProjectServicesPageProps = {
  params: Promise<{ projectId: string }>;
  searchParams: Promise<{ source?: string }>;
};

const SOURCE_FILTERS = [
  { key: 'all', label: 'Tất cả' },
  { key: 'repo', label: 'Từ repository' },
  { key: 'internal', label: 'Nội bộ' },
] as const;

export default async function ProjectServicesPage({ params, searchParams }: ProjectServicesPageProps) {
  const { projectId } = await params;
  const { source } = await searchParams;
  const sourceFilter = source === 'repo' || source === 'internal' ? source : 'all';

  return (
    <div className="flex flex-col gap-6">
      <PageHeader
        title="Dịch vụ"
        subtitle="Mỗi service là một phần chạy độc lập của project. Hãy cấu hình source, database và cách truy cập thật rõ ràng."
      />
      <div className="flex flex-wrap gap-3">
        {SOURCE_FILTERS.map((filter) => {
          const href =
            filter.key === 'all'
              ? `/projects/${projectId}/services`
              : `/projects/${projectId}/services?source=${filter.key}`;
          const active = filter.key === sourceFilter;
          return (
            <Link
              key={filter.key}
              href={href}
              className={`rounded-full border px-4 py-2 text-sm font-semibold transition-colors ${
                active
                  ? 'border-[#0EA5E9] bg-[#0EA5E9]/10 text-[#38bdf8]'
                  : 'border-[#334155] text-[#cbd5e1] hover:bg-[#0B1120]'
              }`}
            >
              {filter.label}
            </Link>
          );
        })}
      </div>
      <ProjectServiceInventory
        projectId={projectId}
        description="Bạn có thể xem, sửa và deploy từng service ngay tại đây. Thông tin kỹ thuật sâu hơn được đặt trong phần Nâng cao."
        compact
        sourceFilter={sourceFilter}
      />
    </div>
  );
}
