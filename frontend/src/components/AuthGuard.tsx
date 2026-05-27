import { useEffect, useState, type ReactNode } from 'react'
import { Navigate } from 'react-router-dom'
import { useAuthStore } from '../store/auth'
import { apiFetch } from '../lib/api'
import type { RefreshResponse } from '../types/api'

interface AuthGuardProps {
  children: ReactNode
}

type State = 'checking' | 'authenticated' | 'unauthenticated'

export function AuthGuard({ children }: AuthGuardProps) {
  const { accessToken, setAuth, user } = useAuthStore()
  const [state, setState] = useState<State>(accessToken ? 'authenticated' : 'checking')

  useEffect(() => {
    if (accessToken) {
      setState('authenticated')
      return
    }

    // Attempt a silent refresh
    apiFetch<RefreshResponse>('/api/auth/refresh', {
      method: 'POST',
      credentials: 'include',
    })
      .then((data) => {
        // We need the user object too — fetch /api/me after refresh
        setAuth(data.access_token, user!)
        setState('authenticated')
      })
      .catch(() => {
        setState('unauthenticated')
      })
  }, [accessToken, setAuth, user])

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
