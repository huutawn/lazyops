'use client';

import { useState, type ChangeEvent } from 'react';
import { zodResolver } from '@hookform/resolvers/zod';
import { useForm } from 'react-hook-form';
import { AlertCircle, FolderGit2, GitBranch, Hash, Layers, Rocket } from 'lucide-react';
import { FormButton, FormField, FormInput } from '@/components/forms/form-fields';
import { cn } from '@/lib/utils';
import { useCreateProject } from '@/modules/projects/project-hooks';
import { createProjectSchema, type CreateProjectFormData } from '@/modules/projects/project-types';

type CreateProjectFormProps = {
  onSuccess?: () => void;
};

function slugify(value: string): string {
  return value
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-+|-+$/g, '');
}

export function CreateProjectForm({ onSuccess }: CreateProjectFormProps) {
  const [autoSlug, setAutoSlug] = useState(true);
  const [nameValue, setNameValue] = useState('');
  const createProject = useCreateProject();

  const {
    register,
    handleSubmit,
    setValue,
    formState: { errors, isSubmitting },
  } = useForm<CreateProjectFormData>({
    resolver: zodResolver(createProjectSchema),
    defaultValues: {
      name: '',
      slug: '',
      default_branch: 'main',
      services: [],
      internal_services: [],
    },
  });

  const handleNameChange = (event: ChangeEvent<HTMLInputElement>) => {
    const nextName = event.target.value;
    setNameValue(nextName);
    if (autoSlug) {
      setValue('slug', slugify(nextName), { shouldValidate: true });
    }
  };

  const handleToggleAutoSlug = () => {
    if (!autoSlug && nameValue) {
      setValue('slug', slugify(nameValue), { shouldValidate: true });
    }
    setAutoSlug((current) => !current);
  };

  const onSubmit = (data: CreateProjectFormData) => {
    return createProject
      .mutateAsync({
        ...data,
        services: [],
        internal_services: [],
      })
      .then(() => onSuccess?.());
  };

  const serverError = createProject.error?.message ?? null;

  return (
    <form onSubmit={handleSubmit(onSubmit)} className="flex flex-col gap-8" noValidate>
      <div className="grid gap-8 xl:grid-cols-[minmax(0,1fr)_minmax(0,0.9fr)]">
        <div className="min-w-0 rounded-3xl border border-[#1e293b] bg-[#0B1120]/80 p-6">
          <div className="mb-5 flex items-center gap-3">
            <span className="rounded-2xl bg-[#0EA5E9]/10 p-3 text-[#38BDF8]">
              <FolderGit2 className="size-5" />
            </span>
            <div>
              <h3 className="text-xl font-bold text-white">Khởi tạo Project</h3>
              <p className="text-base text-[#94a3b8]">
                Đặt tên và định danh cho Project của bạn.
              </p>
            </div>
          </div>

          <div className="grid gap-5">
            <FormField label="Tên project" error={errors.name?.message}>
              <FormInput
                type="text"
                placeholder="Ví dụ: LazyOps Commerce"
                icon={<FolderGit2 className="size-5" />}
                error={!!errors.name}
                {...register('name', { onChange: handleNameChange })}
              />
            </FormField>

            <FormField label="Slug project" error={errors.slug?.message}>
              <div className="flex min-w-0 items-center gap-3">
                <FormInput
                  type="text"
                  placeholder="lazyops-commerce"
                  icon={<Hash className="size-5" />}
                  error={!!errors.slug}
                  className="min-w-0 flex-1"
                  {...register('slug')}
                />
                <button
                  type="button"
                  onClick={handleToggleAutoSlug}
                  className={cn(
                    'h-12 rounded-xl border px-6 text-sm font-bold transition-all',
                    autoSlug
                      ? 'border-[#0EA5E9]/30 bg-[#0EA5E9]/10 text-[#38BDF8]'
                      : 'border-[#334155] bg-[#111827] text-[#94a3b8]',
                  )}
                >
                  {autoSlug ? 'Tự động' : 'Thủ công'}
                </button>
              </div>
            </FormField>

            <FormField label="Default branch" error={errors.default_branch?.message}>
              <FormInput
                type="text"
                placeholder="main"
                icon={<GitBranch className="size-5" />}
                error={!!errors.default_branch}
                {...register('default_branch')}
              />
            </FormField>
          </div>
        </div>

        <div className="min-w-0 rounded-3xl border border-[#1e293b] bg-[#0B1120]/80 p-6">
          <div className="mb-4 flex items-center gap-3">
            <span className="rounded-2xl bg-[#14B8A6]/10 p-3 text-[#2dd4bf]">
              <Layers className="size-5" />
            </span>
            <div>
              <h3 className="text-xl font-bold text-white">Hướng dẫn kết nối tiếp theo</h3>
              <p className="text-base text-[#94a3b8]">
                Sau khi tạo xong Project, bạn sẽ cần thiết lập thêm các thành phần khác.
              </p>
            </div>
          </div>

          <div className="grid gap-3">
            <NextStepCard
              title="1. Thêm Service từ Repository"
              description="Liên kết mã nguồn ứng dụng (Web, API...) từ Repository của bạn."
            />
            <NextStepCard
              title="2. Thêm Database"
              description="Khởi tạo cơ sở dữ liệu nếu Project của bạn yêu cầu."
            />
            <NextStepCard
              title="3. Bắt đầu Deploy"
              description="Sau khi cài đặt xong, bạn có thể thực hiện cấu hình khởi chạy dễ dàng."
            />
          </div>
        </div>
      </div>

      {serverError ? (
        <div className="flex items-center gap-3 rounded-2xl border border-[#ef4444]/30 bg-[#ef4444]/10 px-6 py-3 text-base text-[#fecaca]">
          <AlertCircle className="size-5 shrink-0" />
          {serverError}
        </div>
      ) : null}

      <div className="border-t border-[#1e293b] pt-6">
        <FormButton type="submit" loading={isSubmitting || createProject.isPending} className="h-14 text-lg">
          <Rocket className="mr-2 size-5" />
          Tạo Project
        </FormButton>
      </div>
    </form>
  );
}

function NextStepCard({ title, description }: { title: string; description: string }) {
  return (
    <div className="rounded-2xl border border-[#1e293b] bg-[#0F172A] px-6 py-6">
      <div className="text-base font-semibold text-white">{title}</div>
      <p className="mt-2 text-base leading-relaxed text-[#94a3b8]">{description}</p>
    </div>
  );
}
