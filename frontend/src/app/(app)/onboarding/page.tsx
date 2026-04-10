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
    <div className="relative flex flex-col gap-12 max-w-[1400px] mx-auto py-12 lg:px-10 min-h-screen">
      {/* Background Decor */}
      <div className="absolute top-0 left-1/4 size-[600px] rounded-full bg-[#0EA5E9]/5 blur-[120px] pointer-events-none" />
      <div className="absolute bottom-1/4 right-0 size-[400px] rounded-full bg-[#38BDF8]/5 blur-[100px] pointer-events-none" />

      <div className="relative z-10 text-center md:text-left mb-4">
        <h1 className="text-5xl font-extrabold tracking-tight text-white mb-5">
          Triển khai dự án <span className="bg-gradient-to-r from-[#0EA5E9] to-[#38BDF8] bg-clip-text text-transparent underline decoration-[#0EA5E9]/30 underline-offset-8">trong tích tắc</span>
        </h1>
        <p className="text-[#94a3b8] text-[19px] max-w-3xl leading-relaxed font-medium">
          Giải pháp PaaS mạnh mẽ giúp bạn tập trung vào mã nguồn, còn hạ tầng và DevOps cứ để LazyOps lo!
        </p>
      </div>

      {projectItems.length === 0 ? (
        <div className="relative z-10 animate-in fade-in slide-in-from-bottom-6 duration-700">
          {!showCreateForm ? (
            <div className="rounded-3xl border border-[#1e293b] bg-[#0F172A]/80 backdrop-blur-xl p-16 text-center shadow-2xl relative overflow-hidden group">
              <div className="absolute top-0 left-0 w-full h-1 bg-gradient-to-r from-transparent via-[#0EA5E9] to-transparent opacity-50" />
              <div className="mb-8 flex justify-center">
                <div className="size-24 rounded-2xl bg-gradient-to-br from-[#0EA5E9]/20 to-[#38BDF8]/10 flex items-center justify-center text-6xl shadow-inner border border-white/5 group-hover:scale-110 transition-transform duration-500">
                  🚀
                </div>
              </div>
              <h2 className="text-3xl font-bold text-white mb-4">Chào mừng bạn đến với LazyOps</h2>
              <p className="text-[#94a3b8] mb-10 text-lg max-w-lg mx-auto leading-relaxed">
                Hệ thống chưa ghi nhận dự án nào. Hãy tạo dự án đầu tiên để bắt đầu hành trình triển khai không giới hạn.
              </p>
              <button
                type="button"
                className="rounded-2xl bg-gradient-to-r from-[#0EA5E9] to-[#38BDF8] px-10 py-5 text-xl font-bold text-white shadow-xl shadow-[#0ea5e9]/20 transition-all hover:scale-105 active:scale-95 flex items-center gap-3 mx-auto"
                onClick={() => setShowCreateForm(true)}
              >
                + Bắt đầu ngay bây giờ
              </button>
            </div>
          ) : (
            <div className="animate-in zoom-in-95 duration-500 shadow-2xl rounded-3xl border border-[#1e293b] bg-[#0F172A]/90 backdrop-blur-2xl p-10">
              <div className="flex items-center justify-between mb-8">
                <h2 className="text-3xl font-extrabold text-white tracking-tight">Cấu hình dự án mới</h2>
                <button 
                  onClick={() => setShowCreateForm(false)}
                  className="rounded-xl px-4 py-2 text-sm font-bold text-[#64748b] hover:text-white hover:bg-white/5 transition-all"
                >
                  Hủy và quay lại
                </button>
              </div>
              <CreateProjectForm onSuccess={() => setShowCreateForm(false)} />
            </div>
          )}
        </div>
      ) : (
        <div className="relative z-10 space-y-12 animate-in fade-in slide-in-from-bottom-6 duration-700">
          {showCreateForm ? (
            <div className="animate-in zoom-in-95 duration-500 shadow-2xl rounded-3xl border border-[#1e293b] bg-[#0F172A]/90 backdrop-blur-2xl p-10">
              <div className="flex items-center justify-between mb-8">
                <h2 className="text-3xl font-extrabold text-white tracking-tight">Cấu hình dự án mới</h2>
                <button 
                  onClick={() => setShowCreateForm(false)}
                  className="rounded-xl px-4 py-2 text-sm font-bold text-[#64748b] hover:text-white hover:bg-white/5 transition-all"
                >
                  Hủy và quay lại
                </button>
              </div>
              <CreateProjectForm onSuccess={() => setShowCreateForm(false)} />
            </div>
          ) : (
            <div className="rounded-3xl border border-[#1e293b] bg-[#0F172A]/60 backdrop-blur-xl p-10 shadow-2xl">
              <div className="flex flex-col md:flex-row items-start md:items-center justify-between gap-8 mb-10">
                <div>
                  <h2 className="text-2xl font-bold text-white tracking-tight flex items-center gap-3">
                    <span className="size-8 rounded-lg bg-[#0EA5E9]/20 flex items-center justify-center text-[#0EA5E9]">
                      <svg className="size-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2.5} d="M3 7v10a2 2 0 002 2h14a2 2 0 002-2V9a2 2 0 00-2-2h-6l-2-2H5a2 2 0 00-2 2z" />
                      </svg>
                    </span>
                    Danh sách dự án
                  </h2>
                  <p className="text-[16px] text-[#94a3b8] mt-2 font-medium">Chọn dự án bên dưới để tiếp tục quy trình thiết lập deployment.</p>
                </div>
                <div className="flex items-center gap-4">
                  <button
                    onClick={() => setShowCreateForm(true)}
                    className="rounded-2xl bg-gradient-to-r from-[#0EA5E9] to-[#38BDF8] text-white px-6 py-3.5 text-sm font-bold transition-all hover:scale-105 shadow-lg shadow-[#0ea5e9]/20 flex items-center gap-2"
                  >
                    <span>+</span> Tạo dự án mới
                  </button>
                  {selectedProject && (
                    <Link
                      href={`/projects/${selectedProject.id}`}
                      className="rounded-2xl bg-white/5 text-white hover:bg-white/10 px-6 py-3.5 text-sm font-bold transition-all border border-white/10"
                    >
                      Chi tiết &rarr;
                    </Link>
                  )}
                </div>
              </div>

              <div className="grid gap-6 sm:grid-cols-2 lg:grid-cols-4">
                {projectItems.map((project) => {
                  const isSelected = project.id === effectiveProjectID;
                  return (
                    <button
                      key={project.id}
                      type="button"
                      className={cn(
                        "relative rounded-2xl border-2 p-6 text-left transition-all duration-300 group overflow-hidden",
                        isSelected
                          ? 'border-[#0EA5E9] bg-[#0EA5E9]/5 shadow-[0_0_30px_rgba(14,165,233,0.15)] ring-1 ring-[#0EA5E9]/30 scale-105 z-10'
                          : 'border-[#1e293b] bg-[#0F172A] hover:border-[#334155] hover:bg-[#131c31]'
                      )}
                      onClick={() => setSelectedProjectID(project.id)}
                    >
                      {isSelected && (
                        <div className="absolute top-0 left-0 w-1 h-full bg-[#0EA5E9]" />
                      )}
                      <div className="mb-3 flex items-center justify-between">
                        <span className={cn(
                          "text-lg font-bold transition-colors",
                          isSelected ? "text-[#0EA5E9]" : "text-white group-hover:text-[#0EA5E9]"
                        )}>{project.name}</span>
                        <span className="text-[11px] text-[#64748b] font-mono bg-[#1e293b] px-2 py-1 rounded-lg border border-[#334155]/30 group-hover:bg-[#334155] transition-colors">{project.default_branch}</span>
                      </div>
                      <p className="text-[15px] text-[#64748b] font-mono font-medium opacity-80 group-hover:opacity-100 transition-opacity">/{project.slug}</p>
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
              <div className="py-24 flex flex-col items-center justify-center rounded-3xl border-2 border-dashed border-[#1e293b] bg-white/5 backdrop-blur-sm">
                <div className="size-16 rounded-full border-4 border-[#0EA5E9]/20 border-t-[#0EA5E9] animate-spin mb-6" />
                <p className="text-xl font-bold text-[#94a3b8]">Đang đồng bộ dữ liệu dự án...</p>
              </div>
            )}
          </div>
        </div>
      )}
    </div>
  );
}
