import type { CodegenConfig } from '@graphql-codegen/cli';

// Schema is mirrored from the Go server by `make sync-schema`.
// graphql-codegen produces typed React Query hooks consumed by the SPA.
const config: CodegenConfig = {
  schema: 'src/gql/schema.graphql',
  documents: 'src/gql/operations/**/*.graphql',
  generates: {
    'src/gql/generated.ts': {
      plugins: [
        'typescript',
        'typescript-operations',
        'typescript-react-query',
      ],
      config: {
        fetcher: { endpoint: '/graphql' },
        exposeFetcher: true,
        exposeQueryKeys: true,
      },
    },
  },
};

export default config;
