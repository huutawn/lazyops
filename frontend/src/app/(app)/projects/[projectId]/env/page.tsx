'use client';

import { useEffect, useMemo, useRef, useState } from 'react';
import Link from 'next/link';
import { useParams } from 'next/navigation';
import { PageHeader } from '@/components/primitives/page-header';
import { SectionCard } from '@/components/primitives/section-card';
import { ErrorState } from '@/components/primitives/error-state';
import { SkeletonPage } from '@/components/primitives/skeleton';
import { FormButton } from '@/components/forms/form-fields';
import { useDeleteProjectEnv, useProjectEnv, useUpsertProjectEnv } from '@/modules/project-env/project-env-hooks';
import type { ProjectEnvHelperSnippet } from '@/modules/project-env/project-env-types';

function helperSnippetToText(snippet: ProjectEnvHelperSnippet) {
  return Object.entries(snippet.env)
    .sort(([left], [right]) => left.localeCompare(right))
    .map(([key, value]) => `${key}=${value}`)
    .join('\n');
}

export default function ProjectEnvPage() {
  const params = useParams();
  const projectId = params?.projectId as string;
  const fileInputRef = useRef<HTMLInputElement | null>(null);
  const { data, isLoading, isError } = useProjectEnv(projectId);
  const saveMutation = useUpsertProjectEnv(projectId);
  const clearMutation = useDeleteProjectEnv(projectId);
  const [draftContent, setDraftContent] = useState('');
  const [copiedKey, setCopiedKey] = useState('');

  useEffect(() => {
    if (!copiedKey) {
      return;
    }
    const timer = window.setTimeout(() => setCopiedKey(''), 1500);
    return () => window.clearTimeout(timer);
  }, [copiedKey]);

  const saveError = (saveMutation.error as Error | null)?.message;
  const clearError = (clearMutation.error as Error | null)?.message;
  const combinedError = saveError || clearError;
  const keys = data?.keys ?? [];
  const helperSnippets = data?.helper_snippets ?? [];
  const canSave = draftContent.trim().length > 0 && !saveMutation.isPending;
  const envSummary = useMemo(() => {
    if (!data?.configured) {
      return 'Chưa có env bundle nào được lưu cho project này.';
    }
    return `Đã lưu ${keys.length} biến runtime cho standalone deploy.`;
  }, [data?.configured, keys.length]);

  if (isLoading) {
    return <SkeletonPage title cards={2} />;
  }

  if (isError) {
    return (
      <div className="flex flex-col gap-6">
        <PageHeader title="Biến môi trường" subtitle="Quản lý runtime env cho standalone project." />
        <ErrorState title="Không thể tải dữ liệu" message="Vui lòng thử lại sau." />
      </div>
    );
  }

  return (
    <div className="flex flex-col gap-6">
      <PageHeader
        title="Biến môi trường"
        subtitle="Paste hoặc upload raw .env để LazyOps inject vào app container trong lần deploy kế tiếp. Đây là runtime env, không phải build-time env."
        actions={
          <Link
            href={`/projects/${projectId}`}
            className="rounded-lg border border-lazyops-border px-4 py-2 text-sm font-semibold text-lazyops-text transition-colors hover:bg-lazyops-border/10"
          >
            Quay lại dự án
          </Link>
        }
      />

      <SectionCard title="Env bundle" description={envSummary}>
        <div className="flex flex-col gap-4">
          <div className="rounded-xl border border-[#1e293b] bg-[#0B1120]/40 p-4 text-sm text-lazyops-muted">
            <p className="font-semibold text-lazyops-text">Lưu ý</p>
            <p className="mt-2">
              Giá trị đã lưu sẽ không được trả lại ở UI. Muốn cập nhật, hãy paste toàn bộ file <code className="rounded bg-lazyops-border/20 px-1.5 py-0.5">.env</code> mới để thay thế.
            </p>
          </div>

          <textarea
            value={draftContent}
            onChange={(event) => setDraftContent(event.target.value)}
            placeholder={'APP_ENV=production\nAPI_BASE_URL=https://api.example.com'}
            className="min-h-[220px] w-full rounded-xl border border-[#1e293b] bg-[#0B1120]/40 px-4 py-3 font-mono text-sm text-white outline-none transition-all placeholder:text-[#64748b]/60 focus:border-[#0EA5E9]/50 focus:ring-4 focus:ring-[#0EA5E9]/10"
          />

          <input
            ref={fileInputRef}
            type="file"
            accept=".env,text/plain"
            className="hidden"
            onChange={async (event) => {
              const file = event.target.files?.[0];
              if (!file) {
                return;
              }
              const text = await file.text();
              setDraftContent(text);
              event.target.value = '';
            }}
          />

          {combinedError && (
            <div className="rounded-lg border border-health-unhealthy/30 bg-health-unhealthy/10 px-3 py-2 text-xs text-health-unhealthy">
              {combinedError}
            </div>
          )}

          <div className="flex flex-col gap-3 sm:flex-row sm:justify-end">
            <button
              type="button"
              className="rounded-xl border border-lazyops-border px-4 py-3 text-sm font-semibold text-lazyops-text transition-colors hover:bg-lazyops-border/10"
              onClick={() => fileInputRef.current?.click()}
            >
              Upload .env
            </button>
            <button
              type="button"
              className="rounded-xl border border-lazyops-border px-4 py-3 text-sm font-semibold text-lazyops-text transition-colors hover:bg-lazyops-border/10 disabled:cursor-not-allowed disabled:opacity-50"
              onClick={() => {
                void clearMutation.mutateAsync().then(() => setDraftContent(''));
              }}
              disabled={!data?.configured || clearMutation.isPending}
            >
              {clearMutation.isPending ? 'Đang xóa...' : 'Xóa bundle'}
            </button>
            <div className="sm:w-48">
              <FormButton
                type="button"
                loading={saveMutation.isPending}
                disabled={!canSave}
                onClick={() => {
                  void saveMutation.mutateAsync({ content: draftContent.trim() }).then(() => {
                    setDraftContent('');
                  });
                }}
              >
                Lưu env bundle
              </FormButton>
            </div>
          </div>
        </div>
      </SectionCard>

      <SectionCard title="Metadata đã lưu" description="UI chỉ hiển thị preview an toàn, không trả lại plaintext secret values.">
        <div className="grid gap-4 lg:grid-cols-[1.1fr_0.9fr]">
          <div className="rounded-xl border border-[#1e293b] bg-[#0B1120]/30 p-4">
            <div className="flex items-center justify-between">
              <h3 className="text-sm font-semibold text-white">Danh sách keys</h3>
              <span className="text-xs text-lazyops-muted">{keys.length} keys</span>
            </div>
            <div className="mt-3 flex flex-wrap gap-2">
              {keys.length > 0 ? keys.map((key) => (
                <span key={key} className="rounded-full border border-[#1e293b] bg-[#111827] px-3 py-1 text-xs font-medium text-[#cbd5e1]">
                  {key}
                </span>
              )) : (
                <p className="text-sm text-lazyops-muted">Chưa có env bundle nào được lưu.</p>
              )}
            </div>
          </div>

          <div className="rounded-xl border border-[#1e293b] bg-[#0B1120]/30 p-4">
            <h3 className="text-sm font-semibold text-white">Thông tin lưu trữ</h3>
            <dl className="mt-3 space-y-2 text-sm text-lazyops-muted">
              <div className="flex items-start justify-between gap-4">
                <dt>Configured</dt>
                <dd className="text-right text-lazyops-text">{data?.configured ? 'Có' : 'Chưa'}</dd>
              </div>
              <div className="flex items-start justify-between gap-4">
                <dt>Updated</dt>
                <dd className="text-right text-lazyops-text">{data?.updated_at ?? 'Chưa có'}</dd>
              </div>
              <div className="flex items-start justify-between gap-4">
                <dt>Fingerprint</dt>
                <dd className="max-w-[220px] break-all text-right font-mono text-xs text-lazyops-text">{data?.fingerprint ?? 'Chưa có'}</dd>
              </div>
            </dl>
          </div>
        </div>

        <div className="mt-4 rounded-xl border border-[#1e293b] bg-[#0B1120]/30 p-4">
          <h3 className="text-sm font-semibold text-white">Parse warnings</h3>
          <div className="mt-3 space-y-2 text-sm text-lazyops-muted">
            {data?.parse_warnings.length ? data.parse_warnings.map((warning) => (
              <div key={warning} className="rounded-lg border border-[#334155] bg-[#111827] px-3 py-2">
                {warning}
              </div>
            )) : (
              <p>Không có warning nào.</p>
            )}
          </div>
        </div>
      </SectionCard>

      <SectionCard title="Helper snippets" description="LazyOps chỉ điền helper values cho các key còn thiếu. Nếu bạn đã tự khai báo DB_HOST hoặc tương tự, LazyOps sẽ giữ nguyên giá trị của bạn.">
        <div className="grid gap-4 lg:grid-cols-2">
          {helperSnippets.length > 0 ? helperSnippets.map((snippet) => {
            const snippetText = helperSnippetToText(snippet);
            const copyKey = `${snippet.service_kind}:${snippet.alias}`;
            return (
              <div key={copyKey} className="rounded-xl border border-[#1e293b] bg-[#0B1120]/30 p-4">
                <div className="flex items-center justify-between gap-3">
                  <div>
                    <h3 className="text-sm font-semibold text-white">{snippet.alias}</h3>
                    <p className="text-xs uppercase tracking-wider text-lazyops-muted">{snippet.service_kind}</p>
                  </div>
                  <button
                    type="button"
                    className="rounded-lg border border-lazyops-border px-3 py-2 text-xs font-semibold text-lazyops-text transition-colors hover:bg-lazyops-border/10"
                    onClick={async () => {
                      await navigator.clipboard.writeText(snippetText);
                      setCopiedKey(copyKey);
                    }}
                  >
                    {copiedKey === copyKey ? 'Đã copy' : 'Copy'}
                  </button>
                </div>
                <pre className="mt-3 overflow-x-auto rounded-xl bg-[#020617] px-4 py-3 text-xs text-[#cbd5e1]">
                  <code>{snippetText}</code>
                </pre>
              </div>
            );
          }) : (
            <div className="rounded-xl border border-dashed border-[#334155] bg-[#0B1120]/20 p-6 text-sm text-lazyops-muted lg:col-span-2">
              Chưa có internal service nào được cấu hình cho project này.
            </div>
          )}
        </div>
      </SectionCard>
    </div>
  );
}
