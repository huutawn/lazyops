import { cn } from '@/lib/utils';

type FormFieldProps = {
  label: string;
  error?: string;
  children: React.ReactNode;
  className?: string;
};

export function FormField({ label, error, children, className }: FormFieldProps) {
  return (
    <div className={cn('flex flex-col gap-1.5', className)}>
      <label className="text-sm font-medium text-lazyops-text">{label}</label>
      {children}
      {error && <p className="text-xs text-health-unhealthy">{error}</p>}
    </div>
  );
}

type FormInputProps = React.InputHTMLAttributes<HTMLInputElement> & {
  error?: boolean;
};

export function FormInput({ className, error, ...props }: FormInputProps) {
  return (
    <input
      className={cn(
        'h-10 w-full rounded-lg border bg-lazyops-bg-accent/60 px-3 text-sm text-lazyops-text outline-none transition-colors placeholder:text-lazyops-muted/60',
        'focus:border-primary/60 focus:ring-1 focus:ring-primary/30',
        error && 'border-health-unhealthy/60 focus:border-health-unhealthy/60 focus:ring-health-unhealthy/30',
        'disabled:cursor-not-allowed disabled:opacity-50',
        className,
      )}
      {...props}
    />
  );
}

type FormButtonProps = React.ButtonHTMLAttributes<HTMLButtonElement> & {
  loading?: boolean;
};

export function FormButton({ children, loading, disabled, className, ...props }: FormButtonProps) {
  return (
    <button
      className={cn(
        'h-10 w-full rounded-lg bg-primary px-4 text-sm font-semibold text-lazyops-bg shadow transition-colors',
        'hover:bg-primary/90 focus:outline-none focus:ring-2 focus:ring-primary/50 focus:ring-offset-2 focus:ring-offset-lazyops-bg',
        'disabled:cursor-not-allowed disabled:opacity-50',
        className,
      )}
      disabled={disabled || loading}
      {...props}
    >
      {loading ? (
        <span className="flex items-center justify-center gap-2">
          <span className="size-4 animate-spin rounded-full border-2 border-lazyops-bg/30 border-t-lazyops-bg" />
          Processing…
        </span>
      ) : (
        children
      )}
    </button>
  );
}
