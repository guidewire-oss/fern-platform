import { describe, it, expect } from 'vitest';
import { cn } from './cn';

describe('cn', () => {
  it('joins truthy parts with spaces', () => {
    expect(cn('a', 'b', 'c')).toBe('a b c');
  });
  it('filters out falsy values', () => {
    expect(cn('a', false, null, undefined, '', 'b')).toBe('a b');
  });
  it('handles conditional classes', () => {
    const active = true;
    const disabled = false;
    expect(cn('btn', active && 'btn--active', disabled && 'btn--disabled'))
      .toBe('btn btn--active');
  });
  it('returns empty string when nothing is truthy', () => {
    expect(cn(false, null)).toBe('');
  });
});
