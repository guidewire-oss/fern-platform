import { cn } from '@/lib/cn';

const STATUS_STYLES: Record<string, string> = {
  passed:  'bg-status-passed-bg  text-status-passed-fg',
  failed:  'bg-status-failed-bg  text-status-failed-fg',
  flaky:   'bg-status-flaky-bg   text-status-flaky-fg',
  skipped: 'bg-status-skipped-bg text-status-skipped-fg',
  running: 'bg-status-running-bg text-status-running-fg',
};

const STATUS_DOTS: Record<string, string> = {
  passed:  'bg-emerald-500',
  failed:  'bg-red-500',
  flaky:   'bg-amber-500',
  skipped: 'bg-gray-400',
  running: 'bg-blue-500 animate-pulse',
};

export function StatusBadge({ status }: { status: string }) {
  const style = STATUS_STYLES[status] ?? 'bg-status-skipped-bg text-status-skipped-fg';
  const dot   = STATUS_DOTS[status]   ?? 'bg-gray-400';
  return (
    <span
      className={cn(
        'inline-flex items-center gap-1.5 rounded-full px-2 py-0.5 text-[11px] font-medium',
        style,
      )}
    >
      <span className={cn('h-1.5 w-1.5 rounded-full', dot)} aria-hidden />
      {status}
    </span>
  );
}
