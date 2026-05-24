import { useMemo } from 'react';
import { Star } from 'lucide-react';
import { Card, CardBody, CardHeader, CardTitle } from '@/components/ui/Card';
import { Button } from '@/components/ui/Button';
import { Input } from '@/components/ui/Input';
import { cn } from '@/lib/cn';
import type { FacetCount } from '@/lib/types';
import { useUserPreferences } from '../profile/hooks';
import type { TestRunsFilter } from './hooks';

interface Props {
  filter: TestRunsFilter;
  onChange: (next: TestRunsFilter) => void;
  facets: {
    byStatus: FacetCount[];
    byBranch: FacetCount[];
    byTag: FacetCount[];
    byProject: FacetCount[];
  };
}

// Status is intentionally workflow-ordered, not alphabetical: green
// states first, then degraded, then in-flight. This is the order users
// scan for triage.
const KNOWN_STATUSES = ['passed', 'failed', 'flaky', 'skipped', 'running'];

// Branches users care about most should float to the top; everything
// else is alphabetical so the list is predictable.
const PRIORITY_BRANCHES = new Set(['main', 'master', 'develop', 'trunk']);

function sortBranches(values: string[]): string[] {
  return [...values].sort((a, b) => {
    const ap = PRIORITY_BRANCHES.has(a) ? 0 : 1;
    const bp = PRIORITY_BRANCHES.has(b) ? 0 : 1;
    if (ap !== bp) return ap - bp;
    return a.localeCompare(b, undefined, { numeric: true, sensitivity: 'base' });
  });
}

function sortNatural(values: string[]): string[] {
  return [...values].sort((a, b) =>
    a.localeCompare(b, undefined, { numeric: true, sensitivity: 'base' }),
  );
}

const DATE_PRESETS: Array<{ label: string; days: number }> = [
  { label: '24h', days: 1 },
  { label: '7d',  days: 7 },
  { label: '30d', days: 30 },
  { label: '90d', days: 90 },
];

