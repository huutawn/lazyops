'use client';

import { useCallback, useMemo, useState } from 'react';
import { useParams } from 'next/navigation';
import {
  ReactFlow,
  Controls,
  Background,
  useNodesState,
  useEdgesState,
  type Node,
  type Edge,
  type NodeTypes,
  MarkerType,
  BackgroundVariant,
  Panel,
} from '@xyflow/react';
import '@xyflow/react/dist/style.css';
import { useTopology } from '@/modules/topology/topology-hooks';
import type { TopologyHealth, TopologyDisplayNode, TopologyDisplayEdge } from '@/modules/topology/topology-types';
import { PageHeader } from '@/components/primitives/page-header';
import { SectionCard } from '@/components/primitives/section-card';
import { EmptyState } from '@/components/primitives/empty-state';
import { ErrorState } from '@/components/primitives/error-state';
import { SkeletonPage } from '@/components/primitives/skeleton';
import { HealthChip } from '@/components/primitives/health-chip';
import { StatusBadge } from '@/components/primitives/status-badge';
import { Drawer } from '@/components/primitives/drawer';
import { TopologyNode, type TopologyNodeData } from '@/components/topology/topology-node';
import { TopologyEdgeLabel } from '@/components/topology/topology-edge-label';

type TopologyFlowNode = Node<TopologyNodeData, 'topologyNode'>;

const NODE_TYPES: NodeTypes = {
  topologyNode: TopologyNode,
};

const EDGE_TYPES = {
  topologyEdge: TopologyEdgeLabel,
};

const NODE_KIND_COLORS: Record<string, string> = {
  instance: '#7dd3fc',
  mesh_network: '#8de1b5',
  cluster: '#a78bfa',
  service: '#fbbf24',
};

const NODE_KIND_LABELS: Record<string, string> = {
  instance: 'Instance',
  mesh_network: 'Mesh Network',
  cluster: 'K3s Cluster',
  service: 'Service',
};

const STATUS_BORDER_COLORS: Record<TopologyHealth, string> = {
  healthy: '#8de1b5',
  degraded: '#fbbf24',
  unhealthy: '#f87171',
  offline: '#6366f1',
  unknown: '#94a3b8',
};

const STATUS_BG_COLORS: Record<TopologyHealth, string> = {
  healthy: 'rgba(141, 225, 181, 0.12)',
  degraded: 'rgba(251, 191, 36, 0.12)',
  unhealthy: 'rgba(248, 113, 113, 0.12)',
  offline: 'rgba(99, 102, 241, 0.12)',
  unknown: 'rgba(148, 163, 184, 0.08)',
};

const EDGE_KIND_LABELS: Record<string, string> = {
  dependency: 'Dependency',
  mesh_peer: 'Mesh Peer',
  routing: 'Routing',
};

function toReactFlowNodes(nodes: TopologyDisplayNode[]): TopologyFlowNode[] {
  const total = nodes.length;
  const cols = Math.min(total, 4);
  const rows = Math.ceil(total / cols);
  const xSpacing = 280;
  const ySpacing = 180;
  const startX = 100;
  const startY = 80;

  return nodes.map((node, i) => {
    const col = i % cols;
    const row = Math.floor(i / cols);
    return {
      id: node.id,
      type: 'topologyNode' as const,
      position: { x: startX + col * xSpacing, y: startY + row * ySpacing },
      data: {
        label: node.label,
        kind: node.kind,
        status: node.status,
        runtimeMode: node.runtime_mode,
        metadata: node.metadata,
        color: NODE_KIND_COLORS[node.kind] ?? '#94a3b8',
        borderColor: STATUS_BORDER_COLORS[node.status] ?? '#94a3b8',
        bgColor: STATUS_BG_COLORS[node.status] ?? 'rgba(148, 163, 184, 0.08)',
      },
      draggable: true,
      selectable: true,
    };
  });
}

function toReactFlowEdges(edges: TopologyDisplayEdge[], showLatency: boolean): Edge[] {
  return edges.map((edge) => ({
    id: edge.id,
    source: edge.source,
    target: edge.target,
    type: 'topologyEdge',
    markerEnd: { type: MarkerType.ArrowClosed, width: 16, height: 16, color: '#a6b7d4' },
    style: {
      stroke: edge.health === 'unhealthy' || edge.health === 'offline' ? '#f87171' : edge.health === 'degraded' ? '#fbbf24' : '#4a6080',
      strokeWidth: 2,
      strokeDasharray: edge.health === 'offline' ? '6 4' : undefined,
    },
    label: showLatency && edge.latency_ms != null ? `${edge.latency_ms}ms` : edge.protocol,
    labelStyle: { fill: '#a6b7d4', fontSize: 10 },
    labelBgStyle: { fill: 'rgba(8, 17, 31, 0.85)', rx: 4 },
    animated: edge.health === 'healthy',
  }));
}

