import { describe, it, expect } from 'vitest';
import { formatDuration } from './duration';

describe('formatDuration', () => {
  it('returns "0ms" for invalid input', () => {
    expect(formatDuration(undefined)).toBe('0ms');
    expect(formatDuration(null)).toBe('0ms');
    expect(formatDuration(-1)).toBe('0ms');
    expect(formatDuration(NaN)).toBe('0ms');
  });

  it('formats sub-second as ms', () => {
    expect(formatDuration(0)).toBe('0ms');
    expect(formatDuration(42)).toBe('42ms');
    expect(formatDuration(999)).toBe('999ms');
  });

  it('formats sub-minute as decimal seconds', () => {
    expect(formatDuration(1000)).toBe('1.0s');
    expect(formatDuration(59500)).toBe('59.5s');
  });

  it('formats minutes', () => {
    expect(formatDuration(60_000)).toBe('1m 0s');
    expect(formatDuration(90_000)).toBe('1m 30s');
  });

  it('formats hours', () => {
    expect(formatDuration(3_600_000)).toBe('1h 0m');
    expect(formatDuration(3_661_000)).toBe('1h 1m 1s');
  });
});
