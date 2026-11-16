import type { ReactNode } from 'react';
import { cn } from '@/lib/cn';

export interface FilterOption {
  value: string;
  label?: string;
  count?: number;
}

interface Props {
  label: string;
  options: readonly FilterOption[];
  selected: readonly string[];
  onChange: (next: string[]) => void;
}

// FilterChips is a multi-select row of toggle chips for an in-table
// filter. Nothing selected means "no constraint" — every row shows —
// so an "All" chip clears the selection rather than selecting
// everything.
export function FilterChips({ label, options, selected, onChange }: Props) {
  if (options.length === 0) return null;
  const toggle = (value: string) =>
    onChange(
      selected.includes(value)
        ? selected.filter((v) => v !== value)
        : [...selected, value],
    );
  return (
    <div className="flex flex-wrap items-center gap-1.5" role="group" aria-label={label}>
      <span className="mr-1 text-[11px] font-medium uppercase tracking-wider text-muted">{label}</span>
      <Chip active={selected.length === 0} onClick={() => onChange([])}>
        All
      </Chip>
      {options.map((o) => (
        <Chip
          key={o.value}
          active={selected.includes(o.value)}
          onClick={() => toggle(o.value)}
        >
          {o.label ?? o.value}
          {o.count != null && <span className="ml-1 tabular-nums opacity-70">{o.count}</span>}
        </Chip>
      ))}
    </div>
  );
}

function Chip({
  active,
  onClick,
  children,
}: {
  active: boolean;
  onClick: () => void;
  children: ReactNode;
}) {
  return (
    <button
      type="button"
      aria-pressed={active}
      onClick={onClick}
      className={cn(
        'rounded-full border px-2 py-0.5 text-[11px] font-medium transition-colors',
        active
          ? 'border-primary bg-primary/10 text-primary'
          : 'border-border bg-surface text-muted hover:bg-surface-2 hover:text-foreground',
      )}
    >
      {children}
    </button>
  );
}