export default function TopologyPage() {
  const params = useParams();
  const projectId = params?.projectId as string | undefined;
  const { data, isLoading, isError } = useTopology(projectId);

  const [nodeDrawer, setNodeDrawer] = useState<TopologyDisplayNode | null>(null);
  const [edgeDrawer, setEdgeDrawer] = useState<TopologyDisplayEdge | null>(null);
  const [showComparison, setShowComparison] = useState(false);
  const [planningMode, setPlanningMode] = useState(false);

  const rfNodes = useMemo(() => (data ? toReactFlowNodes(data.nodes) : []), [data]);
  const rfEdges = useMemo(
    () => (data ? toReactFlowEdges(data.edges, data.displayConfig.showLatency) : []),
    [data],
  );

  const [nodes, setNodes, onNodesChange] = useNodesState<TopologyFlowNode>(rfNodes);
  const [edges, setEdges, onEdgesChange] = useEdgesState(rfEdges);

  const handleNodeClick = useCallback(
    (_: React.MouseEvent, node: Node<TopologyNodeData>) => {
      const topoNode = data?.nodes.find((n) => n.id === node.id);
      if (topoNode) setNodeDrawer(topoNode);
    },
    [data],
  );

  const handleEdgeClick = useCallback(
    (_: React.MouseEvent, edge: Edge) => {
      const topoEdge = data?.edges.find((e) => e.id === edge.id);
      if (topoEdge) setEdgeDrawer(topoEdge);
    },
    [data],
  );

  if (isLoading) {
    return <SkeletonPage title cards={2} />;
  }

  if (isError || !data) {
    return (
      <div className="flex flex-col gap-6">
        <PageHeader title="Topology" subtitle="Service and infrastructure topology." />
        <ErrorState title="Failed to load topology" message="Could not fetch topology data." />
      </div>
    );
  }

  const { nodes: topoNodes, edges: topoEdges, runtime_mode, displayConfig, source } = data;

  if (topoNodes.length === 0) {
    return (
      <div className="flex flex-col gap-6">
        <PageHeader title="Topology" subtitle="Service and infrastructure topology." />
        <SectionCard title="No topology data" description="Topology data is generated from your targets and services.">
          <EmptyState
            title="No topology available"
            description="Add targets and services to see the topology graph."
          />
        </SectionCard>
      </div>
    );
  }

  const isStandalone = runtime_mode === 'standalone';

  return (
    <div className="flex flex-col gap-4">
      <PageHeader
        title="Topology"
        subtitle={`Runtime mode: ${runtime_mode} · ${topoNodes.length} nodes · ${topoEdges.length} edges · Source: ${source}`}
        actions={
          <div className="flex items-center gap-2">
            {!isStandalone && (
              <button
                type="button"
                className={`rounded-lg border px-3 py-1.5 text-xs font-medium transition-colors ${
                  showComparison
                    ? 'border-primary/40 bg-primary/10 text-primary'
                    : 'border-lazyops-border text-lazyops-muted hover:text-lazyops-text'
                }`}
                onClick={() => setShowComparison(!showComparison)}
              >
                Compare
              </button>
            )}
            {!isStandalone && (
              <button
                type="button"
                className={`rounded-lg border px-3 py-1.5 text-xs font-medium transition-colors ${
                  planningMode
                    ? 'border-primary/40 bg-primary/10 text-primary'
                    : 'border-lazyops-border text-lazyops-muted hover:text-lazyops-text'
                }`}
                onClick={() => setPlanningMode(!planningMode)}
              >
                Plan
              </button>
            )}
          </div>
        }
      />

      <div className="flex flex-wrap gap-2">
        <StatusBadge label={runtime_mode} variant="info" size="md" dot={false} />
        <StatusBadge label={`${topoNodes.length} nodes`} variant="neutral" size="md" dot={false} />
        <StatusBadge label={`${topoEdges.length} edges`} variant="neutral" size="md" dot={false} />
        <StatusBadge label={`Layout: ${displayConfig.layout}`} variant="neutral" size="md" dot={false} />
        {planningMode && <StatusBadge label="Planning mode" variant="warning" size="md" dot={false} />}
        {showComparison && <StatusBadge label="Comparison mode" variant="info" size="md" dot={false} />}
      </div>

      <SectionCard className="p-0 overflow-hidden" bordered>
        <div className="h-[600px] w-full" style={{ background: 'var(--bg)' }}>
          <ReactFlow
            nodes={nodes}
            edges={edges}
            onNodesChange={onNodesChange}
            onEdgesChange={onEdgesChange}
            onNodeClick={handleNodeClick}
            onEdgeClick={handleEdgeClick}
            nodeTypes={NODE_TYPES}
            edgeTypes={EDGE_TYPES}
            fitView
            fitViewOptions={{ padding: 0.2 }}
            minZoom={0.2}
            maxZoom={2}
            defaultEdgeOptions={{ type: 'topologyEdge' }}
            proOptions={{ hideAttribution: true }}
            nodesDraggable={planningMode}
          >
            <Controls
              className="!bg-lazyops-bg-accent !border-lazyops-border [&>button]:!bg-lazyops-bg-accent [&>button]:!border-lazyops-border [&>button]:!text-lazyops-muted [&>button:hover]:!text-lazyops-text [&>button]:!fill-lazyops-muted"
            />
            <Background
              variant={BackgroundVariant.Dots}
              gap={24}
              size={1.5}
              color="rgba(137, 179, 255, 0.08)"
            />
            <Panel position="top-right" className="flex gap-2">
              <StatusBadge label={runtime_mode} variant="info" size="sm" dot={false} />
              <StatusBadge label={source} variant="neutral" size="sm" dot={false} />
            </Panel>
          </ReactFlow>
        </div>
      </SectionCard>

      {showComparison && !isStandalone && (
        <SectionCard title="Current vs Desired topology" description="Comparison between running and intended state.">
          <div className="grid gap-4 sm:grid-cols-2">
            <div className="rounded-lg border border-health-healthy/30 bg-health-healthy/5 p-4">
              <h4 className="mb-2 text-sm font-medium text-health-healthy">Current</h4>
              <div className="flex flex-col gap-1 text-xs text-lazyops-muted">
                <span>{topoNodes.length} nodes active</span>
                <span>{topoEdges.length} connections</span>
                <span>{topoNodes.filter((n) => n.status === 'healthy').length} healthy</span>
                <span>{topoNodes.filter((n) => n.status === 'degraded').length} degraded</span>
                <span>{topoNodes.filter((n) => n.status === 'unhealthy' || n.status === 'offline').length} unhealthy</span>
              </div>
            </div>
            <div className="rounded-lg border border-lazyops-border bg-lazyops-bg-accent/30 p-4">
              <h4 className="mb-2 text-sm font-medium text-lazyops-text">Desired</h4>
              <div className="flex flex-col gap-1 text-xs text-lazyops-muted">
                <span>{topoNodes.length} nodes expected</span>
                <span>{topoEdges.length} connections expected</span>
                <span>All nodes should be healthy</span>
                <span>No degraded or unhealthy nodes</span>
              </div>
            </div>
          </div>
        </SectionCard>
      )}

      {planningMode && !isStandalone && (
        <SectionCard title="Planning mode" description="Drag nodes to adjust placement. Changes are local until saved.">
          <div className="flex items-center justify-between">
            <p className="text-sm text-lazyops-muted">
              Drag nodes to reposition them. This does not affect the actual deployment.
            </p>
            <button
              type="button"
              className="rounded-lg bg-primary px-4 py-2 text-sm font-semibold text-lazyops-bg transition-colors hover:bg-primary/90"
              onClick={() => {
                setPlanningMode(false);
                setNodes(rfNodes);
              }}
            >
              Reset positions
            </button>
          </div>
        </SectionCard>
      )}

      <SectionCard title="Legend" description="Node types and status meanings.">
        <div className="grid gap-4 sm:grid-cols-2">
          <div>
            <h4 className="mb-2 text-sm font-medium text-lazyops-text">Node Types</h4>
            <div className="flex flex-col gap-2">
              {Object.entries(NODE_KIND_COLORS).map(([kind, color]) => (
                <div key={kind} className="flex items-center gap-2">
                  <div className="size-3 rounded-full" style={{ backgroundColor: color }} />
                  <span className="text-xs text-lazyops-muted">{NODE_KIND_LABELS[kind] ?? kind}</span>
                </div>
              ))}
            </div>
          </div>
          <div>
            <h4 className="mb-2 text-sm font-medium text-lazyops-text">Status</h4>
            <div className="flex flex-col gap-2">
              {(Object.entries(STATUS_BORDER_COLORS) as [TopologyHealth, string][]).map(([status]) => (
                <div key={status} className="flex items-center gap-2">
                  <HealthChip label={status} status={status} size="sm" />
                </div>
              ))}
            </div>
          </div>
        </div>
      </SectionCard>

      <Drawer
        open={!!nodeDrawer}
        onClose={() => setNodeDrawer(null)}
        title={nodeDrawer?.label ?? 'Node details'}
        size="md"
      >
        {nodeDrawer && <NodeDetailDrawer node={nodeDrawer} />}
      </Drawer>

      <Drawer
        open={!!edgeDrawer}
        onClose={() => setEdgeDrawer(null)}
        title="Edge details"
        size="md"
      >
        {edgeDrawer && <EdgeDetailDrawer edge={edgeDrawer} nodes={topoNodes} />}
      </Drawer>
    </div>
  );
}

