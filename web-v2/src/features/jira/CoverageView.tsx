import { useEffect, useMemo, useRef, useState } from 'react';
import { Link, useParams } from '@tanstack/react-router';
import { ArrowLeft, ChevronDown, Search } from 'lucide-react';
import { Spinner } from '@/components/ui/Spinner';
import { EmptyState } from '@/components/ui/EmptyState';
import { CoverageTree } from './CoverageTree';
import { coveragePercent, defaultRelease, donutBreakdown, type DonutBreakdown, type JiraRelease, type RequirementCoverageTree } from './coverage';
import { useJiraFixVersions, useRequirementCoverage } from './coverageHooks';

// Searchable release combobox — a plain <select> is unusable with 100+
// fix versions, so filter as you type.
function ReleasePicker({ releases, value, onChange }: { releases: JiraRelease[]; value: string | undefined; onChange: (name: string) => void }) {
  const [open, setOpen] = useState(false);
  const [q, setQ] = useState('');
  const ref = useRef<HTMLDivElement>(null);
  const inputRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    if (!open) return;
    inputRef.current?.focus();
    const onDoc = (e: MouseEvent) => {
      if (ref.current && !ref.current.contains(e.target as Node)) setOpen(false);
    };
    const onKey = (e: KeyboardEvent) => e.key === 'Escape' && setOpen(false);
    document.addEventListener('mousedown', onDoc);
    document.addEventListener('keydown', onKey);
    return () => {
      document.removeEventListener('mousedown', onDoc);
      document.removeEventListener('keydown', onKey);
    };
  }, [open]);

  const filtered = useMemo(() => {
    const s = q.trim().toLowerCase();
    return s ? releases.filter((r) => r.name.toLowerCase().includes(s)) : releases;
  }, [releases, q]);

  return (
    <div ref={ref} className="relative">
      <button
        type="button"
        onClick={() => setOpen((o) => !o)}
        className="flex w-64 items-center justify-between gap-2 rounded border border-border bg-surface px-2.5 py-1.5 text-sm focus:border-primary focus:outline-none"
      >
        <span className="truncate">{value ?? 'Select release…'}</span>
        <ChevronDown className="h-3.5 w-3.5 shrink-0 text-muted" />
      </button>
      {open && (
        <div className="absolute right-0 z-20 mt-1 w-72 rounded-md border border-border bg-surface shadow-lg">
          <div className="relative border-b border-border p-2">
            <Search className="pointer-events-none absolute left-3.5 top-3.5 h-3.5 w-3.5 text-muted" />
            <input
              ref={inputRef}
              value={q}
              onChange={(e) => setQ(e.target.value)}
              placeholder="Search releases…"
              className="w-full rounded border border-border bg-surface py-1 pl-7 pr-2 text-sm focus:border-primary focus:outline-none"
            />
          </div>
          <div className="max-h-64 overflow-y-auto py-1">
            {filtered.length === 0 ? (
              <p className="px-3 py-2 text-xs text-muted">No releases match “{q}”.</p>
            ) : (
              filtered.map((r) => (
                <button
                  key={r.id}
                  type="button"
                  onClick={() => { onChange(r.name); setOpen(false); setQ(''); }}
                  className={`flex w-full items-center justify-between gap-2 px-3 py-1.5 text-left text-sm hover:bg-surface-2 ${r.name === value ? 'font-medium text-primary' : ''}`}
                >
                  <span className="truncate">{r.name}</span>
                  {r.released && <span className="shrink-0 text-[10px] text-muted">released</span>}
                </button>
              ))
            )}
          </div>
        </div>
      )}
    </div>
  );
}

