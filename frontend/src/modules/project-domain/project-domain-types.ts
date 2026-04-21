export type ProjectDomain = {
  id: string;
  project_id: string;
  hostname: string;
  label: string;
  kind: string;
  status: 'pending' | 'active' | 'error' | string;
  status_reason?: string;
  cloudflare_record_id?: string;
  target_kind?: string;
  target_id?: string;
  last_synced_ip?: string;
  public_url?: string;
  created_at: string;
  updated_at: string;
};

export type AllocateProjectDomainRequest = {
  regenerate?: boolean;
};

export type RenameProjectDomainRequest = {
  label: string;
};
