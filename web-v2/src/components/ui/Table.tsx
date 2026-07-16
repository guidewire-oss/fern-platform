import { type HTMLAttributes, type TdHTMLAttributes, type ThHTMLAttributes } from 'react';
import { cn } from '@/lib/cn';

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
