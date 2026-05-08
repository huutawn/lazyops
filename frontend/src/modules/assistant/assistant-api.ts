import { apiGet, apiPost } from '@/lib/api/api-client';
import type { ApiResponse } from '@/lib/types';
import type {
  AssistantConversation,
  AssistantSession,
  AssistantSessionListResponse,
  CreateAssistantSessionRequest,
  PostAssistantMessageRequest,
} from '@/modules/assistant/assistant-types';

export async function createAssistantSession(body: CreateAssistantSessionRequest): Promise<ApiResponse<AssistantSession>> {
  return apiPost<AssistantSession>('/assistant/sessions', body);
}

export async function listAssistantSessions(): Promise<ApiResponse<AssistantSessionListResponse>> {
  return apiGet<AssistantSessionListResponse>('/assistant/sessions');
}

export async function getAssistantSession(sessionId: string): Promise<ApiResponse<AssistantConversation>> {
  return apiGet<AssistantConversation>(`/assistant/sessions/${sessionId}`);
}

export async function postAssistantMessage(sessionId: string, body: PostAssistantMessageRequest): Promise<ApiResponse<AssistantConversation>> {
  return apiPost<AssistantConversation>(`/assistant/sessions/${sessionId}/messages`, body);
}

export async function confirmAssistantPlan(planId: string): Promise<ApiResponse<AssistantConversation>> {
  return apiPost<AssistantConversation>(`/assistant/action-plans/${planId}/confirm`, {});
}

export async function cancelAssistantPlan(planId: string): Promise<ApiResponse<AssistantConversation>> {
  return apiPost<AssistantConversation>(`/assistant/action-plans/${planId}/cancel`, {});
}
