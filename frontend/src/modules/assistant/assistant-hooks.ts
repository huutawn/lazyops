import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import {
  cancelAssistantPlan,
  confirmAssistantPlan,
  createAssistantSession,
  getAssistantSession,
  listAssistantSessions,
  postAssistantMessage,
} from '@/modules/assistant/assistant-api';
import type {
  AssistantConversation,
  AssistantSession,
  AssistantSessionListResponse,
  CreateAssistantSessionRequest,
  PostAssistantMessageRequest,
} from '@/modules/assistant/assistant-types';

export function assistantSessionsQueryKey() {
  return ['assistant-sessions'] as const;
}

export function assistantSessionQueryKey(sessionId: string | null) {
  return ['assistant-session', sessionId] as const;
}

export function useAssistantSessions() {
  return useQuery({
    queryKey: assistantSessionsQueryKey(),
    queryFn: async (): Promise<AssistantSession[]> => {
      const result = await listAssistantSessions();
      if (result.error) {
        throw new Error(result.error.message);
      }
      const payload = result.data as AssistantSessionListResponse | undefined;
      return payload?.items ?? [];
    },
    staleTime: 10 * 1000,
  });
}

export function useAssistantSession(sessionId: string | null) {
  return useQuery({
    queryKey: assistantSessionQueryKey(sessionId),
    queryFn: async (): Promise<AssistantConversation> => {
      if (!sessionId) {
        throw new Error('Assistant session is missing');
      }
      const result = await getAssistantSession(sessionId);
      if (result.error) {
        throw new Error(result.error.message);
      }
      if (!result.data) {
        throw new Error('Assistant session unavailable');
      }
      return result.data;
    },
    enabled: !!sessionId,
    staleTime: 5 * 1000,
  });
}

export function useCreateAssistantSession() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (data: CreateAssistantSessionRequest): Promise<AssistantSession> => {
      const result = await createAssistantSession(data);
      if (result.error) {
        throw new Error(result.error.message);
      }
      if (!result.data) {
        throw new Error('Assistant session creation failed');
      }
      return result.data;
    },
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: assistantSessionsQueryKey() });
    },
  });
}

export function useSendAssistantMessage(sessionId: string | null) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (data: PostAssistantMessageRequest): Promise<AssistantConversation> => {
      if (!sessionId) {
        throw new Error('Assistant session is missing');
      }
      const result = await postAssistantMessage(sessionId, data);
      if (result.error) {
        throw new Error(result.error.message);
      }
      if (!result.data) {
        throw new Error('Assistant response unavailable');
      }
      return result.data;
    },
    onSuccess: (result) => {
      queryClient.setQueryData(assistantSessionQueryKey(sessionId), result);
      void queryClient.invalidateQueries({ queryKey: assistantSessionsQueryKey() });
    },
  });
}

export function useConfirmAssistantPlan(sessionId: string | null) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (planId: string): Promise<AssistantConversation> => {
      const result = await confirmAssistantPlan(planId);
      if (result.error) {
        throw new Error(result.error.message);
      }
      if (!result.data) {
        throw new Error('Assistant plan confirmation failed');
      }
      return result.data;
    },
    onSuccess: (result) => {
      queryClient.setQueryData(assistantSessionQueryKey(sessionId), result);
      void queryClient.invalidateQueries({ queryKey: assistantSessionsQueryKey() });
    },
  });
}

export function useCancelAssistantPlan(sessionId: string | null) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (planId: string): Promise<AssistantConversation> => {
      const result = await cancelAssistantPlan(planId);
      if (result.error) {
        throw new Error(result.error.message);
      }
      if (!result.data) {
        throw new Error('Assistant plan cancellation failed');
      }
      return result.data;
    },
    onSuccess: (result) => {
      queryClient.setQueryData(assistantSessionQueryKey(sessionId), result);
      void queryClient.invalidateQueries({ queryKey: assistantSessionsQueryKey() });
    },
  });
}
