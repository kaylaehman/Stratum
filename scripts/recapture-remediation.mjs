// Focused re-capture of remediation-approval.png. Logs in (the app keeps the
// access token in memory, so storageState alone doesn't authenticate), selects
// the demo-docker node so the per-node remediation panel renders the seeded
// "proposed" proposal, then raises the TOTP step-up modal.
import path from 'node:path'
import crypto from 'node:crypto'
import { fileURLToPath } from 'node:url'
import { createRequire } from 'node:module'

const SCRIPT_DIR = path.dirname(fileURLToPath(import.meta.url))
const ROOT = path.resolve(SCRIPT_DIR, '..')
const require = createRequire(path.join(ROOT, 'frontend', 'package.json'))
const { chromium } = require('playwright')

const BASE = 'http://localhost:5173'
const OUT = path.join(ROOT, 'assets', 'screenshots')
const SECRET = 'JBSWY3DPEHPK3PXP'
const sleep = (ms) => new Promise((r) => setTimeout(r, ms))

function b32(s) { const a = 'ABCDEFGHIJKLMNOPQRSTUVWXYZ234567'; let b = ''; for (const c of s.replace(/=+$/, '')) { const v = a.indexOf(c); if (v >= 0) b += v.toString(2).padStart(5, '0') } const out = []; for (let i = 0; i + 8 <= b.length; i += 8) out.push(parseInt(b.slice(i, i + 8), 2)); return Buffer.from(out) }
function totp(secret) { const key = b32(secret); let c = Math.floor(Date.now() / 1000 / 30); const buf = Buffer.alloc(8); for (let i = 7; i >= 0; i--) { buf[i] = c & 0xff; c = Math.floor(c / 256) } const h = crypto.createHmac('sha1', key).update(buf).digest(); const o = h[h.length - 1] & 0xf; const code = ((h[o] & 0x7f) << 24) | (h[o + 1] << 16) | (h[o + 2] << 8) | h[o + 3]; return (code % 1e6).toString().padStart(6, '0') }

const browser = await chromium.launch()
const ctx = await browser.newContext({ viewport: { width: 1440, height: 900 } })
const page = await ctx.newPage()

// login
await page.goto(`${BASE}/login`, { waitUntil: 'networkidle' })
await page.locator('input[autocomplete="username"]').fill('demo-admin')
await page.locator('input[autocomplete="current-password"]').fill('demo-admin-pw')
await page.getByRole('button', { name: /sign in|log in/i }).click()
const otp = page.locator('input[autocomplete="one-time-code"]')
await otp.waitFor({ state: 'visible', timeout: 8000 })
await otp.fill(totp(SECRET))
await page.getByRole('button', { name: /verify|sign in|continue|submit/i }).first().click()
await page.waitForURL((u) => !u.pathname.endsWith('/login'), { timeout: 12000 })

await page.goto(`${BASE}/security`, { waitUntil: 'networkidle' })
await sleep(1500)
const sel = page.locator('select').first()
await sel.selectOption({ label: 'demo-docker' }).catch(async () => { await sel.selectOption({ index: 2 }).catch(() => {}) })
// Posture does a live SSH-key audit + update check, which hangs ~30s on the
// fake node's SSH dial before resolving and rendering the remediation panel.
await page.getByText(/computing posture score/i).waitFor({ state: 'detached', timeout: 70000 }).catch(() => {})
await sleep(1500)

const proposal = page.getByText(/bind-mount permissions|proposed|remediation/i).first()
if (await proposal.count().catch(() => 0)) await proposal.scrollIntoViewIfNeeded().catch(() => {})
await sleep(500)
for (const name of [/approve/i, /execute/i]) {
  const b = page.getByRole('button', { name }).first()
  if (await b.count().catch(() => 0)) { await b.click().catch(() => {}); await sleep(1000) }
}
await sleep(700)
await page.screenshot({ path: path.join(OUT, 'remediation-approval.png') })
console.log('captured remediation-approval.png')
await ctx.close()
await browser.close()
