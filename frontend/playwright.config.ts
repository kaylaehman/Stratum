import { defineConfig, devices } from '@playwright/test'

/**
 * Playwright config for Stratum frontend smoke tests.
 *
 * These tests require the app to be running. The CI job should start the
 * dev server (or a preview of the built app) before running e2e tests.
 *
 * The test:e2e script: `playwright test`
 * In CI (separate non-blocking job):
 *   1. npm run build
 *   2. npm run preview &   (or docker compose up)
 *   3. npx playwright install --with-deps chromium
 *   4. npm run test:e2e
 */
export default defineConfig({
  testDir: './e2e',
  fullyParallel: true,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 1 : 0,
  workers: process.env.CI ? 1 : undefined,
  reporter: 'list',
  use: {
    baseURL: process.env.PLAYWRIGHT_BASE_URL ?? 'http://localhost:4173',
    trace: 'on-first-retry',
  },
  projects: [
    {
      name: 'chromium',
      use: { ...devices['Desktop Chrome'] },
    },
  ],
  // No webServer block: the CI job is responsible for starting the server
  // before invoking playwright test.
})
