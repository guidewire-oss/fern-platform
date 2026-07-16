import { useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { Tag as TagIcon, Search } from 'lucide-react';
import { graphqlFetch } from '@/lib/api';
import { Spinner } from '@/components/ui/Spinner';
import { Card } from '@/components/ui/Card';
import { EmptyState } from '@/components/ui/EmptyState';

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

interface TagsResp {
  tags: { totalCount: number; edges: { node: Tag }[] };
}

export default function TagsManagement() {
  const [q, setQ] = useState('');

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
    const key = node.category || 'uncategorized';
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
            <ul className="divide-y divide-border">
              {tags.map((tag) => (
                <li key={tag.id} className="flex items-center gap-2 px-4 py-2">
                  <TagIcon className="h-3.5 w-3.5 text-primary" />
                  <span className="font-mono text-xs text-foreground">{tag.name}</span>
                  {tag.description && (
                    <span className="ml-auto truncate text-[11px] text-muted" title={tag.description}>
                      {tag.description}
                    </span>
                  )}
                </li>
              ))}
            </ul>
          </Card>
        ))}
      </div>
    </div>
  );
}
