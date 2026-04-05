type SummaryFieldProps = {
  label: string;
  value: string;
};

export function SummaryField({ label, value }: SummaryFieldProps) {
  return (
    <div className="flex flex-col gap-0.5">
      <span className="text-xs text-lazyops-muted">{label}</span>
      <span className="truncate text-sm text-lazyops-text" title={value}>{value}</span>
    </div>
  );
}
