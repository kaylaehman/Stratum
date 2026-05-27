import { useAuthStore } from '../store/auth'
import { useStepUpStore } from '../store/stepup'
import type { RefreshResponse } from '../types/api'

export class ApiError extends Error {
  constructor(
    public readonly status: number,
    public readonly body: unknown,
  ) {
    super(`API error ${status}`)
    this.name = 'ApiError'
  }
}

let isRefreshing = false
let refreshPromise: Promise<string | null> | null = null

async function refreshToken(): Promise<string | null> {
  if (isRefreshing && refreshPromise) {
    return refreshPromise
  }
  isRefreshing = true
  refreshPromise = (async () => {
    try {
      const res = await fetch('/api/auth/refresh', {
        method: 'POST',
        credentials: 'include',
      })
      if (!res.ok) return null
      const data = (await res.json()) as RefreshResponse
      useAuthStore.getState().setAuth(data.access_token, useAuthStore.getState().user!)
      return data.access_token
    } catch {
      return null
    } finally {
      isRefreshing = false
      refreshPromise = null
    }
  })()
  return refreshPromise
}

function isAuthRoute(url: string): boolean {
  return url.startsWith('/api/auth/')
}

/** Submit a TOTP code to open the 5-minute step-up grace window. */
async function submitStepUpCode(code: string): Promise<void> {
  const { accessToken } = useAuthStore.getState()
  const headers: Record<string, string> = { 'Content-Type': 'application/json' }
  if (accessToken) headers['Authorization'] = `Bearer ${accessToken}`
  const res = await fetch('/api/me/2fa/challenge', {
    method: 'POST',
    headers,
    body: JSON.stringify({ code }),
  })
  if (res.status === 204) return
  let body: unknown
  try { body = await res.json() } catch { body = { error: res.statusText } }
  throw new ApiError(res.status, body)
}

// Exported so StepUpModal can call it directly
export { submitStepUpCode }

/**
 * Exported for raw-fetch call sites that can't route through apiFetch.
 * Shows the step-up modal and resolves when the challenge succeeds.
 * Throws with message "step_up_cancelled" if the user cancels.
 */
export function requestStepUp(): Promise<void> {
  return useStepUpStore.getState().prompt()
}

export async function apiFetch<T>(
  url: string,
  options: RequestInit = {},
): Promise<T> {
  const { accessToken } = useAuthStore.getState()

  const headers: Record<string, string> = {
    'Content-Type': 'application/json',
    ...(options.headers as Record<string, string> | undefined),
  }

  if (accessToken) {
    headers['Authorization'] = `Bearer ${accessToken}`
  }

  const fetchOptions: RequestInit = {
    ...options,
    headers,
    ...(isAuthRoute(url) ? { credentials: 'include' } : {}),
  }

  let res = await fetch(url, fetchOptions)

  // Attempt token refresh on 401 for non-auth routes
  if (res.status === 401 && !isAuthRoute(url) && accessToken) {
    const newToken = await refreshToken()
    if (newToken) {
      headers['Authorization'] = `Bearer ${newToken}`
      res = await fetch(url, { ...fetchOptions, headers })
    } else {
      useAuthStore.getState().clearAuth()
      throw new ApiError(401, { error: 'unauthorized' })
    }
  }

  // Step-up 2FA interceptor: HTTP 428 with error "2fa_required"
  if (res.status === 428) {
    let body428: unknown
    try { body428 = await res.json() } catch { body428 = {} }
    if ((body428 as { error?: string }).error === '2fa_required') {
      // Show modal (coalesces concurrent challenges into one promise)
      await useStepUpStore.getState().prompt()
      // Retry the original request once after successful challenge
      res = await fetch(url, fetchOptions)
      if (res.status === 204) return undefined as T
      if (res.ok) return res.json() as Promise<T>
      let retryBody: unknown
      try { retryBody = await res.json() } catch { retryBody = { error: res.statusText } }
      throw new ApiError(res.status, retryBody)
    }
    throw new ApiError(428, body428)
  }

  if (!res.ok) {
    let body: unknown
    try {
      body = await res.json()
    } catch {
      body = { error: res.statusText }
    }
    throw new ApiError(res.status, body)
  }

  if (res.status === 204) {
    return undefined as T
  }

  return res.json() as Promise<T>
}

export async function apiPost<T>(
  url: string,
  body: unknown,
  options: RequestInit = {},
): Promise<T> {
  return apiFetch<T>(url, {
    method: 'POST',
    body: JSON.stringify(body),
    ...options,
  })
}

export async function apiGet<T>(url: string, options: RequestInit = {}): Promise<T> {
  return apiFetch<T>(url, { method: 'GET', ...options })
}

export async function apiPut<T>(
  url: string,
  body: unknown,
  options: RequestInit = {},
): Promise<T> {
  return apiFetch<T>(url, {
    method: 'PUT',
    body: JSON.stringify(body),
    ...options,
  })
}

export async function apiDelete<T>(url: string, options: RequestInit = {}): Promise<T> {
  return apiFetch<T>(url, { method: 'DELETE', ...options })
}
