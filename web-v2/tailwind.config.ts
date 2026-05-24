import type { Config } from 'tailwindcss';

const config: Config = {
  // Class-based dark mode — src/lib/theme.ts toggles 'dark' on <html>.
  darkMode: 'class',
  content: ['./index.html', './src/**/*.{ts,tsx}'],
  theme: {
    extend: {
      colors: {
        background: 'var(--color-background)',
        surface:    'var(--color-surface)',
        'surface-2':'var(--color-surface-2)',
        foreground: 'var(--color-foreground)',
        muted:      'var(--color-muted)',
        border:     'var(--color-border)',
        primary: {
          DEFAULT: 'var(--color-primary)',
          hover:   'var(--color-primary-hover)',
          soft:    'var(--color-primary-soft)',
        },
        status: {
          'passed-bg':  'var(--status-passed-bg)',
          'passed-fg':  'var(--status-passed-fg)',
          'failed-bg':  'var(--status-failed-bg)',
          'failed-fg':  'var(--status-failed-fg)',
          'flaky-bg':   'var(--status-flaky-bg)',
          'flaky-fg':   'var(--status-flaky-fg)',
          'skipped-bg': 'var(--status-skipped-bg)',
          'skipped-fg': 'var(--status-skipped-fg)',
          'running-bg': 'var(--status-running-bg)',
          'running-fg': 'var(--status-running-fg)',
        },
        sidebar: {
          fg:       'var(--sidebar-fg)',
          'fg-active': 'var(--sidebar-fg-active)',
          accent:   'var(--sidebar-accent)',
        },
      },
      fontFamily: {
        sans: ['Inter', 'system-ui', 'sans-serif'],
        mono: ['JetBrains Mono', 'ui-monospace', 'SFMono-Regular', 'monospace'],
      },
      borderRadius: {
        xl: '0.75rem',
        '2xl': '1rem',
      },
      backgroundImage: {
        'gradient-primary': 'linear-gradient(135deg, #0ea5e9 0%, #6366f1 100%)',
        'gradient-success': 'linear-gradient(135deg, #10b981 0%, #059669 100%)',
        'gradient-danger':  'linear-gradient(135deg, #ef4444 0%, #dc2626 100%)',
        'gradient-warning': 'linear-gradient(135deg, #f59e0b 0%, #d97706 100%)',
      },
    },
  },
  plugins: [],
};

export default config;
