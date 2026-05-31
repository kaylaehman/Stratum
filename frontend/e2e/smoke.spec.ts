/**
 * Smoke test: login → resource tree renders.
 *
 * This test hits a real running Stratum instance. It is intentionally minimal:
 * just enough to confirm the app shell loads and the resource tree is present
 * after a successful login.
 *
 * Prerequisites:
 *   - PLAYWRIGHT_BASE_URL env var (default: http://localhost:4173)
 *   - E2E_USERNAME / E2E_PASSWORD env vars for a valid test account
 */
import { test, expect } from '@playwright/test'

const USERNAME = process.env.E2E_USERNAME ?? 'admin'
const PASSWORD = process.env.E2E_PASSWORD ?? 'changeme'

test.describe('Smoke: login and resource tree', () => {
  test('should land on the login page when unauthenticated', async ({ page }) => {
    await page.goto('/')
    // The app redirects unauthenticated users to /login
    await expect(page).toHaveURL(/\/login/)
    await expect(page.getByRole('heading', { name: /stratum/i })).toBeVisible()
  })

  test('should log in successfully and show the resource tree', async ({ page }) => {
    await page.goto('/login')

    // Fill in credentials
    await page.getByLabel(/username/i).fill(USERNAME)
    await page.getByLabel(/password/i).fill(PASSWORD)
    await page.getByRole('button', { name: /sign in|log in/i }).click()

    // After login, we should land on the dashboard or infrastructure page
    await expect(page).not.toHaveURL(/\/login/, { timeout: 10_000 })

    // The resource tree sidebar / infrastructure tree should be rendered.
    // The AppShell renders nav items regardless of page.
    await expect(
      page.getByRole('link', { name: /infrastructure|resources/i }).first(),
    ).toBeVisible({ timeout: 10_000 })
  })

  test('should show Connected Hosts section on the infrastructure page', async ({ page }) => {
    // Login
    await page.goto('/login')
    await page.getByLabel(/username/i).fill(USERNAME)
    await page.getByLabel(/password/i).fill(PASSWORD)
    await page.getByRole('button', { name: /sign in|log in/i }).click()
    await expect(page).not.toHaveURL(/\/login/, { timeout: 10_000 })

    // Navigate to the infrastructure / resources page
    await page.goto('/resources')

    // The resource tree should load — either showing nodes or the empty state
    await expect(
      page.getByText(/connected hosts|no nodes/i).first(),
    ).toBeVisible({ timeout: 15_000 })
  })
})
