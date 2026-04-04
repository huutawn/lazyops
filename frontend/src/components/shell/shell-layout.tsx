'use client';

import type { ReactNode } from 'react';
import { Sidebar } from './sidebar';
import { TopBar } from './top-bar';
import { useMobileSidebar } from './use-mobile-sidebar';
import { cn } from '@/lib/utils';

type ShellLayoutProps = {
  children: ReactNode;
  breadcrumb?: ReactNode;
  className?: string;
};

export function ShellLayout({ children, breadcrumb, className }: ShellLayoutProps) {
  const { open: mobileOpen, toggle, close } = useMobileSidebar();

  return (
    <div className="flex min-h-screen">
      <Sidebar mobileOpen={mobileOpen} onClose={close} />

      <div className="flex min-w-0 flex-1 flex-col">
        <TopBar onMenuClick={toggle} breadcrumb={breadcrumb} />
        <main className={cn('flex-1 p-4 lg:p-6', className)}>{children}</main>
      </div>
    </div>
  );
}
