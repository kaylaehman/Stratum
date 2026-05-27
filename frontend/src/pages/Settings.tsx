import { useState } from 'react'
import { ShieldCheck, Copy, Loader, KeyRound } from 'lucide-react'
import { useAuthStore } from '../store/auth'
import {
  useTwoFAStatus,
  useTwoFASetup,
  useEnableTwoFA,
  useDisableTwoFA,
} from '../lib/api/twofa'
import { ApiError } from '../lib/api'
import type { TwoFASetupResponse } from '../types/api'

// ── helpers ──────────────────────────────────────────────────────────────────

function inputStyle(focused: boolean): React.CSSProperties {
  return {
    backgroundColor: 'var(--bg-elevated)',
    border: `1px solid ${focused ? 'var(--accent)' : 'var(--border-default)'}`,
    color: 'var(--text-primary)',
    borderRadius: '3px',
    outline: 'none',
    width: '100%',
    padding: '6px 10px',
    fontFamily: 'monospace',
    fontSize: '0.75rem',
  }
}

function CopyButton({ value }: { value: string }) {
  const [copied, setCopied] = useState(false)
  function copy() {
    void navigator.clipboard.writeText(value).then(() => {
      setCopied(true)
      setTimeout(() => setCopied(false), 1500)
    })
  }
  return (
    <button
      type="button"
      onClick={copy}
      title="Copy to clipboard"
      style={{
        background: 'transparent',
        border: 'none',
        cursor: 'pointer',
        color: copied ? 'var(--status-ok)' : 'var(--text-muted)',
        padding: '0 4px',
        lineHeight: 1,
      }}
    >
      <Copy size={13} />
    </button>
  )
}

// ── Setup (enrollment) sub-panel ─────────────────────────────────────────────

interface SetupPanelProps {
  setup: TwoFASetupResponse
  onCancel: () => void
  onDone: () => void
}

function SetupPanel({ setup, onCancel, onDone }: SetupPanelProps) {
  const [code, setCode] = useState('')
  const [focused, setFocused] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const enable = useEnableTwoFA()

  async function handleConfirm() {
    setError(null)
    try {
      await enable.mutateAsync({ code: code.trim() })
      onDone()
    } catch (err) {
      if (err instanceof ApiError && (err.body as { error?: string })?.error === 'invalid_code') {
        setError('Invalid code. Check your authenticator and try again.')
      } else {
        setError('Something went wrong. Please try again.')
      }
    }
  }

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: '16px' }}>
      <p className="text-xs" style={{ color: 'var(--text-secondary)' }}>
        Scan the URI below in any TOTP authenticator (e.g. Aegis, Authy, Google
        Authenticator) or enter the secret manually.
      </p>

      {/* Secret */}
      <div style={{ display: 'flex', flexDirection: 'column', gap: '4px' }}>
        <span className="text-xs font-medium" style={{ color: 'var(--text-secondary)' }}>
          Secret key
        </span>
        <div style={{ display: 'flex', alignItems: 'center', gap: '6px' }}>
          <code
            className="font-mono text-xs"
            style={{
              backgroundColor: 'var(--bg-elevated)',
              border: '1px solid var(--border-subtle)',
              borderRadius: '3px',
              padding: '4px 8px',
              color: 'var(--accent)',
              letterSpacing: '0.05em',
              wordBreak: 'break-all',
              flex: 1,
            }}
          >
            {setup.secret}
          </code>
          <CopyButton value={setup.secret} />
        </div>
      </div>

      {/* Provisioning URI */}
      <div style={{ display: 'flex', flexDirection: 'column', gap: '4px' }}>
        <span className="text-xs font-medium" style={{ color: 'var(--text-secondary)' }}>
          otpauth URI
        </span>
        <div style={{ display: 'flex', alignItems: 'flex-start', gap: '6px' }}>
          <code
            className="font-mono text-xs"
            style={{
              backgroundColor: 'var(--bg-elevated)',
              border: '1px solid var(--border-subtle)',
              borderRadius: '3px',
              padding: '4px 8px',
              color: 'var(--text-secondary)',
              wordBreak: 'break-all',
              flex: 1,
            }}
          >
            {setup.provisioning_uri}
          </code>
          <CopyButton value={setup.provisioning_uri} />
        </div>
      </div>

      {/* Recovery codes */}
      <div
        style={{
          backgroundColor: 'var(--bg-elevated)',
          border: '1px solid var(--status-warn)',
          borderRadius: '3px',
          padding: '12px',
        }}
      >
        <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: '8px' }}>
          <span className="text-xs font-semibold" style={{ color: 'var(--status-warn)' }}>
            Recovery codes — save these now, shown once
          </span>
          <CopyButton value={setup.recovery_codes.join('\n')} />
        </div>
        <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '4px' }}>
          {setup.recovery_codes.map((rc) => (
            <code
              key={rc}
              className="font-mono text-xs"
              style={{ color: 'var(--text-primary)', letterSpacing: '0.05em' }}
            >
              {rc}
            </code>
          ))}
        </div>
      </div>

      {/* Confirm step */}
      <div style={{ display: 'flex', flexDirection: 'column', gap: '6px' }}>
        <label className="text-xs font-medium" style={{ color: 'var(--text-secondary)' }}>
          Enter code from your authenticator to confirm
        </label>
        <input
          type="text"
          inputMode="numeric"
          maxLength={8}
          placeholder="6-digit code"
          value={code}
          onChange={(e) => setCode(e.target.value.replace(/\D/g, ''))}
          onFocus={() => setFocused(true)}
          onBlur={() => setFocused(false)}
          style={inputStyle(focused)}
        />
        {error && (
          <p className="text-xs" style={{ color: 'var(--status-error)' }}>
            {error}
          </p>
        )}
      </div>

      <div style={{ display: 'flex', gap: '8px' }}>
        <button
          type="button"
          onClick={handleConfirm}
          disabled={code.length < 6 || enable.isPending}
          style={{
            backgroundColor: 'var(--accent)',
            color: '#fff',
            border: 'none',
            borderRadius: '3px',
            padding: '6px 16px',
            fontSize: '0.75rem',
            fontWeight: 500,
            cursor: code.length < 6 || enable.isPending ? 'not-allowed' : 'pointer',
            opacity: code.length < 6 || enable.isPending ? 0.5 : 1,
            display: 'flex',
            alignItems: 'center',
            gap: '6px',
          }}
        >
          {enable.isPending && <Loader size={12} className="animate-spin" />}
          Confirm &amp; Enable
        </button>
        <button
          type="button"
          onClick={onCancel}
          style={{
            background: 'transparent',
            border: '1px solid var(--border-default)',
            borderRadius: '3px',
            color: 'var(--text-secondary)',
            padding: '6px 16px',
            fontSize: '0.75rem',
            cursor: 'pointer',
          }}
        >
          Cancel
        </button>
      </div>
    </div>
  )
}

