import { formatDuration } from '@/lib/duration';
import type { TestRunNode } from '@/lib/types';

// runDuration renders a run's wall-clock time.
//
// The server's recorded duration_ms wins: it is authoritative, and it is
// the only source for a run with no end_time (still running, or ended
// abnormally) — those used to render as an em dash. The end - start
// fallback keeps the column populated against a backend that predates
// duration_ms on this endpoint.
//
// A zero duration_ms is treated as "not recorded", not as a zero-length
// run: the field has no omitempty, so an unset duration arrives as 0,
// and rendering "0ms" for a run that is still going would be worse than
// falling back.
export function runDuration(node: TestRunNode): string {
  if (node.duration_ms) return formatDuration(node.duration_ms);
  if (node.end_time) {
    return formatDuration(
      new Date(node.end_time).getTime() - new Date(node.start_time).getTime(),
    );
  }
  return '—';
}
