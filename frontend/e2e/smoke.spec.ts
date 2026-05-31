/**
 * Smoke test: the app loads, redirects unauthenticated users to /login, and
 * (when real credentials + a live backend are provided) login lands on the app
 * shell with the resource tree.
 *
 * Runs out of the box via `npm run test:e2e`: Playwright builds + serves the SPA
 * on :4173 (see playwright.config.ts), so the unauthenticated-redirect check
 * passes against the static app with NO backend. The login-dependent checks need
 * a real backend and are SKIPPED unless both env vars are set:
 *   - E2E_USERNAME / E2E_PASSWORD  (a valid account on the target instance)
 *   - PLAYWRIGHT_BASE_URL          (optional; point at a live full stack)
 */
import { test, expect, type Page } from '@playwright/test'

const USERNAME = process.env.E2E_USERNAME ?? ''
const PASSWORD = process.env.E2E_PASSWORD ?? ''
const hasCreds = USERNAME !== '' && PASSWORD !== ''

// The login inputs aren't wired to <label for=...>, so target them by their
// stable autocomplete / type attributes and the form button by its role.
const usernameInput = (page: Page) => page.locator('input[autocomplete="username"]')
const passwordInput = (page: Page) => page.locator('input[type="password"]')
const signInButton = (page: Page) => page.getByRole('button', { name: /sign in|log in/i })

async function login(page: Page) {
  await page.goto('/login')
  await usernameInput(page).fill(USERNAME)
  await passwordInput(page).fill(PASSWORD)
  await signInButton(page).click()
  await expect(page).not.toHaveURL(/\/login/, { timeout: 10_000 })
}

test.describe('Smoke: login and resource tree', () => {
  test('redirects unauthenticated users to the login page', async ({ page }) => {
    await page.goto('/')
    // The AuthGuard redirects unauthenticated users to /login (client-side, no
    // backend required), and the sign-in form renders.
    await expect(page).toHaveURL(/\/login/)
    await expect(page.getByRole('heading', { name: /sign in/i })).toBeVisible()
    await expect(usernameInput(page)).toBeVisible()
  })

  test('logs in and shows the app shell', async ({ page }) => {
    test.skip(!hasCreds, 'requires a live backend + E2E_USERNAME/E2E_PASSWORD')
    await login(page)
    await expect(
      page.getByRole('link', { name: /infrastructure|resources/i }).first(),
    ).toBeVisible({ timeout: 10_000 })
  })

  test('shows the Connected Hosts resource tree', async ({ page }) => {
    test.skip(!hasCreds, 'requires a live backend + E2E_USERNAME/E2E_PASSWORD')
    await login(page)
    await page.goto('/resources')
    await expect(
      page.getByText(/connected hosts|no nodes/i).first(),
    ).toBeVisible({ timeout: 15_000 })
  })
})
