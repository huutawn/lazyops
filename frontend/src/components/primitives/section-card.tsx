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
        'rounded-2xl bg-[#0F172A]/60 backdrop-blur-xl p-6',
        bordered && 'border border-[#1e293b]',
        'shadow-2xl',
        className,
      )}
    >
      {(title || description || actions) && (
        <div className="mb-6 flex items-start justify-between gap-4">
          <div className="flex flex-col gap-1">
            {title && <h3 className="text-lg font-bold text-white tracking-tight">{title}</h3>}
            {description && <p className="text-[14px] text-[#94a3b8] font-medium leading-relaxed">{description}</p>}
          </div>
          {actions && <div className="flex items-center gap-3">{actions}</div>}
        </div>
      )}
      {children}
    </section>
  );
}
