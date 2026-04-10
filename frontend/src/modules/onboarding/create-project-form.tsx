'use client';

import { useState } from 'react';
import { zodResolver } from '@hookform/resolvers/zod';
import { useForm, useWatch } from 'react-hook-form';
import { useCreateProject } from '@/modules/projects/project-hooks';
import { createProjectSchema, type CreateProjectFormData, INTERNAL_SERVICE_KINDS } from '@/modules/projects/project-types';
import { FormField, FormInput, FormButton } from '@/components/forms/form-fields';
import { cn } from '@/lib/utils';
import { FolderGit2, Hash, GitBranch, Database, Zap, MessageSquare, Box, Rocket, AlertCircle } from 'lucide-react';

const SERVICE_ICONS: Record<string, any> = {
  postgres: Database,
  mysql: Database,
  redis: Zap,
  rabbitmq: MessageSquare,
};

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

  const {
    register,
    handleSubmit,
    setValue,
    control,
    formState: { errors, isSubmitting },
  } = useForm<CreateProjectFormData>({
    resolver: zodResolver(createProjectSchema),
    defaultValues: { name: '', slug: '', default_branch: 'main', internal_services: [] },
  });

  const selectedServices = useWatch<CreateProjectFormData>({
    control,
    name: 'internal_services',
    defaultValue: [],
  }) as string[];
  const createProject = useCreateProject();

  const handleNameChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const name = e.target.value;
    setNameValue(name);
    if (autoSlug) {
      setValue('slug', slugify(name), { shouldValidate: true });
    }
  };

  const handleToggleAutoSlug = () => {
    if (!autoSlug && nameValue) {
      setValue('slug', slugify(nameValue), { shouldValidate: true });
    }
    setAutoSlug((prev) => !prev);
  };

  const onSubmit = (data: CreateProjectFormData) => {
    return createProject.mutateAsync(data).then(() => onSuccess?.());
  };

  const serverError = createProject.error?.message ?? null;

  return (
    <form onSubmit={handleSubmit(onSubmit)} className="flex flex-col gap-8" noValidate>
      <div className="grid grid-cols-1 md:grid-cols-2 gap-8">
        <div className="flex flex-col gap-6">
          <FormField label="Tên dự án" error={errors.name?.message}>
            <FormInput
              type="text"
              placeholder="Ví dụ: LazyOps Dashboard"
              icon={<FolderGit2 className="size-5" />}
              error={!!errors.name}
              {...register('name', { onChange: handleNameChange })}
            />
          </FormField>

          <FormField label="Slug dự án" error={errors.slug?.message}>
            <div className="flex items-center gap-3">
              <FormInput
                type="text"
                placeholder="lazyops-dashboard"
                icon={<Hash className="size-5" />}
                error={!!errors.slug}
                className="flex-1"
                {...register('slug')}
              />
              <button
                type="button"
                onClick={handleToggleAutoSlug}
                className={cn(
                  'h-12 px-4 rounded-xl text-xs font-bold transition-all border',
                  autoSlug
                    ? 'bg-[#0EA5E9]/10 text-[#0EA5E9] border-[#0EA5E9]/30'
                    : 'bg-[#1e293b] text-[#94a3b8] border-[#334155]'
                )}
              >
                {autoSlug ? 'Tự động' : 'Thủ công'}
              </button>
            </div>
          </FormField>

          <FormField label="Nhánh mặc định" error={errors.default_branch?.message}>
            <FormInput
              type="text"
              placeholder="main"
              icon={<GitBranch className="size-5" />}
              error={!!errors.default_branch}
              {...register('default_branch')}
            />
          </FormField>
        </div>

        <div className="flex flex-col gap-4">
          <div className="flex flex-col gap-1 mb-2">
            <span className="text-[13px] font-bold text-[#94a3b8] uppercase tracking-wider ml-1">Dịch vụ nội bộ</span>
            <p className="text-xs text-[#64748b] ml-1">Kích hoạt sẵn các hạ tầng bổ trợ cho ứng dụng.</p>
          </div>
          
          <div className="grid grid-cols-2 gap-3">
            {INTERNAL_SERVICE_KINDS.map((kind) => {
              const isSelected = selectedServices.includes(kind);
              const Icon = SERVICE_ICONS[kind] || Box;
              return (
                <label
                  key={kind}
                  className={cn(
                    "relative flex items-center gap-3 p-4 rounded-2xl border-2 transition-all cursor-pointer group",
                    isSelected 
                      ? "border-[#0EA5E9] bg-[#0EA5E9]/5 shadow-[0_0_15px_rgba(14,165,233,0.1)]" 
                      : "border-[#1e293b] bg-[#0F172A] hover:border-[#334155] hover:bg-[#131c31]"
                  )}
                >
                  <input
                    type="checkbox"
                    value={kind}
                    className="sr-only"
                    {...register('internal_services')}
                  />
                  <div className={cn(
                    "p-2 rounded-lg transition-colors",
                    isSelected ? "bg-[#0EA5E9] text-white" : "bg-[#1e293b] text-[#64748b] group-hover:text-white"
                  )}>
                    <Icon className="size-5" />
                  </div>
                  <span className={cn(
                    "text-sm font-bold capitalize transition-colors",
                    isSelected ? "text-white" : "text-[#94a3b8] group-hover:text-white"
                  )}>
                    {kind}
                  </span>
                  {isSelected && (
                    <div className="absolute top-2 right-2 size-2 rounded-full bg-[#0EA5E9] shadow-[0_0_5px_#0EA5E9]" />
                  )}
                </label>
              );
            })}
          </div>
          {errors.internal_services?.message && (
            <p className="text-xs text-[#ef4444] mt-1">{errors.internal_services.message}</p>
          )}
        </div>
      </div>

      {serverError && (
        <div className="p-4 rounded-xl border border-[#ef4444]/30 bg-[#ef4444]/10 flex items-center gap-3 text-sm text-[#ef4444] animate-in shake-in duration-300">
          <AlertCircle className="size-5 shrink-0" />
          {serverError}
        </div>
      )}

      <div className="pt-4 border-t border-[#1e293b]">
        <FormButton 
          type="submit" 
          loading={isSubmitting || createProject.isPending}
          className="h-14 text-lg"
        >
          <Rocket className="size-5 mr-2" />
          Tạo dự án & Bắt đầu thôi!
        </FormButton>
      </div>
    </form>
  );
}
