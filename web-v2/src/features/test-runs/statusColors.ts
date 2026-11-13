// Status colors for history marks, matching StatusBadge's dots so a
// status reads the same in the strip, the duration chart and the Status
// column. Two literal maps because Tailwind only generates classes it
// can find spelled out: `bg-*` for HTML cells, `fill-*` for SVG bars.
export const STATUS_BG: Record<string, string> = {
  passed: 'bg-emerald-500',
  failed: 'bg-red-500',
  flaky: 'bg-amber-500',
  pending: 'bg-amber-300',
  skipped: 'bg-gray-400',
  running: 'bg-blue-500',
};

export const STATUS_FILL: Record<string, string> = {
  passed: 'fill-emerald-500',
  failed: 'fill-red-500',
  flaky: 'fill-amber-500',
  pending: 'fill-amber-300',
  skipped: 'fill-gray-400',
  running: 'fill-blue-500',
};
