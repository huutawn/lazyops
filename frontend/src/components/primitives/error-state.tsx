import type { ReactNode } from 'react';
import { cn } from '@/lib/utils';

type ErrorStateProps = {
  title?: string;
  message?: string;
  action?: ReactNode;
  className?: string;
};

export function ErrorState({
  title = 'Something went wrong',
  message = 'An unexpected error occurred. Please try again.',
  action,
  className,
}: ErrorStateProps) {
  return (
    <div
      className={cn(
        'flex flex-col items-center justify-center gap-4 rounded-xl border border-health-unhealthy/30 bg-health-unhealthy/10 py-12 text-center',
        className,
      )}
      role="alert"
    >
      <h3 className="text-lg font-medium text-health-unhealthy">{title}</h3>
      <p className="max-w-sm text-sm text-lazyops-muted">{message}</p>
      {action && <div className="mt-2">{action}</div>}
    </div>
  );
}
