'use client';

import type { AssistantActionPlan } from '@/modules/assistant/assistant-types';

type AssistantConfirmationCardProps = {
  plan: AssistantActionPlan;
  isConfirming: boolean;
  isCancelling: boolean;
  onConfirm: () => void;
  onCancel: () => void;
};

export function AssistantConfirmationCard({ plan, isConfirming, isCancelling, onConfirm, onCancel }: AssistantConfirmationCardProps) {
  return (
    <div className="rounded-2xl border border-[#0EA5E9]/30 bg-[#0EA5E9]/10 p-4">
      <div className="text-sm font-semibold text-[#e0f2fe]">Production confirmation required</div>
      <p className="mt-2 text-sm leading-6 text-[#bae6fd]">{plan.summary}</p>
      <div className="mt-3 flex flex-wrap gap-2 text-xs text-[#e0f2fe]">
        {typeof plan.plan.target_environment === 'string' ? <span className="rounded-full border border-[#38bdf8]/30 px-2 py-1">env: {plan.plan.target_environment}</span> : null}
        {typeof plan.plan.source_ref === 'string' ? <span className="rounded-full border border-[#38bdf8]/30 px-2 py-1">ref: {plan.plan.source_ref}</span> : null}
      </div>
      <div className="mt-4 flex gap-3">
        <button
          type="button"
          onClick={onConfirm}
          disabled={isConfirming || isCancelling}
          className="rounded-xl bg-[#0EA5E9] px-4 py-2 text-sm font-semibold text-[#020617] disabled:opacity-60"
        >
          Confirm deploy
        </button>
        <button
          type="button"
          onClick={onCancel}
          disabled={isConfirming || isCancelling}
          className="rounded-xl border border-[#38bdf8]/30 px-4 py-2 text-sm font-semibold text-[#e0f2fe] disabled:opacity-60"
        >
          Cancel
        </button>
      </div>
    </div>
  );
}
