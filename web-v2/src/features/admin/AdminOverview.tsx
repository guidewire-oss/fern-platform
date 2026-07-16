import { useQuery } from '@tanstack/react-query';
import { Activity, Database, Server, ShieldCheck, AlertCircle } from 'lucide-react';
import { restFetch } from '@/lib/api';
import { Card, CardBody, CardHeader, CardTitle } from '@/components/ui/Card';
import { Spinner } from '@/components/ui/Spinner';
import { cn } from '@/lib/cn';

interface HealthResp {
  status: string;
  db?: string;
  error?: string;
}

function useHealth(path: string) {
  return useQuery({
    queryKey: ['admin-health', path],
    queryFn: () => restFetch<HealthResp>(path),
    staleTime: 10_000,
    refetchInterval: 30_000,
  });
}

export default function AdminOverview() {
  const liveness  = useHealth('/healthz');
  const readiness = useHealth('/readyz');

  // /metrics returns Prometheus text — fetch as text and surface the count
  const metrics = useQuery({
    queryKey: ['admin-metrics'],
    queryFn: () =>
      fetch('/metrics').then(async (r) => {
        const text = await r.text();
        const lines = text.split('\n');
        return {
          totalLines: lines.length,
          totalRequests:
            lines
              .filter((l) => l.startsWith('fern_http_requests_total'))
              .reduce((sum, l) => {
                const m = l.match(/\}\s+(\d+)$/);
                return sum + (m ? parseInt(m[1] ?? '0', 10) : 0);
              }, 0),
        };
      }),
    staleTime: 30_000,
    refetchInterval: 60_000,
  });

  return (
    <div className="space-y-6">
      <header>
        <h1 className="text-3xl font-semibold tracking-tight">Admin</h1>
        <p className="mt-1 text-sm text-muted">
          Live operational status of this Fern instance.
        </p>
      </header>

      <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
        <HealthCard
          label="Liveness"
          icon={Activity}
          ok={liveness.data?.status === 'ok'}
          loading={liveness.isLoading}
          detail="/healthz"
        />
        <HealthCard
          label="Readiness"
          icon={Database}
          ok={readiness.data?.status === 'ok'}
          loading={readiness.isLoading}
          detail={readiness.data?.error ?? '/readyz (DB ping)'}
        />
        <Card className="p-5">
          <div className="mb-3 inline-flex h-10 w-10 items-center justify-center rounded-lg bg-gradient-to-br from-indigo-500 to-purple-600 text-white shadow-md">
            <Server className="h-5 w-5" />
          </div>
          <div className="text-[11px] font-medium uppercase tracking-wider text-muted">
            HTTP requests
          </div>
          <div className="mt-0.5 text-3xl font-semibold tabular-nums">
            {metrics.isLoading ? <Spinner /> : metrics.data?.totalRequests.toLocaleString() ?? 0}
          </div>
          <div className="mt-2 text-xs text-muted">Since process start</div>
        </Card>
        <Card className="p-5">
          <div className="mb-3 inline-flex h-10 w-10 items-center justify-center rounded-lg bg-gradient-success text-white shadow-md">
            <ShieldCheck className="h-5 w-5" />
          </div>
          <div className="text-[11px] font-medium uppercase tracking-wider text-muted">
            CSP
          </div>
          <div className="mt-0.5 text-3xl font-semibold">strict</div>
          <div className="mt-2 text-xs text-muted">HTML responses only</div>
        </Card>
      </div>

      <div className="grid gap-4 lg:grid-cols-2">
        <Card>
          <CardHeader>
            <CardTitle>Endpoint surface</CardTitle>
          </CardHeader>
          <CardBody className="space-y-2 text-sm">
            <EndpointRow path="/healthz"            label="Liveness probe" />
            <EndpointRow path="/readyz"             label="Readiness probe (DB ping)" />
            <EndpointRow path="/metrics"            label="Prometheus exposition" />
            <EndpointRow path="/api/v2/test-runs"   label="Filtered test runs" />
            <EndpointRow path="/api/v2/me/saved-views" label="User-scoped saved views" />
            <EndpointRow path="/api/v2/telemetry/vitals" label="Core Web Vitals ingest" />
            <EndpointRow path="/query"              label="GraphQL" />
          </CardBody>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>Build info</CardTitle>
          </CardHeader>
          <CardBody className="grid gap-3 sm:grid-cols-2 text-sm">
            <InfoRow k="Mode"              v="local dev" />
            <InfoRow k="Auth"              v="disabled" />
            <InfoRow k="v2 API"            v="enabled" />
            <InfoRow k="v2 SPA"            v="enabled" />
            <InfoRow k="v1 deprecation"    v="off (set FERN_V1_DEPRECATED=true)" />
            <InfoRow k="Migration suffix"  v="DB_HOST_SUFFIX=''" />
          </CardBody>
        </Card>
      </div>
    </div>
  );
}

function HealthCard({
  label,
  icon: Icon,
  ok,
  loading,
  detail,
}: {
  label: string;
  icon: typeof Activity;
  ok: boolean;
  loading: boolean;
  detail: string;
}) {
  return (
    <Card className="p-5">
      <div
        className={cn(
          'mb-3 inline-flex h-10 w-10 items-center justify-center rounded-lg text-white shadow-md',
          loading ? 'bg-slate-400' : ok ? 'bg-gradient-success' : 'bg-gradient-danger',
        )}
      >
        {loading ? <Spinner className="text-white" /> : ok ? <Icon className="h-5 w-5" /> : <AlertCircle className="h-5 w-5" />}
      </div>
      <div className="text-[11px] font-medium uppercase tracking-wider text-muted">{label}</div>
      <div className={cn('mt-0.5 text-2xl font-semibold tracking-tight', ok ? 'text-status-passed-fg' : 'text-status-failed-fg')}>
        {loading ? '…' : ok ? 'Healthy' : 'Unhealthy'}
      </div>
      <div className="mt-1 text-xs text-muted">{detail}</div>
    </Card>
  );
}

function EndpointRow({ path, label }: { path: string; label: string }) {
  return (
    <div className="flex items-center justify-between border-b border-border py-1 last:border-0">
      <span className="text-muted">{label}</span>
      <code className="font-mono text-xs text-foreground">{path}</code>
    </div>
  );
}

function InfoRow({ k, v }: { k: string; v: string }) {
  return (
    <div>
      <div className="text-[11px] uppercase tracking-wider text-muted">{k}</div>
      <div className="mt-0.5 font-mono text-xs text-foreground">{v}</div>
    </div>
  );
}
