import { useEffect, useMemo, useRef, useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import * as d3 from 'd3';
import { Spinner } from '@/components/ui/Spinner';
import { Card } from '@/components/ui/Card';
import { EmptyState } from '@/components/ui/EmptyState';
import { graphqlFetch } from '@/lib/api';
import { formatDuration } from '@/lib/duration';

const GET_TREEMAP = /* GraphQL */ `
  query GetTreemapData($projectId: String, $suiteName: String, $days: Int) {
    treemapData(projectId: $projectId, suiteName: $suiteName, days: $days) {
      projects {
        project { id projectId name }
        suites {
          suite { id suiteName status }
          specs {
            spec { id specName status duration isFlaky }
            duration
            status
            isFlaky
            totalRuns
            passedRuns
            failedRuns
            skippedRuns
            passRate
          }
          totalDuration
          totalSpecs
          passRate
        }
        totalDuration
        totalTests
        passRate
        totalRuns
      }
      totalDuration
      totalTests
      overallPassRate
    }
  }
`;

interface ProjectAgg {
  project: { id: string; projectId: string; name: string };
  suites: SuiteAgg[];
  totalDuration: number;
  totalTests: number;
  passRate: number;
  totalRuns: number;
}
interface SuiteAgg {
  suite: { id: string; suiteName: string; status: string };
  specs: SpecAgg[];
  totalDuration: number;
  totalSpecs: number;
  passRate: number;
}
interface SpecAgg {
  spec: { id: string; specName: string; status: string; duration: number; isFlaky: boolean };
  duration: number;
  status: string;        // majority outcome — used for the label/badge, not for color
  isFlaky: boolean;
  totalRuns: number;
  passedRuns: number;
  failedRuns: number;
  skippedRuns: number;
  passRate: number;      // continuous, same shape as suite/project levels — drives tile color
}
interface TreemapResp {
  treemapData: {
    projects: ProjectAgg[];
    totalDuration: number;
    totalTests: number;
    overallPassRate: number;
  };
}

// DEPRECATED — TreemapPage was the standalone /v2/treemap route that the
// router no longer wires up. The treemap UX now lives inside the Test
// Summaries page's Tree view (one entry point, with filtering). Kept as
// a no-op default export so old bookmarks don't fail to import. Drop
// once Phase 5 (legacy removal) lands.
// eslint-disable-next-line react-refresh/only-export-components
export default function _TreemapPageDeprecated() {
  const [days, setDays] = useState(7);
  return (
    <div className="space-y-6">
      <header className="flex flex-wrap items-end justify-between gap-3">
        <div>
          <h1 className="text-3xl font-semibold tracking-tight">Treemap</h1>
          <p className="mt-1 text-sm text-muted">
            Project &amp; suite test volume. Tile area scales with test count;
            color scales with pass rate.
          </p>
        </div>
        <DaysToggle days={days} setDays={setDays} options={[1, 7, 30]} />
      </header>
      <TreemapView days={days} />
    </div>
  );
}

function DaysToggle({
  days,
  setDays,
  options,
}: {
  days: number;
  setDays: (d: number) => void;
  options: number[];
}) {
  return (
    <div className="flex overflow-hidden rounded-md border border-border bg-surface">
      {options.map((d) => (
        <button
          key={d}
          onClick={() => setDays(d)}
          className={`px-2.5 py-1 text-xs transition-colors ${
            days === d
              ? 'bg-primary text-white'
              : 'text-foreground hover:bg-surface-2'
          }`}
        >
          {d === 1 ? '24h' : `${d}d`}
        </button>
      ))}
    </div>
  );
}

// TreemapView renders the treemap card (drill-down, tiles, stats) for the
// given `days` window. It owns its own drill state so callers (Summaries
// page) can embed it without managing drilling. The standalone /treemap
// page is just this view + a page-level header.
//
// `projectIdAllowlist` (optional) restricts top-level tiles and the stat
// rollup to the listed projects. The Summaries page passes its current
// filter (team/category/search/favorites) through this prop so the tree
// view honours the same scope as the card view. `undefined` = no filter;
// `[]` = filter is active but nothing matches.
export function TreemapView({
  days,
  projectIdAllowlist,
}: {
  days: number;
  projectIdAllowlist?: readonly string[];
}) {
  const [drillProjectId, setDrillProjectId] = useState<string | null>(null);
  const [drillSuiteName, setDrillSuiteName] = useState<string | null>(null);

  const { data, isLoading, error } = useQuery({
    queryKey: ['treemap', { days, projectId: drillProjectId, suiteName: drillSuiteName }],
    queryFn: () =>
      graphqlFetch<TreemapResp>(GET_TREEMAP, {
        days,
        ...(drillProjectId ? { projectId: drillProjectId } : {}),
        ...(drillSuiteName ? { suiteName: drillSuiteName } : {}),
      }),
    staleTime: 30_000,
  });

  // When a filter scope is active and includes 0 projects, drilling
  // doesn't make sense — but we still hit the server every time. The
  // resolver doesn't accept an allowlist today, so the cheapest fix is
  // a client-side filter on the returned projects array. The treemap
  // server response is already 60s-cached.
  const allowSet = useMemo(
    () =>
      projectIdAllowlist ? new Set(projectIdAllowlist) : null,
    [projectIdAllowlist],
  );

  const filteredProjects = useMemo(() => {
    const projects = data?.treemapData.projects ?? [];
    if (!allowSet) return projects;
    return projects.filter((p) => allowSet.has(p.project.projectId));
  }, [data, allowSet]);

  // Re-roll stat tiles from the filtered projects so the header stats
  // line up with the visible tiles. (Server's `totalTests` etc. cover
  // the unfiltered universe.)
  const filteredTotals = useMemo(() => {
    if (!data) return null;
    if (!allowSet) {
      return {
        totalTests: data.treemapData.totalTests,
        totalDuration: data.treemapData.totalDuration,
        overallPassRate: data.treemapData.overallPassRate ?? 0,
      };
    }
    let totalTests = 0;
    let totalDuration = 0;
    let weightedPass = 0;
    for (const p of filteredProjects) {
      totalTests += p.totalTests;
      totalDuration += p.totalDuration;
      weightedPass += p.passRate * p.totalTests;
    }
    return {
      totalTests,
      totalDuration,
      overallPassRate: totalTests > 0 ? weightedPass / totalTests : 0,
    };
  }, [data, allowSet, filteredProjects]);

  // Tile data depends on drill state. Three levels:
  //   no drill           → one tile per project (drillable)
  //   project only       → one tile per suite of that project (drillable to specs)
  //   project + suite    → one tile per spec of that suite (leaf level)
  const tiles = useMemo<Tile[]>(() => {
    if (!data) return [];

    // Bottom level: specs inside a single suite. Reads from the matching
    // suite's `specs` array (populated only when the server sees both
    // projectId + suiteName).
    if (drillProjectId && drillSuiteName) {
      const proj = data.treemapData.projects.find(
        (p) => p.project.projectId === drillProjectId,
      );
      const suite = proj?.suites.find((s) => s.suite.suiteName === drillSuiteName);
      return (suite?.specs ?? []).map((sp) => ({
        id: sp.spec.id,
        label: sp.spec.specName,
        value: Math.max(sp.duration, 1),
        // Use the continuous passRate (PassedRuns/TotalRuns) so a spec
        // with 99/100 runs passing reads as the same green as its 99%
        // parent suite — not bright red because of a single failure.
        // Status string is still surfaced via the `sub` label/tooltip
        // for the majority-outcome badge; flaky specs get the `flaky`
        // flag so the SVG can mark them distinctly.
        passRate: sp.passRate,
        duration: sp.duration,
        sub: `${sp.passedRuns}/${sp.totalRuns} passed${sp.isFlaky ? ' · flaky' : ''}`,
        meta: sp,
        flaky: sp.isFlaky,
      }));
    }

    // Middle level: suites inside a project. Each suite tile is
    // drillable to its specs.
    if (drillProjectId) {
      const proj = data.treemapData.projects.find(
        (p) => p.project.projectId === drillProjectId,
      );
      return (proj?.suites ?? []).map((s) => ({
        id: s.suite.id,
        label: s.suite.suiteName,
        value: Math.max(s.totalSpecs, 1),
        passRate: s.passRate,
        duration: s.totalDuration,
        sub: `${s.totalSpecs} specs`,
        meta: s,
        drillKey: s.suite.suiteName,
      }));
    }

    // Top level: projects. Drillable to suites.
    return filteredProjects.map((p) => ({
      id: p.project.id,
      label: p.project.name,
      value: Math.max(p.totalTests, 1),
      passRate: p.passRate,
      duration: p.totalDuration,
      sub: `${p.totalRuns} runs · ${p.totalTests} tests`,
      meta: p,
      drillKey: p.project.projectId,
    }));
  }, [data, drillProjectId, drillSuiteName, filteredProjects]);

  // Breadcrumb shape for the three levels.
  const breadcrumbs = [
    {
      label: 'Projects',
      onClick: () => {
        setDrillProjectId(null);
        setDrillSuiteName(null);
      },
      active: !drillProjectId,
    },
    ...(drillProjectId
      ? [
          {
            label: drillProjectId,
            onClick: () => setDrillSuiteName(null),
            active: !drillSuiteName,
          },
        ]
      : []),
    ...(drillSuiteName
      ? [
          {
            label: drillSuiteName,
            onClick: () => undefined,
            active: true,
          },
        ]
      : []),
  ];

  return (
    <div className="space-y-4">
      {drillProjectId && (
        <nav aria-label="Breadcrumb" className="flex items-center gap-1.5 text-xs">
          {breadcrumbs.map((b, i) => (
            <span key={i} className="inline-flex items-center gap-1.5">
              {i > 0 && <span className="text-muted">/</span>}
              {b.active ? (
                <span className="font-medium text-foreground">{b.label}</span>
              ) : (
                <button
                  type="button"
                  onClick={b.onClick}
                  className="text-muted hover:text-primary"
                >
                  {b.label}
                </button>
              )}
            </span>
          ))}
        </nav>
      )}

      {filteredTotals && (
        <div className="grid gap-3 sm:grid-cols-3">
          <StatTile label="Total tests"  value={filteredTotals.totalTests.toLocaleString()} />
          <StatTile label="Total runtime" value={formatDuration(filteredTotals.totalDuration)} />
          <StatTile
            label="Overall pass rate"
            value={`${Math.round(filteredTotals.overallPassRate * 100)}%`}
          />
        </div>
      )}

      <Card className="overflow-hidden">
        {isLoading && (
          <div className="flex items-center gap-2 p-6 text-sm text-muted">
            <Spinner /> Loading treemap…
          </div>
        )}
        {error && (
          <div className="p-6">
            <EmptyState title="Couldn't load treemap" description={(error as Error).message} />
          </div>
        )}
        {!isLoading && !error && tiles.length === 0 && (
          <div className="p-6">
            <EmptyState
              title="No data in this window"
              description="Try a wider window or seed more runs (`make docker-test-seed`)."
            />
          </div>
        )}
        {tiles.length > 0 && (
          <TreemapSvg
            tiles={tiles}
            onDrill={(t) => {
              if (!t.drillKey) return;
              // Top-level click → drill into project. Middle-level
              // click → drill into suite. Leaf (spec) tiles have no
              // drillKey so onDrill is a no-op there.
              if (!drillProjectId) {
                setDrillProjectId(t.drillKey);
              } else if (!drillSuiteName) {
                setDrillSuiteName(t.drillKey);
              }
            }}
          />
        )}
        {drillProjectId && (
          <div className="border-t border-border bg-surface-2 px-4 py-2 text-xs text-muted">
            {drillSuiteName ? (
              <>
                Showing specs in{' '}
                <code className="font-mono text-foreground">{drillSuiteName}</code>
                {' '}of{' '}
                <code className="font-mono text-foreground">{drillProjectId}</code>
              </>
            ) : (
              <>
                Showing suites in{' '}
                <code className="font-mono text-foreground">{drillProjectId}</code>
              </>
            )}
          </div>
        )}
      </Card>
    </div>
  );
}

interface Tile {
  id: string;
  label: string;
  value: number;        // drives area
  passRate: number;     // drives color
  duration: number;
  sub: string;
  meta: ProjectAgg | SuiteAgg | SpecAgg;
  drillKey?: string;
  flaky?: boolean;      // spec-level only
}

function passRateColor(rate: number): string {
  // Diverging scale: red (0) → amber (0.7) → green (1)
  const stops: Array<[number, string]> = [
    [0,   '#ef4444'],
    [0.5, '#f59e0b'],
    [0.85,'#84cc16'],
    [1,   '#10b981'],
  ];
  for (let i = 0; i < stops.length - 1; i++) {
    const [a, ca] = stops[i]!;
    const [b, cb] = stops[i + 1]!;
    if (rate <= b) {
      const t = (rate - a) / (b - a);
      return d3.interpolateRgb(ca, cb)(t);
    }
  }
  return stops[stops.length - 1]![1];
}

function TreemapSvg({
  tiles,
  onDrill,
}: {
  tiles: Tile[];
  onDrill: (tile: Tile) => void;
}) {
  const ref = useRef<SVGSVGElement>(null);
  const [hover, setHover] = useState<{
    tile: Tile;
    x0: number;
    y0: number;
    x1: number;
    y1: number;
  } | null>(null);

  const layout = useMemo(() => {
    // d3.hierarchy expects a recursive shape where every node has the
    // same type. Our root is a synthetic { children: Tile[] }, so the
    // leaf type is technically Tile | { children: Tile[] }. We narrow
    // back to Tile after computing the layout — d3 only returns leaves.
    type HierarchyNode = Tile | { children: Tile[] };
    const root = d3
      .hierarchy<HierarchyNode>({ children: tiles })
      .sum((d) => ('value' in d ? d.value ?? 0 : 0))
      .sort((a, b) => (b.value ?? 0) - (a.value ?? 0));

    d3.treemap<HierarchyNode>()
      .size([1000, 560])
      .paddingInner(3)
      .paddingOuter(4)
      .round(true)(root as d3.HierarchyRectangularNode<HierarchyNode>);

    return root.leaves() as unknown as Array<d3.HierarchyRectangularNode<Tile>>;
  }, [tiles]);

  useEffect(() => {
    // No-op: we render via React for accessibility. d3 owns layout only.
    void ref.current;
  }, [layout]);

  return (
    <div>
      <div className="relative">
      <svg
        ref={ref}
        viewBox="0 0 1000 560"
        preserveAspectRatio="xMidYMid meet"
        className="block w-full"
        role="figure"
        aria-label="Treemap of test volume by project and pass rate"
      >
        {layout.map((node) => {
          const tile = node.data as unknown as Tile;
          const w = node.x1 - node.x0;
          const h = node.y1 - node.y0;
          const fill = passRateColor(tile.passRate);
          const hasRoom = w > 70 && h > 32;
          return (
            <g
              key={tile.id}
              transform={`translate(${node.x0},${node.y0})`}
              onMouseEnter={() =>
                setHover({
                  tile,
                  x0: node.x0,
                  y0: node.y0,
                  x1: node.x1,
                  y1: node.y1,
                })
              }
              onMouseLeave={() => setHover(null)}
              onClick={() => onDrill(tile)}
              role={tile.drillKey ? 'button' : undefined}
              tabIndex={tile.drillKey ? 0 : undefined}
              onKeyDown={(e) => {
                if (tile.drillKey && (e.key === 'Enter' || e.key === ' ')) {
                  e.preventDefault();
                  onDrill(tile);
                }
              }}
              className={tile.drillKey ? 'cursor-pointer' : 'cursor-default'}
            >
              <rect
                width={w}
                height={h}
                fill={fill}
                rx={4}
                ry={4}
                stroke="white"
                strokeWidth={1}
                opacity={hover && hover.tile.id !== tile.id ? 0.45 : 1}
              >
                <title>
                  {`${tile.label}\n${Math.round(tile.passRate * 100)}% pass · ${tile.sub} · ${formatDuration(tile.duration)}`}
                </title>
              </rect>
              {hasRoom && (
                <>
                  <text
                    x={8}
                    y={18}
                    fill="white"
                    style={{ fontSize: 12, fontWeight: 600, pointerEvents: 'none', textShadow: '0 1px 2px rgba(0,0,0,0.3)' }}
                  >
                    {truncate(tile.label, Math.floor(w / 7))}
                  </text>
                  <text
                    x={8}
                    y={32}
                    fill="white"
                    style={{ fontSize: 10, opacity: 0.9, pointerEvents: 'none', textShadow: '0 1px 2px rgba(0,0,0,0.3)' }}
                  >
                    {Math.round(tile.passRate * 100)}%
                  </text>
                </>
              )}
            </g>
          );
        })}
      </svg>

      {hover && (() => {
        // Anchor the tooltip next to the hovered tile.
        //
        // Horizontal: by default put the tooltip just to the right of
        // the tile (left edge of tooltip = right edge of tile + gap).
        // If the tile's right edge is past 75% of the SVG width, flip
        // to the left side so the tooltip doesn't clip off-screen.
        //
        // Vertical: by default align the tooltip's top with the tile's
        // top so the popup sits alongside the tile, not below it. If
        // the tile's top is in the bottom 30% of the SVG, anchor the
        // tooltip's bottom to the tile's bottom and grow upward.
        const showOnLeft = hover.x1 > 750;
        const showAbove = hover.y0 > 392; // 70% of 560
        const style: React.CSSProperties = showOnLeft
          ? { right: `calc(${(1000 - hover.x0) / 10}% + 8px)` }
          : { left: `calc(${hover.x1 / 10}% + 8px)` };
        if (showAbove) {
          style.bottom = `calc(${(560 - hover.y1) / 5.6}% + 0px)`;
        } else {
          style.top = `calc(${hover.y0 / 5.6}% + 0px)`;
        }
        return (
          <div
            className="pointer-events-none absolute z-10 max-w-xs rounded-lg border border-border bg-surface/95 p-3 text-xs shadow-lg backdrop-blur"
            style={style}
          >
            <div className="font-semibold text-foreground">{hover.tile.label}</div>
            <div className="mt-1 grid grid-cols-2 gap-x-3 gap-y-0.5 text-muted">
              <span>Pass rate</span>
              <span className="text-right text-foreground tabular-nums">
                {Math.round(hover.tile.passRate * 100)}%
              </span>
              <span>Duration</span>
              <span className="text-right text-foreground tabular-nums">
                {formatDuration(hover.tile.duration)}
              </span>
              <span>Size</span>
              <span className="text-right text-foreground tabular-nums">
                {hover.tile.sub}
              </span>
            </div>
            {hover.tile.drillKey && (
              <div className="mt-2 border-t border-border pt-1.5 text-primary">
                Click to drill in →
              </div>
            )}
          </div>
        );
      })()}
      </div>

      <div className="border-t border-border bg-surface-2 px-4 py-2">
        <div className="flex items-center gap-3 text-[11px] text-muted">
          <span>Pass rate</span>
          <div
            className="h-2 w-40 rounded"
            style={{
              background:
                'linear-gradient(to right, #ef4444 0%, #f59e0b 50%, #84cc16 85%, #10b981 100%)',
            }}
          />
          <span>0% → 100%</span>
          <span className="ml-auto">Tile area = test volume</span>
        </div>
      </div>
    </div>
  );
}

function StatTile({ label, value }: { label: string; value: string }) {
  return (
    <Card className="p-4">
      <div className="text-[11px] font-medium uppercase tracking-wider text-muted">
        {label}
      </div>
      <div className="mt-1 text-2xl font-semibold tracking-tight tabular-nums">{value}</div>
    </Card>
  );
}

function truncate(s: string, max: number): string {
  if (s.length <= max) return s;
  if (max < 4) return '…';
  return s.slice(0, max - 1) + '…';
}
