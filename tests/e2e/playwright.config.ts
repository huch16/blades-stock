import { defineConfig, devices } from '@playwright/test';

export default defineConfig({
  testDir: './tests/e2e',
  fullyParallel: true,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 2 : 0,
  workers: process.env.CI ? 1 : undefined,
  reporter: [['html', { outputFolder: 'playwright-report' }], ['json', { outputFile: 'test-results/e2e-results.json' }]],
  use: {
    baseURL: 'http://localhost:34115', // Wails dev server default
    trace: 'on-first-retry',
    screenshot: 'only-on-failure',
    video: 'retain-on-failure',
  },
  projects: [
    {
      name: 'chromium',
      use: { ...devices['Desktop Chrome'] },
    },
  ],
  webServer: {
    command: 'cd ../.. && wails dev -tags "desktop,production"',
    url: 'http://localhost:34115',
    reuseExistingServer: !process.env.CI,
    timeout: 120000,
    env: {
      BLADES_STOCK_LOG_CONSOLE: '1',
    },
  },
  timeout: 60000,
  expect: {
    timeout: 10000,
  },
});
