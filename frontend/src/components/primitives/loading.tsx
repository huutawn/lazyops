import { cn } from '@/lib/utils';

type LoadingBlockProps = {
  label?: string;
  className?: string;
};

export function LoadingBlock({ label = 'Loading…', className }: LoadingBlockProps) {
  return (
    <div
      className={cn('flex flex-col items-center gap-3 py-12', className)}
      role="status"
      aria-live="polite"
    >
      <div className="size-8 animate-spin rounded-full border-2 border-lazyops-border border-t-primary" />
      <span className="text-sm text-lazyops-muted">{label}</span>
    </div>
  );
}

type LoadingPageProps = {
  label?: string;
  className?: string;
};

export function LoadingPage({ label, className }: LoadingPageProps) {
  return (
    <div className={cn('flex min-h-[60vh] items-center justify-center', className)}>
      <LoadingBlock label={label} />
    </div>
  );
}
