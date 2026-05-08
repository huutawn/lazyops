'use client';

import { createContext, useContext, useEffect, useMemo, useState, type ReactNode } from 'react';
import { usePathname } from 'next/navigation';
import { useAssistantSession, useAssistantSessions, useCreateAssistantSession } from '@/modules/assistant/assistant-hooks';
import type { AssistantSession } from '@/modules/assistant/assistant-types';

type AssistantContextValue = {
  open: boolean;
  setOpen: (open: boolean) => void;
  activeSessionId: string | null;
  setActiveSessionId: (sessionId: string | null) => void;
  currentProjectId: string | null;
  ensureSession: () => Promise<string>;
  sessions: AssistantSession[];
};

const AssistantContext = createContext<AssistantContextValue | null>(null);

function deriveProjectId(pathname: string): string | null {
  const match = pathname.match(/^\/projects\/([^/]+)/);
  return match?.[1] ?? null;
}

type AssistantProviderProps = {
  children: ReactNode;
};

export function AssistantProvider({ children }: AssistantProviderProps) {
  const pathname = usePathname();
  const createSession = useCreateAssistantSession();
  const sessionsQuery = useAssistantSessions();
  const [open, setOpen] = useState(false);
  const [activeSessionId, setActiveSessionId] = useState<string | null>(null);
  const currentProjectId = deriveProjectId(pathname);

  const ensureSession = async () => {
    if (activeSessionId) {
      return activeSessionId;
    }
    const session = await createSession.mutateAsync({ project_id: currentProjectId ?? undefined });
    setActiveSessionId(session.id);
    return session.id;
  };

  useEffect(() => {
    if (!open) {
      return;
    }
    void ensureSession();
  }, [open]);

  const value = useMemo<AssistantContextValue>(() => ({
    open,
    setOpen,
    activeSessionId,
    setActiveSessionId,
    currentProjectId,
    ensureSession,
    sessions: sessionsQuery.data ?? [],
  }), [open, activeSessionId, currentProjectId, sessionsQuery.data]);

  return <AssistantContext.Provider value={value}>{children}</AssistantContext.Provider>;
}

export function useAssistantPanel() {
  const context = useContext(AssistantContext);
  if (!context) {
    throw new Error('AssistantProvider is missing');
  }
  return context;
}

export function useActiveAssistantConversation() {
  const { activeSessionId } = useAssistantPanel();
  return useAssistantSession(activeSessionId);
}
