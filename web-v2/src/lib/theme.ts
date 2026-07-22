// Theme handling — keep the theme value in a single place so all
// component-level switches just call setTheme(...) and the page
// re-paints. The actual visual flip is driven by the .dark class on
// <html>; CSS variables in globals.css respond to that.

export type Theme = 'light' | 'dark' | 'system';

const STORAGE_KEY = 'fern.v2.theme';

let mediaQuery: MediaQueryList | null = null;
let mediaListener: ((e: MediaQueryListEvent) => void) | null = null;

/**
 * Apply a theme to <html>. Returns the effective theme that was applied
 * (resolves 'system' to whichever the OS reports).
 */
export function applyTheme(theme: Theme): 'light' | 'dark' {
  const root = document.documentElement;

  // Tear down any prior system listener — switching from 'system' to
  // an explicit theme should not keep responding to OS changes.
  if (mediaQuery && mediaListener) {
    mediaQuery.removeEventListener('change', mediaListener);
    mediaListener = null;
  }

  let effective: 'light' | 'dark';
  if (theme === 'system') {
    mediaQuery = window.matchMedia('(prefers-color-scheme: dark)');
    effective = mediaQuery.matches ? 'dark' : 'light';
    mediaListener = (e) => {
      document.documentElement.classList.toggle('dark', e.matches);
    };
    mediaQuery.addEventListener('change', mediaListener);
  } else {
    effective = theme;
  }

  root.classList.toggle('dark', effective === 'dark');
  return effective;
}

export function loadThemeFromStorage(): Theme {
  try {
    const v = localStorage.getItem(STORAGE_KEY);
    if (v === 'light' || v === 'dark' || v === 'system') return v;
  } catch {
    // localStorage can throw in private-browsing / sandboxed contexts.
  }
  return 'system';
}

export function saveThemeToStorage(theme: Theme) {
  try {
    localStorage.setItem(STORAGE_KEY, theme);
  } catch {
    // ignore
  }
}
