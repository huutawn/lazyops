import type { ReactNode } from 'react';
import { cn } from '@/lib/utils';

type PageHeaderProps = {
  title: ReactNode;
  subtitle?: ReactNode;
  breadcrumb?: ReactNode;
  actions?: ReactNode;
  className?: string;
};

export function PageHeader({ title, subtitle, breadcrumb, actions, className }: PageHeaderProps) {
  return (
    <header className={cn('relative flex flex-col gap-5 pb-6', className)}>
      <div className="pointer-events-none absolute -top-10 right-0 h-24 w-24 rounded-full bg-[#0EA5E9]/10 blur-3xl" />
      {breadcrumb && <nav className="text-base font-medium text-lazyops-muted">{breadcrumb}</nav>}
      <div className="relative flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between">
        <div className="flex max-w-4xl flex-col gap-2">
          <h1 className="text-4xl font-black tracking-tight text-lazyops-text sm:text-4xl">{title}</h1>
          {subtitle && <p className="max-w-3xl text-[15px] leading-relaxed text-lazyops-muted">{subtitle}</p>}
        </div>
        {actions && <div className="relative flex flex-wrap items-center gap-2">{actions}</div>}
      </div>
    </header>
  );
}
