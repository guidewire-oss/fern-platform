import { useCallback, useEffect, useLayoutEffect, useMemo, useRef, useState } from 'react';
import { Save, RotateCcw, X, Search, Plug, Link2 } from 'lucide-react';
import { Card, CardBody, CardHeader, CardTitle } from '@/components/ui/Card';
import { Button } from '@/components/ui/Button';
import { Spinner } from '@/components/ui/Spinner';
import { useJiraConnections } from './hooks';
import {
  FERN_FIELDS,
  REDUCTION_STRATEGIES,
  validateMapping,
  type FernField,
  type FieldMappingEntry,
  type ReductionStrategy,
} from './fieldMapping';
import {
  useJiraFieldMapping,
  useJiraFields,
  useResetJiraFieldMapping,
  useSaveJiraFieldMapping,
} from './fieldMappingHooks';
import { portCoords, bezierPath, type LineCoords } from './mappingLines';

type Mappings = Partial<Record<FernField, string>>;
type Strategies = Partial<Record<FernField, ReductionStrategy>>;
interface DragState {
  jiraFieldId: string;
  startX: number;
  startY: number;
}
type Line = LineCoords & { fern: FernField; jira: string };

// Friendly strategy labels, matching v1's field-mapping modal.
const STRATEGY_LABELS: Record<ReductionStrategy, string> = {
  FIRST_VALUE: 'First value only',
  CONCATENATE: 'Concatenate all values',
  SEPARATE_ENTRIES: 'Create separate entries',
};

