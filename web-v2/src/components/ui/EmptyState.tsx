import { type ReactNode } from 'react';

export function EmptyState({
  title,
  description,
  action,
}: {
  title: string;
  description?: string;
  action?: ReactNode;
}) {
  return (
    <div className="flex flex-col items-center justify-center rounded border border-dashed border-border bg-surface py-12 text-center">
      <h3 className="text-sm font-medium text-foreground">{title}</h3>
      {description && (
        <p className="mt-1 max-w-md text-sm text-muted">{description}</p>
      )}
      {action && <div className="mt-4">{action}</div>}
    </div>
  );
}