function NodeDetailDrawer({ node }: { node: TopologyDisplayNode }) {
  return (
    <div className="flex flex-col gap-4">
      <div className="flex items-center gap-3">
        <div
          className="size-4 rounded-full"
          style={{ backgroundColor: NODE_KIND_COLORS[node.kind] ?? '#94a3b8' }}
        />
        <div>
          <h4 className="text-base font-semibold text-lazyops-text">{node.label}</h4>
          <span className="text-xs text-lazyops-muted">{NODE_KIND_LABELS[node.kind] ?? node.kind}</span>
        </div>
      </div>

      <div className="flex items-center gap-2">
        <span className="text-xs text-lazyops-muted">Status:</span>
        <HealthChip label={node.status} status={node.status} size="sm" />
      </div>

      {node.runtime_mode && (
        <div className="flex items-center gap-2">
          <span className="text-xs text-lazyops-muted">Runtime:</span>
          <StatusBadge label={node.runtime_mode} variant="info" size="sm" dot={false} />
        </div>
      )}

      {Object.keys(node.metadata).length > 0 && (
        <div>
          <h5 className="mb-2 text-sm font-medium text-lazyops-text">Metadata</h5>
          <div className="rounded-lg border border-lazyops-border bg-lazyops-bg-accent/30 p-3">
            <div className="flex flex-col gap-1">
              {Object.entries(node.metadata).map(([k, v]) => (
                <div key={k} className="flex items-center justify-between text-xs">
                  <span className="text-lazyops-muted">{k}</span>
                  <span className="font-mono text-lazyops-text">{String(v)}</span>
                </div>
              ))}
            </div>
          </div>
        </div>
      )}

      <div className="rounded-lg border border-lazyops-border/50 bg-lazyops-bg-accent/30 p-3 text-xs text-lazyops-muted">
        <p className="mb-1 font-medium text-lazyops-text">Node ID</p>
        <code className="font-mono">{node.id}</code>
      </div>
    </div>
  );
}

