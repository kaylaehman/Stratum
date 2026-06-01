// Stratum README screenshot + tour capture.
// Runs against the LOCAL DEMO stack (frontend :5173 -> backend :8080) seeded by
// `task seed:demo`. Logs in as the demo admin (password + RFC-6238 TOTP from the
// known demo secret), saves an authenticated storageState, then captures the
// README shots at 1440x900 in LIGHT theme and records a short tour video which is
// converted to an optimized GIF via Playwright's bundled ffmpeg.
//
// Run from the `frontend/` dir:  node ../scripts/capture-screenshots.mjs
import crypto from 'node:crypto'
import fs from 'node:fs'
import path from 'node:path'
import { createRequire } from 'node:module'
import { fileURLToPath } from 'node:url'

const SCRIPT_DIR = path.dirname(fileURLToPath(import.meta.url))
const ROOT = path.resolve(SCRIPT_DIR, '..')
// playwright is installed under frontend/node_modules; resolve it from there so
// this script can live under /scripts.
const require = createRequire(path.join(ROOT, 'frontend', 'package.json'))
const { chromium } = require('playwright')
const { PNG } = require('pngjs')
const { GIFEncoder, quantize, applyPalette } = require('gifenc')

const BASE = process.env.DEMO_BASE_URL || 'http://localhost:5173'
const USER = 'demo-admin'
const PASS = 'demo-admin-pw'
const TOTP_SECRET = 'JBSWY3DPEHPK3PXP' // demo only — matches seeddemo
const OUT = path.join(ROOT, 'assets', 'screenshots')
const AUTH = path.join(ROOT, 'scripts', '.auth')
const VP = { width: 1440, height: 900 }

fs.mkdirSync(OUT, { recursive: true })
fs.mkdirSync(AUTH, { recursive: true })

// ---- RFC 6238 TOTP (base32 secret, SHA1, 6 digits, 30s step) ----
function base32Decode(s) {
  const alpha = 'ABCDEFGHIJKLMNOPQRSTUVWXYZ234567'
  let bits = ''
  for (const c of s.replace(/=+$/, '').toUpperCase()) {
    const v = alpha.indexOf(c)
    if (v < 0) continue
    bits += v.toString(2).padStart(5, '0')
  }
  const bytes = []
  for (let i = 0; i + 8 <= bits.length; i += 8) bytes.push(parseInt(bits.slice(i, i + 8), 2))
  return Buffer.from(bytes)
}
function totp(secret, when = Date.now()) {
  const key = base32Decode(secret)
  let counter = Math.floor(when / 1000 / 30)
  const buf = Buffer.alloc(8)
  for (let i = 7; i >= 0; i--) { buf[i] = counter & 0xff; counter = Math.floor(counter / 256) }
  const hmac = crypto.createHmac('sha1', key).update(buf).digest()
  const off = hmac[hmac.length - 1] & 0xf
  const code = ((hmac[off] & 0x7f) << 24) | (hmac[off + 1] << 16) | (hmac[off + 2] << 8) | hmac[off + 3]
  return (code % 1_000_000).toString().padStart(6, '0')
}

const sleep = (ms) => new Promise((r) => setTimeout(r, ms))
async function light(page) {
  // index.html ships <html class="dark">; the theme store never re-applies on
  // load, so stripping the class gives the light palette for the shot.
  await page.evaluate(() => document.documentElement.classList.remove('dark')).catch(() => {})
}
async function shot(page, name, { full = false } = {}) {
  await light(page)
  await sleep(400)
  await page.screenshot({ path: path.join(OUT, name), fullPage: full })
  console.log('  captured', name)
}

async function login(page) {
  await page.goto(`${BASE}/login`, { waitUntil: 'networkidle' })
  await page.locator('input[autocomplete="username"]').fill(USER)
  await page.locator('input[autocomplete="current-password"]').fill(PASS)
  await page.getByRole('button', { name: /sign in|log in/i }).click()
  // 2FA step: a one-time-code field appears.
  const otp = page.locator('input[autocomplete="one-time-code"]')
  await otp.waitFor({ state: 'visible', timeout: 8000 })
  await otp.fill(totp(TOTP_SECRET))
  await page.getByRole('button', { name: /verify|sign in|continue|submit/i }).first().click()
  await page.waitForURL((u) => !u.pathname.endsWith('/login'), { timeout: 12000 })
  console.log('  logged in')
}

