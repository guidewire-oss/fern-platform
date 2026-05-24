import { useEffect, useMemo, useRef } from 'react';
import { Search, Star, X } from 'lucide-react';
import { Input, Select } from '@/components/ui/Input';
import { cn } from '@/lib/cn';

export type SortKey = 'name' | 'runs' | 'rate' | 'last';
export type SortDir = 'asc' | 'desc';

export interface ProjectsFilter {
  q: string;
  teams: string[];
  categories: string[];
  favoritesOnly: boolean;
  sortKey: SortKey;
  sortDir: SortDir;
}

// DEFAULT_FILTER lives next to the component so callers don't need to know
// which fields drive the filter shape. Constant export keeps Fast Refresh
// behavior correct for the component itself.
// eslint-disable-next-line react-refresh/only-export-components
export const DEFAULT_FILTER: ProjectsFilter = {
  q: '',
  teams: [],
  categories: [],
  favoritesOnly: false,
  sortKey: 'name',
  sortDir: 'asc',
};

const CATEGORY_PREFIXES = [
  { value: 'java',  label: 'Java' },
  { value: 'infra', label: 'Infra' },
  { value: 'flux',  label: 'FluxCD' },
  { value: 'helm',  label: 'Helm' },
  { value: 'web',   label: 'Web' },
];

interface Props {
  filter: ProjectsFilter;
  onChange: (next: ProjectsFilter) => void;
  /** All teams currently observed in the project set. */
  availableTeams: string[];
  /** Number of starred projects (drives the Favorites toggle's visibility). */
  favoritesCount: number;
  /** Result count for the active filter, for display next to the bar. */
  visibleCount: number;
  totalCount: number;
}

