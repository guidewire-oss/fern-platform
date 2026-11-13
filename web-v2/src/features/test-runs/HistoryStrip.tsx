import { cn } from '@/lib/cn';
import { formatDuration } from '@/lib/duration';
import type { HistoryRun } from '@/lib/types';
import { historySummary, type HistoryResults } from './runHistory';
import { STATUS_BG } from './statusColors';

// HistoryStrip shows one suite or spec across the history window, oldest
// left, with the viewed run (always the last cell) outlined. A run the
// suite or spec was absent from shows as an empty dashed cell. Details
// for each run are in the cell's tooltip.
export function HistoryStrip({
  runs,
  results,
}: {
  runs: readonly HistoryRun[];
  results: HistoryResults;
}) {
  return (
    <span
      role="img"
      aria-label={historySummary(results)}
      className="inline-flex items-center gap-px"
      data-testid="history-strip"
    >
      {results.map((cell, i) => {
        const run = runs[i];
        const viewed = i === results.length - 1;
        const when = run ? new Date(run.startTime).toLocaleString() : '';
        const branch = run?.branch ? ` · ${run.branch}` : '';
        const title = cell
          ? `${when}${branch} · ${cell.status} · ${formatDuration(cell.durationMs)}`
          : `${when}${branch} · not in this run`;
        return (
          <span
            key={run?.id ?? i}
            title={viewed ? `${title} (this run)` : title}
            data-testid="history-cell"
            data-status={cell?.status ?? 'absent'}
            data-viewed={viewed || undefined}
            className={cn(
              'h-4 w-2 rounded-sm',
              cell
                ? STATUS_BG[cell.status] ?? 'bg-gray-400'
                : 'border border-dashed border-border',
              viewed && 'ring-2 ring-foreground/60 ring-offset-1 ring-offset-surface',
            )}
          />
        );
      })}
    </span>
  );
}
