export type ProjectAIPromptServiceSnapshot = {
  name: string;
  kind: string;
  role: string;
  runtime_profile?: string;
  source_type?: string;
  public: boolean;
  managed: boolean;
  websocket?: boolean;
  public_path?: string;
  internal_url?: string;
};

export type ProjectAIPromptSourceSection = {
  key: string;
  title: string;
  description: string;
  item_count: number;
};

export type ProjectAIPromptResponse = {
  title: string;
  summary: string;
  prompt: string;
  service_snapshot: ProjectAIPromptServiceSnapshot[];
  effective_public_paths: Array<{
    path: string;
    service: string;
    audience?: string;
    source?: string;
    websocket?: boolean;
  }>;
  managed_keys: string[];
  migration_findings: Array<{
    category: string;
    severity: string;
    service_name?: string;
    current_value?: string;
    recommended_value?: string;
    message: string;
  }>;
  source_sections: ProjectAIPromptSourceSection[];
};
