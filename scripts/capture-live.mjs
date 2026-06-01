// Captures the LIVE diagnostic + UID/GID shots against the real demo-lab node
// (dind sandbox: grafana container running as uid 472 with a bind mount it can't
// read). Logs in (password + TOTP), expands demo-lab, selects grafana, captures
// the UID/GID visualizer, then runs "why is this broken?" on the host path.
import crypto from 'node:crypto'
import path from 'node:path'
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
function b32(s) { const a = 'ABCDEFGHIJKLMNOPQRSTUVWXYZ234567'; let b = ''; for (const c of s.replace(/=+$/, '')) { const v = a.indexOf(c); if (v >= 0) b += v.toString(2).padStart(5, '0') } const o = []; for (let i = 0; i + 8 <= b.length; i += 8) o.push(parseInt(b.slice(i, i + 8), 2)); return Buffer.from(o) }
function totp(sec) { const k = b32(sec); let c = Math.floor(Date.now() / 1000 / 30); const buf = Buffer.alloc(8); for (let i = 7; i >= 0; i--) { buf[i] = c & 0xff; c = Math.floor(c / 256) } const h = crypto.createHmac('sha1', k).update(buf).digest(); const off = h[h.length - 1] & 0xf; const code = ((h[off] & 0x7f) << 24) | (h[off + 1] << 16) | (h[off + 2] << 8) | h[off + 3]; return (code % 1e6).toString().padStart(6, '0') }
const light = (p) => p.evaluate(() => document.documentElement.classList.remove('dark')).catch(() => {})

const browser = await chromium.launch()
const ctx = await browser.newContext({ viewport: { width: 1440, height: 900 } })
const p = await ctx.newPage()

await p.goto(`${BASE}/login`, { waitUntil: 'networkidle' })
await p.locator('input[autocomplete="username"]').fill('demo-admin')
await p.locator('input[autocomplete="current-password"]').fill('demo-admin-pw')
await p.getByRole('button', { name: /sign in|log in/i }).click()
const otp = p.locator('input[autocomplete="one-time-code"]')
await otp.waitFor({ state: 'visible', timeout: 8000 })
await otp.fill(totp(SECRET))
await p.getByRole('button', { name: /verify|sign in|continue|submit/i }).first().click()
await p.waitForURL((u) => !u.pathname.endsWith('/login'), { timeout: 12000 })

// Deep-link straight to the real grafana container on demo-lab (avoids tree
// disambiguation; IDs passed via env). Falls back to bare /resources.
const LAB = process.env.LAB_NODE_ID || ''
const GRAFANA = process.env.GRAFANA_ID || ''
const url = LAB && GRAFANA ? `${BASE}/resources?node=${LAB}&container=${GRAFANA}` : `${BASE}/resources`
await p.goto(url, { waitUntil: 'networkidle' })
await sleep(5000) // let the UID/GID visualizer resolve via live docker exec + SSH
await light(p)
await p.screenshot({ path: path.join(OUT, 'uid-gid-visualizer.png') })
console.log('captured uid-gid-visualizer.png')

// "why is this broken?" on the host bind-mount path (lives lower in the pane).
const inp = p.getByPlaceholder('/var/data/app.conf').first()
if (await inp.count().catch(() => 0)) {
  await inp.scrollIntoViewIfNeeded().catch(() => {})
  await inp.fill('/demo/grafana/grafana.ini')
  await p.getByRole('button', { name: /^analyze$/i }).first().click().catch(() => {})
  await sleep(3500) // live FileUID resolve (SSH stat + docker inspect)
  const why = p.getByRole('button', { name: /why is this broken/i }).first()
  if (await why.count().catch(() => 0)) {
    await why.scrollIntoViewIfNeeded().catch(() => {})
    await why.click().catch(() => {})
    await sleep(6000) // live diagnostic resolve
  } else { console.log('no "why is this broken" button') }
  // Scroll the diagnostic card (bottom of pane) into view for the shot.
  const card = p.getByText(/read denied|read allowed|owned by|category|other/i).last()
  if (await card.count().catch(() => 0)) await card.scrollIntoViewIfNeeded().catch(() => {})
  await sleep(800)
  await light(p)
  await p.screenshot({ path: path.join(OUT, 'diagnostic.png') })
  console.log('captured diagnostic.png')
} else {
  console.log('NO host-path input found (container detail not open?)')
}
await ctx.close(); await browser.close()
