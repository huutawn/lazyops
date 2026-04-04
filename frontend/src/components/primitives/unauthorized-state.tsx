import type { ReactNode } from 'react';
import { cn } from '@/lib/utils';

type UnauthorizedStateProps = {
  message?: string;
  action?: ReactNode;
  className?: string;
};

export function UnauthorizedState({
  message = 'You are not authorized to view this page.',
  action,
  className,
}: UnauthorizedStateProps) {
  return (
    <div
      className={cn(
        'flex flex-col items-center justify-center gap-4 rounded-xl border border-health-degraded/30 bg-health-degraded/10 py-12 text-center',
        className,
      )}
      role="alert"
    >
      <h3 className="text-lg font-medium text-health-degraded">Access denied</h3>
      <p className="max-w-sm text-sm text-lazyops-muted">{message}</p>
      {action && <div className="mt-2">{action}</div>}
    </div>
  );
}
