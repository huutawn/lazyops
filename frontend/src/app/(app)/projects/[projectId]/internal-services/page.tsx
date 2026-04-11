'use client';

import { useMemo, useState } from 'react';
import Link from 'next/link';
import { useParams } from 'next/navigation';
import { PageHeader } from '@/components/primitives/page-header';
import { SectionCard } from '@/components/primitives/section-card';
import { ErrorState } from '@/components/primitives/error-state';
import { SkeletonPage } from '@/components/primitives/skeleton';
import { FormButton } from '@/components/forms/form-fields';
import {
  INTERNAL_SERVICE_OPTIONS,
  type InternalServiceKind,
} from '@/modules/internal-services/internal-service-types';
import {
  useConfigureProjectInternalServices,
  useProjectInternalServices,
} from '@/modules/internal-services/internal-service-hooks';

export default function ProjectInternalServicesPage() {
  const params = useParams();
  const projectId = params?.projectId as string;
  const { data, isLoading, isError } = useProjectInternalServices(projectId);
  const configure = useConfigureProjectInternalServices(projectId);

  const selectedKinds = useMemo(
    () => new Set((data?.items ?? []).map((item) => item.kind)),
    [data?.items],
  );
  const [draftKinds, setDraftKinds] = useState<Set<InternalServiceKind> | null>(null);

  if (isLoading) {
    return <SkeletonPage title cards={1} />;
  }

  if (isError) {
    return (
      <div className="flex flex-col gap-6">
        <PageHeader title="Dịch vụ nội bộ" subtitle="Cấu hình dịch vụ do LazyOps tự tạo và tự nối localhost." />
        <ErrorState title="Không thể tải dữ liệu" message="Vui lòng thử lại sau." />
      </div>
    );
  }

  const working = draftKinds ?? selectedKinds;
  const toggleKind = (kind: InternalServiceKind) => {
    const base = new Set(working);
    if (base.has(kind)) {
      base.delete(kind);
    } else {
      base.add(kind);
    }
    setDraftKinds(base);
  };

  const dirty =
    draftKinds !== null &&
    (draftKinds.size !== selectedKinds.size ||
      [...draftKinds].some((item) => !selectedKinds.has(item)));

  return (
    <div className="flex flex-col gap-6">
      <PageHeader
        title="Dịch vụ nội bộ"
        subtitle="Chọn dịch vụ bạn cần, LazyOps sẽ tự tạo sidecar để app dùng localhost."
        actions={
          <Link
            href={`/projects/${projectId}`}
            className="rounded-lg border border-lazyops-border px-4 py-2 text-sm font-semibold text-lazyops-text transition-colors hover:bg-lazyops-border/10"
          >
            Quay lại thiết lập dự án
          </Link>
        }
      />

      <SectionCard title="Danh sách dịch vụ" description="Không cần tự cấu hình network hoặc policy JSON.">
        <div className="grid gap-3 sm:grid-cols-2">
          {INTERNAL_SERVICE_OPTIONS.map((option) => {
            const checked = working.has(option.kind);
            return (
              <button
                key={option.kind}
                type="button"
                className={`rounded-lg border px-4 py-3 text-left transition-colors ${
                  checked
                    ? 'border-primary/40 bg-primary/10'
                    : 'border-lazyops-border hover:bg-lazyops-border/10'
                }`}
                onClick={() => toggleKind(option.kind)}
              >
                <div className="mb-1 flex items-center justify-between">
                  <span className="text-sm font-semibold text-lazyops-text">{option.label}</span>
                  <span className="text-xs text-lazyops-muted">{checked ? 'Đã chọn' : 'Chưa chọn'}</span>
                </div>
                <p className="text-xs text-lazyops-muted">
                  Ứng dụng có thể gọi trực tiếp qua <code className="rounded bg-lazyops-border/20 px-1.5 py-0.5">{option.localhost}</code>
                </p>
              </button>
            );
          })}
        </div>

        {configure.error && (
          <div className="mt-3 rounded-lg border border-health-unhealthy/30 bg-health-unhealthy/10 px-3 py-2 text-xs text-health-unhealthy">
            {(configure.error as Error).message}
          </div>
        )}

        <div className="mt-4 flex justify-end">
          <FormButton
            type="button"
            loading={configure.isPending}
            disabled={configure.isPending}
            onClick={() => {
              const kinds = [...working].sort() as InternalServiceKind[];
              void configure.mutateAsync({ kinds }).then(() => {
                setDraftKinds(null);
              });
            }}
          >
            {dirty ? 'Lưu cấu hình' : 'Đồng bộ lại'}
          </FormButton>
        </div>
      </SectionCard>
    </div>
  );
}
