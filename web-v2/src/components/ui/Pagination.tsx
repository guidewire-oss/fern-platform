import { ChevronFirst, ChevronLeft, ChevronRight } from 'lucide-react';
import { cn } from '@/lib/cn';
import { Select } from './Input';

const PAGE_SIZES = [25, 50, 100, 200] as const;

interface Props {
  /** Total matching rows, exact or estimated. */
  totalCount: number;
  /** When true, displays totalCount with a `≈` hint. */
  isEstimate: boolean;
  /** Page size — server will clamp; UI offers 25/50/100/200. */
  pageSize: number;
  onPageSizeChange: (size: number) => void;
  /** Current page index, 1-based. */
  page: number;
  /** Whether the API reports another page after this one. */
  hasNext: boolean;
  /** Move forward/back. The page-level component owns the cursor stack. */
  onFirst: () => void;
  onPrev: () => void;
  onNext: () => void;
  /** Number of rows currently rendered on this page. */
  renderedCount: number;
}

export function Pagination({
  totalCount,
  isEstimate,
  pageSize,
  onPageSizeChange,
  page,
  hasNext,
  onFirst,
  onPrev,
  onNext,
  renderedCount,
}: Props) {
  // 1-indexed range — "Showing X – Y of N".
  const firstRow = renderedCount === 0 ? 0 : (page - 1) * pageSize + 1;
  const lastRow = firstRow + renderedCount - 1;
  const totalPages = totalCount > 0 ? Math.ceil(totalCount / pageSize) : 1;

  return (
    <div className="flex flex-wrap items-center justify-between gap-3 rounded border border-border bg-surface px-3 py-2 text-xs">
      <div className="text-muted">
        Showing{' '}
        <span className="font-medium text-foreground tabular-nums">
          {firstRow.toLocaleString()}–{lastRow.toLocaleString()}
        </span>{' '}
        of{' '}
        <span className="font-medium text-foreground tabular-nums">
          {isEstimate && '≈ '}
          {totalCount.toLocaleString()}
        </span>
      </div>

      <div className="flex items-center gap-3">
        <label className="flex items-center gap-1.5 text-muted">
          Page size
          <Select
            value={pageSize}
            onChange={(e) => onPageSizeChange(parseInt(e.target.value, 10))}
            className="!w-auto"
          >
            {PAGE_SIZES.map((s) => (
              <option key={s} value={s}>{s}</option>
            ))}
          </Select>
        </label>

        <div className="flex items-center gap-1">
          <PageButton onClick={onFirst} disabled={page <= 1} title="First page">
            <ChevronFirst className="h-3.5 w-3.5" />
          </PageButton>
          <PageButton onClick={onPrev} disabled={page <= 1} title="Previous page">
            <ChevronLeft className="h-3.5 w-3.5" />
          </PageButton>
          <span className="px-2 text-muted">
            Page{' '}
            <span className="font-medium text-foreground tabular-nums">{page}</span>
            {' / '}
            <span className="tabular-nums">
              {totalPages.toLocaleString()}
              {isEstimate ? '~' : ''}
            </span>
          </span>
          <PageButton onClick={onNext} disabled={!hasNext} title="Next page">
            <ChevronRight className="h-3.5 w-3.5" />
          </PageButton>
        </div>
      </div>
    </div>
  );
}

function PageButton({
  onClick,
  disabled,
  title,
  children,
}: {
  onClick: () => void;
  disabled?: boolean;
  title: string;
  children: React.ReactNode;
}) {
  return (
    <button
      onClick={onClick}
      disabled={disabled}
      title={title}
      aria-label={title}
      className={cn(
        'inline-flex h-7 w-7 items-center justify-center rounded border border-slate-300 bg-slate-100 text-slate-900 shadow-sm transition-colors',
        'hover:bg-slate-200 hover:border-slate-400',
        'disabled:cursor-not-allowed disabled:opacity-40 disabled:hover:bg-slate-100',
        'dark:bg-slate-800 dark:text-slate-100 dark:border-slate-600 dark:hover:bg-slate-700',
      )}
    >
      {children}
    </button>
  );
}
