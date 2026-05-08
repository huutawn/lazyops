'use client';

import { useEffect, useState } from 'react';

export type AssistantLiveEvent = {
  id: string;
  type: string;
  occurred_at?: string;
  message: string;
  payload?: Record<string, unknown>;
};

function normalizeAssistantLiveEvent(envelope: unknown, currentProjectId?: string | null): AssistantLiveEvent | null {
  if (!envelope || typeof envelope !== 'object') {
    return null;
  }
  const value = envelope as Record<string, unknown>;
  const type = typeof value.type === 'string' ? value.type : '';
  const occurredAt = typeof value.occurred_at === 'string' ? value.occurred_at : undefined;
  const payload = typeof value.payload === 'object' && value.payload !== null ? value.payload as Record<string, unknown> : {};
  const projectID = typeof payload.project_id === 'string' ? payload.project_id : '';
  if (currentProjectId && projectID && projectID !== currentProjectId) {
    return null;
  }
  const deploymentID = typeof payload.deployment_id === 'string' ? payload.deployment_id : '';
  const revisionID = typeof payload.revision_id === 'string' ? payload.revision_id : '';
  const state = typeof payload.state === 'string' ? payload.state : '';
  const label = typeof payload.label === 'string' ? payload.label : '';
  if (type === 'assistant.incident_detected') {
    const severity = typeof payload.severity === 'string' ? payload.severity : 'incident';
    const service = typeof payload.service_name === 'string' ? payload.service_name : 'runtime';
    const summary = typeof payload.summary === 'string' ? payload.summary : typeof payload.message === 'string' ? payload.message : '';
    return {
      id: `${type}:${payload.incident_id ?? occurredAt ?? Math.random().toString(36).slice(2)}`,
      type,
      occurred_at: occurredAt,
      message: ['Assistant detected incident', severity, service, summary].filter(Boolean).join(' · '),
      payload,
    };
  }
  const summaryParts = [type, deploymentID, revisionID, state, label].filter(Boolean);
  if (summaryParts.length === 0) {
    return null;
  }
  return {
    id: `${type}:${deploymentID || revisionID || occurredAt || Math.random().toString(36).slice(2)}`,
    type,
    occurred_at: occurredAt,
    message: summaryParts.join(' · '),
    payload,
  };
}

export function useAssistantLiveEvents(projectId?: string | null, enabled = false) {
  const [events, setEvents] = useState<AssistantLiveEvent[]>([]);

  useEffect(() => {
    if (!enabled || typeof window === 'undefined') {
      setEvents([]);
      return;
    }

    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const socket = new WebSocket(`${protocol}//${window.location.host}/api/v1/ws/operators/stream`);

    socket.onmessage = (event) => {
      try {
        const envelope = JSON.parse(event.data);
        const next = normalizeAssistantLiveEvent(envelope, projectId);
        if (!next) {
          return;
        }
        setEvents((current) => [...current, next].slice(-20));
      } catch {
        // Ignore malformed stream envelopes.
      }
    };

    return () => {
      socket.close();
    };
  }, [enabled, projectId]);

  return events;
}
