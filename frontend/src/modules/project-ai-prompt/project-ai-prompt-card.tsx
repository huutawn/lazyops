'use client';

import { useEffect, useMemo, useState } from 'react';
import { useProjectAIPrompt } from '@/modules/project-ai-prompt/project-ai-prompt-hooks';

type ProjectAIPromptCardProps = {
  projectId: string;
  contextLabel: string;
  title?: string;
  description?: string;
};

function buildPromptCopyText(contextLabel: string, prompt: string) {
  const intro = `Context from LazyOps UI: the user opened this AI migration prompt from the ${contextLabel}. Prioritize that area first, but keep the answer project-wide.`;
  return `${intro}\n\n${prompt}`.trim();
}

export function ProjectAIPromptCard({
  projectId,
  contextLabel,
  title = 'AI migration prompt',
  description = 'Copy một prompt đầy đủ để đưa cho ChatGPT, Codex, hoặc Gemini sửa config/env/routing giúp user.',
}: ProjectAIPromptCardProps) {
  const { data, isLoading, error } = useProjectAIPrompt(projectId);
  const [copied, setCopied] = useState(false);

  useEffect(() => {
    if (!copied) {
      return;
    }
    const timer = window.setTimeout(() => setCopied(false), 1500);
    return () => window.clearTimeout(timer);
  }, [copied]);

  const copyText = useMemo(() => {
    if (!data?.prompt) {
      return '';
    }
    return buildPromptCopyText(contextLabel, data.prompt);
  }, [contextLabel, data?.prompt]);

  return (
    <div className="rounded-xl border border-[#1e293b] bg-[#0B1120]/30 p-6">
      <div className="flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between">
        <div className="space-y-2">
          <h3 className="text-base font-semibold text-white">{title}</h3>
          <p className="text-sm text-lazyops-muted">{description}</p>
          {data?.summary ? (
            <p className="text-sm text-[#cbd5e1]">{data.summary}</p>
          ) : null}
        </div>

        <button
          type="button"
          className="rounded-lg border border-lazyops-border px-4 py-2 text-sm font-semibold text-lazyops-text transition-colors hover:bg-lazyops-border/10 disabled:cursor-not-allowed disabled:opacity-50"
          disabled={!copyText || isLoading}
          onClick={async () => {
            if (!copyText) {
              return;
            }
            await navigator.clipboard.writeText(copyText);
            setCopied(true);
          }}
        >
          {isLoading ? 'Đang tải...' : copied ? 'Đã copy' : 'Copy AI Prompt'}
        </button>
      </div>

      {error ? (
        <div className="mt-4 rounded-lg border border-health-unhealthy/30 bg-health-unhealthy/10 px-3 py-2 text-sm text-health-unhealthy">
          {error.message}
        </div>
      ) : null}

      {data?.source_sections?.length ? (
        <div className="mt-4 grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
          {data.source_sections.map((section) => (
            <div key={section.key} className="rounded-lg border border-[#1e293b] bg-[#020617] px-4 py-3">
              <p className="text-sm font-semibold text-white">{section.title}</p>
              <p className="mt-1 text-2xl font-semibold text-[#38BDF8]">{section.item_count}</p>
              <p className="mt-1 text-xs text-lazyops-muted">{section.description}</p>
            </div>
          ))}
        </div>
      ) : null}

      {data?.prompt ? (
        <div className="mt-4 rounded-lg border border-[#1e293b] bg-[#020617] px-4 py-3">
          <p className="text-sm font-semibold text-white">Prompt preview</p>
          <pre className="mt-2 max-h-64 overflow-auto whitespace-pre-wrap text-sm text-[#cbd5e1]">
            <code>{copyText}</code>
          </pre>
        </div>
      ) : null}
    </div>
  );
}
