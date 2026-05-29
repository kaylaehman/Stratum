import { useEffect, useState, type ReactNode } from 'react'
import { Navigate } from 'react-router-dom'
import { useAuthStore } from '../store/auth'
import { apiFetch, apiGet } from '../lib/api'
import type { RefreshResponse, User } from '../types/api'

interface AuthGuardProps {
  children: ReactNode
}

type State = 'checking' | 'authenticated' | 'unauthenticated'

export function AuthGuard({ children }: AuthGuardProps) {
  const accessToken = useAuthStore((s) => s.accessToken)
  const [state, setState] = useState<State>('checking')

  useEffect(() => {
    let cancelled = false

    async function bootstrap() {
      try {
        // 1) Ensure we have an access token: a page reload clears the in-memory
        //    store, so recover it via the refresh cookie.
        let token = accessToken
        if (!token) {
          const data = await apiFetch<RefreshResponse>('/api/auth/refresh', {
            method: 'POST',
            credentials: 'include',
          })
          token = data.access_token
          useAuthStore.setState({ accessToken: token })
        }
        // 2) Ensure the user (WITH role) is loaded. Without this, useCan() sees
        //    no role and every admin/operator-gated control (AI settings, user
        //    management, the assistant launcher) is hidden after a reload.
        if (!useAuthStore.getState().user) {
          const me = await apiGet<User>('/api/me')
          useAuthStore.setState({ user: me })
        }
        if (!cancelled) setState('authenticated')
      } catch {
        if (!cancelled) setState('unauthenticated')
      }
    }

    void bootstrap()
    return () => {
      cancelled = true
    }
  }, [accessToken])

  if (state === 'checking') {
    return (
      <div
        className="min-h-screen flex items-center justify-center"
        style={{ backgroundColor: 'var(--bg-base)' }}
      >
        <span className="text-xs" style={{ color: 'var(--text-muted)' }}>
          Authenticating...
        </span>
      </div>
    )
  }

  if (state === 'unauthenticated') {
    return <Navigate to="/login" replace />
  }

  return <>{children}</>
}
