import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'
import { fileURLToPath, URL } from 'node:url'

export default defineConfig({
  resolve: {
    alias: {
      '@': fileURLToPath(new URL('./src', import.meta.url)),
    },
  },
  plugins: [react(), tailwindcss()],
  server: {
    port: 3000,
    proxy: {
      '/api': { target: 'http://localhost:9003', changeOrigin: true },
      '/healthz': { target: 'http://localhost:9003', changeOrigin: true },
      '/readyz': { target: 'http://localhost:9003', changeOrigin: true },
    },
  },
  build: {
    outDir: 'dist',
  },
})
