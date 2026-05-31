import { defineConfig, devices } from '@playwright/test'

/**
 * Playwright config for Stratum frontend smoke tests.
 *
 * Self-sufficient by default: when targeting localhost (the default), Playwright
 * builds the SPA and serves it via `vite preview` on :4173 automatically, so
 * `npm run test:e2e` works with no manual server setup. The unauthenticated
 * redirect test passes against the static app alone; the login/tree tests need a
 * real backend and skip unless E2E_USERNAME/E2E_PASSWORD are set (see smoke.spec).
 *
 * To run against a live full stack instead, set PLAYWRIGHT_BASE_URL (e.g. the
 * deployed URL) and E2E_USERNAME/E2E_PASSWORD — the local webServer is then
 * skipped automatically.
 */
const baseURL = process.env.PLAYWRIGHT_BASE_URL ?? 'http://localhost:4173'
const isLocal = baseURL.includes('localhost') || baseURL.includes('127.0.0.1')

export default defineConfig({
  testDir: './e2e',
  fullyParallel: true,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 1 : 0,
  workers: process.env.CI ? 1 : undefined,
  reporter: 'list',
  use: {
    baseURL,
    trace: 'on-first-retry',
  },
  projects: [
    {
      name: 'chromium',
      use: { ...devices['Desktop Chrome'] },
    },
  ],
  // When targeting localhost, build + preview the app automatically. When
  // PLAYWRIGHT_BASE_URL points at an external stack, don't start a server.
  webServer: isLocal
    ? {
        command: 'npm run build && npm run preview -- --port 4173 --strictPort',
        url: 'http://localhost:4173',
        reuseExistingServer: !process.env.CI,
        timeout: 180_000,
      }
    : undefined,
})
