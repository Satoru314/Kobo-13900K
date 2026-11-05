import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';

// The build goes into the Go package that embeds it; `npm run dev` proxies the API to a running kobo.
const api = 'http://127.0.0.1:8787';

export default defineConfig({
  plugins: [react()],
  build: { outDir: '../api/src/controller/web/dist', emptyOutDir: true },
  server: {
    proxy: Object.fromEntries(['/tasks', '/projects', '/queue', '/events', '/health'].map((p) => [p, api])),
  },
});
