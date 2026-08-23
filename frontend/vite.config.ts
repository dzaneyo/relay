import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import { resolve } from 'node:path'

export default defineConfig({
  plugins: [vue()],
  server: {
    port: 5173,
    proxy: {
      '/api': {
        target: 'http://127.0.0.1:17321',
        changeOrigin: true,
      },
    },
  },

  build: {
    outDir: resolve(import.meta.dirname, '../internal/web/dist'),
    emptyOutDir: true,
  },
})
