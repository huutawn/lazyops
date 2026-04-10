'use client';

import { useState } from 'react';
import { zodResolver } from '@hookform/resolvers/zod';
import { useForm } from 'react-hook-form';
import { useCreateProject } from '@/modules/projects/project-hooks';
import { INTERNAL_SERVICE_KINDS, createProjectSchema, type CreateProjectFormData } from '@/modules/projects/project-types';
import { FormField, FormInput, FormButton } from '@/components/forms/form-fields';
import { SectionCard } from '@/components/primitives/section-card';
import { cn } from '@/lib/utils';

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
    formState: { errors, isSubmitting },
  } = useForm<CreateProjectFormData>({
    resolver: zodResolver(createProjectSchema),
    defaultValues: { name: '', slug: '', default_branch: 'main', internal_services: [] },
  });

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
    <SectionCard title="Tạo dự án đầu tiên" description="Dự án là nền tảng cho toàn bộ luồng triển khai LazyOps.">
      <form onSubmit={handleSubmit(onSubmit)} className="flex flex-col gap-4" noValidate>
        <FormField label="Tên dự án" error={errors.name?.message}>
          <FormInput
            type="text"
            placeholder="my-awesome-app"
            error={!!errors.name}
            {...register('name', { onChange: handleNameChange })}
          />
        </FormField>

        <FormField label="Slug" error={errors.slug?.message}>
          <div className="flex items-center gap-2">
            <FormInput
              type="text"
              placeholder="my-awesome-app"
              error={!!errors.slug}
              className="flex-1"
              {...register('slug')}
            />
            <button
              type="button"
              className={cn(
                'shrink-0 rounded-lg border px-2.5 py-2 text-xs transition-colors',
                autoSlug
                  ? 'border-primary/30 bg-primary/10 text-primary'
                  : 'border-lazyops-border text-lazyops-muted hover:text-lazyops-text',
              )}
              onClick={handleToggleAutoSlug}
            >
              {autoSlug ? 'Tự động' : 'Thủ công'}
            </button>
          </div>
        </FormField>

        <FormField label="Nhánh mặc định" error={errors.default_branch?.message}>
          <FormInput
            type="text"
            placeholder="main"
            error={!!errors.default_branch}
            {...register('default_branch')}
          />
        </FormField>

        <FormField label="Dịch vụ nội bộ" error={errors.internal_services?.message}>
          <div className="grid grid-cols-2 gap-2 sm:grid-cols-4">
            {INTERNAL_SERVICE_KINDS.map((kind) => (
              <label
                key={kind}
                className="flex items-center gap-2 rounded-lg border border-lazyops-border bg-lazyops-surface px-3 py-2 text-sm text-lazyops-text"
              >
                <input
                  type="checkbox"
                  value={kind}
                  className="size-4 rounded border-lazyops-border bg-transparent"
                  {...register('internal_services')}
                />
                <span className="capitalize">{kind}</span>
              </label>
            ))}
          </div>
          <p className="text-xs text-lazyops-muted">
            Chọn dịch vụ nội bộ để LazyOps tự nối sidecar/localhost cho ứng dụng.
          </p>
        </FormField>

        {serverError && (
          <div className="rounded-lg border border-health-unhealthy/30 bg-health-unhealthy/10 px-3 py-2 text-xs text-health-unhealthy">
            {serverError}
          </div>
        )}

        <FormButton type="submit" loading={isSubmitting || createProject.isPending}>
          Tạo dự án
        </FormButton>
      </form>
    </SectionCard>
  );
}
