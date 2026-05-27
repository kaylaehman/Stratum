import { useState, type FormEvent } from 'react'
import { useNavigate } from 'react-router-dom'
import { useAuthStore } from '../store/auth'
import { apiPost } from '../lib/api'
import { ApiError } from '../lib/api'
import type { LoginResponse } from '../types/api'
import { Server, KeyRound } from 'lucide-react'

type LoginStep = 'credentials' | 'totp'

function inputProps(focused: boolean): React.CSSProperties {
  return {
    backgroundColor: 'var(--bg-elevated)',
    border: `1px solid ${focused ? 'var(--accent)' : 'var(--border-default)'}`,
    color: 'var(--text-primary)',
    borderRadius: '3px',
    outline: 'none',
    width: '100%',
    padding: '8px 12px',
    fontFamily: 'monospace',
    fontSize: '0.75rem',
    boxSizing: 'border-box',
  }
}

export default function Login() {
  const navigate = useNavigate()
  const setAuth = useAuthStore((s) => s.setAuth)

  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [totp, setTotp] = useState('')
  const [step, setStep] = useState<LoginStep>('credentials')
  const [error, setError] = useState<string | null>(null)
  const [loading, setLoading] = useState(false)

  // focus state per field
  const [focusedField, setFocusedField] = useState<string | null>(null)

  async function handleSubmit(e: FormEvent) {
    e.preventDefault()
    setError(null)
    setLoading(true)

    try {
      const body: Record<string, string> = { username, password }
      if (step === 'totp') body.totp = totp.trim()

      const data = await apiPost<LoginResponse>('/api/auth/login', body)
      setAuth(data.access_token, data.user)
      navigate('/')
    } catch (err) {
      if (err instanceof ApiError && err.status === 401) {
        const apiBody = err.body as { error?: string } | null
        const code = apiBody?.error

        if (code === 'totp_required') {
          // Transition to TOTP step — do not show an error, just prompt for code
          setStep('totp')
          setTotp('')
        } else if (code === 'invalid_totp') {
          setError('Invalid code. Try again.')
        } else {
          setError('Invalid username or password.')
        }
      } else {
        setError('Login failed. Please try again.')
      }
    } finally {
      setLoading(false)
    }
  }

  return (
    <div
      className="min-h-screen flex items-center justify-center"
      style={{ backgroundColor: 'var(--bg-base)' }}
    >
      <div
        className="w-full max-w-sm rounded border p-8"
        style={{
          backgroundColor: 'var(--bg-surface)',
          borderColor: 'var(--border-default)',
          borderRadius: '3px',
        }}
      >
        {/* Logo */}
        <div className="flex items-center gap-3 mb-8">
          <Server size={20} style={{ color: 'var(--accent)' }} />
          <span className="text-lg font-semibold tracking-tight" style={{ color: 'var(--text-primary)' }}>
            Stratum
          </span>
        </div>

        {step === 'credentials' ? (
          <>
            <h1 className="text-base font-medium mb-6" style={{ color: 'var(--text-primary)' }}>
              Sign in
            </h1>

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
                  style={inputProps(focusedField === 'username')}
                  onFocus={() => setFocusedField('username')}
                  onBlur={() => setFocusedField(null)}
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
                  autoComplete="current-password"
                  style={inputProps(focusedField === 'password')}
                  onFocus={() => setFocusedField('password')}
                  onBlur={() => setFocusedField(null)}
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
                {loading ? 'Signing in…' : 'Sign in'}
              </button>
            </form>
          </>
        ) : (
          <>
            {/* TOTP step */}
            <div className="flex items-center gap-2 mb-6">
              <KeyRound size={16} style={{ color: 'var(--accent)' }} />
              <h1 className="text-base font-medium" style={{ color: 'var(--text-primary)', margin: 0 }}>
                Two-Factor Auth
              </h1>
            </div>

            <p className="text-xs mb-5" style={{ color: 'var(--text-secondary)' }}>
              Enter the 6-digit code from your authenticator app. You can also use a
              recovery code.
            </p>

            <form onSubmit={handleSubmit} className="flex flex-col gap-4">
              <div className="flex flex-col gap-1">
                <label className="text-xs font-medium" style={{ color: 'var(--text-secondary)' }}>
                  Code
                </label>
                <input
                  type="text"
                  inputMode="numeric"
                  placeholder="000000"
                  value={totp}
                  onChange={(e) => setTotp(e.target.value)}
                  required
                  autoFocus
                  autoComplete="one-time-code"
                  style={{
                    ...inputProps(focusedField === 'totp'),
                    letterSpacing: '0.15em',
                    textAlign: 'center',
                  }}
                  onFocus={() => setFocusedField('totp')}
                  onBlur={() => setFocusedField(null)}
                />
              </div>

              {error && (
                <p className="text-xs" style={{ color: 'var(--status-error)' }}>
                  {error}
                </p>
              )}

              <button
                type="submit"
                disabled={loading || totp.length < 6}
                className="w-full py-2 px-4 font-medium text-xs disabled:opacity-50"
                style={{
                  backgroundColor: 'var(--accent)',
                  color: 'var(--text-inverse)',
                  borderRadius: '3px',
                  border: 'none',
                  cursor: loading || totp.length < 6 ? 'not-allowed' : 'pointer',
                }}
              >
                {loading ? 'Verifying…' : 'Verify'}
              </button>

              <button
                type="button"
                onClick={() => { setStep('credentials'); setError(null); setTotp('') }}
                className="text-xs"
                style={{
                  background: 'transparent',
                  border: 'none',
                  color: 'var(--text-muted)',
                  cursor: 'pointer',
                  textAlign: 'center',
                  padding: 0,
                }}
              >
                Back to sign in
              </button>
            </form>
          </>
        )}
      </div>
    </div>
  )
}
