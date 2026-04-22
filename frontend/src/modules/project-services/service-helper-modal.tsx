'use client';

import { useEffect, useMemo, useState, type ReactNode } from 'react';
import { Database, Globe, KeyRound, Network, Sparkles } from 'lucide-react';
import { Modal } from '@/components/primitives/modal';
import { ProjectAIPromptCard } from '@/modules/project-ai-prompt/project-ai-prompt-card';
import { useProjectAIPrompt } from '@/modules/project-ai-prompt/project-ai-prompt-hooks';
import { useProjectEnv } from '@/modules/project-env/project-env-hooks';
import type { ProjectEnvHelperPack, ProjectEnvHelperSnippet } from '@/modules/project-env/project-env-types';
import type { ProjectService } from '@/modules/project-services/project-service-types';

type ServiceHelperModalProps = {
  open: boolean;
  onClose: () => void;
  onComplete: () => void;
  projectId: string;
  services: ProjectService[];
};

const LANGUAGE_TABS: Array<{ key: ProjectEnvHelperSnippet['language']; label: string }> = [
  { key: 'nodejs', label: 'Node.js' },
  { key: 'python', label: 'Python' },
  { key: 'java', label: 'Java/Spring' },
  { key: 'csharp', label: 'C#' },
  { key: 'go', label: 'Go' },
  { key: 'php', label: 'PHP' },
];

function envMapToText(value: Record<string, string>) {
  return Object.entries(value)
    .sort(([left], [right]) => left.localeCompare(right))
    .map(([key, itemValue]) => `${key}=${itemValue}`)
    .join('\n');
}

function runtimeKeysToText(keys: string[]) {
  return keys.map((key) => `${key} (auto-injected)`).join('\n');
}

function helperPackCopyText(pack: ProjectEnvHelperPack, mode: 'env' | 'placeholder' | 'local' | 'runtime') {
  if (mode === 'placeholder') {
    return envMapToText(pack.placeholder_env);
  }
  if (mode === 'local') {
    return envMapToText(pack.local_example_env);
  }
  if (mode === 'runtime') {
    return runtimeKeysToText(pack.provisioned_keys.length > 0 ? pack.provisioned_keys : pack.runtime_keys);
  }
  return envMapToText(pack.env_example);
}

function matchesService(pack: ProjectEnvHelperPack, serviceName: string) {
  if (pack.source_service === serviceName) {
    return true;
  }
  return pack.related_services.includes(serviceName);
}

