import { memo } from 'react';
import { Handle, Position, type Node, type NodeProps } from '@xyflow/react';
import { cn } from '@/lib/utils';

export type TopologyNodeData = {
  label: string;
  kind: string;
  status: string;
  runtimeMode?: string;
  metadata: Record<string, unknown>;
  color: string;
  borderColor: string;
  bgColor: string;
};

type TopologyNode = Node<TopologyNodeData, 'topologyNode'>;

export const TopologyNode = memo(({ data }: NodeProps<TopologyNode>) => {
  const { label, kind, status, color, borderColor, bgColor } = data;

  const isDegraded = status === 'degraded';
  const isUnhealthy = status === 'unhealthy';
  const isOffline = status === 'offline';

  return (
    <div
      className={cn(
        'min-w-[180px] rounded-xl border-2 bg-lazyops-bg-accent/95 px-4 py-3 backdrop-blur-sm transition-shadow',
        isDegraded && 'animate-pulse',
        isUnhealthy && 'animate-pulse',
        isOffline && 'opacity-60',
      )}
      style={{ borderColor, backgroundColor: bgColor }}
    >
      <Handle type="target" position={Position.Top} className="!w-3 !h-3 !bg-lazyops-border" />

      <div className="flex items-center gap-2 mb-1">
        <div className="size-2.5 rounded-full" style={{ backgroundColor: color }} />
        <span className="text-sm font-semibold text-lazyops-text">{label}</span>
      </div>

      <div className="flex items-center justify-between">
        <span className="text-[10px] uppercase tracking-wider text-lazyops-muted/70">
          {kind.replace(/_/g, ' ')}
        </span>
        <div
          className={cn(
            'size-2 rounded-full',
            status === 'healthy' && 'bg-health-healthy',
            status === 'degraded' && 'bg-health-degraded',
            status === 'unhealthy' && 'bg-health-unhealthy',
            status === 'offline' && 'bg-health-offline',
          )}
        />
      </div>

      <Handle type="source" position={Position.Bottom} className="!w-3 !h-3 !bg-lazyops-border" />
    </div>
  );
});

TopologyNode.displayName = 'TopologyNode';
