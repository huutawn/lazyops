'use client';

import { useState } from 'react';
import { zodResolver } from '@hookform/resolvers/zod';
import { useForm } from 'react-hook-form';
import { useCreateProject } from '@/modules/projects/project-hooks';
import { createProjectSchema, type CreateProjectFormData } from '@/modules/projects/project-types';
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
    defaultValues: { name: '', slug: '', default_branch: 'main' },
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
    <SectionCard title="Create your first project" description="Projects are the foundation of your LazyOps setup.">
      <form onSubmit={handleSubmit(onSubmit)} className="flex flex-col gap-4" noValidate>
        <FormField label="Project name" error={errors.name?.message}>
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
              {autoSlug ? 'Auto' : 'Manual'}
            </button>
          </div>
        </FormField>

        <FormField label="Default branch" error={errors.default_branch?.message}>
          <FormInput
            type="text"
            placeholder="main"
            error={!!errors.default_branch}
            {...register('default_branch')}
          />
        </FormField>

        {serverError && (
          <div className="rounded-lg border border-health-unhealthy/30 bg-health-unhealthy/10 px-3 py-2 text-xs text-health-unhealthy">
            {serverError}
          </div>
        )}

        <FormButton type="submit" loading={isSubmitting || createProject.isPending}>
          Create project
        </FormButton>
      </form>
    </SectionCard>
  );
}
