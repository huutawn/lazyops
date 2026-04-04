'use client';

import { UserMenu } from '@/components/session/user-menu';
import { cn } from '@/lib/utils';

type TopBarProps = {
  onMenuClick: () => void;
  breadcrumb?: React.ReactNode;
};

export function TopBar({ onMenuClick, breadcrumb }: TopBarProps) {
  return (
    <header className="sticky top-0 z-30 flex h-14 items-center justify-between border-b border-lazyops-border bg-lazyops-bg-accent/90 px-4 backdrop-blur-sm lg:px-6">
      <div className="flex items-center gap-3">
        <button
          type="button"
          className="flex size-8 items-center justify-center rounded-md text-lazyops-muted transition-colors hover:bg-lazyops-border/30 hover:text-lazyops-text lg:hidden"
          onClick={onMenuClick}
          aria-label="Toggle navigation"
        >
          <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
            <line x1="3" y1="6" x2="21" y2="6" />
            <line x1="3" y1="12" x2="21" y2="12" />
            <line x1="3" y1="18" x2="21" y2="18" />
          </svg>
        </button>

        {breadcrumb && <div className="text-sm text-lazyops-muted">{breadcrumb}</div>}
      </div>

      <UserMenu />
    </header>
  );
}
