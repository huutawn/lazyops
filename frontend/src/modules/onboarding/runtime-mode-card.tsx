import { cn } from '@/lib/utils';
import type { RuntimeModeInfo } from '@/modules/onboarding/runtime-modes';

type RuntimeModeCardProps = RuntimeModeInfo & {
  selected?: boolean;
  onClick?: () => void;
};

export function RuntimeModeCard({ title, description, useCase, icon, selected, onClick }: RuntimeModeCardProps) {
  return (
    <button
      type="button"
      onClick={onClick}
      className={cn(
        'flex flex-col gap-3 rounded-xl border p-5 text-left transition-colors',
        selected
          ? 'border-primary/40 bg-primary/10'
          : 'border-lazyops-border bg-lazyops-card hover:border-lazyops-border/80',
      )}
    >
      <div className="flex items-center gap-3">
        <span className="text-xl" aria-hidden="true">{icon}</span>
        <h3 className="text-base font-medium text-lazyops-text">{title}</h3>
      </div>
      <p className="text-sm leading-relaxed text-lazyops-muted">{description}</p>
      <p className="text-xs text-lazyops-muted/70">{useCase}</p>
    </button>
  );
}
