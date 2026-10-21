import { defineConfig } from '@playwright/test'

// Separate ports and temporary browser profile; never attach to the running VPN.
export default defineConfig({
  testDir: './tests/browser',
  timeout: 30000,
  expect: { timeout: 7000 },
  fullyParallel: false,
  workers: 1,
  retries: 0,
  reporter: [['list'], ['html', { outputFolder: '../../.artifacts/tests/browser-report', open: 'never' }]],
  outputDir: '../../.artifacts/tests/browser-results',
  use: {
    baseURL: 'http://127.0.0.1:5374',
    headless: true,
    trace: 'retain-on-failure',
    screenshot: 'only-on-failure',
  },
  projects: [{ name: 'chromium-offline' }],
  webServer: [
    { command: 'npm run dev -- --host 127.0.0.1 --port 5374 --strictPort', url: 'http://127.0.0.1:5374', reuseExistingServer: false },
    { command: 'npm run preview -- --host 127.0.0.1 --port 5375 --strictPort', url: 'http://127.0.0.1:5375', reuseExistingServer: false },
  ],
})
