import type { ReactNode } from 'react';
import { cn } from '@/lib/utils';

type SkeletonProps = {
  className?: string;
};

export function Skeleton({ className }: SkeletonProps) {
  return (
    <div
      className={cn('animate-pulse rounded-md bg-lazyops-border/30', className)}
      aria-hidden="true"
    />
  );
}

type SkeletonLineProps = {
  lines?: number;
  className?: string;
};

export function SkeletonLine({ lines = 1, className }: SkeletonLineProps) {
  return (
    <div className="flex flex-col gap-2">
      {Array.from({ length: lines }).map((_, i) => (
        <Skeleton key={i} className={cn('h-4 w-full', className)} />
      ))}
    </div>
  );
}

type SkeletonCardProps = {
  count?: number;
};

export function SkeletonCard({ count = 3 }: SkeletonCardProps) {
  return (
    <div className="flex flex-col gap-4">
      {Array.from({ length: count }).map((_, i) => (
        <div key={i} className="rounded-xl border border-lazyops-border bg-lazyops-card p-5">
          <Skeleton className="mb-3 h-5 w-1/3" />
          <SkeletonLine lines={2} />
        </div>
      ))}
    </div>
  );
}

type SkeletonPageProps = {
  title?: boolean;
  cards?: number;
};

export function SkeletonPage({ title = true, cards = 3 }: SkeletonPageProps) {
  return (
    <div className="flex flex-col gap-6">
      {title && (
        <div className="flex flex-col gap-2">
          <Skeleton className="h-8 w-48" />
          <Skeleton className="h-4 w-72" />
        </div>
      )}
      <SkeletonCard count={cards} />
    </div>
  );
}
