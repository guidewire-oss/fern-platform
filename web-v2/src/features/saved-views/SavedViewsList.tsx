import { Link } from '@tanstack/react-router';
import { Trash2 } from 'lucide-react';
import { Card } from '@/components/ui/Card';
import { Spinner } from '@/components/ui/Spinner';
import { Button } from '@/components/ui/Button';
import { EmptyState } from '@/components/ui/EmptyState';
import { Table, TBody, TD, TH, THead, TR } from '@/components/ui/Table';
import { useDeleteSavedView, useSavedViews } from './integration';

// The management page lists saved views across ALL pages and deletes them.
// It shares the single saved-views query (useSavedViews) with the Test runs
// Views bar, so a create/delete on either surface updates both. Creation
// lives on the pages that own a live filter (e.g. Test runs) — this page
// only lists and deletes.
export default function SavedViewsList() {
  const { data, isLoading, error } = useSavedViews();

  // Shared idempotent delete: a 404 (already deleted elsewhere / stale row)
  // resolves as success so the row is always cleared from the list.
  const remove = useDeleteSavedView();

  return (
    <div className="space-y-4">
      <header className="flex items-baseline justify-between">
        <div>
          <h1 className="text-2xl font-semibold">Saved views</h1>
          <p className="text-sm text-muted">
            Named filter presets stored on the server; survive logout. Create
            one from the{' '}
            <Link to="/test-runs" className="text-primary hover:underline">
              Test runs
            </Link>{' '}
            page — set the filters you want, then click <em>Save view</em>.
          </p>
        </div>
        <span className="text-sm text-muted">
          {data?.totalCount ?? 0} total
        </span>
      </header>

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
          description="Configure a filter on the Test runs page and click “Save view” to create one."
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
                    <pre className="max-w-md overflow-x-auto rounded bg-surface-2 p-1 font-mono text-[11px] text-foreground">
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
