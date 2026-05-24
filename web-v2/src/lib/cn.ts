// cn — tiny classnames joiner. Filters out falsy entries so callers
// can write `cn('foo', cond && 'bar', other)` without ternary noise.
// We do not depend on clsx/twind to keep the bundle lean.
export function cn(...parts: Array<string | false | null | undefined>): string {
  return parts.filter(Boolean).join(' ');
}