async function main() {
  const browser = await chromium.launch()

  // ---- login.png : pre-auth, with placeholder creds typed ----
  {
    const ctx = await browser.newContext({ viewport: VP })
    const page = await ctx.newPage()
    await page.goto(`${BASE}/login`, { waitUntil: 'networkidle' })
    await light(page)
    await page.locator('input[autocomplete="username"]').fill('admin')
    await page.locator('input[autocomplete="current-password"]').fill('••••••••')
    await sleep(300)
    await page.screenshot({ path: path.join(OUT, 'login.png') })
    console.log('  captured login.png')
    await ctx.close()
  }

  // ---- authenticate once, reuse storageState ----
  const authCtx = await browser.newContext({ viewport: VP })
  const authPage = await authCtx.newPage()
  await login(authPage)
  await authCtx.storageState({ path: path.join(AUTH, 'state.json') })

  const tries = []
  const tryShot = async (label, fn) => {
    try { await fn(); tries.push([label, 'ok']) }
    catch (e) { console.log('  !', label, 'failed:', e.message); tries.push([label, 'FAILED: ' + e.message]) }
  }

  // ---- dashboard.png ----
  await tryShot('dashboard', async () => {
    await authPage.goto(`${BASE}/`, { waitUntil: 'networkidle' })
    await sleep(1500)
    await shot(authPage, 'dashboard.png')
  })

  // ---- resource-tree.png : expand a node, then a container ----
  await tryShot('resource-tree', async () => {
    await authPage.goto(`${BASE}/resources`, { waitUntil: 'networkidle' })
    await sleep(1200)
    // Expand every collapsed node/stack row we can find (chevrons / node rows).
    for (const txt of ['demo-prox', 'demo-docker', 'media', 'monitoring', 'db']) {
      const el = authPage.getByText(txt, { exact: false }).first()
      if (await el.count().catch(() => 0)) { await el.click().catch(() => {}); await sleep(400) }
    }
    await sleep(600)
    await shot(authPage, 'resource-tree.png')
  })

  // ---- cve.png ----
  await tryShot('cve', async () => {
    await authPage.goto(`${BASE}/cve`, { waitUntil: 'networkidle' })
    await sleep(1500)
    await shot(authPage, 'cve.png')
  })

  // ---- remediation-approval.png : Security page, open a proposal, trigger step-up ----
  await tryShot('remediation-approval', async () => {
    await authPage.goto(`${BASE}/security`, { waitUntil: 'networkidle' })
    await sleep(1500)
    // If there's a node selector, pick the docker node that owns the proposal.
    const sel = authPage.locator('select').first()
    if (await sel.count().catch(() => 0)) {
      await sel.selectOption({ label: /demo-docker/i }).catch(() => {})
      await sleep(800)
    }
    // Click an Approve/Execute button to raise the step-up (TOTP) modal.
    const approve = authPage.getByRole('button', { name: /approve|execute/i }).first()
    if (await approve.count().catch(() => 0)) { await approve.click().catch(() => {}); await sleep(800) }
    await shot(authPage, 'remediation-approval.png')
  })

  // ---- diagnostic.png + uid-gid-visualizer.png : container detail (best-effort;
  //      these compute live and may show an empty/unavailable state for fake nodes) ----
  await tryShot('uid-gid + diagnostic', async () => {
    await authPage.goto(`${BASE}/resources`, { waitUntil: 'networkidle' })
    await sleep(1000)
    for (const txt of ['demo-docker', 'monitoring', 'grafana']) {
      const el = authPage.getByText(txt, { exact: false }).first()
      if (await el.count().catch(() => 0)) { await el.click().catch(() => {}); await sleep(500) }
    }
    await sleep(800)
    await shot(authPage, 'uid-gid-visualizer.png')
    // Try the "why is this broken?" path.
    const pathInput = authPage.locator('input[placeholder*="/var/data"], input[placeholder^="/"]').first()
    if (await pathInput.count().catch(() => 0)) {
      await pathInput.fill('/opt/demo/grafana').catch(() => {})
      await authPage.getByRole('button', { name: /analyze/i }).first().click().catch(() => {})
      await sleep(800)
      const why = authPage.getByRole('button', { name: /why is this broken/i }).first()
      if (await why.count().catch(() => 0)) { await why.click().catch(() => {}); await sleep(1000) }
    }
    await shot(authPage, 'diagnostic.png')
  })

  await authCtx.close()

  // ---- tour.gif : walk the app, grab frames, encode a GIF in pure JS ----
  // Playwright's bundled ffmpeg is a stripped build with no gif/palettegen, so
  // we collect viewport screenshots (fixed size) and encode with gifenc.
  await tryShot('tour', async () => {
    const TW = 1000, TH = 640
    // The access token is in-memory (not persisted), so storageState does NOT
    // re-authenticate a fresh context — log in for the tour too.
    const tourCtx = await browser.newContext({ viewport: { width: TW, height: TH } })
    const p = await tourCtx.newPage()
    await login(p)
    const frames = []
    const grab = async () => { await light(p); frames.push(await p.screenshot({ type: 'png' })) }
    const dwell = async (n, ms = 320) => { for (let i = 0; i < n; i++) { await grab(); await sleep(ms) } }
    const visit = async (url) => { await p.goto(`${BASE}${url}`, { waitUntil: 'networkidle' }); await sleep(600) }

    await visit('/'); await dwell(6)                 // dashboard
    await visit('/resources'); await dwell(2)
    for (const txt of ['demo-prox', 'demo-docker', 'media', 'monitoring']) {
      const el = p.getByText(txt, { exact: false }).first()
      if (await el.count().catch(() => 0)) { await el.click().catch(() => {}); await grab(); await sleep(350) }
    }
    await dwell(3)
    await visit('/cve'); await dwell(5)
    await visit('/security'); await dwell(5)
    await p.close(); await tourCtx.close()

    if (!frames.length) throw new Error('no frames captured')
    const gif = path.join(ROOT, 'assets', 'tour.gif')
    const enc = GIFEncoder()
    for (const buf of frames) {
      const png = PNG.sync.read(buf)
      const rgba = new Uint8Array(png.data.buffer, png.data.byteOffset, png.data.length)
      const palette = quantize(rgba, 256)
      const index = applyPalette(rgba, palette)
      enc.writeFrame(index, png.width, png.height, { palette, delay: 110 })
    }
    enc.finish()
    fs.writeFileSync(gif, Buffer.from(enc.bytes()))
    console.log(`  captured tour.gif (${frames.length} frames, ${(fs.statSync(gif).size / 1024 / 1024).toFixed(1)} MB)`)
  })

  await browser.close()
  console.log('\n=== capture summary ===')
  for (const [l, s] of tries) console.log(`  ${s.startsWith('ok') ? 'OK ' : 'XX '} ${l}: ${s}`)
}

main().catch((e) => { console.error('FATAL', e); process.exit(1) })