export function ProjectsFilterBar({
  filter,
  onChange,
  availableTeams,
  favoritesCount,
  visibleCount,
  totalCount,
}: Props) {
  const set = <K extends keyof ProjectsFilter>(k: K, v: ProjectsFilter[K]) =>
    onChange({ ...filter, [k]: v });

  const toggleArr = (k: 'teams' | 'categories', value: string) => {
    const current = new Set(filter[k]);
    if (current.has(value)) current.delete(value);
    else current.add(value);
    set(k, Array.from(current));
  };

  const activeChips = useMemo(() => {
    const out: Array<{ key: string; label: string; onClear: () => void }> = [];
    if (filter.q) out.push({ key: 'q', label: `“${filter.q}”`, onClear: () => set('q', '') });
    for (const t of filter.teams) {
      out.push({ key: `t:${t}`, label: `team: ${t}`, onClear: () => toggleArr('teams', t) });
    }
    for (const c of filter.categories) {
      const label = CATEGORY_PREFIXES.find((x) => x.value === c)?.label ?? c;
      out.push({ key: `c:${c}`, label, onClear: () => toggleArr('categories', c) });
    }
    if (filter.favoritesOnly) {
      out.push({ key: 'fav', label: '⭐ favorites', onClear: () => set('favoritesOnly', false) });
    }
    return out;
    // `set` and `toggleArr` are inline closures over `filter` and `onChange`
    // — they're recreated each render anyway, so depending on them would
    // defeat the memo. Recomputing when `filter` changes is exactly correct.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [filter]);

  return (
    <div className="space-y-2 rounded-lg border border-border bg-surface p-3">
      <div className="grid gap-2 md:grid-cols-[1fr_auto_auto_auto_auto] md:items-center">
        <div className="relative">
          <Search className="absolute left-2 top-1/2 h-3.5 w-3.5 -translate-y-1/2 text-muted" />
          <Input
            type="search"
            placeholder="Search by name, ID, or team…"
            value={filter.q}
            onChange={(e) => set('q', e.target.value)}
            className="pl-7"
          />
        </div>

        <MultiDropdown
          label={filter.teams.length ? `Team (${filter.teams.length})` : 'Team'}
          options={availableTeams.map((t) => ({ value: t, label: t }))}
          selected={filter.teams}
          onToggle={(v) => toggleArr('teams', v)}
          onClear={() => set('teams', [])}
        />

        <MultiDropdown
          label={filter.categories.length ? `Category (${filter.categories.length})` : 'Category'}
          options={CATEGORY_PREFIXES}
          selected={filter.categories}
          onToggle={(v) => toggleArr('categories', v)}
          onClear={() => set('categories', [])}
        />

        <button
          type="button"
          onClick={() => set('favoritesOnly', !filter.favoritesOnly)}
          disabled={favoritesCount === 0}
          className={cn(
            'inline-flex items-center gap-1 rounded border px-2.5 py-1.5 text-xs font-medium transition-colors',
            filter.favoritesOnly
              ? 'border-amber-400 bg-status-flaky-bg text-status-flaky-fg'
              : 'border-border bg-surface text-foreground hover:bg-surface-2 disabled:opacity-40 disabled:cursor-not-allowed',
          )}
          title={favoritesCount === 0 ? 'Star a project to enable' : 'Filter to starred projects'}
        >
          <Star className={cn('h-3.5 w-3.5', filter.favoritesOnly && 'fill-current')} />
          Favorites
          {favoritesCount > 0 && (
            <span className="ml-0.5 text-muted">({favoritesCount})</span>
          )}
        </button>

        <div className="flex items-center gap-1 text-xs text-muted">
          <span>Sort</span>
          <Select
            value={filter.sortKey}
            onChange={(e) => set('sortKey', e.target.value as SortKey)}
            className="!w-auto"
          >
            <option value="name">Name</option>
            <option value="runs">Runs</option>
            <option value="rate">Pass rate</option>
            <option value="last">Last activity</option>
          </Select>
          <button
            type="button"
            onClick={() => set('sortDir', filter.sortDir === 'asc' ? 'desc' : 'asc')}
            className="inline-flex h-7 w-7 items-center justify-center rounded border border-border bg-surface text-foreground hover:bg-surface-2"
            aria-label="Toggle sort direction"
            title={filter.sortDir === 'asc' ? 'Ascending — click for descending' : 'Descending — click for ascending'}
          >
            {filter.sortDir === 'asc' ? '↑' : '↓'}
          </button>
        </div>
      </div>

      <div className="flex flex-wrap items-center gap-2 text-xs">
        <span className="text-muted">
          Showing <span className="font-medium text-foreground tabular-nums">{visibleCount}</span> of{' '}
          <span className="tabular-nums">{totalCount}</span>
        </span>
        {activeChips.map((c) => (
          <button
            key={c.key}
            onClick={c.onClear}
            className="inline-flex items-center gap-1 rounded-full bg-primary/10 px-2 py-0.5 text-primary hover:bg-primary/20"
          >
            {c.label}
            <X className="h-3 w-3" />
          </button>
        ))}
        {activeChips.length > 0 && (
          <button
            onClick={() => onChange(DEFAULT_FILTER)}
            className="ml-auto text-muted hover:text-foreground"
          >
            Clear all
          </button>
        )}
      </div>
    </div>
  );
}

interface DropdownOption {
  value: string;
  label: string;
}

function MultiDropdown({
  label,
  options,
  selected,
  onToggle,
  onClear,
}: {
  label: string;
  options: DropdownOption[];
  selected: string[];
  onToggle: (v: string) => void;
  onClear: () => void;
}) {
  // Native <details> dropdown — keyboard-accessible, no extra deps.
  // Native <details> stays open on outside click and Escape; we close
  // it manually so the popup goes away after selection.
  const ref = useRef<HTMLDetailsElement>(null);
  useEffect(() => {
    const onDocClick = (e: MouseEvent) => {
      const el = ref.current;
      if (!el || !el.open) return;
      if (!el.contains(e.target as Node)) el.open = false;
    };
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape' && ref.current?.open) ref.current.open = false;
    };
    document.addEventListener('mousedown', onDocClick);
    document.addEventListener('keydown', onKey);
    return () => {
      document.removeEventListener('mousedown', onDocClick);
      document.removeEventListener('keydown', onKey);
    };
  }, []);
  return (
    <details ref={ref} className="relative">
      <summary
        className={cn(
          'inline-flex cursor-pointer list-none items-center gap-1 rounded border border-border bg-surface px-2.5 py-1.5 text-xs font-medium text-foreground hover:bg-surface-2',
          '[&::-webkit-details-marker]:hidden',
        )}
      >
        {label}
        <span className="ml-0.5 text-muted">▾</span>
      </summary>
      <div className="absolute right-0 z-20 mt-1 max-h-72 w-56 overflow-y-auto rounded border border-border bg-surface p-2 shadow-lg">
        {options.length === 0 ? (
          <p className="px-2 py-1 text-xs text-muted">No options</p>
        ) : (
          <>
            {options.map((opt) => {
              const sel = selected.includes(opt.value);
              return (
                <label
                  key={opt.value}
                  className="flex cursor-pointer items-center gap-2 rounded px-2 py-1 text-sm hover:bg-surface-2"
                >
                  <input
                    type="checkbox"
                    checked={sel}
                    onChange={() => onToggle(opt.value)}
                    className="h-3.5 w-3.5 accent-primary"
                  />
                  <span>{opt.label}</span>
                </label>
              );
            })}
            {selected.length > 0 && (
              <button
                type="button"
                onClick={onClear}
                className="mt-1 w-full rounded px-2 py-1 text-left text-xs text-primary hover:bg-surface-2"
              >
                Clear selection
              </button>
            )}
          </>
        )}
      </div>
    </details>
  );
}
