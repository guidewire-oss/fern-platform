import { useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Trash2 } from 'lucide-react';
import { Card } from '@/components/ui/Card';
import { Spinner } from '@/components/ui/Spinner';
import { Button } from '@/components/ui/Button';
import { EmptyState } from '@/components/ui/EmptyState';
import { Table, TBody, TD, TH, THead, TR } from '@/components/ui/Table';
import { restFetch } from '@/lib/api';
import type { SavedViewList } from '@/lib/types';

const QK = ['saved-views'];

export default function SavedViewsList() {
  const qc = useQueryClient();
  const { data, isLoading, error } = useQuery({
    queryKey: QK,
    queryFn: () => restFetch<SavedViewList>('/api/v2/me/saved-views'),
  });

  const create = useMutation({
    mutationFn: (body: { page: string; name: string; filter: object }) =>
      restFetch('/api/v2/me/saved-views', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(body),
      }),
    onSuccess: () => qc.invalidateQueries({ queryKey: QK }),
  });

  const remove = useMutation({
    mutationFn: (id: number) =>
      restFetch(`/api/v2/me/saved-views/${id}`, { method: 'DELETE' }),
    onSuccess: () => qc.invalidateQueries({ queryKey: QK }),
  });

  const [name, setName] = useState('');

  return (
    <div className="space-y-4">
      <header className="flex items-baseline justify-between">
        <div>
          <h1 className="text-2xl font-semibold">Saved views</h1>
          <p className="text-sm text-muted">
            Named filter presets stored on the server; survive logout.
          </p>
        </div>
        <span className="text-sm text-muted">
          {data?.totalCount ?? 0} total
        </span>
      </header>

      <Card className="p-4">
        <div className="flex flex-wrap items-end gap-2">
          <label className="flex-1 text-xs font-medium text-muted">
            Save current filter as…
            <input
              type="text"
              className="mt-1 w-full rounded border border-border bg-surface text-foreground px-2 py-1.5 text-sm focus:border-primary focus:outline-none"
              placeholder="e.g. Failures on main"
              value={name}
              onChange={(e) => setName(e.target.value)}
            />
          </label>
          <Button
            disabled={!name.trim() || create.isPending}
            onClick={() => {
              create.mutate({
                page: 'test-runs',
                name: name.trim(),
                // Demo payload — the real flow takes the active filter
                // from the test-runs page state.
                filter: { status: ['failed'], branch: ['main'] },
              });
              setName('');
            }}
          >
            {create.isPending ? <Spinner className="text-white" /> : 'Save view'}
          </Button>
        </div>
        {create.error && (
          <p className="mt-2 text-xs text-red-700">
            {(create.error as Error).message}
          </p>
        )}
      </Card>

      {error && (
        <EmptyState
          title="Couldn't load saved views"
          description={(error as Error).message}
        />
      )}

      {isLoading ? (
        <div className="flex items-center gap-2 text-muted">
          <Spinner /> Loading…
        </div>
      ) : (data?.views.length ?? 0) === 0 ? (
        <EmptyState
          title="No saved views yet"
          description="Use the form above to save one."
        />
      ) : (
        <Card>
          <Table>
            <THead>
              <TR>
                <TH>Name</TH>
                <TH>Page</TH>
                <TH>Filter</TH>
                <TH className="w-12 text-right" aria-label="Actions" />
              </TR>
            </THead>
            <TBody>
              {data?.views.map((v) => (
                <TR key={v.id}>
                  <TD className="font-medium">{v.name}</TD>
                  <TD className="font-mono text-xs">{v.page}</TD>
                  <TD>
                    <pre className="max-w-md overflow-x-auto rounded bg-gray-50 p-1 font-mono text-[11px] text-gray-700">
{JSON.stringify(v.filter)}
                    </pre>
                  </TD>
                  <TD className="text-right">
                    <Button
                      variant="ghost"
                      size="sm"
                      aria-label={`Delete ${v.name}`}
                      onClick={() => remove.mutate(v.id)}
                    >
                      <Trash2 className="h-4 w-4" />
                    </Button>
                  </TD>
                </TR>
              ))}
            </TBody>
          </Table>
        </Card>
      )}
    </div>
  );
}
