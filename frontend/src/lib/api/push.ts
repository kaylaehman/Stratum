/**
 * Push notification API client for C9 (Web Push).
 * Handles VAPID key fetch, subscription management, and test dispatch.
 */
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { apiFetch } from '../api'

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

export interface VAPIDKeyResponse {
  public_key: string
}

export interface PushSubscriptionRequest {
  endpoint: string
  keys: {
    p256dh: string
    auth: string
  }
}

// ---------------------------------------------------------------------------
// React Query hooks
// ---------------------------------------------------------------------------

export function useVAPIDKey() {
  return useQuery({
    queryKey: ['push', 'vapid-key'],
    queryFn: () => apiFetch<VAPIDKeyResponse>('/api/push/vapid-key'),
    // VAPID key is stable; refetch rarely.
    staleTime: 60 * 60 * 1000,
  })
}

export function useSubscribePush() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (sub: PushSubscriptionRequest) =>
      apiFetch<void>('/api/push/subscribe', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(sub),
      }),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ['push'] })
    },
  })
}

export function useUnsubscribePush() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (endpoint: string) =>
      apiFetch<void>('/api/push/unsubscribe', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ endpoint }),
      }),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ['push'] })
    },
  })
}

export function useTestPush() {
  return useMutation({
    mutationFn: () => apiFetch<{ status: string }>('/api/push/test', { method: 'POST' }),
  })
}

// ---------------------------------------------------------------------------
// Browser Push API helpers
// ---------------------------------------------------------------------------

/** Returns true when the browser + context support Web Push. */
export function isPushSupported(): boolean {
  return (
    typeof window !== 'undefined' &&
    'serviceWorker' in navigator &&
    'PushManager' in window &&
    'Notification' in window
  )
}

/** Request notification permission. Returns the resulting permission state. */
export async function requestPermission(): Promise<NotificationPermission> {
  return Notification.requestPermission()
}

/**
 * Subscribe the current browser to push notifications using the given
 * VAPID public key (URL-safe base64). Returns the subscription object
 * ready to be posted to the backend, or null if the SW is not registered.
 */
export async function subscribeToPush(
  vapidPublicKey: string,
): Promise<PushSubscriptionRequest | null> {
  const reg = await navigator.serviceWorker.ready
  const existing = await reg.pushManager.getSubscription()
  if (existing) {
    return subscriptionToRequest(existing)
  }

  const subscription = await reg.pushManager.subscribe({
    userVisibleOnly: true,
    applicationServerKey: urlBase64ToUint8Array(vapidPublicKey),
  })
  return subscriptionToRequest(subscription)
}

/**
 * Unsubscribe the current browser from push and return the endpoint
 * so it can be removed from the server.
 */
export async function unsubscribeFromPush(): Promise<string | null> {
  const reg = await navigator.serviceWorker.ready
  const sub = await reg.pushManager.getSubscription()
  if (!sub) return null
  const endpoint = sub.endpoint
  await sub.unsubscribe()
  return endpoint
}

/** Check whether this browser is currently subscribed. */
export async function getCurrentSubscription(): Promise<PushSubscription | null> {
  if (!isPushSupported()) return null
  const reg = await navigator.serviceWorker.ready
  return reg.pushManager.getSubscription()
}

// ---------------------------------------------------------------------------
// Internal utils
// ---------------------------------------------------------------------------

function subscriptionToRequest(sub: PushSubscription): PushSubscriptionRequest {
  const json = sub.toJSON()
  return {
    endpoint: sub.endpoint,
    keys: {
      p256dh: json.keys?.p256dh ?? '',
      auth: json.keys?.auth ?? '',
    },
  }
}

/** Convert a URL-safe base64 VAPID public key to a Uint8Array. */
function urlBase64ToUint8Array(base64String: string): Uint8Array {
  const padding = '='.repeat((4 - (base64String.length % 4)) % 4)
  const base64 = (base64String + padding).replace(/-/g, '+').replace(/_/g, '/')
  const raw = atob(base64)
  return Uint8Array.from([...raw].map((c) => c.charCodeAt(0)))
}