export function FilterSidebar({ filter, onChange, facets }: Props) {
  const prefs = useUserPreferences();
  const favorites = useMemo(
    () => prefs.data?.userPreferences?.favorites ?? [],
    [prefs.data],
  );
  const favsActive = !!filter.project?.length &&
    filter.project!.length === favorites.length &&
    filter.project!.every((p) => favorites.includes(p));

  const setSearch = (q: string) =>
    onChange({ ...filter, q: q || undefined, after: undefined });

  const toggleArr = (key: 'project' | 'status' | 'branch' | 'tag', value: string) => {
    const current = new Set((filter as Record<string, string[] | undefined>)[key] ?? []);
    if (current.has(value)) current.delete(value);
    else current.add(value);
    onChange({
      ...filter,
      [key]: current.size ? Array.from(current) : undefined,
      after: undefined,
    });
  };

  const applyDateRange = (days: number | null) => {
    if (days == null) {
      onChange({ ...filter, from: undefined, to: undefined, after: undefined });
      return;
    }
    const to = new Date();
    const from = new Date(to.getTime() - days * 24 * 60 * 60 * 1000);
    onChange({
      ...filter,
      from: from.toISOString(),
      to: to.toISOString(),
      after: undefined,
    });
  };

  const activeDayPreset = useMemo(() => {
    if (!filter.from || !filter.to) return null;
    const span = new Date(filter.to).getTime() - new Date(filter.from).getTime();
    for (const p of DATE_PRESETS) {
      const diff = Math.abs(span - p.days * 24 * 60 * 60 * 1000);
      if (diff < 60 * 1000) return p.days;
    }
    return null;
  }, [filter.from, filter.to]);

  // UI works in seconds; the server's duration_ms column is ms, so we
  // convert at the edge.
  const setDurationBoundSec = (key: 'durationGte' | 'durationLte', raw: string) => {
    if (raw === '') {
      onChange({ ...filter, [key]: undefined, after: undefined });
      return;
    }
    const seconds = Number(raw);
    if (!Number.isFinite(seconds) || seconds < 0) return;
    onChange({
      ...filter,
      [key]: Math.round(seconds * 1000),
      after: undefined,
    });
  };

  const msToSec = (ms: number | undefined): number | '' =>
    ms == null ? '' : Math.round(ms / 1000);

  const toggleFavoritesOnly = () => {
    if (favsActive) {
      onChange({ ...filter, project: undefined, after: undefined });
    } else if (favorites.length > 0) {
      onChange({ ...filter, project: favorites, after: undefined });
    }
  };

  const reset = () => onChange({ first: filter.first ?? 50 });

  return (
    <Card className="self-start">
      <CardHeader className="flex items-center justify-between">
        <CardTitle>Filters</CardTitle>
        <Button variant="ghost" size="sm" onClick={reset}>Reset</Button>
      </CardHeader>
      <CardBody className="space-y-5">
        <section>
          <label htmlFor="filter-sidebar-search" className="block text-xs font-medium text-muted">Search</label>
          <Input
            id="filter-sidebar-search"
            type="search"
            className="mt-1"
            placeholder="error message, spec name…"
            value={filter.q ?? ''}
            onChange={(e) => setSearch(e.target.value)}
          />
        </section>

        <section>
          <div className="mb-1 flex items-center justify-between text-xs font-medium uppercase tracking-wider text-muted">
            <span>Date range</span>
            {(filter.from || filter.to) && (
              <button
                onClick={() => applyDateRange(null)}
                className="text-[10px] normal-case text-primary hover:underline"
              >
                clear
              </button>
            )}
          </div>
          <div className="flex flex-wrap gap-1">
            {DATE_PRESETS.map(({ label, days }) => (
              <button
                key={days}
                onClick={() => applyDateRange(days)}
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
          </div>
        </section>

        <section>
          <div className="mb-1 flex items-center justify-between text-xs font-medium uppercase tracking-wider text-muted">
            <span>Run time (seconds)</span>
            {(filter.durationGte != null || filter.durationLte != null) && (
              <button
                onClick={() =>
                  onChange({ ...filter, durationGte: undefined, durationLte: undefined })
                }
                className="text-[10px] normal-case text-primary hover:underline"
              >
                clear
              </button>
            )}
          </div>
          <div className="grid grid-cols-2 gap-2">
            <Input
              type="number"
              min={0}
              placeholder="min"
              value={msToSec(filter.durationGte)}
              onChange={(e) => setDurationBoundSec('durationGte', e.target.value)}
            />
            <Input
              type="number"
              min={0}
              placeholder="max"
              value={msToSec(filter.durationLte)}
              onChange={(e) => setDurationBoundSec('durationLte', e.target.value)}
            />
          </div>
          <p className="mt-1 text-[10px] text-muted">
            Wall-clock time for the whole run, in seconds
          </p>
        </section>

        {favorites.length > 0 && (
          <section>
            <button
              type="button"
              onClick={toggleFavoritesOnly}
              className={cn(
                'flex w-full items-center justify-between rounded px-2 py-1.5 text-sm transition-colors',
                favsActive
                  ? 'bg-status-flaky-bg text-status-flaky-fg'
                  : 'text-foreground hover:bg-surface-2',
              )}
            >
              <span className="flex items-center gap-1.5">
                <Star className={cn('h-3.5 w-3.5', favsActive && 'fill-current')} />
                Favorites only
              </span>
              <span className="text-xs text-muted">{favorites.length}</span>
            </button>
          </section>
        )}

        <FacetGroup
          title="Status"
          values={KNOWN_STATUSES}
          counts={facets.byStatus}
          selected={filter.status}
          onToggle={(v) => toggleArr('status', v)}
        />
        <FacetGroup
          title="Branch"
          values={sortBranches(facets.byBranch.map((f) => f.value))}
          counts={facets.byBranch}
          selected={filter.branch}
          onToggle={(v) => toggleArr('branch', v)}
        />
        <FacetGroup
          title="Project"
          values={sortNatural(facets.byProject.map((f) => f.value))}
          counts={facets.byProject}
          selected={filter.project}
          onToggle={(v) => toggleArr('project', v)}
        />
        <TagFacetSection
          facets={facets.byTag}
          selected={filter.tag}
          includeTagFacet={filter.includeTagFacet ?? false}
          onToggle={(v) => toggleArr('tag', v)}
          onEnable={() =>
            onChange({ ...filter, includeTagFacet: true })
          }
        />
      </CardBody>
    </Card>
  );
}

function TagFacetSection({
  facets,
  selected,
  includeTagFacet,
  onToggle,
  onEnable,
}: {
  facets: FacetCount[];
  selected: string[] | undefined;
  includeTagFacet: boolean;
  onToggle: (v: string) => void;
  onEnable: () => void;
}) {
  // Tag facet is opt-in: the server-side join is expensive and the
  // default list response does not include it. Until the user expands
  // the section, render a single "Load tags" button instead of an
  // empty list.
  if (!includeTagFacet && (!selected || selected.length === 0) && facets.length === 0) {
    return (
      <section>
        <h4 className="mb-1 text-xs font-medium uppercase tracking-wider text-muted">
          Tag
        </h4>
        <button
          type="button"
          onClick={onEnable}
          className="w-full rounded border border-dashed border-border px-2 py-1.5 text-xs text-muted hover:bg-surface-2 hover:text-foreground"
          title="Tag facet is hidden until requested — the join is expensive at scale"
        >
          Load tag facet
        </button>
      </section>
    );
  }
  return (
    <FacetGroup
      title="Tag"
      values={sortNatural(facets.map((f) => f.value))}
      counts={facets}
      selected={selected}
      onToggle={onToggle}
    />
  );
}

function FacetGroup({
  title,
  values,
  counts,
  selected,
  onToggle,
}: {
  title: string;
  values: string[];
  counts: FacetCount[];
  selected: string[] | undefined;
  onToggle: (value: string) => void;
}) {
  if (values.length === 0) return null;
  const countFor = (v: string) => counts.find((c) => c.value === v)?.count ?? 0;
  const isSelected = (v: string) => selected?.includes(v) ?? false;

  return (
    <section>
      <h4 className="mb-1 text-xs font-medium uppercase tracking-wider text-muted">
        {title}
      </h4>
      <ul className="max-h-44 space-y-1 overflow-y-auto">
        {values.map((v) => {
          const sel = isSelected(v);
          return (
            <li key={v}>
              <button
                type="button"
                onClick={() => onToggle(v)}
                className={cn(
                  'flex w-full items-center justify-between rounded px-2 py-1 text-sm transition-colors',
                  sel
                    ? 'bg-primary/10 text-primary'
                    : 'text-foreground hover:bg-surface-2',
                )}
              >
                <span className="truncate" title={v}>{v}</span>
                <span className="ml-2 text-xs text-muted">{countFor(v)}</span>
              </button>
            </li>
          );
        })}
      </ul>
    </section>
  );
}
