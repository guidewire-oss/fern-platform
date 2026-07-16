import { describe, it, expect } from 'vitest';
import { Circle } from 'lucide-react';
import { canSeeNavItem } from './AppShell';

// Mirror the AppShell NavItem shape. `icon` is typed against an actual
// Lucide icon component to satisfy the function's signature under
// exactOptionalPropertyTypes — `unknown` doesn't compile through.
// `requires` is conditionally spread so omitting it yields no
// `requires: undefined` (also rejected under exactOptionalPropertyTypes).
type Item = {
  to: string;
  label: string;
  icon: typeof Circle;
  requires?: Array<'admin' | 'manager'>;
};
const item = (over: Partial<Item> = {}): Item => {
  const base: Item = { to: '/x', label: 'X', icon: Circle };
  const { requires, ...rest } = over;
  return requires === undefined ? { ...base, ...rest } : { ...base, ...rest, requires };
};

describe('canSeeNavItem', () => {
  it('shows items without `requires` to anyone (even unknown role)', () => {
    expect(canSeeNavItem(item(), undefined)).toBe(true);
    expect(canSeeNavItem(item(), 'user')).toBe(true);
    expect(canSeeNavItem(item(), 'admin')).toBe(true);
  });

  it('hides gated items when no role is present', () => {
    expect(canSeeNavItem(item({ requires: ['admin'] }), undefined)).toBe(false);
    expect(canSeeNavItem(item({ requires: ['manager'] }), '')).toBe(false);
  });

  it('admin sees admin-only items', () => {
    expect(canSeeNavItem(item({ requires: ['admin'] }), 'admin')).toBe(true);
    expect(canSeeNavItem(item({ requires: ['admin'] }), 'ADMIN')).toBe(true);
  });

  it('manager sees manager-or-admin items', () => {
    expect(canSeeNavItem(item({ requires: ['admin', 'manager'] }), 'manager')).toBe(true);
    expect(canSeeNavItem(item({ requires: ['manager'] }), 'manager')).toBe(true);
  });

  it('plain user is denied manager- and admin-only items', () => {
    expect(canSeeNavItem(item({ requires: ['manager'] }), 'user')).toBe(false);
    expect(canSeeNavItem(item({ requires: ['admin'] }), 'user')).toBe(false);
    expect(canSeeNavItem(item({ requires: ['admin', 'manager'] }), 'user')).toBe(false);
  });
});
