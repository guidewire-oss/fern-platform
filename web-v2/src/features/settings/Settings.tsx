import { useEffect, useState } from 'react';
import { Link } from '@tanstack/react-router';
import {
  SettingsIcon,
  Database,
  Bell,
  Palette,
  Globe,
  Plug,
  ShieldCheck,
  ExternalLink,
  ArrowRight,
} from 'lucide-react';
import { cn } from '@/lib/cn';
import { Card, CardBody, CardHeader, CardTitle } from '@/components/ui/Card';
import { applyTheme, loadThemeFromStorage, saveThemeToStorage, type Theme } from '@/lib/theme';

interface Section {
  id: string;
  label: string;
  icon: typeof SettingsIcon;
}

const SECTIONS: Section[] = [
  { id: 'general',      label: 'General',      icon: SettingsIcon },
  { id: 'appearance',   label: 'Appearance',   icon: Palette },
  { id: 'notifications',label: 'Notifications',icon: Bell },
  { id: 'integrations', label: 'Integrations', icon: Plug },
  { id: 'database',     label: 'Database',     icon: Database },
  { id: 'security',     label: 'Security',     icon: ShieldCheck },
  { id: 'i18n',         label: 'Language',     icon: Globe },
];

export default function Settings() {
  const [active, setActive] = useState('general');
  return (
    <div className="space-y-6">
      <header>
        <h1 className="text-3xl font-semibold tracking-tight">System settings</h1>
        <p className="mt-1 text-sm text-muted">
          Platform-wide configuration. Most knobs live in env vars; this surface
          surfaces them for admins.
        </p>
      </header>

      <div className="grid gap-6 lg:grid-cols-[200px_1fr]">
        <aside className="space-y-0.5">
          {SECTIONS.map(({ id, label, icon: Icon }) => (
            <button
              key={id}
              onClick={() => setActive(id)}
              className={cn(
                'flex w-full items-center gap-2 rounded-md px-3 py-2 text-sm transition-colors',
                active === id
                  ? 'bg-primary/10 font-medium text-primary'
                  : 'text-foreground hover:bg-surface-2',
              )}
            >
              <Icon className="h-4 w-4" />
              {label}
            </button>
          ))}
        </aside>

        <div>
          {active === 'general' && <GeneralPanel />}
          {active === 'appearance' && <AppearancePanel />}
          {active === 'notifications' && <NotificationsPanel />}
          {active === 'integrations' && <IntegrationsPanel />}
          {active === 'database' && <DatabasePanel />}
          {active === 'security' && <SecurityPanel />}
          {active === 'i18n' && <I18nPanel />}
        </div>
      </div>
    </div>
  );
}

function Field({
  label,
  hint,
  children,
}: {
  label: string;
  hint?: string;
  children: React.ReactNode;
}) {
  return (
    <div className="space-y-1">
      <label className="text-xs font-medium text-foreground">{label}</label>
      {children}
      {hint && <p className="text-[11px] text-muted">{hint}</p>}
    </div>
  );
}

function ReadonlyValue({ value }: { value: string }) {
  return (
    <div className="rounded border border-border bg-surface-2 px-2.5 py-1.5 font-mono text-xs text-foreground">
      {value}
    </div>
  );
}

function GeneralPanel() {
  return (
    <Card>
      <CardHeader>
        <CardTitle>General</CardTitle>
      </CardHeader>
      <CardBody className="grid gap-4 sm:grid-cols-2">
        <Field label="Platform name" hint="Displayed in the topbar.">
          <ReadonlyValue value="Fern Platform" />
        </Field>
        <Field label="Environment" hint="Read from FERN_ENV.">
          <ReadonlyValue value="local-dev" />
        </Field>
        <Field label="Server port" hint="From PORT env var.">
          <ReadonlyValue value="8080" />
        </Field>
        <Field label="Default branch" hint="Used when projects don't override.">
          <ReadonlyValue value="main" />
        </Field>
      </CardBody>
    </Card>
  );
}

