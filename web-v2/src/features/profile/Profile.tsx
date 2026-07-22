import { useEffect, useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { CircleUser, Save, Star, X } from 'lucide-react';
import { graphqlFetch } from '@/lib/api';
import { Card, CardBody, CardHeader, CardTitle } from '@/components/ui/Card';
import { Button } from '@/components/ui/Button';
import { Spinner } from '@/components/ui/Spinner';
import { EmptyState } from '@/components/ui/EmptyState';
import { cn } from '@/lib/cn';
import { applyTheme, saveThemeToStorage, type Theme } from '@/lib/theme';
import { useToggleFavorite } from './hooks';

const GET = /* GraphQL */ `
  query Me {
    currentUser {
      id
      userId
      email
      name
      firstName
      lastName
      role
      profileUrl
      groups
      lastLoginAt
    }
    userPreferences {
      theme
      timezone
      language
      favorites
    }
  }
`;

const UPDATE = /* GraphQL */ `
  mutation UpdatePrefs($input: UpdateUserPreferencesInput!) {
    updateUserPreferences(input: $input) {
      theme timezone language favorites
    }
  }
`;

interface MeResp {
  currentUser: {
    id: string;
    userId: string;
    email: string;
    name: string;
    firstName?: string;
    lastName?: string;
    role: string;
    profileUrl?: string;
    groups?: string[];
    lastLoginAt?: string;
  } | null;
  userPreferences: {
    theme?: string;
    timezone?: string;
    language?: string;
    favorites: string[];
  } | null;
}

const THEMES = ['light', 'dark', 'system'] as const;
const LANGUAGES: Array<{ value: string; label: string }> = [
  { value: 'en', label: 'English' },
  { value: 'es', label: 'Español' },
  { value: 'fr', label: 'Français' },
  { value: 'ja', label: '日本語' },
];

// All IANA zones the host knows about. The browser-detected zone goes
// first so users don't have to scroll. UTC is always present even if
// the browser somehow omits it from the supported list (no known case,
// but the cost of the safety check is one comparison).
function timezoneOptions(): string[] {
  const zones = (typeof Intl.supportedValuesOf === 'function'
    ? Intl.supportedValuesOf('timeZone')
    : []) as string[];
  const local = Intl.DateTimeFormat().resolvedOptions().timeZone;
  const set = new Set<string>(zones);
  set.add('UTC');
  if (local) set.add(local);
  const head = [local, 'UTC'].filter((t): t is string => !!t);
  const rest = Array.from(set)
    .filter((t) => !head.includes(t))
    .sort((a, b) => a.localeCompare(b));
  return [...head, ...rest];
}
const TIMEZONES = timezoneOptions();

export default function Profile() {
  const qc = useQueryClient();
  const me = useQuery({
    queryKey: ['me'],
    queryFn: () => graphqlFetch<MeResp>(GET),
    staleTime: 60_000,
  });

  const [theme, setTheme] = useState<Theme>('system');
  const [timezone, setTimezone] = useState('UTC');
  const [language, setLanguage] = useState('en');

  useEffect(() => {
    const p = me.data?.userPreferences;
    if (p) {
      const t = (p.theme as Theme) || 'system';
      setTheme(t);
      setTimezone(p.timezone || 'UTC');
      setLanguage(p.language || 'en');
    }
  }, [me.data?.userPreferences]);

  // Live preview — flip the page colors the moment the user clicks
  // a theme pill, even before they save.
  const pickTheme = (t: Theme) => {
    setTheme(t);
    applyTheme(t);
    saveThemeToStorage(t);
  };

  const toggleFav = useToggleFavorite();

  const save = useMutation({
    mutationFn: () =>
      graphqlFetch(UPDATE, {
        input: { theme, timezone, language },
      }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['me'] });
    },
  });

  if (me.isLoading) {
    return (
      <div className="flex items-center gap-2 text-muted">
        <Spinner /> Loading profile…
      </div>
    );
  }
  if (me.error) {
    return <EmptyState title="Couldn't load profile" description={(me.error as Error).message} />;
  }
  const u = me.data?.currentUser;
  const p = me.data?.userPreferences;
  if (!u) {
    return <EmptyState title="Not signed in" description="With AUTH_ENABLED=true the OAuth flow populates this." />;
  }

  const initials = (u.name || u.email)
    .split(/\s|@/).filter(Boolean).map((s) => s[0]).join('').slice(0, 2).toUpperCase();

  return (
    <div className="space-y-6">
      <header>
        <h1 className="text-3xl font-semibold tracking-tight">Profile</h1>
        <p className="mt-1 text-sm text-muted">
          Identity, role, favorites and per-user UI preferences.
        </p>
      </header>

      <Card>
        <CardBody className="flex items-center gap-4 p-5">
          <div className="flex h-16 w-16 items-center justify-center rounded-full bg-gradient-primary text-xl font-bold text-white shadow-md">
            {u.profileUrl ? (
              <img src={u.profileUrl} alt="" className="h-16 w-16 rounded-full object-cover" />
            ) : (
              initials
            )}
          </div>
          <div className="min-w-0 flex-1">
            <h2 className="truncate text-lg font-semibold">{u.name || u.email}</h2>
            <div className="text-sm text-muted">{u.email}</div>
            <div className="mt-1 flex flex-wrap items-center gap-2 text-xs">
              <span
                className={cn(
                  'rounded-full px-2 py-0.5 font-medium',
                  u.role === 'admin'
                    ? 'bg-gradient-primary text-white'
                    : u.role === 'manager'
                    ? 'bg-indigo-100 text-indigo-800'
                    : 'bg-slate-100 text-slate-800',
                )}
              >
                {u.role}
              </span>
              {u.lastLoginAt && (
                <span className="text-muted">
                  Last login {new Date(u.lastLoginAt).toLocaleString()}
                </span>
              )}
              {u.groups && u.groups.length > 0 && (
                <span className="text-muted">
                  Groups:{' '}
                  <span className="text-foreground">{u.groups.join(', ')}</span>
                </span>
              )}
            </div>
          </div>
          <CircleUser className="hidden h-10 w-10 shrink-0 text-muted sm:block" />
        </CardBody>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>Preferences</CardTitle>
        </CardHeader>
        <CardBody className="space-y-4">
          <Field label="Theme" hint="Light or dark surface. 'System' follows the OS setting.">
            <div className="flex gap-2">
              {THEMES.map((t) => (
                <button
                  key={t}
                  onClick={() => pickTheme(t)}
                  className={cn(
                    'flex-1 rounded border px-3 py-1.5 text-sm capitalize transition-colors',
                    theme === t
                      ? 'border-primary bg-primary/10 font-medium text-primary'
                      : 'border-border bg-surface text-foreground hover:bg-surface-2',
                  )}
                >
                  {t}
                </button>
              ))}
            </div>
          </Field>

          <div className="grid gap-3 sm:grid-cols-2">
            <Field label="Timezone">
              <select
                value={timezone}
                onChange={(e) => setTimezone(e.target.value)}
                className="w-full rounded border border-border bg-surface px-2.5 py-1.5 text-sm focus:border-primary focus:outline-none"
              >
                {TIMEZONES.map((tz) => (
                  <option key={tz} value={tz}>{tz}</option>
                ))}
              </select>
            </Field>
            <Field label="Language">
              <select
                value={language}
                onChange={(e) => setLanguage(e.target.value)}
                className="w-full rounded border border-border bg-surface px-2.5 py-1.5 text-sm focus:border-primary focus:outline-none"
              >
                {LANGUAGES.map((l) => (
                  <option key={l.value} value={l.value}>{l.label}</option>
                ))}
              </select>
            </Field>
          </div>

          <div className="flex items-center justify-end gap-2 border-t border-border pt-3">
            {save.error && (
              <span className="text-xs text-red-600 dark:text-red-400">{(save.error as Error).message}</span>
            )}
            {save.isSuccess && (
              <span className="text-xs text-status-passed-fg">✓ Saved</span>
            )}
            <Button onClick={() => save.mutate()} disabled={save.isPending}>
              {save.isPending ? <Spinner className="text-white" /> : <><Save className="h-3.5 w-3.5" /> Save preferences</>}
            </Button>
          </div>
        </CardBody>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>Favorite projects</CardTitle>
        </CardHeader>
        <CardBody>
          {(p?.favorites ?? []).length === 0 ? (
            <p className="text-sm text-muted">
              You haven't starred any projects yet. Click the{' '}
              <Star className="inline h-3 w-3" /> icon on a project card to add one.
            </p>
          ) : (
            <ul className="flex flex-wrap gap-2">
              {p!.favorites.map((id) => (
                <li
                  key={id}
                  className="inline-flex items-center gap-1 rounded-full bg-status-flaky-bg pl-2.5 pr-1 py-0.5 text-xs text-status-flaky-fg"
                >
                  <Star className="h-3 w-3 fill-current" />
                  <span className="font-mono">{id}</span>
                  <button
                    onClick={() => toggleFav.mutate(id)}
                    aria-label={`Remove ${id} from favorites`}
                    className="ml-1 rounded-full p-0.5 hover:bg-status-flaky-fg/10"
                  >
                    <X className="h-3 w-3" />
                  </button>
                </li>
              ))}
            </ul>
          )}
        </CardBody>
      </Card>
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
    <div>
      <label className="block text-xs font-medium text-foreground">{label}</label>
      <div className="mt-1">{children}</div>
      {hint && <p className="mt-0.5 text-[11px] text-muted">{hint}</p>}
    </div>
  );
}
