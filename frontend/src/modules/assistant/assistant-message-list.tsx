'use client';

import Link from 'next/link';
import { useState } from 'react';
import type { AssistantMessage } from '@/modules/assistant/assistant-types';
import { AssistantRichCard } from '@/modules/assistant/assistant-rich-cards';

function historicalMatches(message: AssistantMessage) {
  const value = message.content_data?.historical_matches;
  return Array.isArray(value) ? value as Array<Record<string, unknown>> : [];
}

type AssistantMessageListProps = {
  messages: AssistantMessage[];
};

export function AssistantMessageList({ messages }: AssistantMessageListProps) {
  const [expandedMatches, setExpandedMatches] = useState<Record<string, boolean>>({});

  if (messages.length === 0) {
    return (
      <div className="rounded-2xl border border-dashed border-[#334155] bg-[#020617]/60 p-4 text-sm text-[#94a3b8]">
        Ask about deploys, runtime status, logs, or topology.
      </div>
    );
  }

  return (
    <div className="flex flex-col gap-3">
      {messages.map((message) => {
        const isUser = message.role === 'user';
        const matches = historicalMatches(message);
        const expanded = !!expandedMatches[message.id];
        const visibleMatches = expanded ? matches : matches.slice(0, 2);
        return (
          <div
            key={message.id}
            className={isUser ? 'ml-10 rounded-2xl bg-[#0f172a] px-4 py-3 text-sm text-white' : 'mr-10 rounded-2xl border border-[#1e293b] bg-[#020617]/80 px-4 py-3 text-sm text-[#cbd5e1]'}
          >
            <div className="mb-1 text-[11px] font-semibold uppercase tracking-wide text-[#64748b]">
              {isUser ? 'You' : 'Assistant'}
            </div>
            <div className="whitespace-pre-wrap leading-6">{message.content}</div>
            {!isUser ? <AssistantRichCard message={message} /> : null}
            {!isUser && matches.length > 0 ? (
              <div className="mt-3 flex flex-col gap-2">
                <div className="flex items-center justify-between gap-3">
                  <div className="text-xs font-semibold uppercase tracking-wide text-[#38bdf8]">Historical matches</div>
                  {matches.length > 2 ? (
                    <button
                      type="button"
                      onClick={() => setExpandedMatches((current) => ({ ...current, [message.id]: !expanded }))}
                      className="text-xs font-semibold text-[#38bdf8] hover:underline"
                    >
                      {expanded ? 'Collapse' : `Show all (${matches.length})`}
                    </button>
                  ) : null}
                </div>
                {visibleMatches.map((item, index) => (
                  <div key={`${message.id}-${index}`} className="rounded-xl border border-[#334155] bg-[#0b1120]/70 px-3 py-2 text-xs text-[#cbd5e1]">
                    <div className="font-semibold text-white">{String(item.title ?? 'Historical error')}</div>
                    <div className="mt-1">service: {String(item.service_name ?? 'unknown')} · severity: {String(item.severity ?? 'unknown')}</div>
                    <div className="mt-1 whitespace-pre-wrap text-[#94a3b8]">{String(item.body ?? '')}</div>
                    <div className="mt-2 flex flex-wrap gap-3">
                      {typeof item.project_id === 'string' && item.project_id ? (
                        <Link href={`/projects/${item.project_id}/observability`} className="text-[#38bdf8] hover:underline">
                          Open observability
                        </Link>
                      ) : null}
                      {typeof item.project_id === 'string' && item.project_id && typeof item.correlation_id === 'string' && item.correlation_id ? (
                        <Link href={`/projects/${item.project_id}/observability?correlation_id=${encodeURIComponent(item.correlation_id)}`} className="text-[#38bdf8] hover:underline">
                          Correlation logs
                        </Link>
                      ) : null}
                      {typeof item.project_id === 'string' && item.project_id && typeof item.service_name === 'string' && item.service_name ? (
                        <Link href={`/projects/${item.project_id}/observability?service=${encodeURIComponent(item.service_name)}`} className="text-[#38bdf8] hover:underline">
                          Service logs
                        </Link>
                      ) : null}
                    </div>
                  </div>
                ))}
              </div>
            ) : null}
          </div>
        );
      })}
    </div>
  );
}
