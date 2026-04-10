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
    <div className="flex flex-col gap-10 max-w-[1400px] mx-auto py-10 lg:px-8">
      <div className="flex flex-col md:flex-row justify-between items-start md:items-center gap-6">
        <div>
          <h1 className="text-4xl font-bold tracking-tight mb-2 text-white">Danh sách Dự án</h1>
          <p className="text-[#94a3b8] text-lg font-medium">Quản lý mã nguồn, cấu hình hạ tầng và xem lịch sử triển khai của bạn.</p>
        </div>
        <Link
          href="/projects/new"
          className="rounded-xl bg-[#0EA5E9] px-6 py-3.5 text-base font-bold text-white shadow-xl shadow-[#0ea5e9]/20 transition-all hover:bg-[#0284c7] hover:scale-105 active:scale-95"
        >
          + Dự án mới
        </Link>
      </div>

      {projects.length === 0 ? (
        <div className="rounded-2xl border border-dashed border-[#1e293b] bg-[#0F172A] p-16 text-center shadow-lg">
          <div className="mb-6 flex justify-center">
            <span className="text-6xl text-muted-foreground">📂</span>
          </div>
          <h2 className="text-2xl font-bold text-white mb-3">Bạn chưa có dự án nào</h2>
          <p className="text-[#94a3b8] mb-8 max-w-md mx-auto">
            Tạo dự án mới để kết nối mã nguồn và tự động triển khai hệ thống của bạn.
          </p>
          <Link
            href="/projects/new"
            className="inline-block rounded-xl bg-[#0EA5E9] px-10 py-4 text-lg font-bold text-white transition-all hover:bg-[#0284c7] shadow-lg shadow-[#0ea5e9]/20"
          >
            Tạo dự án đầu tiên
          </Link>
        </div>
      ) : (
        <div className="grid gap-6 mt-2">
          {projects.map((project) => (
            <Link
              key={project.id}
              href={`/projects/${project.id}`}
              className="group flex flex-col md:flex-row items-start md:items-center justify-between rounded-2xl border border-[#1e293b] bg-[#0F172A] p-8 transition-all hover:border-[#38BDF8]/40 hover:bg-[#131c31] hover:shadow-xl hover:scale-[1.01]"
            >
              <div className="flex flex-col gap-1 mb-4 md:mb-0">
                <span className="text-2xl font-bold text-white group-hover:text-[#38BDF8] transition-colors">{project.name}</span>
                <span className="text-[15px] text-[#64748b] font-mono tracking-tight">/{project.slug}</span>
              </div>
              <div className="flex flex-wrap items-center gap-8">
                <div className="flex items-center gap-3">
                  <span className="text-sm font-semibold text-[#64748b] uppercase tracking-wider">Nhánh:</span>
                  <div className="rounded-lg bg-[#1e293b] px-3 py-1.5 text-[13px] font-mono text-white border border-[#334155]">
                    {project.default_branch}
                  </div>
                </div>
                <div className="flex size-12 items-center justify-center rounded-xl bg-[#1e293b] text-white group-hover:bg-[#0EA5E9] group-hover:text-white transition-all shadow-sm">
                  <span className="text-xl">&rarr;</span>
                </div>
              </div>
            </Link>
          ))}
        </div>
      )}
    </div>
  );
}
