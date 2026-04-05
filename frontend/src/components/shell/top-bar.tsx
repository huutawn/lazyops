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
    <header className="sticky top-0 z-30 flex h-16 items-center justify-between border-b border-border/40 bg-background/60 px-6 backdrop-blur-xl transition-all supports-[backdrop-filter]:bg-background/60">
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

      <UserMenu />
    </header>
  );
}
