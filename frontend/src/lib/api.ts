import { useAuthStore } from '../store/auth'
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
