import { useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { Tag as TagIcon, Search } from 'lucide-react';
import { graphqlFetch } from '@/lib/api';
import { Spinner } from '@/components/ui/Spinner';
import { Card } from '@/components/ui/Card';
import { EmptyState } from '@/components/ui/EmptyState';
import { FilterChips } from '@/components/ui/FilterChips';
import { SortableTH, Table, TBody, TD, THead, TR } from '@/components/ui/Table';
import { optionsWithCounts } from '@/lib/tableFilter';
import { sortRows, type SortState, type SortValue } from '@/lib/tableSort';

const TAGS_Q = /* GraphQL */ `
  query AllTags($first: Int) {
    tags(first: $first) {
      totalCount
      edges {
        node {
          id
          name
          category
          value
          description
        }
      }
    }
  }
`;

interface Tag {
  id: string;
  name: string;
  category: string | null;
  value: string | null;
  description: string | null;
}

type TagSortKey = 'name' | 'description';

const TAG_ACCESSORS: Record<TagSortKey, (t: Tag) => SortValue> = {
  name: (t) => t.name,
  description: (t) => t.description,
};

const categoryOf = (t: Tag) => t.category || 'uncategorized';

interface TagsResp {
  tags: { totalCount: number; edges: { node: Tag }[] };
}

export default function TagsManagement() {
  const [q, setQ] = useState('');
  // One sort and one category selection shared by every category table,
  // so the cards stay consistent with each other.
  const [sort, setSort] = useState<SortState<TagSortKey> | null>(null);
  const [categories, setCategories] = useState<string[]>([]);

  const { data, isLoading, error } = useQuery({
    queryKey: ['tags'],
    queryFn: () => graphqlFetch<TagsResp>(TAGS_Q, { first: 200 }),
    staleTime: 60_000,
  });

  const edges = data?.tags.edges ?? [];
  const filtered = q
    ? edges.filter(({ node }) =>
        [node.name, node.category, node.value, node.description]
          .filter(Boolean)
          .some((s) => s!.toLowerCase().includes(q.toLowerCase())),
      )
    : edges;

  const byCategory = new Map<string, Tag[]>();
  for (const { node } of filtered) {
    const key = categoryOf(node);
    if (categories.length > 0 && !categories.includes(key)) continue;
    if (!byCategory.has(key)) byCategory.set(key, []);
    byCategory.get(key)!.push(node);
  }

  return (
    <div className="space-y-6">
      <header className="flex items-baseline justify-between">
        <div>
          <h1 className="text-3xl font-semibold tracking-tight">Tags</h1>
          <p className="mt-1 text-sm text-muted">
            {data?.tags.totalCount ?? 0} tags across {byCategory.size} categories
          </p>
        </div>
        <div className="relative">
          <Search className="absolute left-2 top-1/2 h-3.5 w-3.5 -translate-y-1/2 text-muted" />
          <input
            type="search"
            value={q}
            onChange={(e) => setQ(e.target.value)}
            placeholder="Search tags…"
            className="w-64 rounded-md border border-border bg-surface py-1.5 pl-7 pr-2 text-sm focus:border-primary focus:outline-none"
          />
        </div>
      </header>

      <FilterChips
        label="Category"
        options={optionsWithCounts(edges.map(({ node }) => categoryOf(node)))}
        selected={categories}
        onChange={setCategories}
      />

      {isLoading && (
        <div className="flex items-center gap-2 text-muted">
          <Spinner /> Loading tags…
        </div>
      )}
      {error && (
        <EmptyState
          title="Couldn't load tags"
          description={(error as Error).message}
        />
      )}
      {!isLoading && filtered.length === 0 && (
        <EmptyState title="No tags match your search" />
      )}

      <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-3">
        {Array.from(byCategory.entries()).map(([category, tags]) => (
          <Card key={category}>
            <div className="border-b border-border bg-gradient-to-r from-primary-soft to-transparent px-4 py-2.5">
              <div className="flex items-center justify-between">
                <h3 className="text-sm font-semibold capitalize">{category}</h3>
                <span className="text-xs text-muted">{tags.length}</span>
              </div>
            </div>
            <Table className="table-fixed">
              <THead>
                <TR>
                  <SortableTH sortKey="name" sort={sort} onSortChange={setSort}>Name</SortableTH>
                  <SortableTH sortKey="description" sort={sort} onSortChange={setSort}>Description</SortableTH>
                </TR>
              </THead>
              <TBody>
                {sortRows(tags, sort, TAG_ACCESSORS).map((tag) => (
                  <TR key={tag.id}>
                    <TD>
                      <span className="flex items-center gap-2">
                        <TagIcon className="h-3.5 w-3.5 shrink-0 text-primary" />
                        <span className="truncate font-mono text-xs" title={tag.name}>{tag.name}</span>
                      </span>
                    </TD>
                    <TD className="truncate text-[11px] text-muted" title={tag.description ?? undefined}>
                      {tag.description || '—'}
                    </TD>
                  </TR>
                ))}
              </TBody>
            </Table>
          </Card>
        ))}
      </div>
    </div>
  );
}