// Three-segment donut (passing green / failing red / uncovered grey track)
// with the covered % in the centre.
function Donut({ b }: { b: DonutBreakdown }) {
  const R = 42;
  const C = 2 * Math.PI * R;
  const seg = (n: number) => (b.total > 0 ? (n / b.total) * C : 0);
  const pass = seg(b.passing);
  const fail = seg(b.failing);
  return (
    <div className="relative h-28 w-28 shrink-0">
      <svg viewBox="0 0 100 100" className="h-full w-full -rotate-90">
        <circle cx="50" cy="50" r={R} fill="none" strokeWidth="12" stroke="currentColor" className="text-surface-2" />
        {b.total > 0 && (
          <>
            <circle cx="50" cy="50" r={R} fill="none" strokeWidth="12" stroke="currentColor"
              className="text-green-600 dark:text-green-400" strokeDasharray={`${pass} ${C - pass}`} />
            <circle cx="50" cy="50" r={R} fill="none" strokeWidth="12" stroke="currentColor"
              className="text-red-600 dark:text-red-400" strokeDasharray={`${fail} ${C - fail}`} strokeDashoffset={-pass} />
          </>
        )}
      </svg>
      <div className="absolute inset-0 flex flex-col items-center justify-center">
        <span className="text-xl font-semibold tabular-nums">{b.percent}%</span>
        <span className="text-[10px] text-muted">{b.covered}/{b.total} covered</span>
      </div>
    </div>
  );
}

function LegendDot({ cls, label, n }: { cls: string; label: string; n: number }) {
  return (
    <span className="flex items-center gap-1.5 text-xs text-muted">
      <span className={`h-2.5 w-2.5 rounded-full ${cls}`} /> {label} <span className="tabular-nums text-foreground">{n}</span>
    </span>
  );
}

function EpicGrid({ tree, onOpen }: { tree: RequirementCoverageTree; onOpen: (key: string) => void }) {
  return (
    <div className="grid flex-1 grid-cols-2 gap-x-6 gap-y-1.5 lg:grid-cols-3">
      {tree.epics.map((e) => {
        const pct = coveragePercent(e.coveredCount, e.totalCount);
        return (
          <button key={e.issue.key} type="button" onClick={() => onOpen(e.issue.key)}
            className="flex items-center gap-2 rounded px-1 py-0.5 text-left hover:bg-surface-2"
            title={`${e.issue.key} — ${e.issue.summary} (jump to epic)`}>
            <span className="truncate font-mono text-[11px] text-primary">{e.issue.key}</span>
            <span className="ml-auto h-1.5 w-16 shrink-0 overflow-hidden rounded-full bg-surface-2">
              <span className="block h-full bg-green-500" style={{ width: `${pct}%` }} />
            </span>
            <span className="w-7 shrink-0 text-right text-[10px] tabular-nums text-muted">{pct}%</span>
          </button>
        );
      })}
    </div>
  );
}