function AppearancePanel() {
  const [theme, setThemeState] = useState<Theme>(() => loadThemeFromStorage());

  // Keep local state in sync if the user changes theme from elsewhere
  // (e.g. the Profile page) while this panel is mounted.
  useEffect(() => {
    const sync = () => setThemeState(loadThemeFromStorage());
    window.addEventListener('storage', sync);
    return () => window.removeEventListener('storage', sync);
  }, []);

  const choose = (t: Theme) => {
    setThemeState(t);
    applyTheme(t);
    saveThemeToStorage(t);
  };

  return (
    <Card>
      <CardHeader>
        <CardTitle>Appearance</CardTitle>
      </CardHeader>
      <CardBody className="space-y-4">
        <Field
          label="Color scheme"
          hint="Persisted in localStorage. Visit /profile to also save it to your server preferences."
        >
          <div className="flex gap-2">
            {(['light', 'dark', 'system'] as const).map((mode) => (
              <button
                key={mode}
                onClick={() => choose(mode)}
                className={cn(
                  'flex-1 rounded border px-3 py-1.5 text-sm capitalize transition-colors',
                  theme === mode
                    ? 'border-primary bg-primary/10 font-medium text-primary'
                    : 'border-border bg-surface text-foreground hover:bg-surface-2',
                )}
              >
                {mode}
              </button>
            ))}
          </div>
        </Field>
        <Field label="Density" hint="Coming soon. Density tokens land in a future release.">
          <div className="flex gap-2 opacity-60">
            {['Compact', 'Comfortable', 'Spacious'].map((d) => (
              <button
                key={d}
                disabled
                className="cursor-not-allowed rounded border border-border bg-surface px-3 py-1.5 text-sm text-muted"
              >
                {d}
              </button>
            ))}
          </div>
        </Field>
      </CardBody>
    </Card>
  );
}

const NOTIF_KEYS = ['failed-runs', 'flaky-detected', 'slow-run'] as const;
type NotifKey = (typeof NOTIF_KEYS)[number];
const NOTIF_LABELS: Record<NotifKey, [string, string]> = {
  'failed-runs':    ['Failed test runs',    'Notify when a run finishes with failed > 0.'],
  'flaky-detected': ['Flaky test detected', 'Notify when a spec turns flaky.'],
  'slow-run':       ['Slow run',            'Notify when duration exceeds project baseline.'],
};
const NOTIF_STORAGE = 'fern.v2.notifications';

function loadNotifs(): Record<NotifKey, boolean> {
  try {
    const v = localStorage.getItem(NOTIF_STORAGE);
    if (v) return { ...defaultNotifs(), ...JSON.parse(v) };
  } catch {
    // ignore
  }
  return defaultNotifs();
}
function defaultNotifs(): Record<NotifKey, boolean> {
  return { 'failed-runs': true, 'flaky-detected': true, 'slow-run': false };
}

function NotificationsPanel() {
  const [enabled, setEnabled] = useState<Record<NotifKey, boolean>>(() => loadNotifs());

  const toggle = (k: NotifKey) => {
    const next = { ...enabled, [k]: !enabled[k] };
    setEnabled(next);
    try {
      localStorage.setItem(NOTIF_STORAGE, JSON.stringify(next));
    } catch {
      // ignore
    }
  };

  return (
    <Card>
      <CardHeader>
        <CardTitle>Notifications</CardTitle>
      </CardHeader>
      <CardBody className="space-y-3">
        <p className="text-xs text-muted">
          Toggles persist to localStorage. Hooking them up to a real
          notification backend (Slack/email) lands with the integrations panel.
        </p>
        {NOTIF_KEYS.map((k) => {
          const [label, hint] = NOTIF_LABELS[k];
          return (
            <label
              key={k}
              aria-label={label}
              className="flex cursor-pointer items-center justify-between rounded-md border border-border bg-surface px-3 py-2 hover:bg-surface-2"
            >
              <div>
                <div className="text-sm font-medium">{label}</div>
                <div className="text-[11px] text-muted">{hint}</div>
              </div>
              <input
                type="checkbox"
                checked={enabled[k]}
                onChange={() => toggle(k)}
                className="h-4 w-4 accent-primary"
              />
            </label>
          );
        })}
      </CardBody>
    </Card>
  );
}

interface IntegrationItem {
  name: string;
  desc: string;
  // Where 'Configure' takes the user. JIRA is the only one wired today;
  // others are honestly marked 'coming soon' so we don't pretend they
  // work. Past-self lying about it surfaced as your "doesn't work" report.
  configureTo?: string;
  supported: boolean;
}