// ── Disable sub-panel ─────────────────────────────────────────────────────────

interface DisablePanelProps {
  onCancel: () => void
  onDone: () => void
}

function DisablePanel({ onCancel, onDone }: DisablePanelProps) {
  const [code, setCode] = useState('')
  const [focused, setFocused] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const disable = useDisableTwoFA()

  async function handleDisable() {
    setError(null)
    try {
      await disable.mutateAsync({ code: code.trim() })
      onDone()
    } catch (err) {
      if (err instanceof ApiError && (err.body as { error?: string })?.error === 'invalid_code') {
        setError('Invalid code. Try again.')
      } else {
        setError('Something went wrong. Please try again.')
      }
    }
  }

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: '12px' }}>
      <p className="text-xs" style={{ color: 'var(--text-secondary)' }}>
        Enter your current TOTP code (or a recovery code) to disable two-factor
        authentication.
      </p>
      <div style={{ display: 'flex', flexDirection: 'column', gap: '6px' }}>
        <label className="text-xs font-medium" style={{ color: 'var(--text-secondary)' }}>
          Code
        </label>
        <input
          type="text"
          inputMode="numeric"
          placeholder="6-digit code or recovery code"
          value={code}
          onChange={(e) => setCode(e.target.value)}
          onFocus={() => setFocused(true)}
          onBlur={() => setFocused(false)}
          style={inputStyle(focused)}
        />
        {error && (
          <p className="text-xs" style={{ color: 'var(--status-error)' }}>
            {error}
          </p>
        )}
      </div>
      <div style={{ display: 'flex', gap: '8px' }}>
        <button
          type="button"
          onClick={handleDisable}
          disabled={code.length < 6 || disable.isPending}
          style={{
            backgroundColor: 'var(--status-error)',
            color: '#fff',
            border: 'none',
            borderRadius: '3px',
            padding: '6px 16px',
            fontSize: '0.75rem',
            fontWeight: 500,
            cursor: code.length < 6 || disable.isPending ? 'not-allowed' : 'pointer',
            opacity: code.length < 6 || disable.isPending ? 0.5 : 1,
            display: 'flex',
            alignItems: 'center',
            gap: '6px',
          }}
        >
          {disable.isPending && <Loader size={12} className="animate-spin" />}
          Disable 2FA
        </button>
        <button
          type="button"
          onClick={onCancel}
          style={{
            background: 'transparent',
            border: '1px solid var(--border-default)',
            borderRadius: '3px',
            color: 'var(--text-secondary)',
            padding: '6px 16px',
            fontSize: '0.75rem',
            cursor: 'pointer',
          }}
        >
          Cancel
        </button>
      </div>
    </div>
  )
}

// ── TwoFactor section ─────────────────────────────────────────────────────────

type TwoFAView = 'idle' | 'setup' | 'disable'

