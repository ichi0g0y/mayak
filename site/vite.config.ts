import { fileURLToPath } from 'node:url'
import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'

export default defineConfig({
  plugins: [react(), tailwindcss()],
  resolve: { alias: { '@': fileURLToPath(new URL('./src', import.meta.url)) } },
  build: {
    rollupOptions: {
      input: {
        main: fileURLToPath(new URL('./index.html', import.meta.url)),
        license: fileURLToPath(new URL('./license.html', import.meta.url)),
        changelog: fileURLToPath(new URL('./changelog.html', import.meta.url)),
      },
    },
  },
  // The changelog page imports CHANGELOG.md from the repository root.
  server: { fs: { allow: ['..'] } },
})
