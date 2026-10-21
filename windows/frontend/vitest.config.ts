import { fileURLToPath, URL } from 'node:url'
import { defineConfig } from 'vitest/config'
import vue from '@vitejs/plugin-vue'

export default defineConfig({
  plugins: [vue()],
  css: { postcss: { plugins: [] } },
  resolve: { alias: { '@': fileURLToPath(new URL('./src', import.meta.url)) } },
  test: {
    environment: 'jsdom',
    include: ['tests/unit/**/*.test.ts'],
    setupFiles: ['tests/setup.ts'],
    clearMocks: true,
    restoreMocks: true,
    testTimeout: 5000,
    coverage: {
      provider: 'v8',
      reporter: ['text', 'html', 'json-summary'],
      reportsDirectory: '../../.artifacts/tests/frontend-coverage',
      include: ['src/composables/{navigation,polling,trafficHistory,backendState,diagnostics}.ts',
        'src/components/{ContextTabs,CommandPalette,DialogFrame}.vue'],
      thresholds: { statements: 80, lines: 80, branches: 75, functions: 70 },
    },
  },
})
