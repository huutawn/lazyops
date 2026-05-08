'use client';

import { Bot } from 'lucide-react';
import { useAssistantPanel } from '@/modules/assistant/assistant-provider';

export function AssistantLauncher() {
  const { setOpen } = useAssistantPanel();

  return (
    <button
      type="button"
      onClick={() => setOpen(true)}
      className="fixed bottom-6 right-6 z-40 flex items-center gap-3 rounded-full border border-[#1e293b] bg-[#0EA5E9] px-5 py-3 text-sm font-semibold text-[#020617] shadow-2xl transition-transform hover:scale-[1.02]"
      aria-label="Open assistant"
    >
      <Bot className="size-4" />
      Assistant
    </button>
  );
}
