import type { ReactNode } from 'react';
import { cn } from '@/lib/utils';

type DrawerProps = {
  open: boolean;
  onClose: () => void;
  title: string;
  children: ReactNode;
  side?: 'right' | 'left' | 'bottom';
  size?: 'sm' | 'md' | 'lg';
};

export function Drawer({ open, onClose, title, children, side = 'right', size = 'md' }: DrawerProps) {
  if (!open) return null;

  const sizeClasses = {
    sm: 'w-80',
    md: 'w-96',
    lg: 'w-[520px]',
  };

  const positionClasses = {
    right: 'right-0 top-0 h-full border-l',
    left: 'left-0 top-0 h-full border-r',
    bottom: 'bottom-0 left-0 right-0 border-t',
  };

  return (
    <div className="fixed inset-0 z-50">
      <div className="absolute inset-0 bg-black/50" onClick={onClose} />
      <div
        className={cn(
          'absolute bg-lazyops-bg shadow-2xl backdrop-blur-sm transition-transform',
          sizeClasses[size],
          positionClasses[side],
        )}
      >
        <div className="flex h-12 items-center justify-between border-b border-lazyops-border px-5">
          <h3 className="text-sm font-semibold text-lazyops-text">{title}</h3>
          <button
            type="button"
            className="rounded-md p-1 text-lazyops-muted transition-colors hover:bg-lazyops-border/30 hover:text-lazyops-text"
            onClick={onClose}
            aria-label="Close"
          >
            <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
              <line x1="18" y1="6" x2="6" y2="18" />
              <line x1="6" y1="6" x2="18" y2="18" />
            </svg>
          </button>
        </div>
        <div className="overflow-y-auto p-5" style={{ maxHeight: 'calc(100vh - 48px)' }}>
          {children}
        </div>
      </div>
    </div>
  );
}
