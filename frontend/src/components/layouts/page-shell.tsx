import type { ReactNode } from 'react';
import { cn } from '@/lib/utils';

type PageShellProps = {
  title?: string;
  subtitle?: string;
  actions?: ReactNode;
  children: ReactNode;
  className?: string;
};

export function PageShell({ title, subtitle, actions, children, className }: PageShellProps) {
  return (
    <div className={cn('flex min-h-screen flex-col p-8', className)}>
      {(title || subtitle || actions) && (
        <header className="mb-6 flex flex-wrap items-start justify-between gap-4">
          <div className="flex flex-col gap-1">
            {title && <h1 className="text-3xl font-semibold tracking-tight">{title}</h1>}
            {subtitle && <p className="text-sm text-lazyops-muted">{subtitle}</p>}
          </div>
          {actions && <div className="flex items-center gap-2">{actions}</div>}
        </header>
      )}
      <div className="flex-1">{children}</div>
    </div>
  );
}
