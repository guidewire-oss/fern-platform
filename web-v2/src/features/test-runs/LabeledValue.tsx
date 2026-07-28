import { cn } from '@/lib/cn';

/**
 * Renders a display label over the raw value it stands for.
 *
 * Both lines are shown because they serve different jobs: the label is
 * what users recognise (a project's name), the value is what links,
 * saved views, and query parameters are keyed on (its id). When there is
 * no label the value stands alone — no empty first line, no duplicate.
 *
 * Shared by the test-runs table's Project cell and the filter facet
 * entries so the two read identically.
 */
export function LabeledValue({
  value,
  label,
  className,
}: {
  value: string;
  label?: string | undefined;
  className?: string;
}) {
  if (!label) {
    return <span className={cn('truncate', className)} title={value}>{value}</span>;
  }
  return (
    <span className={cn('flex min-w-0 flex-col', className)}>
      <span className="truncate" title={label}>{label}</span>
      <span className="truncate font-mono text-[11px] text-muted" title={value}>
        {value}
      </span>
    </span>
  );
}
