'use client';

import { useEffect, useMemo, useRef, useState } from 'react';
import Link from 'next/link';
import { useParams } from 'next/navigation';
import { PageHeader } from '@/components/primitives/page-header';
import { SectionCard } from '@/components/primitives/section-card';
import { ErrorState } from '@/components/primitives/error-state';
import { SkeletonPage } from '@/components/primitives/skeleton';
import { FormButton } from '@/components/forms/form-fields';
import { useProjects } from '@/modules/projects/project-hooks';
import { useDeleteProjectEnv, useProjectEnv, useUpsertProjectEnv } from '@/modules/project-env/project-env-hooks';
import type { ProjectEnvHelperPack } from '@/modules/project-env/project-env-types';

function envMapToText(value: Record<string, string>) {
  return Object.entries(value)
    .sort(([left], [right]) => left.localeCompare(right))
    .map(([key, value]) => `${key}=${value}`)
    .join('\n');
}

function runtimeKeysToText(keys: string[]) {
  return keys.map((key) => `${key} (auto-injected)`).join('\n');
}

function helperPackCopyText(pack: ProjectEnvHelperPack, mode: 'env' | 'placeholder' | 'local') {
  if (mode === 'placeholder') {
    return envMapToText(pack.placeholder_env);
  }
  if (mode === 'local') {
    return envMapToText(pack.local_example_env);
  }
  return envMapToText(pack.env_example);
}

