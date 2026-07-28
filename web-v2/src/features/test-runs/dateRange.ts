// Maps the custom date-range inputs (native <input type="date"> values,
// `YYYY-MM-DD`) to the RFC3339 bounds the test-runs API expects on
// `from`/`to`. `from` anchors to the start of that UTC day and `to` to the
// end of it, so selecting a single day includes the whole day rather than
// an empty instant. Empty inputs map to `undefined` (that bound is left
// unconstrained). UTC boundaries keep the mapping deterministic regardless
// of the viewer's timezone.
export function customRangeToQuery(
  fromDate: string,
  toDate: string,
): { from?: string; to?: string } {
  const out: { from?: string; to?: string } = {};
  if (fromDate) out.from = `${fromDate}T00:00:00.000Z`;
  if (toDate) out.to = `${toDate}T23:59:59.999Z`;
  return out;
}

// Inverse of customRangeToQuery: recover the `YYYY-MM-DD` input values from
// the filter's ISO bounds so the date inputs stay populated across
// re-renders and deep-links. Empty when the bound is unset.
export function queryToRangeInputs(
  from: string | undefined,
  to: string | undefined,
): { fromDate: string; toDate: string } {
  return {
    fromDate: from ? from.slice(0, 10) : '',
    toDate: to ? to.slice(0, 10) : '',
  };
}

// Preset windows offered wherever a test-run list is scoped by time.
// Shared so the test-runs sidebar and the project page offer the same
// choices and label them identically.
export const DATE_PRESETS: Array<{ label: string; days: number }> = [
  { label: '24h', days: 1 },
  { label: '7d', days: 7 },
  { label: '30d', days: 30 },
  { label: '90d', days: 90 },
  { label: '180d', days: 180 },
];

// presetRange turns a day count into the RFC3339 bounds the API expects,
// anchored at the moment it is called.
export function presetRange(days: number, now: Date = new Date()): { from: string; to: string } {
  return {
    from: new Date(now.getTime() - days * 24 * 60 * 60 * 1000).toISOString(),
    to: now.toISOString(),
  };
}

// matchPresetDays returns the preset a from/to span corresponds to, or
// null when the range is custom (or unset). Tolerates a minute of drift
// so a range built a moment ago still lights up its button.
export function matchPresetDays(
  from: string | undefined,
  to: string | undefined,
): number | null {
  if (!from || !to) return null;
  const span = new Date(to).getTime() - new Date(from).getTime();
  for (const p of DATE_PRESETS) {
    if (Math.abs(span - p.days * 24 * 60 * 60 * 1000) < 60_000) return p.days;
  }
  return null;
}
