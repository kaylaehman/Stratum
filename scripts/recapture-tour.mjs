// Focused re-capture of assets/tour.gif. Logs in (password + RFC-6238 TOTP),
// walks the app, and encodes a GIF in pure JS (gifenc + pngjs). The access token
// is in memory, so a real login is required (storageState won't authenticate).
import crypto from 'node:crypto'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
import { createRequire } from 'node:module'

const SCRIPT_DIR = path.dirname(fileURLToPath(import.meta.url))
const ROOT = path.resolve(SCRIPT_DIR, '..')
const require = createRequire(path.join(ROOT, 'frontend', 'package.json'))
const { chromium } = require('playwright')
const { PNG } = require('pngjs')
const { GIFEncoder, quantize, applyPalette } = require('gifenc')

const BASE = 'http://localhost:5173'
const SECRET = 'JBSWY3DPEHPK3PXP'
const sleep = (ms) => new Promise((r) => setTimeout(r, ms))
function b32(s) { const a = 'ABCDEFGHIJKLMNOPQRSTUVWXYZ234567'; let b = ''; for (const c of s.replace(/=+$/, '')) { const v = a.indexOf(c); if (v >= 0) b += v.toString(2).padStart(5, '0') } const o = []; for (let i = 0; i + 8 <= b.length; i += 8) o.push(parseInt(b.slice(i, i + 8), 2)); return Buffer.from(o) }
function totp(sec) { const k = b32(sec); let c = Math.floor(Date.now() / 1000 / 30); const buf = Buffer.alloc(8); for (let i = 7; i >= 0; i--) { buf[i] = c & 0xff; c = Math.floor(c / 256) } const h = crypto.createHmac('sha1', k).update(buf).digest(); const off = h[h.length - 1] & 0xf; const code = ((h[off] & 0x7f) << 24) | (h[off + 1] << 16) | (h[off + 2] << 8) | h[off + 3]; return (code % 1e6).toString().padStart(6, '0') }
async function light(p) { await p.evaluate(() => document.documentElement.classList.remove('dark')).catch(() => {}) }

const browser = await chromium.launch()
const ctx = await browser.newContext({ viewport: { width: 1000, height: 640 } })
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

const frames = []
const DUMP = process.env.DUMP_FRAMES ? path.join(ROOT, 'assets', 'screenshots', '.tmp-frames') : null
if (DUMP) fs.mkdirSync(DUMP, { recursive: true })
const grab = async () => { await light(p); const b = await p.screenshot({ type: 'png' }); if (DUMP) fs.writeFileSync(path.join(DUMP, `${String(frames.length).padStart(2, '0')}.png`), b); frames.push(b) }
const dwell = async (n, ms = 320) => { for (let i = 0; i < n; i++) { await grab(); await sleep(ms) } }
const visit = async (url) => { await p.goto(`${BASE}${url}`, { waitUntil: 'domcontentloaded' }); await sleep(1200); console.log('  at', p.url()) }

await visit('/'); await dwell(6)
await visit('/resources'); await dwell(2)
for (const txt of ['demo-prox', 'demo-docker', 'media', 'monitoring']) {
  const el = p.getByText(txt, { exact: false }).first()
  if (await el.count().catch(() => 0)) { await el.click().catch(() => {}); await grab(); await sleep(350) }
}
await dwell(3)
await visit('/cve'); await dwell(5)
await visit('/security'); await dwell(4)
await ctx.close(); await browser.close()

const gif = path.join(ROOT, 'assets', 'tour.gif')
const enc = GIFEncoder()
for (const buf of frames) {
  const png = PNG.sync.read(buf)
  const rgba = new Uint8Array(png.data.buffer, png.data.byteOffset, png.data.length)
  const palette = quantize(rgba, 256)
  const index = applyPalette(rgba, palette)
  enc.writeFrame(index, png.width, png.height, { palette, delay: 240 })
}
enc.finish()
fs.writeFileSync(gif, Buffer.from(enc.bytes()))
console.log(`tour.gif: ${frames.length} frames, ${(fs.statSync(gif).size / 1024 / 1024).toFixed(2)} MB`)