export function CoverageContent({ projectId }: { projectId: string }) {
  const versions = useJiraFixVersions(projectId);
  const [selected, setSelected] = useState<string | undefined>(undefined);
  const releaseName = selected ?? defaultRelease(versions.data ?? [])?.name;

  const coverage = useRequirementCoverage(projectId, releaseName);
  const tree = coverage.data;
  const breakdown = useMemo(() => (tree ? donutBreakdown(tree) : null), [tree]);

  const [openEpics, setOpenEpics] = useState<Set<string>>(new Set());
  const [highlight, setHighlight] = useState<string | null>(null);
  const [showUncoveredOnly, setShowUncoveredOnly] = useState(false);

  const toggleEpic = (key: string) =>
    setOpenEpics((prev) => {
      const next = new Set(prev);
      if (next.has(key)) next.delete(key);
      else next.add(key);
      return next;
    });
  const openAndScroll = (key: string) => {
    setOpenEpics((prev) => new Set(prev).add(key));
    setHighlight(key);
    // Defer to next frame so the epic is expanded before we scroll to it.
    requestAnimationFrame(() =>
      document.getElementById(`epic-${key}`)?.scrollIntoView({ behavior: 'smooth', block: 'start' }),
    );
    // Fade the highlight after a moment so it's a cue, not permanent.
    window.setTimeout(() => setHighlight((h) => (h === key ? null : h)), 1600);
  };
  const expandAll = () => setOpenEpics(new Set((tree?.epics ?? []).map((e) => e.issue.key)));
  const collapseAll = () => setOpenEpics(new Set());

  return (
    <div className="space-y-4">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <h2 className="text-lg font-semibold">Requirement coverage</h2>
        {(versions.data?.length ?? 0) > 0 && (
          <div className="flex items-center gap-2 text-sm">
            <span className="text-muted">Release</span>
            <ReleasePicker releases={versions.data!} value={releaseName} onChange={setSelected} />
          </div>
        )}
      </div>

      {versions.isLoading ? (
        <div className="flex items-center gap-2 text-muted"><Spinner /> Loading releases…</div>
      ) : versions.error ? (
        <EmptyState title="Couldn't load releases" description="This project needs a connected JIRA project with fix versions. Configure it under Integrations." />
      ) : (versions.data?.length ?? 0) === 0 ? (
        <EmptyState title="No JIRA releases" description="Connect a JIRA project (Integrations) and define fix versions to see coverage." />
      ) : coverage.isLoading ? (
        <div className="flex items-center gap-2 text-muted"><Spinner /> Loading coverage…</div>
      ) : coverage.error ? (
        <EmptyState title="Couldn't load coverage" description={(coverage.error as Error).message} />
      ) : tree && breakdown ? (
        <>
          {/* Overview: donut + legend + coverage-by-epic grid */}
          <div className="flex flex-wrap items-center gap-6 rounded-md border border-border bg-surface p-4">
            <div className="flex items-center gap-4">
              <Donut b={breakdown} />
              <div className="space-y-1.5">
                <LegendDot cls="bg-green-500" label="Passing" n={breakdown.passing} />
                <LegendDot cls="bg-red-500" label="Failing" n={breakdown.failing} />
                <LegendDot cls="bg-surface-2" label="Uncovered" n={breakdown.uncovered} />
              </div>
            </div>
            {tree.epics.length > 0 && (
              <div className="min-w-[16rem] flex-1">
                <div className="mb-1.5 text-xs font-medium text-muted">Coverage by epic ({tree.epics.length})</div>
                <EpicGrid tree={tree} onOpen={openAndScroll} />
              </div>
            )}
          </div>

          {/* Controls */}
          <div className="flex items-center justify-between gap-3">
            <label className="flex items-center gap-2 text-sm">
              <input type="checkbox" checked={showUncoveredOnly} onChange={(e) => setShowUncoveredOnly(e.target.checked)} className="h-4 w-4 accent-primary" />
              Show uncovered only
            </label>
            <div className="flex items-center gap-3 text-xs">
              <button type="button" onClick={expandAll} className="text-primary hover:underline">Expand all</button>
              <span className="text-muted">·</span>
              <button type="button" onClick={collapseAll} className="text-primary hover:underline">Collapse all</button>
            </div>
          </div>

          <CoverageTree tree={tree} openEpics={openEpics} onToggleEpic={toggleEpic} showUncoveredOnly={showUncoveredOnly} highlightedKey={highlight} />
        </>
      ) : null}
    </div>
  );
}

// Standalone route at /projects/:projectId/coverage.
export default function CoverageView() {
  const { projectId } = useParams({ from: '/projects/$projectId/coverage' });
  return (
    <div className="space-y-5">
      <Link to="/projects/$projectId" params={{ projectId }} className="inline-flex items-center gap-1 text-sm text-muted hover:text-foreground">
        <ArrowLeft className="h-3 w-3" /> Back to {projectId}
      </Link>
      <p className="text-sm text-muted">{projectId} · JIRA requirements covered by test runs</p>
      <CoverageContent projectId={projectId} />
    </div>
  );
}
