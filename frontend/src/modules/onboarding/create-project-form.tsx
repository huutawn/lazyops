'use client';

import { useState, type ChangeEvent, type ReactNode } from 'react';
import { zodResolver } from '@hookform/resolvers/zod';
import { useForm } from 'react-hook-form';
import {
  AlertCircle,
  Database,
  FolderGit2,
  GitBranch,
  Globe,
  Hash,
  Layers,
  Lock,
  Rocket,
  Server,
} from 'lucide-react';
import { FormButton, FormField, FormInput } from '@/components/forms/form-fields';
import { cn } from '@/lib/utils';
import { useCreateProject } from '@/modules/projects/project-hooks';
import {
  buildCreateProjectServices,
  createDefaultServiceFirstScaffold,
  type ServiceFirstScaffold,
} from '@/modules/projects/create-project-services';
import { createProjectSchema, type CreateProjectFormData } from '@/modules/projects/project-types';
import {
  POSTGRES_CONNECTION_TEMPLATE_SLOTS,
  formatPostgresConnectionTemplatePreview,
} from '@/modules/project-services/postgres-connection-template';

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
  const [scaffold, setScaffold] = useState<ServiceFirstScaffold>(createDefaultServiceFirstScaffold());
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

  const previewServices = buildCreateProjectServices(scaffold);

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
        services: previewServices,
        internal_services: [],
      })
      .then(() => onSuccess?.());
  };

  const serverError = createProject.error?.message ?? null;

  return (
    <form onSubmit={handleSubmit(onSubmit)} className="flex flex-col gap-8" noValidate>
      <div className="grid gap-8 xl:grid-cols-[1.05fr_0.95fr]">
        <div className="grid gap-6">
          <div className="rounded-3xl border border-[#1e293b] bg-[#0B1120]/80 p-6">
            <div className="mb-5 flex items-center gap-3">
              <span className="rounded-2xl bg-[#0EA5E9]/10 p-3 text-[#38BDF8]">
                <FolderGit2 className="size-5" />
              </span>
              <div>
                <h3 className="text-lg font-bold text-white">Project shell</h3>
                <p className="text-sm text-[#94a3b8]">
                  Project chi dong vai tro namespace/folder logic. Deployment se duoc quyet dinh boi service inventory ben phai.
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
                <div className="flex items-center gap-3">
                  <FormInput
                    type="text"
                    placeholder="lazyops-commerce"
                    icon={<Hash className="size-5" />}
                    error={!!errors.slug}
                    className="flex-1"
                    {...register('slug')}
                  />
                  <button
                    type="button"
                    onClick={handleToggleAutoSlug}
                    className={cn(
                      'h-12 rounded-xl border px-4 text-xs font-bold transition-all',
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

          <div className="rounded-3xl border border-[#1e293b] bg-[#0B1120]/80 p-6">
            <div className="mb-4 flex items-center gap-3">
              <span className="rounded-2xl bg-[#14B8A6]/10 p-3 text-[#2dd4bf]">
                <Layers className="size-5" />
              </span>
              <div>
                <h3 className="text-lg font-bold text-white">Service preview</h3>
                <p className="text-sm text-[#94a3b8]">
                  Payload `/projects` se duoc scaffold san voi service catalog ngay tu dau.
                </p>
              </div>
            </div>

            <div className="grid gap-3">
              {previewServices.map((service) => (
                <div
                  key={`${service.source_type}:${service.name}`}
                  className="flex items-center justify-between rounded-2xl border border-[#1e293b] bg-[#0F172A] px-4 py-3"
                >
                  <div>
                    <div className="text-sm font-semibold text-white">{service.name}</div>
                    <div className="text-xs text-[#64748b]">
                      {service.source_type === 'internal' ? service.path : `${service.path} • ${service.kind || 'app'}`}
                    </div>
                  </div>
                  <span
                    className={cn(
                      'rounded-full px-3 py-1 text-xs font-semibold uppercase tracking-[0.14em]',
                      service.source_type === 'internal'
                        ? 'bg-[#14B8A6]/10 text-[#2dd4bf]'
                        : 'bg-[#0EA5E9]/10 text-[#38BDF8]',
                    )}
                  >
                    {service.source_type}
                  </span>
                </div>
              ))}
            </div>
          </div>
        </div>

        <div className="grid gap-6">
          <ServiceCard
            title="Backend service"
            description="Repo service cho API / worker chinh. Co the noi thang vao internal Postgres neu duoc bat."
            icon={<Server className="size-5" />}
            enabled={scaffold.backend_enabled}
            onToggle={() => setScaffold((current) => ({ ...current, backend_enabled: !current.backend_enabled }))}
          >
            <div className="grid gap-4 md:grid-cols-2">
              <InlineInput
                label="Service name"
                value={scaffold.backend_name}
                onChange={(value) => setScaffold((current) => ({ ...current, backend_name: value }))}
                placeholder="api"
              />
              <InlineInput
                label="Repo path"
                value={scaffold.backend_path}
                onChange={(value) => setScaffold((current) => ({ ...current, backend_path: value }))}
                placeholder="apps/api"
              />
            </div>
            <div className="mt-4 grid gap-3 md:grid-cols-2">
              <TogglePill
                label="Public ingress"
                active={scaffold.backend_public}
                onClick={() => setScaffold((current) => ({ ...current, backend_public: !current.backend_public }))}
                icon={<Globe className="size-4" />}
              />
              <TogglePill
                label="Connect Postgres"
                active={scaffold.backend_connects_to_postgres}
                onClick={() =>
                  setScaffold((current) => ({
                    ...current,
                    backend_connects_to_postgres: !current.backend_connects_to_postgres,
                  }))
                }
                disabled={!scaffold.postgres_enabled}
                icon={<Database className="size-4" />}
              />
            </div>
          </ServiceCard>

          <ServiceCard
            title="Frontend service"
            description="Repo service cho web/app public. Project van chi la shell, frontend la service deploy doc lap."
            icon={<Globe className="size-5" />}
            enabled={scaffold.frontend_enabled}
            onToggle={() => setScaffold((current) => ({ ...current, frontend_enabled: !current.frontend_enabled }))}
          >
            <div className="grid gap-4 md:grid-cols-2">
              <InlineInput
                label="Service name"
                value={scaffold.frontend_name}
                onChange={(value) => setScaffold((current) => ({ ...current, frontend_name: value }))}
                placeholder="web"
              />
              <InlineInput
                label="Repo path"
                value={scaffold.frontend_path}
                onChange={(value) => setScaffold((current) => ({ ...current, frontend_path: value }))}
                placeholder="apps/web"
              />
            </div>
            <div className="mt-4">
              <TogglePill
                label="Public ingress"
                active={scaffold.frontend_public}
                onClick={() => setScaffold((current) => ({ ...current, frontend_public: !current.frontend_public }))}
                icon={<Globe className="size-4" />}
              />
            </div>
          </ServiceCard>

          <ServiceCard
            title="Internal Postgres"
            description="Managed internal service. User nhap ten env can map, LazyOps se tu inject gia tri runtime vao cac env nay."
            icon={<Database className="size-5" />}
            enabled={scaffold.postgres_enabled}
            onToggle={() => setScaffold((current) => ({ ...current, postgres_enabled: !current.postgres_enabled }))}
          >
            <InlineInput
              label="Service name"
              value={scaffold.postgres_service_name}
              onChange={(value) => setScaffold((current) => ({ ...current, postgres_service_name: value }))}
              placeholder="db"
            />
            <div className="mt-4 grid gap-4 md:grid-cols-2">
              {POSTGRES_CONNECTION_TEMPLATE_SLOTS.map((slot) => (
                <InlineInput
                  key={slot}
                  label={slot}
                  value={scaffold.postgres_connection_template[slot]}
                  onChange={(value) =>
                    setScaffold((current) => ({
                      ...current,
                      postgres_connection_template: {
                        ...current.postgres_connection_template,
                        [slot]: value,
                      },
                    }))
                  }
                  placeholder={slot}
                />
              ))}
            </div>
            <div className="mt-4 rounded-2xl border border-[#1e293b] bg-[#020617] p-4">
              <div className="mb-2 flex items-center gap-2 text-xs font-semibold uppercase tracking-[0.14em] text-[#64748b]">
                <Lock className="size-4" />
                Preview env keys
              </div>
              <pre className="overflow-x-auto text-sm text-[#e2e8f0]">
                {formatPostgresConnectionTemplatePreview(scaffold.postgres_connection_template)}
              </pre>
            </div>
          </ServiceCard>
        </div>
      </div>

      {serverError ? (
        <div className="flex items-center gap-3 rounded-2xl border border-[#ef4444]/30 bg-[#ef4444]/10 px-4 py-3 text-sm text-[#fecaca]">
          <AlertCircle className="size-5 shrink-0" />
          {serverError}
        </div>
      ) : null}

      <div className="border-t border-[#1e293b] pt-4">
        <FormButton type="submit" loading={isSubmitting || createProject.isPending} className="h-14 text-lg">
          <Rocket className="mr-2 size-5" />
          Tạo project theo service-first
        </FormButton>
      </div>
    </form>
  );
}

function ServiceCard({
  title,
  description,
  icon,
  enabled,
  onToggle,
  children,
}: {
  title: string;
  description: string;
  icon: ReactNode;
  enabled: boolean;
  onToggle: () => void;
  children: ReactNode;
}) {
  return (
    <div className="rounded-3xl border border-[#1e293b] bg-[#0B1120]/80 p-6">
      <div className="flex items-start justify-between gap-4">
        <div className="flex items-start gap-3">
          <span className={cn('rounded-2xl p-3', enabled ? 'bg-[#0EA5E9]/10 text-[#38BDF8]' : 'bg-[#111827] text-[#64748b]')}>
            {icon}
          </span>
          <div>
            <h3 className="text-lg font-bold text-white">{title}</h3>
            <p className="mt-1 text-sm leading-relaxed text-[#94a3b8]">{description}</p>
          </div>
        </div>
        <button
          type="button"
          onClick={onToggle}
          className={cn(
            'rounded-full border px-4 py-2 text-xs font-bold uppercase tracking-[0.14em] transition-colors',
            enabled
              ? 'border-[#0EA5E9]/30 bg-[#0EA5E9]/10 text-[#38BDF8]'
              : 'border-[#334155] bg-[#111827] text-[#94a3b8]',
          )}
        >
          {enabled ? 'Enabled' : 'Disabled'}
        </button>
      </div>
      {enabled ? <div className="mt-5">{children}</div> : null}
    </div>
  );
}

function InlineInput({
  label,
  value,
  onChange,
  placeholder,
}: {
  label: string;
  value: string;
  onChange: (value: string) => void;
  placeholder: string;
}) {
  return (
    <label className="grid gap-2">
      <span className="text-xs font-semibold uppercase tracking-[0.14em] text-[#64748b]">{label}</span>
      <input
        value={value}
        onChange={(event) => onChange(event.target.value)}
        placeholder={placeholder}
        className="h-12 rounded-2xl border border-[#1e293b] bg-[#0F172A] px-4 text-sm text-white outline-none transition-colors placeholder:text-[#475569] focus:border-[#0EA5E9]"
      />
    </label>
  );
}

function TogglePill({
  label,
  active,
  onClick,
  disabled = false,
  icon,
}: {
  label: string;
  active: boolean;
  onClick: () => void;
  disabled?: boolean;
  icon: ReactNode;
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      disabled={disabled}
      className={cn(
        'inline-flex items-center justify-center gap-2 rounded-2xl border px-4 py-3 text-sm font-semibold transition-colors',
        disabled
          ? 'cursor-not-allowed border-[#1e293b] bg-[#0F172A]/40 text-[#475569]'
          : active
            ? 'border-[#0EA5E9]/30 bg-[#0EA5E9]/10 text-[#38BDF8]'
            : 'border-[#334155] bg-[#111827] text-[#cbd5e1]',
      )}
    >
      {icon}
      {label}
    </button>
  );
}
