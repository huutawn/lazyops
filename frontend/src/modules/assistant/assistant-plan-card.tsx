'use client';

import type { AssistantActionPlan } from '@/modules/assistant/assistant-types';

type AssistantPlanCardProps = {
  plan: AssistantActionPlan;
};

export function AssistantPlanCard({ plan }: AssistantPlanCardProps) {
  return (
    <div className="rounded-2xl border border-[#1e293b] bg-[#020617] p-4">
      <div className="flex items-center justify-between gap-3">
        <div>
          <div className="text-sm font-semibold text-white">Current plan</div>
          <div className="text-xs text-[#94a3b8]">{plan.action_type}</div>
        </div>
        <div className="rounded-full bg-[#0f172a] px-3 py-1 text-xs font-semibold text-[#38bdf8]">
          {plan.status}
        </div>
      </div>
      <p className="mt-3 text-sm leading-6 text-[#cbd5e1]">{plan.summary}</p>
      <div className="mt-3 flex flex-wrap gap-2 text-xs text-[#94a3b8]">
        <span className="rounded-full border border-[#334155] px-2 py-1">risk: {plan.risk_level}</span>
        {typeof plan.plan.target_environment === 'string' ? (
          <span className="rounded-full border border-[#334155] px-2 py-1">env: {plan.plan.target_environment}</span>
        ) : null}
        {typeof plan.plan.source_ref === 'string' ? (
          <span className="rounded-full border border-[#334155] px-2 py-1">ref: {plan.plan.source_ref}</span>
        ) : null}
      </div>
    </div>
  );
}
