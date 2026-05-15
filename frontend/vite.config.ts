import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'
import path from 'node:path'

// https://vite.dev/config/
export default defineConfig({
  plugins: [react(), tailwindcss()],
  resolve: {
    alias: {
      '@': path.resolve(__dirname, './src'),
    },
  },
  // Mermaid v11 lazy-loads each diagram type via dynamic `import('./chunks/...')`.
  // When the app is served behind an SPA fallback, those chunk URLs can resolve
  // to index.html and the browser refuses them with a MIME error. Folding every
  // mermaid module into a single chunk eliminates the lazy imports.
  build: {
    rollupOptions: {
      output: {
        manualChunks(id) {
          if (id.includes('node_modules/mermaid')) return 'mermaid'
        },
      },
    },
  },
  optimizeDeps: {
    include: ['mermaid'],
  },
  server: {
    port: 5173,
    strictPort: true,
    proxy: {
      '/api': {
        target: 'http://localhost:8080',
        changeOrigin: true,
      },
      '/auth': {
        target: 'http://localhost:8080',
        changeOrigin: true,
      },
      '/health': {
        target: 'http://localhost:8080',
        changeOrigin: true,
      },
    },
  },
})
