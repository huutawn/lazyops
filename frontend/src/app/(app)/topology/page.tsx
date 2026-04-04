'use client';

import { useCallback, useMemo } from 'react';
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
import { TopologyNode, type TopologyNodeData } from '@/components/topology/topology-node';
import { TopologyEdgeLabel } from '@/components/topology/topology-edge-label';

type TopologyNode = Node<TopologyNodeData, 'topologyNode'>;

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

function toReactFlowNodes(nodes: TopologyDisplayNode[]): TopologyNode[] {
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

  const rfNodes = useMemo(() => (data ? toReactFlowNodes(data.nodes) : []), [data]);
  const rfEdges = useMemo(
    () => (data ? toReactFlowEdges(data.edges, data.displayConfig.showLatency) : []),
    [data],
  );

  const [nodes, setNodes, onNodesChange] = useNodesState<TopologyNode>(rfNodes);
  const [edges, setEdges, onEdgesChange] = useEdgesState(rfEdges);

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

  return (
    <div className="flex flex-col gap-4">
      <PageHeader
        title="Topology"
        subtitle={`Runtime mode: ${runtime_mode} · ${topoNodes.length} nodes · ${topoEdges.length} edges · Source: ${source}`}
      />

      <div className="flex flex-wrap gap-2">
        <StatusBadge label={runtime_mode} variant="info" size="md" dot={false} />
        <StatusBadge label={`${topoNodes.length} nodes`} variant="neutral" size="md" dot={false} />
        <StatusBadge label={`${topoEdges.length} edges`} variant="neutral" size="md" dot={false} />
        <StatusBadge label={`Layout: ${displayConfig.layout}`} variant="neutral" size="md" dot={false} />
      </div>

      <SectionCard className="p-0 overflow-hidden" bordered>
        <div className="h-[600px] w-full" style={{ background: 'var(--bg)' }}>
          <ReactFlow
            nodes={nodes}
            edges={edges}
            onNodesChange={onNodesChange}
            onEdgesChange={onEdgesChange}
            nodeTypes={NODE_TYPES}
            edgeTypes={EDGE_TYPES}
            fitView
            fitViewOptions={{ padding: 0.2 }}
            minZoom={0.2}
            maxZoom={2}
            defaultEdgeOptions={{ type: 'topologyEdge' }}
            proOptions={{ hideAttribution: true }}
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

      <SectionCard title="Legend" description="Node types and status meanings.">
        <div className="grid gap-4 sm:grid-cols-2">
          <div>
            <h4 className="mb-2 text-sm font-medium text-lazyops-text">Node Types</h4>
            <div className="flex flex-col gap-2">
              {Object.entries(NODE_KIND_COLORS).map(([kind, color]) => (
                <div key={kind} className="flex items-center gap-2">
                  <div className="size-3 rounded-full" style={{ backgroundColor: color }} />
                  <span className="text-xs text-lazyops-muted">{kind.replace(/_/g, ' ')}</span>
                </div>
              ))}
            </div>
          </div>
          <div>
            <h4 className="mb-2 text-sm font-medium text-lazyops-text">Status</h4>
            <div className="flex flex-col gap-2">
              {(Object.entries(STATUS_BORDER_COLORS) as [TopologyHealth, string][]).map(([status, color]) => (
                <div key={status} className="flex items-center gap-2">
                  <HealthChip label={status} status={status} size="sm" />
                </div>
              ))}
            </div>
          </div>
        </div>
      </SectionCard>
    </div>
  );
}
