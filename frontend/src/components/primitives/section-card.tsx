import type { ReactNode } from 'react';
import { cn } from '@/lib/utils';

type SectionCardProps = {
  title?: ReactNode;
  description?: string;
  actions?: ReactNode;
  children: ReactNode;
  className?: string;
  bordered?: boolean;
};

export function SectionCard({
  title,
  description,
  actions,
  children,
  className,
  bordered = true,
}: SectionCardProps) {
  return (
    <section
      className={cn(
        'rounded-xl bg-lazyops-card p-5 backdrop-blur-sm',
        bordered && 'border border-lazyops-border',
        'shadow-lg',
        className,
      )}
    >
      {(title || description || actions) && (
        <div className="mb-4 flex items-start justify-between gap-4">
          <div className="flex flex-col gap-0.5">
            {title && <h3 className="text-base font-medium text-lazyops-text">{title}</h3>}
            {description && <p className="text-sm text-lazyops-muted">{description}</p>}
          </div>
          {actions && <div className="flex items-center gap-2">{actions}</div>}
        </div>
      )}
      {children}
    </section>
  );
}
