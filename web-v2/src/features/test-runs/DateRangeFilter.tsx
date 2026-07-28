import { Input } from '@/components/ui/Input';
import { cn } from '@/lib/cn';
import {
  customRangeToQuery,
  queryToRangeInputs,
  presetRange,
  matchPresetDays,
  DATE_PRESETS,
} from './dateRange';

interface Props {
  /** Current bounds as RFC3339 strings; both undefined means unbounded. */
  from: string | undefined;
  to: string | undefined;
  onChange: (next: { from?: string | undefined; to?: string | undefined }) => void;
  /** Distinguishes the id-prefix of the date inputs when two are on a page. */
  idPrefix?: string;
  /** Label for clearing the range. The pages word this differently. */
  clearLabel?: string;
  /** Renders the section heading. Off inside an already-titled panel. */
  showHeading?: boolean;
}

/**
 * The date-range control shared by the test-runs filter sidebar and the
 * project page, so a window is picked the same way wherever it appears:
 * rolling presets, plus a custom from/to pair that lights up when the
 * span matches no preset.
 *
 * Owns no state — the caller holds the bounds, because each page feeds
 * them into a different query.
 */
export function DateRangeFilter({
  from,
  to,
  onChange,
  idPrefix = 'date-range',
  clearLabel = 'clear',
  showHeading = true,
}: Props) {
  const activeDayPreset = matchPresetDays(from, to);
  const rangeInputs = queryToRangeInputs(from, to);
  // A range is "custom" when a bound is set but matches no preset span —
  // that is what surfaces the date inputs as the active selection.
  const isCustomRange = !!(from || to) && activeDayPreset == null;

  const applyPreset = (days: number) => {
    const { from, to } = presetRange(days);
    onChange({ from, to });
  };

  // Typing a custom bound and clicking a preset are mutually exclusive:
  // both write from/to, so choosing one overwrites the other.
  //
  // Always emit both keys. customRangeToQuery omits a bound whose input
  // is empty, and a caller that spreads the result over an existing
  // filter would then keep the old value — clearing one input would
  // appear to do nothing.
  const setBound = (which: 'fromDate' | 'toDate', value: string) => {
    const next = { ...rangeInputs, [which]: value };
    const q = customRangeToQuery(next.fromDate, next.toDate);
    onChange({ from: q.from, to: q.to });
  };

  return (
    <section>
      {showHeading && (
        <div className="mb-1 flex items-center justify-between text-xs font-medium uppercase tracking-wider text-muted">
          <span>Date range</span>
          {(from || to) && (
            <button
              onClick={() => onChange({ from: undefined, to: undefined })}
              className="text-[10px] normal-case text-primary hover:underline"
            >
              {clearLabel}
            </button>
          )}
        </div>
      )}
      <div className="flex flex-wrap gap-1">
        {DATE_PRESETS.map(({ label, days }) => (
          <button
            key={days}
            type="button"
            aria-pressed={activeDayPreset === days}
            onClick={() => applyPreset(days)}
            className={cn(
              'rounded border px-2 py-0.5 text-xs transition-colors',
              activeDayPreset === days
                ? 'border-primary bg-primary/10 text-primary'
                : 'border-border bg-surface text-foreground hover:bg-surface-2',
            )}
          >
            Last {label}
          </button>
        ))}
        {!showHeading && (
          <button
            type="button"
            aria-pressed={!from && !to}
            onClick={() => onChange({ from: undefined, to: undefined })}
            className={cn(
              'rounded border px-2 py-0.5 text-xs transition-colors',
              !from && !to
                ? 'border-primary bg-primary/10 text-primary'
                : 'border-border bg-surface text-foreground hover:bg-surface-2',
            )}
          >
            {clearLabel}
          </button>
        )}
      </div>
      <div
        className={cn(
          'mt-2 grid grid-cols-2 gap-2 rounded border p-2 transition-colors',
          isCustomRange ? 'border-primary bg-primary/5' : 'border-border',
        )}
      >
        <div>
          <label
            htmlFor={`${idPrefix}-from`}
            className="text-[10px] uppercase tracking-wider text-muted"
          >
            From
          </label>
          <Input
            id={`${idPrefix}-from`}
            type="date"
            className="mt-0.5"
            max={rangeInputs.toDate || undefined}
            value={rangeInputs.fromDate}
            onChange={(e) => setBound('fromDate', e.target.value)}
          />
        </div>
        <div>
          <label
            htmlFor={`${idPrefix}-to`}
            className="text-[10px] uppercase tracking-wider text-muted"
          >
            To
          </label>
          <Input
            id={`${idPrefix}-to`}
            type="date"
            className="mt-0.5"
            min={rangeInputs.fromDate || undefined}
            value={rangeInputs.toDate}
            onChange={(e) => setBound('toDate', e.target.value)}
          />
        </div>
      </div>
    </section>
  );
}
