/// <reference types="vitest" />
import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';
import path from 'node:path';

export default defineConfig({
  // Mount under /v2/ so the SPA can coexist with the legacy UI at /.
  // The Go server serves built assets at <base>assets/* — must match
  // internal/web.RegisterAtPrefix's prefix argument.
  base: '/v2/',
  plugins: [react()],
  resolve: {
    alias: { '@': path.resolve(__dirname, 'src') },
  },
  build: {
    outDir: 'dist',
    sourcemap: true,
    rollupOptions: {
      output: {
        manualChunks: {
          react: ['react', 'react-dom'],
        },
      },
    },
  },
  server: {
    port: 5173,
    proxy: {
      '/api':     'http://localhost:8080',
      '/graphql': 'http://localhost:8080',
      '/auth':    'http://localhost:8080',
    },
  },
  test: {
    environment: 'jsdom',
    setupFiles: ['./src/test-setup.ts'],
    coverage: {
      provider: 'v8',
      reporter: ['text', 'lcov', 'html'],
      thresholds: {
        statements: 80,
        branches:   70,
        functions:  85,
        lines:      80,
      },
    },
  },
});
