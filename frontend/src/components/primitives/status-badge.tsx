import { cva, type VariantProps } from 'class-variance-authority';
import { cn } from '@/lib/utils';

const statusBadgeVariants = cva(
  'inline-flex items-center gap-1.5 rounded-full px-2.5 py-0.5 text-xs font-medium transition-colors',
  {
    variants: {
      variant: {
        default: 'bg-lazyops-border/30 text-lazyops-text',
        success: 'bg-health-healthy/15 text-health-healthy',
        warning: 'bg-health-degraded/15 text-health-degraded',
        danger: 'bg-health-unhealthy/15 text-health-unhealthy',
        info: 'bg-rollout-progress/15 text-rollout-progress',
        neutral: 'bg-lazyops-muted/10 text-lazyops-muted',
      },
      size: {
        sm: 'px-2 py-0.5 text-[10px]',
        md: 'px-2.5 py-0.5 text-xs',
        lg: 'px-3 py-1 text-sm',
      },
    },
    defaultVariants: {
      variant: 'default',
      size: 'md',
    },
  },
);

const statusDotVariants = cva('size-2 rounded-full', {
  variants: {
    variant: {
      default: 'bg-lazyops-muted',
      success: 'bg-health-healthy',
      warning: 'bg-health-degraded',
      danger: 'bg-health-unhealthy',
      info: 'bg-rollout-progress',
      neutral: 'bg-lazyops-muted',
    },
    size: {
      sm: 'size-1.5',
      md: 'size-2',
      lg: 'size-2.5',
    },
  },
  defaultVariants: {
    variant: 'default',
    size: 'md',
  },
});

export type StatusBadgeProps = VariantProps<typeof statusBadgeVariants> & {
  label: string;
  dot?: boolean;
  className?: string;
};

export function StatusBadge({ label, variant, size, dot = true, className }: StatusBadgeProps) {
  return (
    <span className={cn(statusBadgeVariants({ variant, size }), className)}>
      {dot && <span className={cn(statusDotVariants({ variant, size }))} />}
      {label}
    </span>
  );
}
