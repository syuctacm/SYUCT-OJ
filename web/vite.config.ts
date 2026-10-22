import react from '@vitejs/plugin-react'
import { defineConfig } from 'vite'
import { fileURLToPath, URL } from 'node:url'

const apiProxy = process.env.VITE_API_PROXY ?? 'http://127.0.0.1:8080'

export default defineConfig({
  plugins: [react()],
  resolve: {
    alias: {
      'monaco-editor/esm/vs': fileURLToPath(new URL('./node_modules/monaco-editor/esm/vs', import.meta.url)),
      'monaco-editor/min/vs': fileURLToPath(new URL('./node_modules/monaco-editor/min/vs', import.meta.url)),
    },
  },
  server: {
    port: 5173,
    proxy: {
      '/api': {
        target: apiProxy,
        changeOrigin: false,
        ws: true,
      },
    },
  },
})
