'use client';

import Link from 'next/link';
import { useState } from 'react';
import { LoadingBlock } from '@/components/primitives/loading';
import { EmptyState } from '@/components/primitives/empty-state';
import { PageHeader } from '@/components/primitives/page-header';
import { SectionCard } from '@/components/primitives/section-card';
import { SkeletonPage } from '@/components/primitives/skeleton';
import { StatusBadge } from '@/components/primitives/status-badge';
import { ProjectThreeStepWizard } from '@/modules/bootstrap/project-three-step-wizard';
import { CreateProjectForm } from '@/modules/onboarding/create-project-form';
import { useProjects } from '@/modules/projects/project-hooks';
import { cn } from '@/lib/utils';

export default function OnboardingPage() {
  const { data: projects, isLoading } = useProjects();
  const [showCreateForm, setShowCreateForm] = useState(false);
  const [selectedProjectID, setSelectedProjectID] = useState<string | null>(null);

  const projectItems = projects?.items ?? [];
  const effectiveProjectID =
    selectedProjectID && projectItems.some((project) => project.id === selectedProjectID)
      ? selectedProjectID
      : (projectItems[0]?.id ?? null);

  const selectedProject = projectItems.find((project) => project.id === effectiveProjectID) ?? null;

  if (isLoading) {
    return <SkeletonPage title cards={2} />;
  }

  return (
    <div className="flex flex-col gap-10 max-w-[1400px] mx-auto py-10 lg:px-8">
      <div className="text-center md:text-left mb-6">
        <h1 className="text-4xl font-extrabold tracking-tight text-white mb-4">
          Bắt đầu triển khai với LazyOps
        </h1>
        <p className="text-[#94a3b8] text-[17px] max-w-2xl leading-relaxed">
          Chỉ cần 3 bước đơn giản: Tạo dự án, Kết nối GitHub, và Triển khai thẳng lên máy chủ của bạn mà không cần rành DevOps!
        </p>
      </div>

      {projectItems.length === 0 ? (
        <div className="animate-in fade-in slide-in-from-bottom-4 duration-500">
          {!showCreateForm ? (
            <div className="rounded-2xl border border-[#1e293b] bg-[#0F172A] p-12 text-center shadow-xl">
              <div className="mb-6 flex justify-center">
                <span className="text-6xl">🚀</span>
              </div>
              <h2 className="text-2xl font-bold text-white mb-3">Bước 1: Tạo dự án đầu tiên</h2>
              <p className="text-[#94a3b8] mb-8 max-w-md mx-auto">
                Hãy bắt đầu bằng cách tạo một không gian ảo chứa mã nguồn và môi trường của bạn.
              </p>
              <button
                type="button"
                className="rounded-xl bg-[#0EA5E9] px-8 py-4 text-lg font-bold text-white shadow-lg shadow-[#0ea5e9]/20 transition-all hover:bg-[#0284c7] hover:scale-105 active:scale-95"
                onClick={() => setShowCreateForm(true)}
              >
                + Tạo dự án mới ngay
              </button>
            </div>
          ) : (
            <div className="shadow-2xl rounded-2xl border border-[#1e293b] bg-[#0F172A] p-6">
              <CreateProjectForm onSuccess={() => setShowCreateForm(false)} />
            </div>
          )}
        </div>
      ) : (
        <div className="space-y-8 animate-in fade-in slide-in-from-bottom-4 duration-500">
          <div className="rounded-2xl border border-[#1e293b] bg-[#0F172A] p-6 shadow-sm">
            <div className="flex items-start justify-between mb-6">
              <div>
                <h2 className="text-[17px] font-bold text-white">Lựa chọn dự án</h2>
                <p className="text-[14px] text-[#94a3b8] mt-1">Vui lòng chọn dự án bạn muốn tiếp tục cấu hình triển khai.</p>
              </div>
              {selectedProject ? (
                <Link
                  href={`/projects/${selectedProject.id}`}
                  className="rounded-lg bg-[#1e293b] text-white hover:bg-[#2d3a4f] px-4 py-2 text-[13px] font-semibold transition-colors border border-[#334155]"
                >
                  Truy cập trang dự án &rarr;
                </Link>
              ) : null}
            </div>

            <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
              {projectItems.map((project) => {
                const selected = project.id === effectiveProjectID;
                return (
                  <button
                    key={project.id}
                    type="button"
                    className={cn(
                      "rounded-xl border-2 p-4 text-left transition-all duration-200",
                      selected
                        ? 'border-[#0EA5E9] bg-[#0c1a2c] shadow-[0_0_15px_rgba(14,165,233,0.1)]'
                        : 'border-[#1e293b] bg-[#0F172A] hover:border-[#334155] hover:bg-[#131c31]'
                    )}
                    onClick={() => setSelectedProjectID(project.id)}
                  >
                    <div className="mb-2 flex items-center justify-between">
                      <span className="text-base font-bold text-white">{project.name}</span>
                      <span className="text-[11px] text-[#64748b] font-mono">{project.default_branch}</span>
                    </div>
                    <p className="text-[13px] text-[#64748b]">/{project.slug}</p>
                  </button>
                );
              })}
            </div>
          </div>

          <div className="animate-in fade-in slide-in-from-bottom-4 duration-700">
            {selectedProject ? (
              <ProjectThreeStepWizard projectId={selectedProject.id} />
            ) : (
              <LoadingBlock label="Đang tải dữ liệu dự án..." className="py-12" />
            )}
          </div>
        </div>
      )}
    </div>
  );
}
