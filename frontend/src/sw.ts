/// <reference lib="webworker" />
import { cleanupOutdatedCaches, precacheAndRoute } from 'workbox-precaching'

declare const self: ServiceWorkerGlobalScope

// Workbox injectManifest replaces this placeholder with the asset manifest.
// eslint-disable-next-line @typescript-eslint/no-explicit-any
precacheAndRoute((self as any).__WB_MANIFEST)
cleanupOutdatedCaches()

// ---------------------------------------------------------------------------
// Push notifications
// ---------------------------------------------------------------------------

interface PushPayload {
  title: string
  body: string
  icon?: string
  tag?: string
  url?: string
  resource_id?: string
  resource_type?: string
  actions?: Array<{ action: string; title: string }>
}

self.addEventListener('push', (event: PushEvent) => {
  if (!event.data) return

  let payload: PushPayload
  try {
    payload = event.data.json() as PushPayload
  } catch {
    payload = { title: 'Stratum', body: event.data.text() }
  }

  const { title, body, icon = '/icon-192.png', tag, actions = [] } = payload

  event.waitUntil(
    self.registration.showNotification(title, {
      body,
      icon,
      badge: '/icon-192.png',
      tag: tag ?? 'stratum',
      data: payload,
      // Actions surfaces quick-action buttons in supported browsers.
      // The browser clips the list to its supported max (typically 2).
      actions,
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
    } as any),
  )
})

// ---------------------------------------------------------------------------
// Notification click / action handling
// ---------------------------------------------------------------------------

self.addEventListener('notificationclick', (event: NotificationEvent) => {
  event.notification.close()

  const payload = event.notification.data as PushPayload | undefined
  const action = event.action // '' = main body click

  event.waitUntil(handleNotificationClick(action, payload))
})

async function handleNotificationClick(
  action: string,
  payload: PushPayload | undefined,
): Promise<void> {
  const resourceID = payload?.resource_id ?? ''
  const resourceType = payload?.resource_type ?? ''
  const deepURL = payload?.url ?? '/'

  if (action === 'ack' && resourceType === 'container' && resourceID) {
    // Fire-and-forget: acknowledge the incident via the existing ack endpoint.
    // Server enforces operator/admin role; we just send the request.
    await safeFetch(`/api/security/ack/${resourceID}`, { method: 'POST' })
    await focusOrOpen(deepURL)
    return
  }

  if (action === 'restart' && resourceType === 'container' && resourceID) {
    await safeFetch(`/api/containers/${resourceID}/restart`, { method: 'POST' })
    await focusOrOpen(deepURL)
    return
  }

  // Default: open/focus the app at the deeplink URL.
  await focusOrOpen(deepURL)
}

async function safeFetch(url: string, init: RequestInit): Promise<void> {
  try {
    await fetch(url, { ...init, credentials: 'include' })
  } catch {
    // Best-effort; offline or auth failure — ignore.
  }
}

async function focusOrOpen(url: string): Promise<void> {
  const clients = await self.clients.matchAll({ type: 'window', includeUncontrolled: true })
  for (const client of clients) {
    if ('focus' in client) {
      await client.focus()
      if (url !== '/') {
        client.postMessage({ type: 'navigate', url })
      }
      return
    }
  }
  await self.clients.openWindow(url)
}

// ---------------------------------------------------------------------------
// Skip waiting (same as before — main.tsx controls when to call this)
// ---------------------------------------------------------------------------

self.addEventListener('message', (event: ExtendableMessageEvent) => {
  if (event.data?.type === 'SKIP_WAITING') {
    void self.skipWaiting()
  }
})
