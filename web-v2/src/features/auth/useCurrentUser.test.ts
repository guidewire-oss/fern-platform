import { describe, it, expect } from 'vitest';
import { initialsOf } from './useCurrentUser';

describe('initialsOf', () => {
  it('uses firstName + lastName when both present', () => {
    expect(initialsOf({ name: '', firstName: 'Ada', lastName: 'Lovelace', email: '' })).toBe('AL');
  });

  it('falls back to first + last word of `name` when split', () => {
    expect(initialsOf({ name: 'Grace B Hopper', email: '' })).toBe('GH');
  });

  it('returns single initial when name has one word', () => {
    expect(initialsOf({ name: 'Plato', email: '' })).toBe('P');
  });

  it('falls back to email initial when no name parts', () => {
    expect(initialsOf({ name: '', email: 'qwerty@example.com' })).toBe('Q');
  });

  it('returns ? when nothing is available', () => {
    expect(initialsOf({ name: '', email: '' })).toBe('?');
  });
});