function EdgeDetailDrawer({ edge, nodes }: { edge: TopologyDisplayEdge; nodes: TopologyDisplayNode[] }) {
  const sourceNode = nodes.find((n) => n.id === edge.source);
  const targetNode = nodes.find((n) => n.id === edge.target);

  return (
    <div className="flex flex-col gap-4">
      <div className="flex items-center gap-2 text-sm">
        <span className="font-medium text-lazyops-text">{sourceNode?.label ?? edge.source}</span>
        <span className="text-lazyops-muted">→</span>
        <span className="font-medium text-lazyops-text">{targetNode?.label ?? edge.target}</span>
      </div>

      <div className="grid gap-2 sm:grid-cols-2">
        <SummaryField label="Type" value={EDGE_KIND_LABELS[edge.edge_kind] ?? edge.edge_kind} />
        <SummaryField label="Protocol" value={edge.protocol} />
        {edge.latency_ms != null && <SummaryField label="Latency" value={`${edge.latency_ms}ms`} />}
        <SummaryField label="Health" value={edge.health} />
      </div>

      <div className="rounded-lg border border-lazyops-border/50 bg-lazyops-bg-accent/30 p-3 text-xs text-lazyops-muted">
        <p className="mb-1 font-medium text-lazyops-text">Route detail</p>
        <p>
          {sourceNode?.label} communicates with {targetNode?.label} via {edge.protocol}.
          {edge.latency_ms != null && ` Average latency is ${edge.latency_ms}ms.`}
        </p>
      </div>

      {edge.health === 'degraded' && (
        <div className="rounded-lg border border-health-degraded/30 bg-health-degraded/10 p-3 text-xs text-health-degraded">
          <p className="font-medium">Degraded connection</p>
          <p className="mt-1 text-lazyops-muted">
            This connection is experiencing higher than normal latency or intermittent failures.
          </p>
        </div>
      )}

      {edge.health === 'unhealthy' && (
        <div className="rounded-lg border border-health-unhealthy/30 bg-health-unhealthy/10 p-3 text-xs text-health-unhealthy">
          <p className="font-medium">Unhealthy connection</p>
          <p className="mt-1 text-lazyops-muted">
            This connection is failing or experiencing significant errors.
          </p>
        </div>
      )}
    </div>
  );
}

function SummaryField({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex flex-col gap-0.5">
      <span className="text-xs text-lazyops-muted">{label}</span>
      <span className="text-sm text-lazyops-text">{value}</span>
    </div>
  );
}
