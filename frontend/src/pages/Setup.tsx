import { useState, type FormEvent } from 'react'
import { useNavigate } from 'react-router-dom'
import { apiPost, ApiError } from '../lib/api'
import type { SetupAdminResponse, SetupAdminRequest } from '../types/api'
import { Server } from 'lucide-react'

export default function Setup() {
  const navigate = useNavigate()

  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [email, setEmail] = useState('')
  const [error, setError] = useState<string | null>(null)
  const [loading, setLoading] = useState(false)

  async function handleSubmit(e: FormEvent) {
    e.preventDefault()
    setError(null)
    setLoading(true)

    const body: SetupAdminRequest = { username, password }
    if (email.trim()) body.email = email.trim()

    try {
      await apiPost<SetupAdminResponse>('/api/setup/admin', body)
      navigate('/login')
    } catch (err) {
      if (err instanceof ApiError && err.status === 403) {
        setError('Setup is already complete. Please log in.')
        setTimeout(() => navigate('/login'), 2000)
      } else if (err instanceof ApiError && err.status === 400) {
        const body = err.body as { error?: string }
        setError(body.error ?? 'Invalid input.')
      } else {
        setError('Setup failed. Please try again.')
      }
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="min-h-screen flex items-center justify-center" style={{ backgroundColor: 'var(--bg-base)' }}>
      <div
        className="w-full max-w-sm rounded border p-8"
        style={{
          backgroundColor: 'var(--bg-surface)',
          borderColor: 'var(--border-default)',
        }}
      >
        <div className="flex items-center gap-3 mb-6">
          <Server size={20} style={{ color: 'var(--accent)' }} />
          <span className="text-lg font-semibold tracking-tight" style={{ color: 'var(--text-primary)' }}>
            Stratum
          </span>
        </div>

        <h1 className="text-base font-medium mb-1" style={{ color: 'var(--text-primary)' }}>
          Initial Setup
        </h1>
        <p className="text-xs mb-6" style={{ color: 'var(--text-secondary)' }}>
          Create your administrator account to get started.
        </p>

        <form onSubmit={handleSubmit} className="flex flex-col gap-4">
          <div className="flex flex-col gap-1">
            <label className="text-xs font-medium" style={{ color: 'var(--text-secondary)' }}>
              Username
            </label>
            <input
              type="text"
              value={username}
              onChange={(e) => setUsername(e.target.value)}
              required
              autoComplete="username"
              className="w-full px-3 py-2 outline-none font-mono text-xs"
              style={{
                backgroundColor: 'var(--bg-elevated)',
                border: '1px solid var(--border-default)',
                color: 'var(--text-primary)',
                borderRadius: '3px',
              }}
              onFocus={(e) => (e.currentTarget.style.borderColor = 'var(--accent)')}
              onBlur={(e) => (e.currentTarget.style.borderColor = 'var(--border-default)')}
            />
          </div>

          <div className="flex flex-col gap-1">
            <label className="text-xs font-medium" style={{ color: 'var(--text-secondary)' }}>
              Password
            </label>
            <input
              type="password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              required
              autoComplete="new-password"
              className="w-full px-3 py-2 outline-none font-mono text-xs"
              style={{
                backgroundColor: 'var(--bg-elevated)',
                border: '1px solid var(--border-default)',
                color: 'var(--text-primary)',
                borderRadius: '3px',
              }}
              onFocus={(e) => (e.currentTarget.style.borderColor = 'var(--accent)')}
              onBlur={(e) => (e.currentTarget.style.borderColor = 'var(--border-default)')}
            />
          </div>

          <div className="flex flex-col gap-1">
            <label className="text-xs font-medium" style={{ color: 'var(--text-secondary)' }}>
              Email{' '}
              <span style={{ color: 'var(--text-muted)' }}>(optional)</span>
            </label>
            <input
              type="email"
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              autoComplete="email"
              className="w-full px-3 py-2 outline-none font-mono text-xs"
              style={{
                backgroundColor: 'var(--bg-elevated)',
                border: '1px solid var(--border-default)',
                color: 'var(--text-primary)',
                borderRadius: '3px',
              }}
              onFocus={(e) => (e.currentTarget.style.borderColor = 'var(--accent)')}
              onBlur={(e) => (e.currentTarget.style.borderColor = 'var(--border-default)')}
            />
          </div>

          {error && (
            <p className="text-xs" style={{ color: 'var(--status-error)' }}>
              {error}
            </p>
          )}

          <button
            type="submit"
            disabled={loading}
            className="w-full py-2 px-4 font-medium text-xs transition-opacity disabled:opacity-50"
            style={{
              backgroundColor: 'var(--accent)',
              color: 'var(--text-inverse)',
              borderRadius: '3px',
              border: 'none',
              cursor: loading ? 'not-allowed' : 'pointer',
            }}
          >
            {loading ? 'Creating account...' : 'Create admin account'}
          </button>
        </form>
      </div>
    </div>
  )
}
