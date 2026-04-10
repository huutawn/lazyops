import { cva, type VariantProps } from 'class-variance-authority';
import { cn } from '@/lib/utils';

const statusBadgeVariants = cva(
  'inline-flex items-center gap-1.5 rounded-full px-2.5 py-0.5 text-xs font-medium transition-all duration-200 border shadow-sm',
  {
    variants: {
      variant: {
        default: 'bg-[#1e293b]/40 text-[#94a3b8] border-[#334155]/30',
        success: 'bg-[#10b981]/10 text-[#10b981] border-[#10b981]/20',
        warning: 'bg-[#eab308]/10 text-[#eab308] border-[#eab308]/20',
        danger: 'bg-[#ef4444]/10 text-[#ef4444] border-[#ef4444]/20',
        info: 'bg-[#0ea5e9]/10 text-[#0ea5e9] border-[#0ea5e9]/20',
        neutral: 'bg-[#64748b]/10 text-[#64748b] border-[#64748b]/20',
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

const statusDotVariants = cva('size-2 rounded-full shadow-[0_0_5px_currentColor]', {
  variants: {
    variant: {
      default: 'bg-[#64748b]',
      success: 'bg-[#10b981]',
      warning: 'bg-[#eab308]',
      danger: 'bg-[#ef4444]',
      info: 'bg-[#0ea5e9]',
      neutral: 'bg-[#64748b]',
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
