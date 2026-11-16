import { type HTMLAttributes, type TdHTMLAttributes, type ThHTMLAttributes } from 'react';
import { ArrowDown, ArrowUp, ChevronsUpDown } from 'lucide-react';
import { cn } from '@/lib/cn';
import { nextSort, type SortState } from '@/lib/tableSort';

export function Table({ className, ...rest }: HTMLAttributes<HTMLTableElement>) {
  return (
    <div className="overflow-x-auto">
      <table
        className={cn('w-full text-sm border-collapse', className)}
        {...rest}
      />
    </div>
  );
}

export function THead(props: HTMLAttributes<HTMLTableSectionElement>) {
  return <thead className="bg-surface-2 text-left" {...props} />;
}

export function TBody(props: HTMLAttributes<HTMLTableSectionElement>) {
  return <tbody {...props} />;
}

export function TR({ className, ...rest }: HTMLAttributes<HTMLTableRowElement>) {
  return (
    <tr
      className={cn('border-b border-border last:border-0 hover:bg-surface-2', className)}
      {...rest}
    />
  );
}

export function TH({ className, ...rest }: ThHTMLAttributes<HTMLTableCellElement>) {
  return (
    <th
      className={cn('px-3 py-2 text-xs font-medium uppercase tracking-wider text-muted', className)}
      {...rest}
    />
  );
}

export function TD({ className, ...rest }: TdHTMLAttributes<HTMLTableCellElement>) {
  return <td className={cn('px-3 py-2 text-foreground', className)} {...rest} />;
}

interface SortableTHProps<K extends string> extends Omit<ThHTMLAttributes<HTMLTableCellElement>, 'onChange'> {
  sortKey: K;
  sort: SortState<K> | null;
  onSortChange: (next: SortState<K> | null) => void;
}

// SortableTH is a header cell whose label is a button cycling the
// column through ascending → descending → unsorted. aria-sort tells
// assistive tech which column is driving the order.
export function SortableTH<K extends string>({
  sortKey,
  sort,
  onSortChange,
  className,
  children,
  ...rest
}: SortableTHProps<K>) {
  const active = sort?.key === sortKey ? sort.dir : null;
  const Icon = active === 'asc' ? ArrowUp : active === 'desc' ? ArrowDown : ChevronsUpDown;
  const alignRight = className?.includes('text-right');
  return (
    <TH
      aria-sort={active === 'asc' ? 'ascending' : active === 'desc' ? 'descending' : 'none'}
      className={className}
      {...rest}
    >
      <button
        type="button"
        onClick={() => onSortChange(nextSort(sort, sortKey))}
        className={cn(
          'inline-flex items-center gap-1 uppercase tracking-wider hover:text-foreground',
          active && 'text-foreground',
          alignRight && 'flex-row-reverse',
        )}
        data-testid={`sort-${sortKey}`}
      >
        {children}
        <Icon className={cn('h-3 w-3 shrink-0', !active && 'opacity-40')} aria-hidden />
      </button>
    </TH>
  );
}
