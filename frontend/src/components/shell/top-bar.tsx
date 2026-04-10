'use client';

import { Menu } from 'lucide-react';
import { UserMenu } from '@/components/session/user-menu';
import { cn } from '@/lib/utils';

type TopBarProps = {
  onMenuClick: () => void;
  breadcrumb?: React.ReactNode;
};

export function TopBar({ onMenuClick, breadcrumb }: TopBarProps) {
  return (
    <header className="sticky top-0 z-30 flex h-20 items-center justify-between border-b border-[#1e293b] bg-[#0B1120] px-6 transition-all">
      <div className="flex items-center gap-4">
        <button
          type="button"
          className="flex size-9 items-center justify-center rounded-lg bg-card border border-border/50 text-muted-foreground transition-all hover:bg-accent hover:text-accent-foreground lg:hidden shadow-sm"
          onClick={onMenuClick}
          aria-label="Toggle navigation"
        >
          <Menu className="size-5" />
        </button>

        {breadcrumb && (
          <nav className="flex items-center text-sm font-medium text-muted-foreground animate-in fade-in">
            {breadcrumb}
          </nav>
        )}
      </div>

      <div className="flex items-center">
        <UserMenu />
      </div>
    </header>
  );
}
