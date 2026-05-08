'use client';

import Link from 'next/link';
import type { AssistantMessage } from '@/modules/assistant/assistant-types';

function getCardType(message: AssistantMessage): string {
  return typeof message.content_data?.card_type === 'string' ? message.content_data.card_type : '';
}

export function AssistantRichCard({ message }: { message: AssistantMessage }) {
  const cardType = getCardType(message);
  if (cardType === 'deployment_status') {
    const deployment = message.content_data?.deployment as Record<string, unknown> | undefined;
    if (!deployment) return null;
    const projectId = String((message.content_data?.project_id as string | undefined) ?? '');
    const deploymentId = String(deployment.id ?? '');
    return (
      <div className="mt-3 rounded-xl border border-[#334155] bg-[#0b1120]/70 p-3 text-xs text-[#cbd5e1]">
        <div className="font-semibold text-white">Deployment status</div>
        <div className="mt-2">deployment: {String(deployment.id ?? 'unknown')}</div>
        <div>rollout: {String(deployment.rollout_state ?? 'unknown')}</div>
        <div>build: {String(deployment.build_state ?? 'unknown')}</div>
        {projectId && deploymentId ? <Link href={`/projects/${projectId}/deployments/${deploymentId}`} className="mt-3 inline-flex text-xs font-semibold text-[#38bdf8] hover:underline">Open deployment</Link> : null}
      </div>
    );
  }

  if (cardType === 'runtime_review') {
    const runtime = message.content_data?.runtime as Record<string, unknown> | undefined;
    const projectId = String((message.content_data?.project_id as string | undefined) ?? '');
    return (
      <div className="mt-3 rounded-xl border border-[#334155] bg-[#0b1120]/70 p-3 text-xs text-[#cbd5e1]">
        <div className="font-semibold text-white">Runtime review</div>
        <div className="mt-2">services: {Array.isArray(runtime?.services) ? runtime?.services.length : 0}</div>
        <div>nodes: {Array.isArray(runtime?.nodes) ? runtime?.nodes.length : 0}</div>
        <div>degraded or pending: {String(message.content_data?.degraded_count ?? 0)}</div>
        <div>open incidents: {String(message.content_data?.incident_count ?? 0)}</div>
        {projectId ? <Link href={`/projects/${projectId}/observability`} className="mt-3 inline-flex text-xs font-semibold text-[#38bdf8] hover:underline">Open observability</Link> : null}
      </div>
    );
  }

  if (cardType === 'logs') {
    const logs = Array.isArray(message.content_data?.logs) ? message.content_data.logs as Array<Record<string, unknown>> : [];
    const projectId = String((message.content_data?.project_id as string | undefined) ?? '');
    if (logs.length === 0) return null;
    return (
      <div className="mt-3 rounded-xl border border-[#334155] bg-[#0b1120]/70 p-3 text-xs text-[#cbd5e1]">
        <div className="font-semibold text-white">Recent logs</div>
        <div className="mt-2 flex flex-col gap-2">
          {logs.slice(0, 5).map((log, index) => (
            <div key={`${message.id}-log-${index}`} className="rounded-lg border border-[#1e293b] px-2 py-2">
              <div className="font-medium text-white">[{String(log.level ?? 'info')}] {String(log.service ?? 'service')}</div>
              <div className="mt-1 whitespace-pre-wrap text-[#94a3b8]">{String(log.message ?? '')}</div>
            </div>
          ))}
        </div>
        {projectId ? <Link href={`/projects/${projectId}/observability`} className="mt-3 inline-flex text-xs font-semibold text-[#38bdf8] hover:underline">Inspect logs</Link> : null}
      </div>
    );
  }

  if (cardType === 'incident_explanation') {
    const incident = message.content_data?.incident as Record<string, unknown> | undefined;
    if (!incident) return null;
    const projectId = String((message.content_data?.project_id as string | undefined) ?? '');
    return (
      <div className="mt-3 rounded-xl border border-[#334155] bg-[#0b1120]/70 p-3 text-xs text-[#cbd5e1]">
        <div className="font-semibold text-white">Incident explanation</div>
        <div className="mt-2">id: {String(incident.id ?? 'unknown')}</div>
        <div>kind: {String(incident.kind ?? 'unknown')}</div>
        <div>severity: {String(incident.severity ?? 'unknown')}</div>
        <div className="mt-2 whitespace-pre-wrap text-[#94a3b8]">{String(incident.summary ?? '')}</div>
        {projectId ? <Link href={`/projects/${projectId}/observability`} className="mt-3 inline-flex text-xs font-semibold text-[#38bdf8] hover:underline">Open incidents</Link> : null}
      </div>
    );
  }

  if (cardType === 'incident_explanation_engine') {
    const explanation = message.content_data?.explanation as Record<string, unknown> | undefined;
    const timeline = Array.isArray(explanation?.timeline) ? explanation?.timeline as Array<Record<string, unknown>> : [];
    const citations = Array.isArray(explanation?.citations) ? explanation?.citations as Array<Record<string, unknown>> : [];
    const recommendations = Array.isArray(explanation?.recommendations) ? explanation?.recommendations as Array<Record<string, unknown>> : [];
    const matches = Array.isArray(explanation?.historical_matches) ? explanation?.historical_matches as Array<Record<string, unknown>> : [];
    const projectId = String((message.content_data?.project_id as string | undefined) ?? '');
    return (
      <div className="mt-3 rounded-xl border border-[#334155] bg-[#0b1120]/70 p-3 text-xs text-[#cbd5e1]">
        <div className="font-semibold text-white">Incident explanation</div>
        <div className="mt-2 whitespace-pre-wrap text-[#e2e8f0]">{String(explanation?.summary ?? '')}</div>
        <div className="mt-3 rounded-lg border border-[#1e293b] px-2 py-2">
          <div className="font-medium text-white">Likely cause · {String(explanation?.confidence ?? 'unknown')}</div>
          <div className="mt-1 text-[#94a3b8]">{String(explanation?.likely_cause ?? '')}</div>
        </div>
        <div className="mt-3 font-medium text-white">Timeline</div>
        <div className="mt-1 flex flex-col gap-2">
          {timeline.slice(0, 5).map((item, index) => (
            <div key={`${message.id}-timeline-${index}`} className="rounded-lg border border-[#1e293b] px-2 py-2">
              <div className="font-medium text-white">{String(item.kind ?? 'event')} · {String(item.title ?? '')}</div>
              <div className="mt-1 text-[#94a3b8]">{String(item.detail ?? '')}</div>
              {item.timestamp ? <div className="mt-1 text-[#64748b]">{new Date(String(item.timestamp)).toLocaleString()}</div> : null}
            </div>
          ))}
        </div>
        <div className="mt-3 font-medium text-white">Recommendations</div>
        <div className="mt-1 flex flex-col gap-2">
          {recommendations.slice(0, 4).map((item, index) => (
            <div key={`${message.id}-rec-${index}`} className="rounded-lg border border-[#1e293b] px-2 py-2">
              <div className="font-medium text-white">{String(item.priority ?? 'medium')} · {String(item.action ?? '')}</div>
              <div className="mt-1 text-[#94a3b8]">{String(item.reason ?? '')}</div>
            </div>
          ))}
        </div>
        <div className="mt-3 font-medium text-white">Citations</div>
        <div className="mt-1 flex flex-col gap-1 text-[#94a3b8]">
          {citations.slice(0, 4).map((item, index) => <div key={`${message.id}-citation-${index}`}>{String(item.source ?? 'source')}: {String(item.excerpt ?? '')}</div>)}
        </div>
        {matches.length > 0 ? <div className="mt-3 text-[#94a3b8]">Historical matches: {matches.length}</div> : null}
        {projectId ? <Link href={`/projects/${projectId}/observability`} className="mt-3 inline-flex text-xs font-semibold text-[#38bdf8] hover:underline">Open observability</Link> : null}
      </div>
    );
  }

  if (cardType === 'topology') {
    const topology = message.content_data?.topology as Record<string, unknown> | undefined;
    const projectId = String((message.content_data?.project_id as string | undefined) ?? '');
    if (!topology) return null;
    const nodes = Array.isArray(topology.nodes) ? topology.nodes.length : 0;
    const edges = Array.isArray(topology.edges) ? topology.edges.length : 0;
    return (
      <div className="mt-3 rounded-xl border border-[#334155] bg-[#0b1120]/70 p-3 text-xs text-[#cbd5e1]">
        <div className="font-semibold text-white">Topology</div>
        <div className="mt-2">nodes: {nodes}</div>
        <div>edges: {edges}</div>
        {projectId ? <Link href={`/projects/${projectId}/topology`} className="mt-3 inline-flex text-xs font-semibold text-[#38bdf8] hover:underline">Open topology</Link> : null}
      </div>
    );
  }

  if (cardType === 'metrics_dashboard') {
    const metrics = message.content_data?.metrics as Record<string, unknown> | undefined;
    const summary = metrics?.summary as Record<string, unknown> | undefined;
    const projectId = String((message.content_data?.project_id as string | undefined) ?? '');
    return (
      <div className="mt-3 rounded-xl border border-[#334155] bg-[#0b1120]/70 p-3 text-xs text-[#cbd5e1]">
        <div className="font-semibold text-white">Metrics dashboard</div>
        <div className="mt-2 grid grid-cols-2 gap-2">
          <div className="rounded-lg border border-[#1e293b] px-2 py-2">requests: {String(summary?.request_total ?? 0)}</div>
          <div className="rounded-lg border border-[#1e293b] px-2 py-2">latency p95: {String(summary?.latency_p95_ms ?? 0)}ms</div>
          <div className="rounded-lg border border-[#1e293b] px-2 py-2">cpu p95: {String(summary?.cpu_p95 ?? 0)}</div>
          <div className="rounded-lg border border-[#1e293b] px-2 py-2">ram p95: {String(summary?.ram_p95_mb ?? 0)}MB</div>
          <div className="rounded-lg border border-[#1e293b] px-2 py-2">open incidents: {String(summary?.open_incidents ?? 0)}</div>
          <div className="rounded-lg border border-[#1e293b] px-2 py-2">recent errors: {String(summary?.recent_errors ?? 0)}</div>
        </div>
        {projectId ? <Link href={`/projects/${projectId}/observability`} className="mt-3 inline-flex text-xs font-semibold text-[#38bdf8] hover:underline">Open observability</Link> : null}
      </div>
    );
  }

  if (cardType === 'activity_table') {
    const items = Array.isArray(message.content_data?.items) ? message.content_data.items as Array<Record<string, unknown>> : [];
    const projectId = String((message.content_data?.project_id as string | undefined) ?? '');
    return (
      <div className="mt-3 rounded-xl border border-[#334155] bg-[#0b1120]/70 p-3 text-xs text-[#cbd5e1]">
        <div className="font-semibold text-white">Activity table</div>
        <div className="mt-2 flex flex-col gap-2">
          {items.slice(0, 8).map((item, index) => (
            <div key={`${message.id}-activity-${index}`} className="rounded-lg border border-[#1e293b] px-2 py-2">
              <div className="font-medium text-white">{String(item.kind ?? 'activity')} · {String(item.status ?? 'updated')}</div>
              <div className="mt-1 text-[#94a3b8]">{String(item.title ?? '')}</div>
              {item.time ? <div className="mt-1 text-[#64748b]">{new Date(String(item.time)).toLocaleString()}</div> : null}
            </div>
          ))}
          {items.length === 0 ? <div className="text-[#94a3b8]">No recent activity found.</div> : null}
        </div>
        {projectId ? <Link href={`/projects/${projectId}/observability`} className="mt-3 inline-flex text-xs font-semibold text-[#38bdf8] hover:underline">Open observability</Link> : null}
      </div>
    );
  }

  if (cardType === 'system_evaluation') {
    const evaluation = message.content_data?.evaluation as Record<string, unknown> | undefined;
    const signals = evaluation?.signals as Record<string, unknown> | undefined;
    const findings = Array.isArray(evaluation?.findings) ? evaluation?.findings as unknown[] : [];
    const actions = Array.isArray(evaluation?.recommended_actions) ? evaluation?.recommended_actions as unknown[] : [];
    const projectId = String((message.content_data?.project_id as string | undefined) ?? '');
    return (
      <div className="mt-3 rounded-xl border border-[#334155] bg-[#0b1120]/70 p-3 text-xs text-[#cbd5e1]">
        <div className="font-semibold text-white">System evaluation</div>
        <div className="mt-2 text-2xl font-semibold text-white">{String(evaluation?.score ?? 0)}/100</div>
        <div className="text-[#94a3b8]">{String(evaluation?.grade ?? 'unknown')}</div>
        <div className="mt-3 grid grid-cols-2 gap-2">
          <div className="rounded-lg border border-[#1e293b] px-2 py-2">services: {String(signals?.services ?? 0)}</div>
          <div className="rounded-lg border border-[#1e293b] px-2 py-2">nodes: {String(signals?.nodes ?? 0)}</div>
          <div className="rounded-lg border border-[#1e293b] px-2 py-2">degraded: {String(signals?.degraded_services ?? 0)}</div>
          <div className="rounded-lg border border-[#1e293b] px-2 py-2">errors: {String(signals?.recent_errors ?? 0)}</div>
        </div>
        <div className="mt-3 font-medium text-white">Findings</div>
        <div className="mt-1 flex flex-col gap-1 text-[#94a3b8]">
          {findings.slice(0, 4).map((item, index) => <div key={`${message.id}-finding-${index}`}>{String(item)}</div>)}
        </div>
        <div className="mt-3 font-medium text-white">Recommended actions</div>
        <div className="mt-1 flex flex-col gap-1 text-[#94a3b8]">
          {actions.slice(0, 3).map((item, index) => <div key={`${message.id}-action-${index}`}>{String(item)}</div>)}
        </div>
        {projectId ? <Link href={`/projects/${projectId}/observability`} className="mt-3 inline-flex text-xs font-semibold text-[#38bdf8] hover:underline">Open observability</Link> : null}
      </div>
    );
  }

  return null;
}
