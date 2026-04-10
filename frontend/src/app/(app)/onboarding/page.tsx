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
    <div className="flex flex-col gap-8 max-w-5xl mx-auto py-4">
      <div className="text-center md:text-left mb-4">
        <h1 className="text-3xl md:text-4xl font-extrabold tracking-tight text-foreground mb-3">
          Bắt đầu triển khai với LazyOps
        </h1>
        <p className="text-lg text-muted-foreground max-w-2xl">
          Chỉ cần 3 bước đơn giản: Tạo dự án, Kết nối GitHub, và Triển khai thẳng lên máy chủ của bạn mà không cần rành DevOps!
        </p>
      </div>

      {projectItems.length === 0 ? (
        <div className="animate-in fade-in slide-in-from-bottom-4 duration-500">
          {!showCreateForm ? (
            <SectionCard className="shadow-xl rounded-2xl border-primary/20 bg-card/60 backdrop-blur-sm p-4">
              <EmptyState
                icon={<span className="text-5xl">🚀</span>}
                title="Bước 1: Tạo dự án đầu tiên"
                description="Hãy bắt đầu bằng cách tạo một không gian ảo chứa mã nguồn và môi trường của bạn."
                action={
                  <button
                    type="button"
                    className="mt-4 rounded-xl bg-primary px-8 py-4 text-lg font-bold text-primary-foreground shadow-lg shadow-primary/30 transition-all hover:bg-primary/90 hover:scale-105 active:scale-95"
                    onClick={() => setShowCreateForm(true)}
                  >
                    + Tạo dự án mới ngay
                  </button>
                }
              />
            </SectionCard>
          ) : (
            <div className="shadow-2xl rounded-2xl border-primary/20 bg-card p-6">
              <CreateProjectForm onSuccess={() => setShowCreateForm(false)} />
            </div>
          )}
        </div>
      ) : (
        <div className="space-y-8 animate-in fade-in slide-in-from-bottom-4 duration-500">
          <SectionCard
            title="Lựa chọn dự án"
            description="Vui lòng chọn dự án bạn muốn tiếp tục cấu hình triển khai."
            className="shadow-md rounded-xl"
            actions={
              selectedProject ? (
                <Link
                  href={`/projects/${selectedProject.id}`}
                  className="rounded-lg bg-secondary text-secondary-foreground hover:bg-secondary/80 px-4 py-2 text-sm font-semibold transition-colors shadow-sm"
                >
                  Truy cập trang dự án &rarr;
                </Link>
              ) : null
            }
          >
            <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
              {projectItems.map((project) => {
                const selected = project.id === effectiveProjectID;
                return (
                  <button
                    key={project.id}
                    type="button"
                    className={`rounded-xl border-2 p-4 text-left transition-all duration-200 group ${
                      selected
                        ? 'border-primary bg-primary/5 shadow-md scale-[1.02]'
                        : 'border-border hover:border-primary/40 hover:bg-accent hover:shadow-sm'
                    }`}
                    onClick={() => setSelectedProjectID(project.id)}
                  >
                    <div className="mb-2 flex items-center justify-between">
                      <span className="text-base font-bold text-foreground group-hover:text-primary transition-colors">{project.name}</span>
                      <StatusBadge label={project.default_branch} variant="neutral" size="sm" dot={false} />
                    </div>
                    <p className="text-sm text-muted-foreground truncate">/{project.slug}</p>
                  </button>
                );
              })}
            </div>
          </SectionCard>

          <div className="rounded-2xl border bg-card/50 shadow-sm overflow-hidden p-2">
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
