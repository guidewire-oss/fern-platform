// Port of legacy web/js/duration-utils.js. Kept verbatim in behavior so
// any UI surfacing durations during the migration window matches the
// legacy formatting users are used to.

/** Format a millisecond count as a human-readable duration. */
export function formatDuration(ms: number | null | undefined): string {
  if (ms === null || ms === undefined || !Number.isFinite(ms) || ms < 0) {
    return '0ms';
  }
  const rounded = Math.round(ms);
  if (rounded < 1000) return `${rounded}ms`;

  const seconds = rounded / 1000;
  if (seconds < 60) return `${seconds.toFixed(1)}s`;

  let totalMinutes = Math.floor(seconds / 60);
  let remainingSeconds = Math.round(seconds % 60);
  if (remainingSeconds === 60) {
    totalMinutes += 1;
    remainingSeconds = 0;
  }
  if (totalMinutes < 60) {
    return remainingSeconds === 0
      ? `${totalMinutes}m 0s`
      : `${totalMinutes}m ${remainingSeconds}s`;
  }
  const hours = Math.floor(totalMinutes / 60);
  const remainingMinutes = totalMinutes % 60;
  let result = `${hours}h${remainingMinutes > 0 ? ` ${remainingMinutes}m` : ' 0m'}`;
  if (hours < 24 && remainingSeconds > 0) result += ` ${remainingSeconds}s`;
  return result;
}