export function ServiceHelperModal({ open, onClose, onComplete, projectId, services }: ServiceHelperModalProps) {
  const env = useProjectEnv(projectId);
  const prompt = useProjectAIPrompt(projectId);
  const [activeTab, setActiveTab] = useState<string>('');
  const [activeLanguage, setActiveLanguage] = useState<ProjectEnvHelperSnippet['language']>('nodejs');
  const [copiedKey, setCopiedKey] = useState('');

  useEffect(() => {
    if (!copiedKey) {
      return;
    }
    const timer = window.setTimeout(() => setCopiedKey(''), 1500);
    return () => window.clearTimeout(timer);
  }, [copiedKey]);

  useEffect(() => {
    if (!open) {
      return;
    }
    if (!activeTab) {
      setActiveTab(services[0]?.name ?? 'prompt');
      return;
    }
    if (activeTab !== 'prompt' && !services.some((service) => service.name === activeTab)) {
      setActiveTab(services[0]?.name ?? 'prompt');
    }
  }, [activeTab, open, services]);

  const helperPacks = env.data?.helper_packs ?? [];
  const sourceSections = prompt.data?.source_sections ?? [];
  const activeService = activeTab === 'prompt' ? null : services.find((service) => service.name === activeTab) ?? null;
  const activeServiceSnapshot = prompt.data?.service_snapshot.find((item) => item.name === activeService?.name);

  const serviceHelpers = useMemo(() => {
    if (!activeService) {
      return [];
    }
    return helperPacks.filter((pack) => matchesService(pack, activeService.name));
  }, [activeService, helperPacks]);

  const servicePublicPaths = useMemo(() => {
    if (!activeService) {
      return [];
    }
    return prompt.data?.effective_public_paths.filter((item) => item.service === activeService.name) ?? [];
  }, [activeService, prompt.data?.effective_public_paths]);

  return (
    <Modal open={open} onClose={onClose} title="Helper cấu hình dịch vụ" size="full">
      <div className="flex h-full min-h-[72vh] flex-col gap-6">
        <div className="rounded-2xl border border-[#1e293b] bg-[#0B1120]/40 p-6 text-base text-[#cbd5e1]">
          Đây là workspace hướng dẫn sau khi bạn đã định nghĩa service. Bạn có thể copy nhanh `.env.example`, placeholder config, snippet theo ngôn ngữ, hoặc dùng luôn AI prompt tổng hợp ở tab cuối.
          {sourceSections.length > 0 ? (
            <div className="mt-4 grid gap-3 md:grid-cols-4">
              {sourceSections.map((section) => (
                <div key={section.key} className="rounded-xl border border-[#1e293b] bg-[#020617] px-4 py-3">
                  <div className="text-sm font-semibold text-white">{section.title}</div>
                  <div className="mt-1 text-2xl font-semibold text-[#38BDF8]">{section.item_count}</div>
                  <div className="mt-1 text-xs text-[#94a3b8]">{section.description}</div>
                </div>
              ))}
            </div>
          ) : null}
        </div>

        <div className="flex flex-wrap gap-3 border-b border-[#1e293b] pb-4">
          {services.map((service) => (
            <button
              key={service.id}
              type="button"
              onClick={() => setActiveTab(service.name)}
              className={`rounded-xl px-4 py-2 text-sm font-semibold transition-colors ${
                activeTab === service.name
                  ? 'bg-[#0EA5E9]/20 text-[#38bdf8]'
                  : 'border border-[#334155] text-[#cbd5e1] hover:bg-[#111827]'
              }`}
            >
              {service.name}
            </button>
          ))}
          <button
            type="button"
            onClick={() => setActiveTab('prompt')}
            className={`rounded-xl px-4 py-2 text-sm font-semibold transition-colors ${
              activeTab === 'prompt'
                ? 'bg-[#0EA5E9]/20 text-[#38bdf8]'
                : 'border border-[#334155] text-[#cbd5e1] hover:bg-[#111827]'
            }`}
          >
            Prompt
          </button>
        </div>

        {activeTab === 'prompt' ? (
          <div className="min-h-0 flex-1 overflow-y-auto">
            <ProjectAIPromptCard
              projectId={projectId}
              contextLabel="Service helper popup"
              title="Prompt tổng hợp cho AI"
              description="Copy prompt đầy đủ để đưa cho ChatGPT, Codex hoặc Gemini xử lý luôn phần env/config/routing của cả project."
            />
          </div>
        ) : null}

        {activeService ? (
          <div className="flex min-h-0 flex-1 flex-col gap-5 overflow-hidden">
            <div className="grid gap-4 lg:grid-cols-[1.2fr_0.8fr]">
              <div className="rounded-2xl border border-[#1e293b] bg-[#0B1120]/30 p-6">
                <h3 className="text-lg font-semibold text-white">{activeService.name}</h3>
                <div className="mt-3 grid gap-3 md:grid-cols-2">
                  <SummaryLine icon={<Network className="size-4 text-[#38bdf8]" />} label="Loại service" value={formatServiceKind(activeService)} />
                  <SummaryLine icon={<Database className="size-4 text-[#38bdf8]" />} label="DB target" value={formatDatabaseTarget(activeService)} />
                  <SummaryLine
                    icon={<Globe className="size-4 text-[#38bdf8]" />}
                    label="Public path"
                    value={servicePublicPaths.length > 0 ? servicePublicPaths.map((item) => item.path).join(', ') : activeService.public ? 'Đang chờ routing' : 'Chỉ nội bộ'}
                  />
                  <SummaryLine
                    icon={<Sparkles className="size-4 text-[#38bdf8]" />}
                    label="Role prompt"
                    value={activeServiceSnapshot?.role || 'service'}
                  />
                </div>
              </div>

              <div className="rounded-2xl border border-[#1e293b] bg-[#0B1120]/30 p-6">
                <h3 className="text-base font-semibold text-white">Ngôn ngữ mục tiêu</h3>
                <div className="mt-4 flex flex-wrap gap-3">
                  {LANGUAGE_TABS.map((language) => (
                    <button
                      key={language.key}
                      type="button"
                      onClick={() => setActiveLanguage(language.key)}
                      className={`rounded-xl px-4 py-2 text-sm font-semibold transition-colors ${
                        activeLanguage === language.key
                          ? 'bg-[#0EA5E9]/20 text-[#38bdf8]'
                          : 'border border-[#334155] text-[#cbd5e1] hover:bg-[#111827]'
                      }`}
                    >
                      {language.label}
                    </button>
                  ))}
                </div>
              </div>
            </div>

            <div className="min-h-0 flex-1 overflow-y-auto">
              {env.isLoading ? (
                <div className="rounded-2xl border border-[#1e293b] bg-[#0B1120]/30 p-6 text-[#94a3b8]">Đang tải helper...</div>
              ) : serviceHelpers.length === 0 ? (
                <div className="rounded-2xl border border-dashed border-[#334155] bg-[#0B1120]/20 p-8 text-[#94a3b8]">
                  Service này chưa có helper pack riêng. Nếu đây là app repo chưa nối DB hoặc chưa cấu hình public path, bạn vẫn có thể dùng tab Prompt để AI gợi ý thay đổi toàn project.
                </div>
              ) : (
                <div className="grid gap-4">
                  {serviceHelpers.map((pack) => {
                    const snippets = pack.language_snippets.filter((item) => item.language === activeLanguage);
                    return (
                      <div key={`${pack.source_service}-${pack.category}-${pack.alias}`} className="rounded-2xl border border-[#1e293b] bg-[#0B1120]/30 p-6">
                        <div className="flex flex-col gap-3 lg:flex-row lg:items-start lg:justify-between">
                          <div>
                            <div className="text-sm font-semibold uppercase tracking-[0.12em] text-[#64748b]">{pack.category}</div>
                            <h3 className="mt-1 text-lg font-semibold text-white">{pack.primary_key}</h3>
                            <p className="mt-2 text-sm text-[#94a3b8]">
                              Audience: {pack.audience} · Source: {pack.source_service}
                              {pack.public_path ? ` · Path: ${pack.public_path}` : ''}
                            </p>
                          </div>
                          <div className="flex flex-wrap gap-2">
                            {pack.managed ? (
                              <span className="rounded-full border border-emerald-500/30 bg-emerald-500/10 px-3 py-1 text-xs font-semibold text-emerald-300">
                                managed
                              </span>
                            ) : null}
                            {pack.runtime_injected ? (
                              <span className="rounded-full border border-sky-500/30 bg-sky-500/10 px-3 py-1 text-xs font-semibold text-sky-300">
                                auto-injected
                              </span>
                            ) : null}
                          </div>
                        </div>

                        {pack.notes.length > 0 ? (
                          <div className="mt-4 space-y-2 rounded-2xl border border-[#1e293b] bg-[#020617] p-4">
                            {pack.notes.map((note) => (
                              <p key={note} className="text-sm text-[#cbd5e1]">
                                {note}
                              </p>
                            ))}
                          </div>
                        ) : null}

                        <div className="mt-4 grid gap-4 xl:grid-cols-2">
                          <CopyBlock
                            title="Runtime managed keys"
                            icon={<KeyRound className="size-4 text-[#38bdf8]" />}
                            content={helperPackCopyText(pack, 'runtime')}
                            copyKey={`${pack.alias}-runtime`}
                            copiedKey={copiedKey}
                            setCopiedKey={setCopiedKey}
                            emptyText="Không có key auto-injected cho helper này."
                          />
                          <CopyBlock
                            title=".env.example"
                            icon={<KeyRound className="size-4 text-[#38bdf8]" />}
                            content={helperPackCopyText(pack, 'env')}
                            copyKey={`${pack.alias}-env`}
                            copiedKey={copiedKey}
                            setCopiedKey={setCopiedKey}
                          />
                          <CopyBlock
                            title="Config placeholder"
                            icon={<KeyRound className="size-4 text-[#38bdf8]" />}
                            content={helperPackCopyText(pack, 'placeholder')}
                            copyKey={`${pack.alias}-placeholder`}
                            copiedKey={copiedKey}
                            setCopiedKey={setCopiedKey}
                          />
                          <CopyBlock
                            title="Local dev example"
                            icon={<KeyRound className="size-4 text-[#38bdf8]" />}
                            content={helperPackCopyText(pack, 'local')}
                            copyKey={`${pack.alias}-local`}
                            copiedKey={copiedKey}
                            setCopiedKey={setCopiedKey}
                          />
                        </div>

                        <div className="mt-4 grid gap-4">
                          {snippets.length > 0 ? snippets.map((snippet, index) => (
                            <CopyBlock
                              key={`${snippet.language}-${snippet.framework}-${index}`}
                              title={snippet.title}
                              icon={<Sparkles className="size-4 text-[#38bdf8]" />}
                              content={snippet.content}
                              copyKey={`${pack.alias}-snippet-${snippet.language}-${index}`}
                              copiedKey={copiedKey}
                              setCopiedKey={setCopiedKey}
                            />
                          )) : (
                            <div className="rounded-2xl border border-dashed border-[#334155] bg-[#0B1120]/20 p-6 text-sm text-[#94a3b8]">
                              Chưa có snippet riêng cho ngôn ngữ này trong helper pack hiện tại.
                            </div>
                          )}
                        </div>
                      </div>
                    );
                  })}
                </div>
              )}
            </div>
          </div>
        ) : null}

        <div className="flex flex-col-reverse gap-3 border-t border-[#1e293b] pt-4 sm:flex-row sm:justify-end">
          <button
            type="button"
            onClick={onClose}
            className="rounded-xl border border-[#334155] px-5 py-2.5 text-sm font-semibold text-[#cbd5e1] transition-colors hover:bg-[#111827]"
          >
            Đóng
          </button>
          <button
            type="button"
            onClick={onComplete}
            className="rounded-xl border border-[#0EA5E9] bg-[#0EA5E9]/10 px-5 py-2.5 text-sm font-semibold text-[#38bdf8] transition-colors hover:bg-[#0EA5E9]/20"
          >
            Đã thực hiện xong
          </button>
        </div>
      </div>
    </Modal>
  );
}

