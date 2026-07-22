import { ChevronRight, CheckCircle2, XCircle, Circle } from 'lucide-react';
import {
  coveragePercent,
  storyResult,
  type EpicCoverageNode,
  type RequirementCoverageTree,
  type StoryCoverageNode,
  type StoryResult,
  type TestRunCoverage,
} from './coverage';

const DOT: Record<StoryResult, string> = {
  passing: 'text-green-600 dark:text-green-400',
  failing: 'text-red-600 dark:text-red-400',
  uncovered: 'text-muted',
};

function ResultIcon({ result }: { result: StoryResult }) {
  const cls = `h-3.5 w-3.5 shrink-0 ${DOT[result]}`;
  if (result === 'passing') return <CheckCircle2 className={cls} />;
  if (result === 'failing') return <XCircle className={cls} />;
  return <Circle className={cls} />;
}

function CoveragePill({ cov, result }: { cov: TestRunCoverage; result: StoryResult }) {
  const tint = result === 'failing' ? 'text-red-600 dark:text-red-400' : 'text-green-600 dark:text-green-400';
  return (
    <span className="inline-flex items-center gap-1 text-[11px] text-muted">
      <span className={tint}>{cov.passed}✓</span>
      {cov.failed > 0 && <span className="text-red-600 dark:text-red-400">{cov.failed}✗</span>}
      {cov.skipped > 0 && <span>{cov.skipped}⤼</span>}
      <span>of {cov.total}</span>
      {cov.lastRunAt && <span>· {new Date(cov.lastRunAt).toLocaleDateString()}</span>}
    </span>
  );
}

// Segmented bar: green (passing) + red (failing) over a grey track.
function CoverageBar({ passing, failing, total }: { passing: number; failing: number; total: number }) {
  const pct = (n: number) => (total > 0 ? (n / total) * 100 : 0);
  return (
    <span className="inline-flex h-1.5 w-24 overflow-hidden rounded-full bg-surface-2 align-middle">
      <span className="block h-full bg-green-500" style={{ width: `${pct(passing)}%` }} />
      <span className="block h-full bg-red-500" style={{ width: `${pct(failing)}%` }} />
    </span>
  );
}

function StoryRow({ story, depth = 0 }: { story: StoryCoverageNode; depth?: number }) {
  const result = storyResult(story);
  const subTasks = story.subTasks ?? [];
  return (
    <div>
      <div
        className="flex items-center gap-2 rounded px-2 py-1 hover:bg-surface-2"
        style={{ paddingLeft: `${0.5 + depth * 1.25}rem` }}
      >
        <ResultIcon result={result} />
        <span className="font-mono text-[11px] text-muted">{story.issue.key}</span>
        <span className="truncate text-sm">{story.issue.summary}</span>
        <span className="ml-auto shrink-0">
          {story.covered && story.testRunCoverage ? (
            <CoveragePill cov={story.testRunCoverage} result={result} />
          ) : (
            <span className="text-[11px] text-muted">uncovered</span>
          )}
        </span>
      </div>
      {subTasks.map((st) => (
        <StoryRow key={st.issue.key} story={st} depth={depth + 1} />
      ))}
    </div>
  );
}

function epicStats(epic: EpicCoverageNode) {
  let passing = 0;
  let failing = 0;
  for (const s of epic.stories) {
    const r = storyResult(s);
    if (r === 'passing') passing += 1;
    else if (r === 'failing') failing += 1;
  }
  return { passing, failing };
}

function EpicSection({
  epic,
  open,
  onToggle,
  showUncoveredOnly,
  highlighted,
}: {
  epic: EpicCoverageNode;
  open: boolean;
  onToggle: () => void;
  showUncoveredOnly: boolean;
  highlighted: boolean;
}) {
  const pct = coveragePercent(epic.coveredCount, epic.totalCount);
  const { passing, failing } = epicStats(epic);
  const stories = showUncoveredOnly ? epic.stories.filter((s) => !s.covered) : epic.stories;
  return (
    <div
      id={`epic-${epic.issue.key}`}
      className={`scroll-mt-4 rounded-md border bg-surface transition-shadow ${
        highlighted ? 'border-primary ring-2 ring-primary/50' : 'border-border'
      }`}
    >
      <button
        type="button"
        onClick={onToggle}
        className="flex w-full items-center gap-2 px-3 py-2 text-left"
      >
        <ChevronRight className={`h-3.5 w-3.5 shrink-0 text-muted transition-transform ${open ? 'rotate-90' : ''}`} />
        <span className="font-mono text-[11px] text-muted">{epic.issue.key}</span>
        <span className="truncate text-sm font-medium">{epic.issue.summary}</span>
        <span className="ml-auto flex items-center gap-2">
          <span className="text-[11px] text-muted">{epic.coveredCount}/{epic.totalCount}</span>
          <CoverageBar passing={passing} failing={failing} total={epic.totalCount} />
          <span className="w-8 text-right text-[11px] tabular-nums text-muted">{pct}%</span>
        </span>
      </button>
      {open && (
        <div className="border-t border-border py-1">
          {stories.length === 0 ? (
            <p className="px-3 py-1.5 text-[11px] text-muted">
              {showUncoveredOnly ? 'All stories covered.' : 'No stories under this epic.'}
            </p>
          ) : (
            stories.map((s) => <StoryRow key={s.issue.key} story={s} />)
          )}
        </div>
      )}
    </div>
  );
}

export function CoverageTree({
  tree,
  openEpics,
  onToggleEpic,
  showUncoveredOnly = false,
  highlightedKey = null,
}: {
  tree: RequirementCoverageTree;
  openEpics: Set<string>;
  onToggleEpic: (key: string) => void;
  showUncoveredOnly?: boolean;
  highlightedKey?: string | null;
}) {
  const epics = showUncoveredOnly
    ? tree.epics.filter((e) => e.coveredCount < e.totalCount)
    : tree.epics;
  const unassigned = showUncoveredOnly ? tree.unassigned.filter((s) => !s.covered) : tree.unassigned;

  return (
    <div className="space-y-3">
      {epics.map((epic) => (
        <EpicSection
          key={epic.issue.key}
          epic={epic}
          open={openEpics.has(epic.issue.key)}
          onToggle={() => onToggleEpic(epic.issue.key)}
          showUncoveredOnly={showUncoveredOnly}
          highlighted={highlightedKey === epic.issue.key}
        />
      ))}

      {unassigned.length > 0 && (
        <div className="rounded-md border border-border bg-surface">
          <div className="px-3 py-2 text-sm font-medium">
            Unassigned stories
            <span className="ml-2 text-[11px] text-muted">
              ({tree.unassigned.filter((s) => s.covered).length}/{tree.unassigned.length} covered)
            </span>
          </div>
          <div className="border-t border-border py-1">
            {unassigned.map((s) => (
              <StoryRow key={s.issue.key} story={s} />
            ))}
          </div>
        </div>
      )}

      {epics.length === 0 && unassigned.length === 0 && (
        <p className="text-sm text-muted">
          {showUncoveredOnly ? 'Everything in this release is covered. 🎉' : 'No requirements found for this release.'}
        </p>
      )}
    </div>
  );
}