// FieldMappingVisual is the v1-parity drag-to-connect editor: Fern fields
// on the left, JIRA fields on the right, SVG connectors between mapped
// pairs. Drag from a JIRA field to a Fern field to map. A multi-value
// JIRA field mapped onto a single-value Fern field requires a reduction
// strategy (mirrors validateMapping / the server rule).
export function FieldMappingVisual({ projectId, canManage }: { projectId: string; canManage: boolean }) {
  const connections = useJiraConnections(projectId);
  const connection = connections.data?.[0];
  const mappingQ = useJiraFieldMapping(projectId);
  const jiraFieldsQ = useJiraFields(connection?.id);
  const save = useSaveJiraFieldMapping(projectId);
  const reset = useResetJiraFieldMapping(projectId);

  const [mappings, setMappings] = useState<Mappings>({});
  const [strategies, setStrategies] = useState<Strategies>({});
  const [drag, setDrag] = useState<DragState | null>(null);
  const [cursor, setCursor] = useState<{ x: number; y: number } | null>(null);
  const [lines, setLines] = useState<Line[]>([]);
  const [filter, setFilter] = useState('');

  const containerRef = useRef<HTMLDivElement>(null);
  const fernRefs = useRef(new Map<FernField, HTMLElement>());
  const jiraRefs = useRef(new Map<string, HTMLElement>());

  // Seed editable state from the stored mapping.
  useEffect(() => {
    if (!mappingQ.data) return;
    const m: Mappings = {};
    const s: Strategies = {};
    for (const e of mappingQ.data.entries) {
      if (e.jiraFieldId) m[e.fernField] = e.jiraFieldId;
      if (e.reductionStrategy) s[e.fernField] = e.reductionStrategy;
    }
    setMappings(m);
    setStrategies(s);
  }, [mappingQ.data]);

  const fieldOptions = useMemo(() => jiraFieldsQ.data ?? [], [jiraFieldsQ.data]);
  const fieldById = useMemo(() => new Map(fieldOptions.map((f) => [f.id, f])), [fieldOptions]);
  const mappedJiraIds = useMemo(() => new Set(Object.values(mappings)), [mappings]);

  const filteredJira = useMemo(() => {
    const q = filter.trim().toLowerCase();
    return q ? fieldOptions.filter((f) => f.name.toLowerCase().includes(q) || f.id.toLowerCase().includes(q)) : fieldOptions;
  }, [fieldOptions, filter]);

  const entries: FieldMappingEntry[] = useMemo(
    () =>
      FERN_FIELDS.filter((f) => mappings[f.field]).map((f) => {
        const jiraFieldId = mappings[f.field] as string;
        return {
          fernField: f.field,
          jiraFieldId,
          jiraFieldIsMultiValue: fieldById.get(jiraFieldId)?.multiValue ?? false,
          reductionStrategy: strategies[f.field] ?? null,
        };
      }),
    [mappings, strategies, fieldById],
  );

  const errors = useMemo(() => validateMapping(entries), [entries]);
  const errorByField = useMemo(() => {
    const m = new Map<FernField, string>();
    for (const e of errors) if (e.fernField && !m.has(e.fernField)) m.set(e.fernField, e.message);
    return m;
  }, [errors]);

  const recomputeLines = useCallback(() => {
    const c = containerRef.current;
    if (!c) return;
    const cr = c.getBoundingClientRect();
    const next: Line[] = [];
    for (const [fern, jira] of Object.entries(mappings) as [FernField, string][]) {
      const fEl = fernRefs.current.get(fern);
      const jEl = jiraRefs.current.get(jira);
      if (fEl && jEl) {
        next.push({ fern, jira, ...portCoords(jEl.getBoundingClientRect(), fEl.getBoundingClientRect(), cr) });
      }
    }
    setLines(next);
  }, [mappings]);

  // Redraw on mapping/field changes and on any layout shift.
  useLayoutEffect(() => {
    const raf = requestAnimationFrame(recomputeLines);
    return () => cancelAnimationFrame(raf);
  }, [recomputeLines, filteredJira]);

  useEffect(() => {
    const onResize = () => requestAnimationFrame(recomputeLines);
    const ro = new ResizeObserver(onResize);
    const c = containerRef.current;
    if (c) ro.observe(c);
    window.addEventListener('resize', onResize);
    const scrollers = c?.querySelectorAll('[data-scrollable]') ?? [];
    scrollers.forEach((el) => el.addEventListener('scroll', onResize, { passive: true }));
    return () => {
      ro.disconnect();
      window.removeEventListener('resize', onResize);
      scrollers.forEach((el) => el.removeEventListener('scroll', onResize));
    };
  }, [recomputeLines]);

  const toContainer = (clientX: number, clientY: number) => {
    const cr = containerRef.current?.getBoundingClientRect();
    return cr ? { x: clientX - cr.left, y: clientY - cr.top } : { x: 0, y: 0 };
  };

  const startDrag = (jiraFieldId: string, e: React.MouseEvent) => {
    if (!canManage || mappedJiraIds.has(jiraFieldId)) return;
    const jEl = jiraRefs.current.get(jiraFieldId);
    const cr = containerRef.current?.getBoundingClientRect();
    if (!jEl || !cr) return;
    const r = jEl.getBoundingClientRect();
    setDrag({ jiraFieldId, startX: r.right - cr.left, startY: r.top + r.height / 2 - cr.top });
    setCursor(toContainer(e.clientX, e.clientY));
    e.preventDefault();
  };

  const connect = (fern: FernField) => {
    if (!drag) return;
    setMappings((m) => ({ ...m, [fern]: drag.jiraFieldId }));
    setStrategies((s) => {
      const next = { ...s };
      delete next[fern]; // choose a fresh strategy for the new pairing
      return next;
    });
    setDrag(null);
    setCursor(null);
  };

  const disconnect = (fern: FernField) => {
    setMappings((m) => {
      const next = { ...m };
      delete next[fern];
      return next;
    });
    setStrategies((s) => {
      const next = { ...s };
      delete next[fern];
      return next;
    });
  };

  if (connections.isLoading || mappingQ.isLoading) {
    return <Card><CardBody><div className="flex items-center gap-2 text-muted"><Spinner /> Loading field mapping…</div></CardBody></Card>;
  }
  if (!connection) {
    return (
      <Card>
        <CardHeader><CardTitle>JIRA field mapping</CardTitle></CardHeader>
        <CardBody><p className="text-sm text-muted">Connect a JIRA project above to map Fern fields to JIRA fields.</p></CardBody>
      </Card>
    );
  }

  return (
    <Card>
      <CardHeader><CardTitle>JIRA field mapping</CardTitle></CardHeader>
      <CardBody className="space-y-3">
        <p className="text-xs text-muted">
          <strong className="text-foreground">Drag &amp; connect:</strong> press and hold a JIRA field
          (left), drag it onto a Fern field (right), and release to map. A multi-value JIRA field on a
          single-value Fern field needs a reduction strategy. Required fields are marked{' '}
          <span className="text-red-600">*</span>.
        </p>
        {jiraFieldsQ.isError && (
          <p className="text-[11px] text-status-failed-fg">
            Couldn't load JIRA fields — test the connection above so it's authenticated, then reopen.
          </p>
        )}

        {/* eslint-disable-next-line jsx-a11y/no-static-element-interactions -- pointer-driven drag canvas (matches v1); the ✕ / selects / buttons are the accessible controls */}
        <div
          ref={containerRef}
          className="relative grid grid-cols-2 gap-12"
          onMouseMove={(e) => drag && setCursor(toContainer(e.clientX, e.clientY))}
          onMouseUp={() => { setDrag(null); setCursor(null); }}
          onMouseLeave={() => { setDrag(null); setCursor(null); }}
        >
          {/* SVG connector overlay — above the columns (zIndex 5) so the
              opaque row backgrounds don't clip the lines; pointer-events
              stay off so drag still hits the rows underneath. overflow
              visible keeps curves that bow past the box edges drawn. */}
          <svg className="pointer-events-none absolute inset-0 h-full w-full overflow-visible" style={{ zIndex: 5 }}>
            {lines.map((l) => (
              <g key={`${l.fern}-${l.jira}`} className="text-primary">
                <path d={bezierPath(l)} fill="none" stroke="currentColor" strokeWidth={3} strokeLinecap="round" />
                <circle cx={l.x1} cy={l.y1} r={4} fill="currentColor" />
                <circle cx={l.x2} cy={l.y2} r={4} fill="currentColor" />
              </g>
            ))}
            {drag && cursor && (
              <path d={bezierPath({ x1: drag.startX, y1: drag.startY, x2: cursor.x, y2: cursor.y })}
                fill="none" stroke="currentColor" className="text-primary" strokeWidth={3}
                strokeLinecap="round" strokeDasharray="6 5" />
            )}
          </svg>

          {/* JIRA column (left, drag sources) */}
          <div className="space-y-2" style={{ zIndex: 2 }}>
            <div className="flex items-center gap-2 text-xs font-semibold uppercase tracking-wider text-muted">
              <Plug className="h-3.5 w-3.5" /> JIRA fields
              <span className="normal-case text-[10px] font-normal text-muted">({fieldOptions.length})</span>
              <span className="relative ml-auto normal-case">
                <Search className="pointer-events-none absolute left-2 top-1.5 h-3 w-3 text-muted" />
                <input
                  value={filter}
                  onChange={(e) => setFilter(e.target.value)}
                  placeholder="filter…"
                  className="w-32 rounded-md border border-border bg-surface py-1 pl-6 pr-2 text-xs font-normal focus:border-primary focus:outline-none"
                />
              </span>
            </div>
            <div data-scrollable className="max-h-[28rem] space-y-1.5 overflow-y-auto pr-1">
              {jiraFieldsQ.isLoading ? (
                <div className="flex items-center gap-2 text-xs text-muted"><Spinner /> Loading JIRA fields…</div>
              ) : filteredJira.length === 0 ? (
                <p className="rounded-md border border-dashed border-border px-3 py-2 text-[11px] text-muted">
                  {fieldOptions.length === 0 ? 'No JIRA fields yet — connect & test JIRA above so they load.' : 'No fields match the filter.'}
                </p>
              ) : (
                filteredJira.map((jf) => {
                  const used = mappedJiraIds.has(jf.id);
                  return (
                    // eslint-disable-next-line jsx-a11y/no-static-element-interactions -- drag source for pointer drag (v1 parity)
                    <div
                      key={jf.id}
                      ref={(el) => { if (el) jiraRefs.current.set(jf.id, el); else jiraRefs.current.delete(jf.id); }}
                      onMouseDown={(e) => startDrag(jf.id, e)}
                      className={`group flex items-center gap-2 rounded-md border bg-surface px-3 py-2 shadow-sm transition-colors ${
                        used ? 'opacity-45' : canManage ? 'cursor-grab border-border hover:border-primary hover:bg-primary/5' : 'border-border'
                      }`}
                    >
                      <div className="min-w-0 flex-1">
                        <div className="flex items-center gap-1.5">
                          <span className="truncate text-sm font-medium">{jf.name}</span>
                          {jf.custom && <span className="shrink-0 text-[10px] font-medium text-status-passed-fg">Custom</span>}
                        </div>
                        <div className="truncate font-mono text-[10px] text-muted">
                          {jf.id} · {jf.multiValue ? 'multivalue' : 'text'}
                        </div>
                      </div>
                      {/* right-edge port */}
                      <span className="h-2.5 w-2.5 shrink-0 rounded-full border-2 border-primary bg-surface group-hover:bg-primary" />
                    </div>
                  );
                })
              )}
            </div>
          </div>

          {/* Fern column (right, drop targets) */}
          <div className="space-y-2" style={{ zIndex: 2 }}>
            <div className="text-xs font-semibold uppercase tracking-wider text-muted">
              Fern fields <span className="normal-case text-[10px] font-normal">(drop a JIRA field here)</span>
            </div>
            {FERN_FIELDS.map((f) => {
              const jiraId = mappings[f.field];
              const jira = jiraId ? fieldById.get(jiraId) : undefined;
              const needsStrategy = !!jira && jira.multiValue && !f.multiValue;
              const err = errorByField.get(f.field);
              const missingStrategy = needsStrategy && !strategies[f.field];
              return (
                // eslint-disable-next-line jsx-a11y/no-static-element-interactions -- drop target for pointer drag (v1 parity)
                <div
                  key={f.field}
                  ref={(el) => { if (el) fernRefs.current.set(f.field, el); else fernRefs.current.delete(f.field); }}
                  onMouseUp={() => connect(f.field)}
                  className={`rounded-md border bg-surface px-3 py-2 shadow-sm transition-colors ${
                    drag ? 'border-primary ring-1 ring-primary/40' : jiraId ? 'border-primary/40' : 'border-border'
                  }`}
                >
                  <div className="flex items-start gap-2">
                    {/* left-edge port */}
                    <span className={`mt-1 h-2.5 w-2.5 shrink-0 rounded-full border-2 border-primary ${jiraId ? 'bg-primary' : 'bg-surface'}`} />
                    <div className="min-w-0 flex-1">
                      <div className="flex items-center gap-1.5">
                        <span className="text-sm font-medium">{f.label}</span>
                        {f.required && <span className="text-red-600">*</span>}
                        {f.multiValue && (
                          <span className="rounded bg-amber-100 px-1 text-[9px] font-medium uppercase text-amber-800 dark:bg-amber-900/40 dark:text-amber-200">multi-value</span>
                        )}
                      </div>
                      <div className="text-[10px] text-muted">{f.description}</div>
                      {jiraId ? (
                        <div className="mt-1 flex items-center gap-1.5">
                          <Link2 className="h-3 w-3 shrink-0 text-primary" />
                          <span className="max-w-[12rem] truncate text-[11px] text-primary">{jira?.name ?? jiraId}</span>
                          {jira?.multiValue && (
                            <span className="rounded bg-amber-100 px-1 text-[9px] font-medium uppercase text-amber-800 dark:bg-amber-900/40 dark:text-amber-200">multi-value</span>
                          )}
                          {canManage && (
                            <button aria-label={`Unmap ${f.label}`} onClick={() => disconnect(f.field)}
                              className="ml-auto text-muted hover:text-status-failed-fg">
                              <X className="h-3 w-3" />
                            </button>
                          )}
                        </div>
                      ) : (
                        <div className="mt-1 text-[10px] text-muted">Not mapped</div>
                      )}
                    </div>
                  </div>
                  {needsStrategy && (
                    <label className="mt-1.5 block text-[10px] text-muted">
                      Reduction strategy <span className="text-red-600">*</span>
                      <select
                        aria-label={`${f.label} reduction strategy`}
                        disabled={!canManage}
                        value={strategies[f.field] ?? ''}
                        onChange={(e) => setStrategies((s) => ({ ...s, [f.field]: e.target.value as ReductionStrategy }))}
                        className={`mt-0.5 w-full rounded border bg-surface px-2 py-1 text-xs text-foreground focus:outline-none disabled:opacity-60 ${
                          missingStrategy ? 'border-red-400' : 'border-border focus:border-primary'
                        }`}
                      >
                        <option value="">Select strategy…</option>
                        {REDUCTION_STRATEGIES.map((st) => (
                          <option key={st} value={st}>{STRATEGY_LABELS[st]}</option>
                        ))}
                      </select>
                    </label>
                  )}
                  {err && <p className="mt-1 text-[11px] text-status-failed-fg">{err}</p>}
                </div>
              );
            })}
          </div>
        </div>

        {canManage && (
          <div className="flex items-center gap-2 border-t border-border pt-3">
            {(() => {
              const req = FERN_FIELDS.filter((f) => f.required);
              const mapped = req.filter((f) => mappings[f.field]).length;
              return (
                <span className={`text-xs ${mapped === req.length ? 'text-status-passed-fg' : 'text-muted'}`}>
                  {mapped} of {req.length} required fields mapped
                </span>
              );
            })()}
            <span className="ml-auto" />
            {save.error && <span className="text-xs text-red-600 dark:text-red-400">{(save.error as Error).message}</span>}
            {save.isSuccess && <span className="text-xs text-status-passed-fg">✓ Saved</span>}
            <Button variant="ghost" disabled={reset.isPending} onClick={() => reset.mutate()}>
              {reset.isPending ? <Spinner /> : <><RotateCcw className="h-3.5 w-3.5" /> Reset</>}
            </Button>
            <Button
              disabled={errors.length > 0 || save.isPending}
              onClick={() => save.mutate(entries.map((e) => ({
                fernField: e.fernField,
                jiraFieldId: e.jiraFieldId,
                jiraFieldIsMultiValue: e.jiraFieldIsMultiValue,
                reductionStrategy: e.reductionStrategy ?? null,
              })))}
            >
              {save.isPending ? <Spinner className="text-white" /> : <><Save className="h-3.5 w-3.5" /> Save mapping</>}
            </Button>
          </div>
        )}
      </CardBody>
    </Card>
  );
}
