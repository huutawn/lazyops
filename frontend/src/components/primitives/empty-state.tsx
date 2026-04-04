import type { ReactNode } from 'react';
import { cn } from '@/lib/utils';

type EmptyStateProps = {
  title: string;
  description: string;
  action?: ReactNode;
  icon?: ReactNode;
  className?: string;
};

export function EmptyState({ title, description, action, icon, className }: EmptyStateProps) {
  return (
    <div className={cn('flex flex-col items-center justify-center gap-4 py-12 text-center', className)}>
      {icon && <div className="text-lazyops-muted/60">{icon}</div>}
      <div className="flex flex-col gap-1">
        <h3 className="text-lg font-medium text-lazyops-text">{title}</h3>
        <p className="max-w-sm text-sm text-lazyops-muted">{description}</p>
      </div>
      {action && <div className="mt-2">{action}</div>}
    </div>
  );
}
