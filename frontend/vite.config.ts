import { defineConfig } from 'vite';
import { svelte } from '@sveltejs/vite-plugin-svelte';

export default defineConfig({
  plugins: [svelte()],
  resolve: {
    conditions: ['browser'],
  },
  server: {
    port: 5173,
    fs: {
      // The translation catalogs live in /locales at the repository root so a
      // deployment can mount that one directory as a volume. English is
      // imported from there as the bundled fallback, which puts it outside the
      // frontend project root.
      allow: ['..'],
    },
    proxy: {
      '/api': {
        target: 'http://127.0.0.1:8080',
        changeOrigin: true,
        ws: true,
      },
    },
  },
  test: {
    environment: 'jsdom',
  },
});
