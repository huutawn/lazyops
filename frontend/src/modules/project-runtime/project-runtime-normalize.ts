import type { ProjectRuntimeDependency, ProjectRuntimeLogPreview, ProjectRuntimeNode, ProjectRuntimeService, ProjectRuntimeSummary } from '@/modules/project-runtime/project-runtime-types';

function normalizeNode(node: Partial<ProjectRuntimeNode>): ProjectRuntimeNode {
  return {
    cluster_id: node.cluster_id ?? '',
    instance_id: node.instance_id ?? '',
    name: node.name ?? node.instance_id ?? 'unknown-node',
    status: node.status ?? 'unknown',
    k8s_node_name: node.k8s_node_name,
    labels: node.labels ?? {},
    last_seen_at: node.last_seen_at,
    is_ready: Boolean(node.is_ready),
  };
}

function normalizeDependency(dependency: Partial<ProjectRuntimeDependency>): ProjectRuntimeDependency {
  return {
    service_id: dependency.service_id,
    service_name: dependency.service_name ?? '',
    status: dependency.status ?? 'unknown',
    status_reason: dependency.status_reason,
    internal_endpoint: dependency.internal_endpoint,
  };
}

function normalizeRecentLog(log: Partial<ProjectRuntimeLogPreview>): ProjectRuntimeLogPreview {
  return {
    id: log.id,
    source: log.source,
    level: log.level ?? 'info',
    message: log.message ?? '',
    timestamp: log.timestamp ?? new Date(0).toISOString(),
    node: log.node,
    correlation_id: log.correlation_id,
  };
}

function normalizeService(service: Partial<ProjectRuntimeService>): ProjectRuntimeService {
  return {
    service_id: service.service_id ?? '',
    name: service.name ?? '',
    kind: service.kind,
    source_type: service.source_type,
    public: Boolean(service.public),
    runtime_profile: service.runtime_profile,
    runtime_status: service.runtime_status ?? 'missing',
    runtime_reason: service.runtime_reason,
    build_state: service.build_state,
    rollout_state: service.rollout_state,
    placement_mode: service.placement_mode,
    requested_node_id: service.requested_node_id,
    effective_node_ids: service.effective_node_ids ?? [],
    image_ref: service.image_ref,
    image_digest: service.image_digest,
    revision_id: service.revision_id,
    revision: service.revision,
    deployment_id: service.deployment_id,
    public_urls: service.public_urls ?? [],
    internal_endpoints: service.internal_endpoints ?? [],
    dependencies: (service.dependencies ?? []).map(normalizeDependency),
    recent_logs: (service.recent_logs ?? []).map(normalizeRecentLog),
  };
}

export function normalizeProjectRuntimeSummary(projectId: string, summary?: Partial<ProjectRuntimeSummary> | null): ProjectRuntimeSummary {
  return {
    project_id: summary?.project_id ?? projectId,
    runtime_mode: summary?.runtime_mode,
    cluster_id: summary?.cluster_id,
    namespace: summary?.namespace,
    live_revision_id: summary?.live_revision_id,
    live_revision: summary?.live_revision,
    stable_revision_id: summary?.stable_revision_id,
    stable_revision: summary?.stable_revision,
    sync_state: summary?.sync_state ?? 'missing',
    sync_reason: summary?.sync_reason,
    public_urls: summary?.public_urls ?? [],
    nodes: (summary?.nodes ?? []).map(normalizeNode),
    services: (summary?.services ?? []).map(normalizeService),
  };
}