const INTEGRATIONS: IntegrationItem[] = [
  {
    name: 'JIRA',
    desc: 'Link failed tests to JIRA issues. Configured per-project.',
    configureTo: '/projects',
    supported: true,
  },
  {
    name: 'Slack',
    desc: 'Channel notifications on triage. Backend handler not yet implemented.',
    supported: false,
  },
  {
    name: 'Datadog',
    desc: 'Send latency / error metrics. Backend handler not yet implemented.',
    supported: false,
  },
  {
    name: 'GitHub',
    desc: 'Pull PR + commit metadata. Backend handler not yet implemented.',
    supported: false,
  },
];

function IntegrationsPanel() {
  return (
    <Card>
      <CardHeader className="flex items-center justify-between">
        <CardTitle>Integrations</CardTitle>
        <Link
          to="/projects"
          className="inline-flex items-center gap-1 rounded-md border border-slate-300 bg-slate-100 px-2.5 py-1 text-xs font-medium text-slate-900 shadow-sm hover:bg-slate-200 dark:border-slate-600 dark:bg-slate-800 dark:text-slate-100 dark:hover:bg-slate-700"
          title="Pick a project to configure JIRA on"
        >
          + Add integration <ArrowRight className="h-3 w-3" />
        </Link>
      </CardHeader>
      <CardBody>
        <p className="mb-3 text-xs text-muted">
          Most integrations are configured on a per-project basis. Click <strong>Configure</strong>
          to pick a project for the one that's wired (JIRA today). Other entries are listed for
          visibility — their backends ship in later phases.
        </p>
        <div className="grid gap-3 sm:grid-cols-2">
          {INTEGRATIONS.map((it) => (
            <div key={it.name} className="rounded-md border border-border bg-surface p-3">
              <div className="flex items-center justify-between">
                <div className="text-sm font-semibold">{it.name}</div>
                <span
                  className={cn(
                    'rounded-full px-2 py-0.5 text-[10px] font-medium',
                    it.supported
                      ? 'bg-status-passed-bg text-status-passed-fg'
                      : 'bg-status-skipped-bg text-status-skipped-fg',
                  )}
                >
                  {it.supported ? 'available' : 'coming soon'}
                </span>
              </div>
              <p className="mt-1 text-[11px] text-muted">{it.desc}</p>
              <div className="mt-3">
                {it.supported && it.configureTo ? (
                  <Link
                    to={it.configureTo}
                    className="inline-flex items-center gap-1 rounded border border-slate-300 bg-slate-100 px-2 py-1 text-xs font-medium text-slate-900 hover:bg-slate-200 dark:border-slate-600 dark:bg-slate-800 dark:text-slate-100 dark:hover:bg-slate-700"
                  >
                    Configure <ExternalLink className="h-3 w-3" />
                  </Link>
                ) : (
                  <button
                    disabled
                    title="Backend handler not yet implemented"
                    className="cursor-not-allowed rounded border border-border bg-surface-2 px-2 py-1 text-xs text-muted"
                  >
                    Configure
                  </button>
                )}
              </div>
            </div>
          ))}
        </div>
      </CardBody>
    </Card>
  );
}

function DatabasePanel() {
  return (
    <Card>
      <CardHeader>
        <CardTitle>Database</CardTitle>
      </CardHeader>
      <CardBody className="grid gap-4 sm:grid-cols-2">
        <Field label="Engine">
          <ReadonlyValue value="PostgreSQL 14" />
        </Field>
        <Field label="Host">
          <ReadonlyValue value="postgres:5432" />
        </Field>
        <Field label="Pool max">
          <ReadonlyValue value="25" />
        </Field>
        <Field label="Idle timeout">
          <ReadonlyValue value="300s" />
        </Field>
      </CardBody>
    </Card>
  );
}

function SecurityPanel() {
  return (
    <Card>
      <CardHeader>
        <CardTitle>Security</CardTitle>
      </CardHeader>
      <CardBody className="space-y-3">
        <Field label="Authentication" hint="OAuth provider configuration.">
          <ReadonlyValue value="disabled (local dev)" />
        </Field>
        <Field label="Session timeout">
          <ReadonlyValue value="24h" />
        </Field>
        <Field label="CSP">
          <ReadonlyValue value="strict (HTML-scoped)" />
        </Field>
      </CardBody>
    </Card>
  );
}

function I18nPanel() {
  return (
    <Card>
      <CardHeader>
        <CardTitle>Language</CardTitle>
      </CardHeader>
      <CardBody>
        <Field label="Default locale">
          <ReadonlyValue value="en-US" />
        </Field>
      </CardBody>
    </Card>
  );
}
