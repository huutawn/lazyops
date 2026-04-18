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
        'rounded-[28px] bg-[radial-gradient(circle_at_top_left,_rgba(14,165,233,0.12),_transparent_32%),linear-gradient(180deg,rgba(15,23,42,0.88),rgba(2,6,23,0.92))] backdrop-blur-xl p-6 md:p-7',
        bordered && 'border border-[#1e293b]',
        'shadow-[0_24px_80px_rgba(2,6,23,0.4)]',
        className,
      )}
    >
      {(title || description || actions) && (
        <div className="mb-6 flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between">
          <div className="flex max-w-3xl flex-col gap-2">
            {title && <h3 className="text-xl font-black tracking-tight text-white">{title}</h3>}
            {description && <p className="text-[15px] font-medium leading-relaxed text-[#94a3b8]">{description}</p>}
          </div>
          {actions && <div className="flex flex-wrap items-center gap-3">{actions}</div>}
        </div>
      )}
      {children}
    </section>
  );
}
