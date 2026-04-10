'use client';

import Link from 'next/link';
import { useProjects } from '@/modules/projects/project-hooks';
import { PageHeader } from '@/components/primitives/page-header';
import { SectionCard } from '@/components/primitives/section-card';
import { EmptyState } from '@/components/primitives/empty-state';
import { ErrorState } from '@/components/primitives/error-state';
import { SkeletonPage } from '@/components/primitives/skeleton';
import { StatusBadge } from '@/components/primitives/status-badge';

export default function ProjectsPage() {
  const { data, isLoading, isError } = useProjects();

  if (isLoading) {
    return <SkeletonPage title cards={1} />;
  }

  if (isError) {
    return (
      <div className="flex flex-col gap-6 max-w-5xl mx-auto py-4">
        <PageHeader title="Dự án của bạn" subtitle="Quản lý các ứng dụng và dự án đã được kết nối." />
        <ErrorState title="Lỗi tải dữ liệu" message="Không thể lấy danh sách dự án từ máy chủ." />
      </div>
    );
  }

  const projects = data?.items ?? [];

  return (
    <div className="flex flex-col gap-8 max-w-5xl mx-auto py-4">
      <div className="flex flex-col md:flex-row justify-between items-start md:items-center gap-4">
        <div>
          <h1 className="text-3xl font-bold tracking-tight mb-2 text-white">Danh sách Dự án</h1>
          <p className="text-[#94a3b8] text-lg">Quản lý mã nguồn, cấu hình hạ tầng và xem lịch sử triển khai của bạn.</p>
        </div>
        <Link
          href="/projects/new"
          className="rounded-lg bg-[#0EA5E9] px-6 py-2.5 text-[15px] font-semibold text-white shadow-sm transition-all hover:bg-[#0284c7]"
        >
          + Dự án mới
        </Link>
      </div>

      {projects.length === 0 ? (
        <SectionCard className="shadow-lg p-6 rounded-2xl border-dashed border-2">
          <EmptyState
            icon={<span className="text-4xl text-muted-foreground">📂</span>}
            title="Bạn chưa có dự án nào"
            description="Tạo dự án mới để kết nối mã nguồn và tự động triển khai."
            action={
              <Link
                href="/projects/new"
                className="mt-4 inline-block rounded-lg bg-[#0EA5E9] px-6 py-2.5 text-[15px] font-semibold text-white transition-all hover:bg-[#0284c7] shadow-md"
              >
                Tạo dự án đầu tiên
              </Link>
            }
          />
        </SectionCard>
      ) : (
        <div className="grid gap-4 mt-2">
          {projects.map((project) => (
            <Link
              key={project.id}
              href={`/projects/${project.id}`}
              className="group flex flex-col md:flex-row items-start md:items-center justify-between rounded-xl border border-border bg-card p-5 transition-all hover:border-primary/50 hover:bg-accent/50 hover:shadow-md"
            >
              <div className="flex flex-col mb-2 md:mb-0">
                <span className="text-xl font-bold text-foreground group-hover:text-primary transition-colors">{project.name}</span>
                <span className="text-sm text-muted-foreground mt-1 font-mono">/{project.slug}</span>
              </div>
              <div className="flex items-center gap-4">
                <div className="hidden sm:flex items-center gap-2 text-sm text-muted-foreground">
                  <span>Nhánh:</span>
                  <StatusBadge label={project.default_branch} variant="neutral" size="sm" dot={false} />
                </div>
                <div className="flex size-8 items-center justify-center rounded-full bg-secondary/50 text-secondary-foreground group-hover:bg-primary/10 group-hover:text-primary transition-colors">
                  &rarr;
                </div>
              </div>
            </Link>
          ))}
        </div>
      )}
    </div>
  );
}
