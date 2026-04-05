import { useId, Children, cloneElement, isValidElement } from 'react';
import { cn } from '@/lib/utils';

type FormFieldProps = {
  label: string;
  error?: string;
  children: React.ReactNode;
  className?: string;
};

function addIdToFirstInput(children: React.ReactNode, id: string): React.ReactNode {
  const childArray = Children.toArray(children);
  const firstInputIndex = childArray.findIndex(
    (child) => isValidElement(child) && typeof child.type === 'string' && child.type === 'input',
  );

  if (firstInputIndex === -1) return children;

  const firstInput = childArray[firstInputIndex];
  if (!isValidElement(firstInput)) return children;

  const updatedInput = cloneElement(firstInput as React.ReactElement<{ id?: string }>, { id });
  const updatedChildren = [...childArray];
  updatedChildren[firstInputIndex] = updatedInput;

  return updatedChildren;
}

export function FormField({ label, error, children, className }: FormFieldProps) {
  const id = useId();

  return (
    <div className={cn('flex flex-col gap-1.5', className)}>
      <label htmlFor={id} className="text-sm font-medium text-lazyops-text">{label}</label>
      {addIdToFirstInput(children, id)}
      {error && <p className="text-xs text-health-unhealthy" role="alert">{error}</p>}
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

export function FormButton({ children, loading, disabled, className, type = 'submit', ...props }: FormButtonProps) {
  return (
    <button
      className={cn(
        'h-10 w-full rounded-lg bg-primary px-4 text-sm font-semibold text-lazyops-bg shadow transition-colors',
        'hover:bg-primary/90 focus:outline-none focus:ring-2 focus:ring-primary/50 focus:ring-offset-2 focus:ring-offset-lazyops-bg',
        'disabled:cursor-not-allowed disabled:opacity-50',
        className,
      )}
      type={type}
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
