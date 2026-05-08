'use client';

import Link from 'next/link';
import type { AssistantExecutionResult } from '@/modules/assistant/assistant-types';

type AssistantExecutionCardProps = {
  projectId?: string | null;
  execution: AssistantExecutionResult;
};

export function AssistantExecutionCard({ projectId, execution }: AssistantExecutionCardProps) {
  const href = projectId && execution.deployment_id ? `/projects/${projectId}/deployments/${execution.deployment_id}` : null;

  return (
    <div className="rounded-2xl border border-[#1e293b] bg-[#020617] p-4">
      <div className="flex items-center justify-between gap-3">
        <div className="text-sm font-semibold text-white">Execution</div>
        <div className="rounded-full bg-[#0f172a] px-3 py-1 text-xs font-semibold text-[#38bdf8]">{execution.status}</div>
      </div>
      {execution.reason ? <p className="mt-3 text-sm leading-6 text-[#cbd5e1]">{execution.reason}</p> : null}
      <div className="mt-3 flex flex-col gap-1 text-xs text-[#94a3b8]">
        {execution.deployment_id ? <div>deployment: {execution.deployment_id}</div> : null}
        {execution.revision_id ? <div>revision: {execution.revision_id}</div> : null}
        {execution.build_job_id ? <div>build job: {execution.build_job_id}</div> : null}
        {execution.correlation_id ? <div>correlation: {execution.correlation_id}</div> : null}
      </div>
      {href ? (
        <Link href={href} className="mt-4 inline-flex text-sm font-semibold text-[#38bdf8] hover:underline">
          Open deployment
        </Link>
      ) : null}
    </div>
  );
}