function TwoFactorSection() {
  const { data: status, isLoading } = useTwoFAStatus()
  const setup = useTwoFASetup()
  const [view, setView] = useState<TwoFAView>('idle')
  const [setupData, setSetupData] = useState<TwoFASetupResponse | null>(null)

  async function handleStartSetup() {
    const data = await setup.mutateAsync()
    setSetupData(data)
    setView('setup')
  }

  const enabled = status?.enabled ?? false

  return (
    <section
      style={{
        backgroundColor: 'var(--bg-surface)',
        border: '1px solid var(--border-default)',
        borderRadius: '3px',
        padding: '20px',
      }}
    >
      {/* Header */}
      <div style={{ display: 'flex', alignItems: 'center', gap: '10px', marginBottom: '16px' }}>
        <ShieldCheck size={16} style={{ color: enabled ? 'var(--status-ok)' : 'var(--text-muted)' }} />
        <div>
          <h2 className="text-sm font-semibold" style={{ color: 'var(--text-primary)', margin: 0 }}>
            Two-Factor Authentication
          </h2>
          <p className="text-xs" style={{ color: 'var(--text-muted)', margin: 0 }}>
            TOTP-based second factor for your account
          </p>
        </div>
        <span
          style={{
            marginLeft: 'auto',
            fontSize: '0.65rem',
            fontWeight: 600,
            padding: '2px 8px',
            borderRadius: '3px',
            backgroundColor: enabled ? 'color-mix(in srgb, var(--status-ok) 15%, transparent)' : 'var(--bg-elevated)',
            color: enabled ? 'var(--status-ok)' : 'var(--text-muted)',
            border: `1px solid ${enabled ? 'var(--status-ok)' : 'var(--border-subtle)'}`,
            letterSpacing: '0.05em',
            textTransform: 'uppercase',
          }}
        >
          {isLoading ? '...' : enabled ? 'Enabled' : 'Disabled'}
        </span>
      </div>

      {/* Body */}
      {isLoading ? (
        <div style={{ display: 'flex', alignItems: 'center', gap: '8px', color: 'var(--text-muted)' }}>
          <Loader size={14} className="animate-spin" />
          <span className="text-xs">Loading…</span>
        </div>
      ) : view === 'idle' && !enabled ? (
        <div style={{ display: 'flex', flexDirection: 'column', gap: '12px' }}>
          <p className="text-xs" style={{ color: 'var(--text-secondary)' }}>
            Two-factor authentication is currently <strong>off</strong>. Enable it to
            require a TOTP code on every login.
          </p>
          <button
            type="button"
            onClick={handleStartSetup}
            disabled={setup.isPending}
            style={{
              backgroundColor: 'var(--accent)',
              color: '#fff',
              border: 'none',
              borderRadius: '3px',
              padding: '6px 16px',
              fontSize: '0.75rem',
              fontWeight: 500,
              cursor: setup.isPending ? 'not-allowed' : 'pointer',
              opacity: setup.isPending ? 0.6 : 1,
              alignSelf: 'flex-start',
              display: 'flex',
              alignItems: 'center',
              gap: '6px',
            }}
          >
            {setup.isPending && <Loader size={12} className="animate-spin" />}
            <KeyRound size={13} />
            Enable 2FA
          </button>
        </div>
      ) : view === 'setup' && setupData ? (
        <SetupPanel
          setup={setupData}
          onCancel={() => { setView('idle'); setSetupData(null) }}
          onDone={() => { setView('idle'); setSetupData(null) }}
        />
      ) : view === 'idle' && enabled ? (
        <div style={{ display: 'flex', flexDirection: 'column', gap: '12px' }}>
          <p className="text-xs" style={{ color: 'var(--text-secondary)' }}>
            Two-factor authentication is <strong>on</strong>. You will be prompted for a
            TOTP code on every login.
          </p>
          <button
            type="button"
            onClick={() => setView('disable')}
            style={{
              background: 'transparent',
              border: '1px solid var(--status-error)',
              borderRadius: '3px',
              color: 'var(--status-error)',
              padding: '6px 16px',
              fontSize: '0.75rem',
              fontWeight: 500,
              cursor: 'pointer',
              alignSelf: 'flex-start',
            }}
          >
            Disable 2FA
          </button>
        </div>
      ) : view === 'disable' ? (
        <DisablePanel
          onCancel={() => setView('idle')}
          onDone={() => setView('idle')}
        />
      ) : null}
    </section>
  )
}

// ── Page ──────────────────────────────────────────────────────────────────────

export default function Settings() {
  const user = useAuthStore((s) => s.user)

  return (
    <div
      className="flex-1 overflow-auto p-6"
      style={{ backgroundColor: 'var(--bg-base)' }}
    >
      <div style={{ maxWidth: '640px', display: 'flex', flexDirection: 'column', gap: '24px' }}>
        {/* Page header */}
        <div>
          <h1 className="text-base font-semibold" style={{ color: 'var(--text-primary)', margin: 0 }}>
            Settings
          </h1>
          {user && (
            <p className="text-xs" style={{ color: 'var(--text-muted)', marginTop: '2px' }}>
              Signed in as <span style={{ color: 'var(--text-secondary)' }}>{user.username}</span>
            </p>
          )}
        </div>

        <TwoFactorSection />
      </div>
    </div>
  )
}
