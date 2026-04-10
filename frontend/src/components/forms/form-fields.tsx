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
    <div className={cn('flex flex-col gap-2', className)}>
      <label htmlFor={id} className="text-[13px] font-bold text-[#94a3b8] uppercase tracking-wider ml-1">{label}</label>
      {addIdToFirstInput(children, id)}
      {error && <p className="text-xs font-medium text-[#ef4444] animate-in fade-in slide-in-from-left-2 mt-1" role="alert">{error}</p>}
    </div>
  );
}

type FormInputProps = React.InputHTMLAttributes<HTMLInputElement> & {
  error?: boolean;
  icon?: React.ReactNode;
};

export function FormInput({ className, error, icon, ...props }: FormInputProps) {
  return (
    <div className="relative group">
      {icon && (
        <div className="absolute left-3 top-1/2 -translate-y-1/2 text-[#64748b] group-focus-within:text-[#0EA5E9] transition-colors pointer-events-none">
          {icon}
        </div>
      )}
      <input
        className={cn(
          'h-12 w-full rounded-xl border bg-[#0B1120]/40 px-4 text-[15px] text-white outline-none transition-all placeholder:text-[#64748b]/50',
          icon ? 'pl-10' : 'pl-4',
          'border-[#1e293b] focus:border-[#0EA5E9]/50 focus:ring-4 focus:ring-[#0EA5E9]/10 focus:bg-[#0B1120]/60',
          error && 'border-[#ef4444]/50 focus:border-[#ef4444]/60 focus:ring-[#ef4444]/10',
          'disabled:cursor-not-allowed disabled:opacity-50',
          className,
        )}
        {...props}
      />
    </div>
  );
}

type FormButtonProps = React.ButtonHTMLAttributes<HTMLButtonElement> & {
  loading?: boolean;
};

export function FormButton({ children, loading, disabled, className, type = 'submit', ...props }: FormButtonProps) {
  return (
    <button
      className={cn(
        'h-12 w-full rounded-xl bg-gradient-to-r from-[#0EA5E9] to-[#38BDF8] px-6 text-[15px] font-bold text-white shadow-lg transition-all',
        'hover:from-[#0284c7] hover:to-[#0EA5E9] hover:scale-[1.02] active:scale-[0.98] shadow-[#0ea5e9]/20',
        'focus:outline-none focus:ring-4 focus:ring-[#0EA5E9]/30',
        'disabled:cursor-not-allowed disabled:opacity-50 disabled:from-[#1e293b] disabled:to-[#1e293b] disabled:text-[#64748b] disabled:shadow-none',
        className,
      )}
      type={type}
      disabled={disabled || loading}
      {...props}
    >
      {loading ? (
        <span className="flex items-center justify-center gap-2">
          <span className="size-4 animate-spin rounded-full border-2 border-white/30 border-t-white" />
          Đang xử lý...
        </span>
      ) : (
        children
      )}
    </button>
  );
}