function CopyBlock({
  title,
  icon,
  content,
  copyKey,
  copiedKey,
  setCopiedKey,
  emptyText = 'Chưa có nội dung.',
}: {
  title: string;
  icon: ReactNode;
  content: string;
  copyKey: string;
  copiedKey: string;
  setCopiedKey: (value: string) => void;
  emptyText?: string;
}) {
  const displayContent = content.trim() || emptyText;

  return (
    <div className="rounded-2xl border border-[#1e293b] bg-[#020617] p-4">
      <div className="flex items-center justify-between gap-4">
        <div className="flex items-center gap-2 text-sm font-semibold text-white">
          {icon}
          {title}
        </div>
        <button
          type="button"
          onClick={async () => {
            if (!content.trim()) {
              return;
            }
            await navigator.clipboard.writeText(content);
            setCopiedKey(copyKey);
          }}
          disabled={!content.trim()}
          className="rounded-lg border border-[#334155] px-3 py-1.5 text-xs font-semibold text-[#cbd5e1] transition-colors hover:bg-[#111827] disabled:cursor-not-allowed disabled:opacity-50"
        >
          {copiedKey === copyKey ? 'Đã copy' : 'Copy'}
        </button>
      </div>
      <pre className="mt-3 overflow-x-auto whitespace-pre-wrap text-sm text-[#cbd5e1]">
        <code>{displayContent}</code>
      </pre>
    </div>
  );
}

