'use client';

import { useState } from 'react';

type AssistantComposerProps = {
  disabled?: boolean;
  onSubmit: (content: string) => Promise<void>;
};

export function AssistantComposer({ disabled, onSubmit }: AssistantComposerProps) {
  const [content, setContent] = useState('');

  return (
    <form
      className="flex flex-col gap-3"
      onSubmit={async (event) => {
        event.preventDefault();
        const value = content.trim();
        if (!value || disabled) {
          return;
        }
        await onSubmit(value);
        setContent('');
      }}
    >
      <textarea
        value={content}
        onChange={(event) => setContent(event.target.value)}
        placeholder="Ask about deploys, logs, topology, or runtime status"
        rows={4}
        className="w-full rounded-2xl border border-[#1e293b] bg-[#020617] px-4 py-3 text-sm text-white outline-none placeholder:text-[#64748b]"
      />
      <div className="flex justify-end">
        <button
          type="submit"
          disabled={disabled || content.trim().length === 0}
          className="rounded-xl bg-[#0EA5E9] px-4 py-2 text-sm font-semibold text-[#020617] disabled:opacity-60"
        >
          Send
        </button>
      </div>
    </form>
  );
}
