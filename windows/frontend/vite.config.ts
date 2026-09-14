import { fileURLToPath, URL } from 'node:url'
import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import { viteSingleFile } from 'vite-plugin-singlefile'

// singlefile: всё приложение собирается в один dist/index.html — удобно и для
// предпросмотра, и для встраивания в Wails-бинарник (без внешних ассетов).
export default defineConfig({
  plugins: [vue(), viteSingleFile()],
  resolve: {
    alias: {
      '@': fileURLToPath(new URL('./src', import.meta.url)),
    },
  },
  server: {
    port: 5273,
    strictPort: false,
  },
  build: {
    target: 'es2022',
    outDir: 'dist',
    assetsInlineLimit: 4096,
    base: './',
  },
})
