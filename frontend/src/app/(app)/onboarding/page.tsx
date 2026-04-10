'use client';

import Link from 'next/link';
import { useState } from 'react';
import { LoadingBlock } from '@/components/primitives/loading';
import { PageHeader } from '@/components/primitives/page-header';
import { SkeletonPage } from '@/components/primitives/skeleton';
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
            <div className="shadow-2xl rounded-2xl border border-[#1e293b] bg-[#0F172A] p-8">
              <div className="flex items-center justify-between mb-6">
                <h2 className="text-2xl font-bold text-white">Tạo dự án mới</h2>
                <button 
                  onClick={() => setShowCreateForm(false)}
                  className="text-[#64748b] hover:text-white transition-colors"
                >
                  Hủy bỏ
                </button>
              </div>
              <CreateProjectForm onSuccess={() => setShowCreateForm(false)} />
            </div>
          )}
        </div>
      ) : (
        <div className="space-y-10 animate-in fade-in slide-in-from-bottom-4 duration-500">
          {showCreateForm ? (
            <div className="animate-in fade-in slide-in-from-top-4 duration-500 shadow-2xl rounded-2xl border border-[#1e293b] bg-[#0F172A] p-8">
              <div className="flex items-center justify-between mb-6">
                <h2 className="text-2xl font-bold text-white">Tạo dự án mới</h2>
                <button 
                  onClick={() => setShowCreateForm(false)}
                  className="text-[#64748b] hover:text-white transition-colors"
                >
                  Hủy bỏ
                </button>
              </div>
              <CreateProjectForm onSuccess={() => setShowCreateForm(false)} />
            </div>
          ) : (
            <div className="rounded-2xl border border-[#1e293b] bg-[#0F172A] p-8 shadow-sm">
              <div className="flex flex-col md:flex-row items-start md:items-center justify-between gap-6 mb-8">
                <div>
                  <h2 className="text-xl font-bold text-white">Lựa chọn dự án</h2>
                  <p className="text-[15px] text-[#94a3b8] mt-1 font-medium">Chọn dự án bên dưới để xem trạng thái thiết lập.</p>
                </div>
                <div className="flex items-center gap-3">
                  <button
                    onClick={() => setShowCreateForm(true)}
                    className="rounded-xl bg-[#0EA5E9] text-white hover:bg-[#0284c7] px-5 py-2.5 text-sm font-bold transition-all hover:scale-105 shadow-lg shadow-[#0ea5e9]/20"
                  >
                    + Tạo dự án mới
                  </button>
                  {selectedProject && (
                    <Link
                      href={`/projects/${selectedProject.id}`}
                      className="rounded-xl bg-[#1e293b] text-white hover:bg-[#2d3a4f] px-5 py-2.5 text-sm font-bold transition-colors border border-[#334155]"
                    >
                      Chi tiết dự án &rarr;
                    </Link>
                  )}
                </div>
              </div>

              <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
                {projectItems.map((project) => {
                  const isSelected = project.id === effectiveProjectID;
                  return (
                    <button
                      key={project.id}
                      type="button"
                      className={cn(
                        "rounded-xl border-2 p-5 text-left transition-all duration-300",
                        isSelected
                          ? 'border-[#0EA5E9] bg-[#0c1a2c] shadow-[0_0_20px_rgba(14,165,233,0.15)] ring-1 ring-[#0EA5E9]/30'
                          : 'border-[#1e293b] bg-[#0F172A] hover:border-[#334155] hover:bg-[#131c31]'
                      )}
                      onClick={() => setSelectedProjectID(project.id)}
                    >
                      <div className="mb-2 flex items-center justify-between">
                        <span className="text-[15px] font-bold text-white">{project.name}</span>
                        <span className="text-[11px] text-[#64748b] font-mono bg-[#1e293b] px-1.5 py-0.5 rounded border border-[#334155]/30">{project.default_branch}</span>
                      </div>
                      <p className="text-sm text-[#64748b] font-mono">/{project.slug}</p>
                    </button>
                  );
                })}
              </div>
            </div>
          )}

          <div className="animate-in fade-in slide-in-from-bottom-10 duration-1000">
            {selectedProject ? (
              <ProjectThreeStepWizard projectId={selectedProject.id} />
            ) : (
              <div className="py-20 flex flex-col items-center justify-center rounded-2xl border border-dashed border-[#1e293b]">
                <LoadingBlock label="Đang cấu hình dữ liệu..." />
              </div>
            )}
          </div>
        </div>
      )}
    </div>
  );
}
