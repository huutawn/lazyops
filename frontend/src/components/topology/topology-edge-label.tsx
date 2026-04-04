import { memo } from 'react';
import { BaseEdge, EdgeLabelRenderer, type EdgeProps } from '@xyflow/react';

export const TopologyEdgeLabel = memo(
  ({
    id,
    sourceX,
    sourceY,
    targetX,
    targetY,
    sourcePosition,
    targetPosition,
    style = {},
    markerEnd,
    label,
  }: EdgeProps) => {
    const edgeCenterX = (sourceX + targetX) / 2;
    const edgeCenterY = (sourceY + targetY) / 2;

    return (
      <>
        <BaseEdge path={`M ${sourceX},${sourceY} ${targetX},${targetY}`} style={style} markerEnd={markerEnd} />
        {label && (
          <EdgeLabelRenderer>
            <div
              className="pointer-events-none absolute -translate-x-1/2 -translate-y-1/2 rounded px-1.5 py-0.5 text-[10px] font-medium"
              style={{
                transform: `translate(-50%, -50%) translate(${edgeCenterX}px,${edgeCenterY}px)`,
                background: 'rgba(8, 17, 31, 0.85)',
                color: '#a6b7d4',
                border: '1px solid rgba(137, 179, 255, 0.15)',
              }}
            >
              {label}
            </div>
          </EdgeLabelRenderer>
        )}
      </>
    );
  },
);

TopologyEdgeLabel.displayName = 'TopologyEdgeLabel';
