export type AssistantSession = {
  id: string;
  project_id?: string | null;
  title: string;
  status: string;
  last_message_at: string;
  created_at: string;
  updated_at: string;
};

export type AssistantMessage = {
  id: string;
  session_id: string;
  role: 'user' | 'assistant';
  kind: 'chat' | 'plan' | 'confirmation_request' | 'execution_result';
  content: string;
  content_data?: Record<string, unknown>;
  created_at: string;
};

export type AssistantHistoricalMatch = {
  id: string;
  project_id?: string;
  service_name: string;
  severity: string;
  title: string;
  body: string;
  correlation_id?: string;
  revision_id?: string;
  last_seen_at: string;
};

export type AssistantMissingInput = {
  field: string;
  prompt: string;
  example?: string;
};

export type AssistantActionPlan = {
  id: string;
  action_type: string;
  status: string;
  summary: string;
  risk_level: string;
  requires_confirmation: boolean;
  missing_inputs?: AssistantMissingInput[];
  plan: Record<string, unknown>;
  expires_at?: string | null;
  created_at: string;
  updated_at: string;
};

export type AssistantExecutionResult = {
  status: string;
  deployment_id?: string;
  revision_id?: string;
  build_job_id?: string;
  correlation_id?: string;
  agent_id?: string;
  reason?: string;
};

export type AssistantConversation = {
  session: AssistantSession;
  messages: AssistantMessage[];
  pending_plan?: AssistantActionPlan | null;
  ui_state: 'chat' | 'planning' | 'awaiting_confirmation' | 'executing' | 'completed' | 'failed';
  execution_result?: AssistantExecutionResult | null;
};

export type AssistantSessionListResponse = {
  items: AssistantSession[];
};

export type CreateAssistantSessionRequest = {
  project_id?: string;
  title?: string;
};

export type PostAssistantMessageRequest = {
  project_id?: string;
  content: string;
};
