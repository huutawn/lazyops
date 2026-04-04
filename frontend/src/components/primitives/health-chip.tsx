import { cva, type VariantProps } from 'class-variance-authority';
import { cn } from '@/lib/utils';

const healthChipVariants = cva(
  'inline-flex items-center gap-2 rounded-lg border px-3 py-1.5 text-sm font-medium transition-colors',
  {
    variants: {
      status: {
        healthy: 'border-health-healthy/30 bg-health-healthy/10 text-health-healthy',
        degraded: 'border-health-degraded/30 bg-health-degraded/10 text-health-degraded',
        unhealthy: 'border-health-unhealthy/30 bg-health-unhealthy/10 text-health-unhealthy',
        offline: 'border-health-offline/30 bg-health-offline/10 text-health-offline',
        unknown: 'border-health-unknown/30 bg-health-unknown/10 text-health-unknown',
      },
      size: {
        sm: 'px-2 py-1 text-xs',
        md: 'px-3 py-1.5 text-sm',
        lg: 'px-4 py-2 text-base',
      },
    },
    defaultVariants: {
      status: 'unknown',
      size: 'md',
    },
  },
);

const healthIndicatorVariants = cva('rounded-full', {
  variants: {
    status: {
      healthy: 'bg-health-healthy',
      degraded: 'bg-health-degraded',
      unhealthy: 'bg-health-unhealthy',
      offline: 'bg-health-offline',
      unknown: 'bg-health-unknown',
    },
    size: {
      sm: 'size-2',
      md: 'size-2.5',
      lg: 'size-3',
    },
  },
  defaultVariants: {
    status: 'unknown',
    size: 'md',
  },
});

export type HealthChipProps = VariantProps<typeof healthChipVariants> & {
  label: string;
  className?: string;
};

export function HealthChip({ label, status, size, className }: HealthChipProps) {
  return (
    <span className={cn(healthChipVariants({ status, size }), className)}>
      <span className={cn(healthIndicatorVariants({ status, size }))} />
      {label}
    </span>
  );
}
