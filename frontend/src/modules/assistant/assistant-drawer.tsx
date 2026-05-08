'use client';

import { useMemo } from 'react';
import { Drawer } from '@/components/primitives/drawer';
import {
  useCancelAssistantPlan,
  useConfirmAssistantPlan,
  useSendAssistantMessage,
} from '@/modules/assistant/assistant-hooks';
import { useActiveAssistantConversation, useAssistantPanel } from '@/modules/assistant/assistant-provider';
import { AssistantComposer } from '@/modules/assistant/assistant-composer';
import { AssistantConfirmationCard } from '@/modules/assistant/assistant-confirmation-card';
import { AssistantExecutionCard } from '@/modules/assistant/assistant-execution-card';
import { useAssistantLiveEvents } from '@/modules/assistant/assistant-live-events';
import { AssistantMessageList } from '@/modules/assistant/assistant-message-list';
import { AssistantPlanCard } from '@/modules/assistant/assistant-plan-card';

export function AssistantDrawer() {
  const { open, setOpen, activeSessionId, setActiveSessionId, currentProjectId, ensureSession, sessions } = useAssistantPanel();
  const conversation = useActiveAssistantConversation();
  const sendMessage = useSendAssistantMessage(activeSessionId);
  const confirmPlan = useConfirmAssistantPlan(activeSessionId);
  const cancelPlan = useCancelAssistantPlan(activeSessionId);
  const liveEvents = useAssistantLiveEvents(conversation.data?.session.project_id ?? currentProjectId, open);

  const pendingPlan = conversation.data?.pending_plan ?? null;
  const execution = conversation.data?.execution_result ?? null;
  const title = useMemo(() => {
    if (conversation.data?.session.title) {
      return conversation.data.session.title;
    }
    return 'Assistant';
  }, [conversation.data?.session.title]);

  return (
    <Drawer open={open} onClose={() => setOpen(false)} title={title} side="right" size="lg">
      <div className="flex flex-col gap-5">
        <div className="rounded-2xl border border-[#1e293b] bg-[#020617]/80 p-4 text-sm text-[#94a3b8]">
          <div className="font-semibold text-white">Current context</div>
          <div className="mt-2">project: {currentProjectId ?? 'Not pinned yet'}</div>
          <div>session: {activeSessionId ?? 'Creating when needed'}</div>
        </div>

        <div className="rounded-2xl border border-[#1e293b] bg-[#020617]/80 p-4">
          <div className="mb-3 text-sm font-semibold text-white">Sessions</div>
          <div className="flex flex-col gap-2">
            {sessions.length === 0 ? (
              <div className="text-sm text-[#94a3b8]">No prior sessions yet.</div>
            ) : sessions.map((session) => (
              <button
                key={session.id}
                type="button"
                onClick={() => setActiveSessionId(session.id)}
                className={session.id === activeSessionId ? 'rounded-xl border border-[#0EA5E9]/40 bg-[#0EA5E9]/10 px-3 py-2 text-left text-sm text-[#e0f2fe]' : 'rounded-xl border border-[#1e293b] px-3 py-2 text-left text-sm text-[#cbd5e1]'}
              >
                <div className="font-medium">{session.title}</div>
                <div className="text-xs text-[#94a3b8]">{session.project_id ?? 'No project pinned'}</div>
              </button>
            ))}
          </div>
        </div>

        {pendingPlan ? <AssistantPlanCard plan={pendingPlan} /> : null}

        {pendingPlan?.requires_confirmation && pendingPlan.status === 'awaiting_confirmation' ? (
          <AssistantConfirmationCard
            plan={pendingPlan}
            isConfirming={confirmPlan.isPending}
            isCancelling={cancelPlan.isPending}
            onConfirm={() => {
              void confirmPlan.mutateAsync(pendingPlan.id);
            }}
            onCancel={() => {
              void cancelPlan.mutateAsync(pendingPlan.id);
            }}
          />
        ) : null}

        {execution ? <AssistantExecutionCard projectId={conversation.data?.session.project_id ?? currentProjectId} execution={execution} /> : null}

        {liveEvents.length > 0 ? (
          <div className="rounded-2xl border border-[#1e293b] bg-[#020617] p-4">
            <div className="mb-3 text-sm font-semibold text-white">Live progress</div>
            <div className="flex flex-col gap-2 text-sm text-[#cbd5e1]">
              {liveEvents.map((event) => (
                <div key={event.id} className="rounded-xl border border-[#1e293b] px-3 py-2">
                  <div>{event.message}</div>
                  {event.occurred_at ? <div className="mt-1 text-xs text-[#64748b]">{new Date(event.occurred_at).toLocaleString()}</div> : null}
                </div>
              ))}
            </div>
          </div>
        ) : null}

        <AssistantMessageList messages={conversation.data?.messages ?? []} />

        <AssistantComposer
          disabled={sendMessage.isPending || confirmPlan.isPending || cancelPlan.isPending}
          onSubmit={async (content) => {
            const sessionId = await ensureSession();
            if (!sessionId) {
              return;
            }
            await sendMessage.mutateAsync({ project_id: currentProjectId ?? undefined, content });
          }}
        />

        {(sendMessage.error || conversation.error || confirmPlan.error || cancelPlan.error) ? (
          <div className="rounded-2xl border border-[#ef4444]/30 bg-[#ef4444]/10 px-4 py-3 text-sm text-[#fecaca]">
            {(sendMessage.error as Error | null)?.message || (conversation.error as Error | null)?.message || (confirmPlan.error as Error | null)?.message || (cancelPlan.error as Error | null)?.message}
          </div>
        ) : null}
      </div>
    </Drawer>
  );
}
