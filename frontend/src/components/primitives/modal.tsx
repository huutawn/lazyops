'use client';

import { useState } from 'react';
import { cn } from '@/lib/utils';

type ModalProps = {
  open: boolean;
  onClose: () => void;
  title: string;
  children: React.ReactNode;
  size?: 'sm' | 'md' | 'lg';
};

export function Modal({ open, onClose, title, children, size = 'md' }: ModalProps) {
  if (!open) return null;

  const sizeClasses = {
    sm: 'max-w-md',
    md: 'max-w-lg',
    lg: 'max-w-2xl',
  };

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4" role="dialog" aria-modal="true">
      <div className="absolute inset-0 bg-black/60" onClick={onClose} />
      <div
        className={cn(
          'relative w-full rounded-xl border border-lazyops-border bg-lazyops-bg-accent shadow-2xl backdrop-blur-sm',
          sizeClasses[size],
        )}
      >
        <div className="flex items-center justify-between border-b border-lazyops-border px-6 py-4">
          <h2 className="text-lg font-semibold text-lazyops-text">{title}</h2>
          <button
            type="button"
            className="rounded-md p-1 text-lazyops-muted transition-colors hover:bg-lazyops-border/30 hover:text-lazyops-text"
            onClick={onClose}
            aria-label="Close"
          >
            <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
              <line x1="18" y1="6" x2="6" y2="18" />
              <line x1="6" y1="6" x2="18" y2="18" />
            </svg>
          </button>
        </div>
        <div className="px-6 py-4">{children}</div>
      </div>
    </div>
  );
}