export default function ProjectEnvPage() {
  const params = useParams();
  const projectId = params?.projectId as string;
  const fileInputRef = useRef<HTMLInputElement | null>(null);
  const projects = useProjects();
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
  const project = (projects.data?.items ?? []).find((item) => item.id === projectId) ?? null;
  const runtimeMode = project?.runtime_mode || 'distributed-k3s';
  const usesK3sRuntime = runtimeMode === 'distributed-k3s';
  const userKeys = data?.user_keys ?? [];
  const managedKeys = data?.managed_keys ?? [];
  const provisionedKeys = data?.provisioned_keys ?? [];
  const helperPacks = data?.helper_packs ?? [];
  const canSave = draftContent.trim().length > 0 && !saveMutation.isPending;
  const envSummary = useMemo(() => {
    if (!data?.configured) {
      return 'Chưa có env bundle nào được lưu cho project này.';
    }
    return `Đã lưu ${userKeys.length} biến user-defined. Hệ thống đang quản lý ${managedKeys.length} key runtime mặc định.`;
  }, [data?.configured, managedKeys.length, userKeys.length]);

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
        subtitle={
          usesK3sRuntime
            ? 'Paste hoặc upload raw .env để LazyOps inject vào service trong lần deploy kế tiếp. Với K3s, helper snippets ben duoi se uu tien internal DNS thay vi localhost.'
            : 'Paste hoặc upload raw .env để LazyOps inject vào app container trong lần deploy kế tiếp. Đây là runtime env, không phải build-time env.'
        }
        actions={
          <Link
            href={`/projects/${projectId}`}
            className="rounded-lg border border-lazyops-border px-6 py-2 text-base font-semibold text-lazyops-text transition-colors hover:bg-lazyops-border/10"
          >
            Quay lại dự án
          </Link>
        }
      />

      <SectionCard
        title="Env bundle"
        description={
          usesK3sRuntime
            ? `${envSummary} Project nay dang chay service-first tren K3s, nen env helper se noi theo service DNS noi bo.`
            : envSummary
        }
      >
        <div className="flex flex-col gap-4">
          <div className="rounded-xl border border-[#1e293b] bg-[#0B1120]/40 p-6 text-base text-lazyops-muted">
            <p className="font-semibold text-lazyops-text">Lưu ý</p>
            <p className="mt-2">
              Giá trị đã lưu sẽ không được trả lại ở UI. Muốn cập nhật, hãy paste toàn bộ file <code className="rounded bg-lazyops-border/20 px-1.5 py-0.5">.env</code> mới để thay thế.
            </p>
          </div>

          <textarea
            value={draftContent}
            onChange={(event) => setDraftContent(event.target.value)}
            placeholder={'APP_ENV=production\nAPI_BASE_URL=https://api.example.com'}
            className="min-h-[220px] w-full rounded-xl border border-[#1e293b] bg-[#0B1120]/40 px-6 py-3 font-mono text-base text-white outline-none transition-all placeholder:text-[#64748b]/60 focus:border-[#0EA5E9]/50 focus:ring-4 focus:ring-[#0EA5E9]/10"
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
            <div className="rounded-lg border border-health-unhealthy/30 bg-health-unhealthy/10 px-3 py-2 text-sm text-health-unhealthy">
              {combinedError}
            </div>
          )}

          <div className="flex flex-col gap-3 sm:flex-row sm:justify-end">
            <button
              type="button"
              className="rounded-xl border border-lazyops-border px-6 py-3 text-base font-semibold text-lazyops-text transition-colors hover:bg-lazyops-border/10"
              onClick={() => fileInputRef.current?.click()}
            >
              Upload .env
            </button>
            <button
              type="button"
              className="rounded-xl border border-lazyops-border px-6 py-3 text-base font-semibold text-lazyops-text transition-colors hover:bg-lazyops-border/10 disabled:cursor-not-allowed disabled:opacity-50"
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
          <div className="rounded-xl border border-[#1e293b] bg-[#0B1120]/30 p-6">
            <div className="flex items-center justify-between">
              <h3 className="text-base font-semibold text-white">User-defined keys</h3>
              <span className="text-sm text-lazyops-muted">{userKeys.length} keys</span>
            </div>
            <div className="mt-3 flex flex-wrap gap-2">
              {userKeys.length > 0 ? userKeys.map((key) => (
                <span key={key} className="rounded-full border border-[#1e293b] bg-[#111827] px-3 py-1 text-sm font-medium text-[#cbd5e1]">
                  {key}
                </span>
              )) : (
                <p className="text-base text-lazyops-muted">Chưa có env bundle nào được lưu.</p>
              )}
            </div>
          </div>

          <div className="rounded-xl border border-[#1e293b] bg-[#0B1120]/30 p-6">
            <h3 className="text-base font-semibold text-white">Thông tin lưu trữ</h3>
            <dl className="mt-3 space-y-2 text-base text-lazyops-muted">
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
                <dd className="max-w-[220px] break-all text-right font-mono text-sm text-lazyops-text">{data?.fingerprint ?? 'Chưa có'}</dd>
              </div>
            </dl>
          </div>
        </div>

        <div className="mt-4 grid gap-4 lg:grid-cols-2">
          <div className="rounded-xl border border-[#1e293b] bg-[#0B1120]/30 p-6">
            <div className="flex items-center justify-between">
              <h3 className="text-base font-semibold text-white">Managed keys</h3>
              <span className="text-sm text-lazyops-muted">{managedKeys.length} keys</span>
            </div>
            <div className="mt-3 flex flex-wrap gap-2">
              {managedKeys.length > 0 ? managedKeys.map((key) => (
                <span key={key} className="rounded-full border border-emerald-500/30 bg-emerald-500/10 px-3 py-1 text-sm font-medium text-emerald-300">
                  {key}
                </span>
              )) : (
                <p className="text-base text-lazyops-muted">Chưa có key managed nào.</p>
              )}
            </div>
          </div>

          <div className="rounded-xl border border-[#1e293b] bg-[#0B1120]/30 p-6">
            <div className="flex items-center justify-between">
              <h3 className="text-base font-semibold text-white">Provisioned on deploy</h3>
              <span className="text-sm text-lazyops-muted">{provisionedKeys.length} keys</span>
            </div>
            <p className="mt-2 text-sm text-lazyops-muted">
              Đây là các key LazyOps sẽ tự inject ở runtime. Nếu user đã tự khai báo cùng key trong env bundle thì key đó sẽ không nằm trong danh sách này.
            </p>
            <div className="mt-3 flex flex-wrap gap-2">
              {provisionedKeys.length > 0 ? provisionedKeys.map((key) => (
                <span key={key} className="rounded-full border border-sky-500/30 bg-sky-500/10 px-3 py-1 text-sm font-medium text-sky-300">
                  {key}
                </span>
              )) : (
                <p className="text-base text-lazyops-muted">Chưa có key auto-injected nào.</p>
              )}
            </div>
          </div>
        </div>

        <div className="mt-4 rounded-xl border border-[#1e293b] bg-[#0B1120]/30 p-6">
          <h3 className="text-base font-semibold text-white">Parse warnings</h3>
          <div className="mt-3 space-y-2 text-base text-lazyops-muted">
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

      <SectionCard
        title="Managed env guidance"
        description={
          usesK3sRuntime
            ? 'LazyOps hiển thị placeholder an toàn, local examples, và danh sách runtime keys sẽ được auto-inject. Gia tri that khong duoc tra ve UI.'
            : 'LazyOps hiển thị placeholder an toàn và local examples. Runtime keys do hệ thống quản lý chỉ được materialize khi deploy.'
        }
      >
        <div className="grid gap-4 lg:grid-cols-2">
          {helperPacks.length > 0 ? helperPacks.map((pack) => {
            const copyKey = `${pack.category}:${pack.alias}`;
            return (
              <div key={copyKey} className="rounded-xl border border-[#1e293b] bg-[#0B1120]/30 p-6">
                <div className="flex items-center justify-between gap-3">
                  <div>
                    <div className="flex flex-wrap items-center gap-2">
                      <h3 className="text-base font-semibold text-white">{pack.alias}</h3>
                      <span className={`rounded-full px-2 py-0.5 text-xs font-semibold ${pack.managed ? 'bg-emerald-500/10 text-emerald-300' : 'bg-slate-500/10 text-slate-300'}`}>
                        {pack.managed ? 'Managed' : 'Guidance'}
                      </span>
                      {pack.runtime_injected ? (
                        <span className="rounded-full bg-sky-500/10 px-2 py-0.5 text-xs font-semibold text-sky-300">
                          Auto-injected
                        </span>
                      ) : null}
                    </div>
                    <p className="text-sm uppercase tracking-wider text-lazyops-muted">
                      {pack.category} · {pack.audience} · {pack.service_kind}
                    </p>
                    {pack.public_path ? (
                      <p className="mt-1 text-sm text-lazyops-muted">Effective public path: <code className="rounded bg-[#020617] px-1.5 py-0.5 text-[#cbd5e1]">{pack.public_path}</code></p>
                    ) : null}
                  </div>
                </div>

                <div className="mt-4 space-y-4">
                  <div>
                    <div className="mb-2 flex items-center justify-between">
                      <h4 className="text-sm font-semibold text-white">Runtime managed keys</h4>
                      <button
                        type="button"
                        className="rounded-lg border border-lazyops-border px-3 py-2 text-sm font-semibold text-lazyops-text transition-colors hover:bg-lazyops-border/10"
                        onClick={async () => {
                          await navigator.clipboard.writeText(runtimeKeysToText(pack.runtime_keys));
                          setCopiedKey(`${copyKey}:runtime`);
                        }}
                      >
                        {copiedKey === `${copyKey}:runtime` ? 'Đã copy' : 'Copy'}
                      </button>
                    </div>
                    <pre className="overflow-x-auto rounded-xl bg-[#020617] px-6 py-3 text-sm text-[#cbd5e1]"><code>{runtimeKeysToText(pack.runtime_keys) || 'No runtime keys'}</code></pre>
                  </div>

                  <div>
                    <div className="mb-2 flex items-center justify-between">
                      <h4 className="text-sm font-semibold text-white">.env.example</h4>
                      <button
                        type="button"
                        className="rounded-lg border border-lazyops-border px-3 py-2 text-sm font-semibold text-lazyops-text transition-colors hover:bg-lazyops-border/10"
                        onClick={async () => {
                          await navigator.clipboard.writeText(helperPackCopyText(pack, 'env'));
                          setCopiedKey(`${copyKey}:env`);
                        }}
                      >
                        {copiedKey === `${copyKey}:env` ? 'Đã copy' : 'Copy'}
                      </button>
                    </div>
                    <pre className="overflow-x-auto rounded-xl bg-[#020617] px-6 py-3 text-sm text-[#cbd5e1]"><code>{helperPackCopyText(pack, 'env')}</code></pre>
                  </div>

                  <div>
                    <div className="mb-2 flex items-center justify-between">
                      <h4 className="text-sm font-semibold text-white">Config placeholder</h4>
                      <button
                        type="button"
                        className="rounded-lg border border-lazyops-border px-3 py-2 text-sm font-semibold text-lazyops-text transition-colors hover:bg-lazyops-border/10"
                        onClick={async () => {
                          await navigator.clipboard.writeText(helperPackCopyText(pack, 'placeholder'));
                          setCopiedKey(`${copyKey}:placeholder`);
                        }}
                      >
                        {copiedKey === `${copyKey}:placeholder` ? 'Đã copy' : 'Copy'}
                      </button>
                    </div>
                    <pre className="overflow-x-auto rounded-xl bg-[#020617] px-6 py-3 text-sm text-[#cbd5e1]"><code>{helperPackCopyText(pack, 'placeholder')}</code></pre>
                  </div>

                  <div>
                    <div className="mb-2 flex items-center justify-between">
                      <h4 className="text-sm font-semibold text-white">Local dev example</h4>
                      <button
                        type="button"
                        className="rounded-lg border border-lazyops-border px-3 py-2 text-sm font-semibold text-lazyops-text transition-colors hover:bg-lazyops-border/10"
                        onClick={async () => {
                          await navigator.clipboard.writeText(helperPackCopyText(pack, 'local'));
                          setCopiedKey(`${copyKey}:local`);
                        }}
                      >
                        {copiedKey === `${copyKey}:local` ? 'Đã copy' : 'Copy'}
                      </button>
                    </div>
                    <pre className="overflow-x-auto rounded-xl bg-[#020617] px-6 py-3 text-sm text-[#cbd5e1]"><code>{helperPackCopyText(pack, 'local')}</code></pre>
                  </div>

                  {pack.notes.length > 0 ? (
                    <div className="rounded-xl border border-[#1e293b] bg-[#020617] px-4 py-3">
                      {pack.notes.map((note) => (
                        <p key={note} className="text-sm text-lazyops-muted">{note}</p>
                      ))}
                    </div>
                  ) : null}

                  <div className="space-y-3">
                    {pack.language_snippets.map((snippet) => (
                      <div key={`${copyKey}:${snippet.language}:${snippet.title}`} className="rounded-xl border border-[#1e293b] bg-[#020617] p-4">
                        <div className="mb-2 flex items-center justify-between gap-3">
                          <div>
                            <h4 className="text-sm font-semibold text-white">{snippet.title}</h4>
                            <p className="text-xs uppercase tracking-wide text-lazyops-muted">{snippet.language} · {snippet.framework} · {snippet.kind}</p>
                          </div>
                          <button
                            type="button"
                            className="rounded-lg border border-lazyops-border px-3 py-2 text-sm font-semibold text-lazyops-text transition-colors hover:bg-lazyops-border/10"
                            onClick={async () => {
                              await navigator.clipboard.writeText(snippet.content);
                              setCopiedKey(`${copyKey}:${snippet.language}:${snippet.kind}`);
                            }}
                          >
                            {copiedKey === `${copyKey}:${snippet.language}:${snippet.kind}` ? 'Đã copy' : 'Copy'}
                          </button>
                        </div>
                        <pre className="overflow-x-auto text-sm text-[#cbd5e1]"><code>{snippet.content}</code></pre>
                      </div>
                    ))}
                  </div>
                </div>
              </div>
            );
          }) : (
            <div className="rounded-xl border border-dashed border-[#334155] bg-[#0B1120]/20 p-6 text-base text-lazyops-muted lg:col-span-2">
              Chưa có helper pack nào được cấu hình cho project này.
            </div>
          )}
        </div>
      </SectionCard>
    </div>
  );
}
