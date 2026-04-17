import type { LogEntry } from '@/modules/observability/observability-types';

export type LiveLogEnvelope = {
  type?: string;
  payload?: {
    project_id?: string;
    service_name?: string;
    revision_id?: string;
    entries?: Array<{
      timestamp?: string;
      severity?: LogEntry['level'];
      source?: string;
      message?: string;
    }>;
  };
};

export function normalizeLiveLogEnvelope(envelope: LiveLogEnvelope, projectId?: string): LogEntry[] {
  if (!projectId || envelope.type !== 'logs.live' || envelope.payload?.project_id !== projectId) {
    return [];
  }

  const service = envelope.payload.service_name ?? 'app';
  const revisionID = envelope.payload.revision_id?.trim() || undefined;

  return (envelope.payload.entries ?? [])
    .filter((entry) => entry.message && entry.timestamp)
    .map((entry, index) => ({
      id: buildObservedLogID({
        service,
        source: entry.source,
        revisionID,
        timestamp: entry.timestamp ?? '',
        message: entry.message ?? '',
        index,
      }),
      service,
      source: entry.source,
      revision_id: revisionID,
      level: (entry.severity ?? 'info') as LogEntry['level'],
      message: entry.message ?? '',
      timestamp: entry.timestamp ?? new Date().toISOString(),
    }));
}

export function mergeObservedLogs(history: LogEntry[], live: LogEntry[]): LogEntry[] {
  const deduped = new Map<string, LogEntry>();
  for (const entry of [...history, ...live]) {
    deduped.set(buildObservedLogDedupKey(entry), entry);
  }
  return [...deduped.values()].sort((a, b) => a.timestamp.localeCompare(b.timestamp));
}

export function listObservedServices(entries: LogEntry[]): string[] {
  return [...new Set(entries.map((entry) => entry.service).filter(Boolean))].sort((a, b) => a.localeCompare(b));
}

type ObservedLogIDInput = {
  service: string;
  source?: string;
  revisionID?: string;
  timestamp: string;
  message: string;
  index: number;
};

function buildObservedLogID(input: ObservedLogIDInput): string {
  return [
    input.service,
    input.source ?? 'unknown',
    input.revisionID ?? 'no-revision',
    input.timestamp,
    input.index,
  ].join(':');
}

function buildObservedLogDedupKey(entry: LogEntry): string {
  return [
    entry.service,
    entry.source ?? 'unknown',
    entry.revision_id ?? 'no-revision',
    entry.timestamp,
    entry.message,
  ].join(':');
}