function SummaryLine({ icon, label, value }: { icon: ReactNode; label: string; value: string }) {
  return (
    <div className="rounded-2xl border border-[#1e293b] bg-[#020617] px-4 py-3">
      <div className="flex items-center gap-2 text-xs font-semibold uppercase tracking-[0.12em] text-[#64748b]">
        {icon}
        {label}
      </div>
      <div className="mt-2 text-sm font-medium text-white">{value}</div>
    </div>
  );
}

function formatServiceKind(service: ProjectService) {
  const kind = service.kind.toLowerCase();
  if (service.source_type === 'internal') {
    if (kind === 'postgres') return 'PostgreSQL nội bộ';
    if (kind === 'mysql') return 'MySQL nội bộ';
    if (kind === 'redis') return 'Redis nội bộ';
    if (kind === 'rabbitmq') return 'RabbitMQ nội bộ';
  }
  if (kind === 'api') return 'API / backend';
  if (kind === 'web') return 'Frontend web';
  if (kind === 'worker') return 'Worker nền';
  return service.kind || 'App tổng quát';
}

function formatDatabaseTarget(service: ProjectService) {
  if (service.source_type === 'internal') {
    return 'Tự là dịch vụ hạ tầng';
  }
  return service.connection_target_service || 'Chưa kết nối';
}
