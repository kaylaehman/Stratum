import { useState, Fragment } from 'react'
import {
  ShieldCheck,
  Copy,
  Loader,
  KeyRound,
  Users,
  Trash2,
  Shield,
  Monitor,
  LogOut,
  Flag,
  ToggleLeft,
  ToggleRight,
  MessageSquare,
  Plus,
  X,
  Save,
  Pencil,
} from 'lucide-react'
import { useAuthStore } from '../store/auth'
import {
  useTwoFAStatus,
  useTwoFASetup,
  useEnableTwoFA,
  useDisableTwoFA,
} from '../lib/api/twofa'
import {
  useUsers,
  useCreateUser,
  useUpdateUserRole,
  useDeleteUser,
  useSessions,
  useRevokeSession,
  useChangePassword,
  useUpdateUser,
} from '../lib/api/users'
import { useCan } from '../lib/roles'
import { ApiError } from '../lib/api'
import type { TwoFASetupResponse, UserRole, User } from '../types/api'
import { AISettingsSection } from '../components/ai/AISettingsSection'
import { MemoryPanel } from '../components/ai/MemoryPanel'
import { RunbooksSection } from '../components/ai/RunbooksSection'
import { useFeatures, useSetFeature, useFeatureEnabled } from '../lib/api/features'
import { useChatConfig, useSetChatConfig } from '../lib/api/chat'
import { AppShell } from '../components/layout/AppShell'

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

// ── Role badge ────────────────────────────────────────────────────────────────

function RoleBadge({ role }: { role: UserRole }) {
  const color =
    role === 'admin'
      ? 'var(--accent)'
      : role === 'operator'
        ? 'var(--status-warn)'
        : 'var(--text-muted)'
  return (
    <span
      className="font-mono text-xs"
      style={{
        padding: '1px 6px',
        borderRadius: '3px',
        border: `1px solid ${color}`,
        color,
        backgroundColor: `color-mix(in srgb, ${color} 12%, transparent)`,
        letterSpacing: '0.05em',
        textTransform: 'uppercase',
        fontSize: '0.65rem',
        fontWeight: 600,
      }}
    >
      {role}
    </span>
  )
}

// ── Users section (admin-only) ────────────────────────────────────────────────

function errorMsg(err: unknown, field: 'username' | 'password' | 'role' | 'delete' | 'edit' | 'changepw'): string | null {
  if (!(err instanceof ApiError)) return null
  const code = (err.body as { error?: string })?.error
  if (field === 'username' && code === 'username_taken') return 'Username already taken.'
  if (field === 'username' && code === 'username_required_password_min_8') return 'Username required and password must be 8+ chars.'
  if (field === 'password' && code === 'username_required_password_min_8') return 'Password must be at least 8 characters.'
  if (field === 'role' && code === 'invalid_role') return 'Invalid role.'
  if (field === 'role' && code === 'last_admin') return "Can't demote the only admin."
  if (field === 'delete' && code === 'last_admin') return "Can't delete the only admin."
  if (field === 'delete' && code === 'cannot_delete_self') return "Can't delete your own account."
  if (field === 'delete' && code === 'not_found') return 'User not found.'
  if (field === 'edit' && code === 'username_taken') return 'Username already taken.'
  if (field === 'edit' && code === 'username_required') return 'Username cannot be empty.'
  if (field === 'edit' && code === 'password_min_8') return 'Password must be at least 8 characters.'
  if (field === 'edit' && code === 'not_found') return 'User not found.'
  if (field === 'changepw' && code === 'current_password_incorrect') return 'Current password is incorrect.'
  if (field === 'changepw' && code === 'password_min_8') return 'New password must be at least 8 characters.'
  return 'Something went wrong.'
}

function CreateUserForm({ onDone }: { onDone: () => void }) {
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [email, setEmail] = useState('')
  const [role, setRole] = useState<UserRole>('viewer')
  const [focusedField, setFocusedField] = useState<string | null>(null)
  const create = useCreateUser()

  const inp = (focused: boolean): React.CSSProperties => ({
    backgroundColor: 'var(--bg-elevated)',
    border: `1px solid ${focused ? 'var(--accent)' : 'var(--border-default)'}`,
    color: 'var(--text-primary)',
    borderRadius: '3px',
    outline: 'none',
    width: '100%',
    padding: '5px 8px',
    fontFamily: 'monospace',
    fontSize: '0.75rem',
  })

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    try {
      await create.mutateAsync({ username, password, email: email || undefined, role })
      onDone()
    } catch {
      // error displayed below
    }
  }

  const usernameErr = create.isError ? errorMsg(create.error, 'username') : null
  const passwordErr = create.isError ? errorMsg(create.error, 'password') : null

  return (
    <form
      onSubmit={handleSubmit}
      style={{
        display: 'flex',
        flexDirection: 'column',
        gap: '10px',
        backgroundColor: 'var(--bg-elevated)',
        border: '1px solid var(--border-default)',
        borderRadius: '3px',
        padding: '14px',
      }}
    >
      <span className="text-xs font-semibold" style={{ color: 'var(--text-primary)' }}>
        New User
      </span>
      <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '8px' }}>
        <div style={{ display: 'flex', flexDirection: 'column', gap: '3px' }}>
          <label className="text-xs" style={{ color: 'var(--text-secondary)' }}>Username *</label>
          <input
            required
            value={username}
            onChange={e => setUsername(e.target.value)}
            onFocus={() => setFocusedField('username')}
            onBlur={() => setFocusedField(null)}
            style={inp(focusedField === 'username')}
            autoComplete="off"
          />
          {usernameErr && <span className="text-xs" style={{ color: 'var(--status-error)' }}>{usernameErr}</span>}
        </div>
        <div style={{ display: 'flex', flexDirection: 'column', gap: '3px' }}>
          <label className="text-xs" style={{ color: 'var(--text-secondary)' }}>Password * (min 8)</label>
          <input
            required
            type="password"
            value={password}
            onChange={e => setPassword(e.target.value)}
            onFocus={() => setFocusedField('password')}
            onBlur={() => setFocusedField(null)}
            style={inp(focusedField === 'password')}
            autoComplete="new-password"
          />
          {passwordErr && <span className="text-xs" style={{ color: 'var(--status-error)' }}>{passwordErr}</span>}
        </div>
        <div style={{ display: 'flex', flexDirection: 'column', gap: '3px' }}>
          <label className="text-xs" style={{ color: 'var(--text-secondary)' }}>Email (optional)</label>
          <input
            type="email"
            value={email}
            onChange={e => setEmail(e.target.value)}
            onFocus={() => setFocusedField('email')}
            onBlur={() => setFocusedField(null)}
            style={inp(focusedField === 'email')}
          />
        </div>
        <div style={{ display: 'flex', flexDirection: 'column', gap: '3px' }}>
          <label className="text-xs" style={{ color: 'var(--text-secondary)' }}>Role</label>
          <select
            value={role}
            onChange={e => setRole(e.target.value as UserRole)}
            style={{
              ...inp(false),
              cursor: 'pointer',
            }}
          >
            <option value="viewer">viewer</option>
            <option value="operator">operator</option>
            <option value="admin">admin</option>
          </select>
        </div>
      </div>
      <div style={{ display: 'flex', gap: '8px', justifyContent: 'flex-end' }}>
        <button
          type="button"
          onClick={onDone}
          style={{
            background: 'transparent',
            border: '1px solid var(--border-default)',
            borderRadius: '3px',
            color: 'var(--text-secondary)',
            padding: '5px 14px',
            fontSize: '0.75rem',
            cursor: 'pointer',
          }}
        >
          Cancel
        </button>
        <button
          type="submit"
          disabled={create.isPending}
          style={{
            backgroundColor: 'var(--accent)',
            color: '#fff',
            border: 'none',
            borderRadius: '3px',
            padding: '5px 14px',
            fontSize: '0.75rem',
            fontWeight: 500,
            cursor: create.isPending ? 'not-allowed' : 'pointer',
            opacity: create.isPending ? 0.6 : 1,
            display: 'flex',
            alignItems: 'center',
            gap: '6px',
          }}
        >
          {create.isPending && <Loader size={12} className="animate-spin" />}
          Create
        </button>
      </div>
    </form>
  )
}

// EditUserForm lets an admin edit a user's username/email and optionally reset
// their password. Only changed fields are sent. Leaving the password blank
// keeps the existing password.
function EditUserForm({ user, onDone }: { user: User; onDone: () => void }) {
  const updateUser = useUpdateUser()
  const [username, setUsername] = useState(user.username)
  const [email, setEmail] = useState(user.email ?? '')
  const [password, setPassword] = useState('')
  const [err, setErr] = useState('')

  const input: React.CSSProperties = {
    background: 'var(--bg-elevated)',
    border: '1px solid var(--border-default)',
    color: 'var(--text-primary)',
    borderRadius: '3px',
    fontSize: '11px',
    fontFamily: 'monospace',
    padding: '4px 8px',
    width: '100%',
  }

  async function save() {
    setErr('')
    const payload: { id: string; username?: string; email?: string; password?: string } = { id: user.id }
    if (username.trim() !== user.username) payload.username = username.trim()
    if (email.trim() !== (user.email ?? '')) payload.email = email.trim()
    if (password) payload.password = password
    if (payload.username === undefined && payload.email === undefined && payload.password === undefined) {
      onDone()
      return
    }
    try {
      await updateUser.mutateAsync(payload)
      onDone()
    } catch (e) {
      setErr(errorMsg(e, 'edit') ?? 'Error')
    }
  }

  return (
    <div
      style={{
        display: 'flex',
        flexDirection: 'column',
        gap: 8,
        padding: '10px 8px',
        background: 'var(--bg-base)',
      }}
    >
      <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 8 }}>
        <label style={{ display: 'flex', flexDirection: 'column', gap: 3 }}>
          <span style={{ fontSize: 10, color: 'var(--text-muted)' }}>Username</span>
          <input style={input} value={username} onChange={(e) => setUsername(e.target.value)} />
        </label>
        <label style={{ display: 'flex', flexDirection: 'column', gap: 3 }}>
          <span style={{ fontSize: 10, color: 'var(--text-muted)' }}>Email</span>
          <input style={input} value={email} onChange={(e) => setEmail(e.target.value)} placeholder="(none)" />
        </label>
      </div>
      <label style={{ display: 'flex', flexDirection: 'column', gap: 3 }}>
        <span style={{ fontSize: 10, color: 'var(--text-muted)' }}>Reset password (leave blank to keep)</span>
        <input
          style={input}
          type="password"
          value={password}
          onChange={(e) => setPassword(e.target.value)}
          placeholder="new password (8+ chars)"
          autoComplete="new-password"
        />
      </label>
      {err && <span style={{ fontSize: 10, color: 'var(--status-error)', fontFamily: 'monospace' }}>{err}</span>}
      <div style={{ display: 'flex', gap: 8 }}>
        <button
          type="button"
          disabled={updateUser.isPending}
          onClick={() => void save()}
          style={{
            display: 'flex', alignItems: 'center', gap: 4,
            background: 'var(--accent)', color: '#fff', border: 'none',
            borderRadius: '3px', fontSize: 11, padding: '4px 12px', cursor: 'pointer',
          }}
        >
          <Save size={11} />
          {updateUser.isPending ? 'Saving…' : 'Save'}
        </button>
        <button
          type="button"
          onClick={onDone}
          style={{
            background: 'transparent', color: 'var(--text-secondary)',
            border: '1px solid var(--border-default)', borderRadius: '3px',
            fontSize: 11, padding: '4px 12px', cursor: 'pointer',
          }}
        >
          Cancel
        </button>
      </div>
    </div>
  )
}

function UsersSection() {
  const { data, isLoading } = useUsers()
  const updateRole = useUpdateUserRole()
  const deleteUser = useDeleteUser()
  const currentUser = useAuthStore((s) => s.user)
  const [showCreate, setShowCreate] = useState(false)
  const [editingId, setEditingId] = useState<string | null>(null)
  const [deleteErrors, setDeleteErrors] = useState<Record<string, string>>({})
  const [roleErrors, setRoleErrors] = useState<Record<string, string>>({})

  const users = data?.users ?? []

  const cell: React.CSSProperties = {
    padding: '7px 8px',
    fontSize: 11,
    fontFamily: 'monospace',
    color: 'var(--text-secondary)',
    verticalAlign: 'middle',
  }

  const headerCell: React.CSSProperties = {
    fontSize: 10,
    fontFamily: 'monospace',
    textTransform: 'uppercase',
    letterSpacing: '0.08em',
    color: 'var(--text-muted)',
    padding: '6px 8px',
    fontWeight: 600,
    textAlign: 'left',
  }

  async function handleRoleChange(id: string, role: UserRole) {
    setRoleErrors(prev => ({ ...prev, [id]: '' }))
    try {
      await updateRole.mutateAsync({ id, role })
    } catch (err) {
      const msg = errorMsg(err, 'role') ?? 'Error'
      setRoleErrors(prev => ({ ...prev, [id]: msg }))
    }
  }

  async function handleDelete(id: string) {
    setDeleteErrors(prev => ({ ...prev, [id]: '' }))
    try {
      await deleteUser.mutateAsync(id)
    } catch (err) {
      const msg = errorMsg(err, 'delete') ?? 'Error'
      setDeleteErrors(prev => ({ ...prev, [id]: msg }))
    }
  }

  return (
    <section
      style={{
        backgroundColor: 'var(--bg-surface)',
        border: '1px solid var(--border-default)',
        borderRadius: '3px',
        padding: '20px',
      }}
    >
      <div style={{ display: 'flex', alignItems: 'center', gap: '10px', marginBottom: '16px' }}>
        <Users size={16} style={{ color: 'var(--accent)' }} />
        <div style={{ flex: 1 }}>
          <h2 className="text-sm font-semibold" style={{ color: 'var(--text-primary)', margin: 0 }}>
            User Management
          </h2>
          <p className="text-xs" style={{ color: 'var(--text-muted)', margin: 0 }}>
            Manage accounts and roles (admin only)
          </p>
        </div>
        <button
          type="button"
          onClick={() => setShowCreate(v => !v)}
          style={{
            backgroundColor: showCreate ? 'transparent' : 'var(--accent)',
            color: showCreate ? 'var(--text-secondary)' : '#fff',
            border: showCreate ? '1px solid var(--border-default)' : 'none',
            borderRadius: '3px',
            padding: '4px 12px',
            fontSize: '0.7rem',
            fontWeight: 500,
            cursor: 'pointer',
          }}
        >
          {showCreate ? 'Cancel' : '+ New User'}
        </button>
      </div>

      {showCreate && (
        <div style={{ marginBottom: '14px' }}>
          <CreateUserForm onDone={() => setShowCreate(false)} />
        </div>
      )}

      {isLoading ? (
        <div style={{ display: 'flex', alignItems: 'center', gap: '8px', color: 'var(--text-muted)' }}>
          <Loader size={14} className="animate-spin" />
          <span className="text-xs">Loading users…</span>
        </div>
      ) : (
        <div style={{ overflowX: 'auto', border: '1px solid var(--border-subtle)', borderRadius: '3px' }}>
          <table style={{ width: '100%', borderCollapse: 'collapse' }}>
            <thead>
              <tr style={{ borderBottom: '1px solid var(--border-subtle)' }}>
                <th style={headerCell}>Username</th>
                <th style={headerCell}>Role</th>
                <th style={headerCell}>Created</th>
                <th style={{ ...headerCell, textAlign: 'right' }}>Actions</th>
              </tr>
            </thead>
            <tbody>
              {users.map(u => {
                const isSelf = u.id === currentUser?.id
                return (
                  <Fragment key={u.id}>
                  <tr
                    style={{ borderBottom: '1px solid var(--border-subtle)' }}
                  >
                    <td style={{ ...cell, color: 'var(--text-primary)', fontWeight: isSelf ? 600 : 400 }}>
                      {u.username}
                      {isSelf && (
                        <span style={{ marginLeft: 6, fontSize: 9, color: 'var(--text-muted)', textTransform: 'uppercase', letterSpacing: '0.06em' }}>
                          (you)
                        </span>
                      )}
                    </td>
                    <td style={cell}>
                      <div style={{ display: 'flex', flexDirection: 'column', gap: 3 }}>
                        <select
                          value={u.role}
                          onChange={e => void handleRoleChange(u.id, e.target.value as UserRole)}
                          style={{
                            background: 'var(--bg-elevated)',
                            border: '1px solid var(--border-default)',
                            color: 'var(--text-primary)',
                            borderRadius: '3px',
                            fontSize: '11px',
                            fontFamily: 'monospace',
                            padding: '2px 6px',
                            cursor: 'pointer',
                          }}
                        >
                          <option value="viewer">viewer</option>
                          <option value="operator">operator</option>
                          <option value="admin">admin</option>
                        </select>
                        {roleErrors[u.id] && (
                          <span style={{ fontSize: 10, color: 'var(--status-error)', fontFamily: 'monospace' }}>
                            {roleErrors[u.id]}
                          </span>
                        )}
                      </div>
                    </td>
                    <td style={{ ...cell, color: 'var(--text-muted)' }}>
                      {u.created_at
                        ? new Date(u.created_at).toLocaleDateString()
                        : '—'}
                    </td>
                    <td style={{ ...cell, textAlign: 'right' }}>
                      <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'flex-end', gap: 3 }}>
                        <div style={{ display: 'flex', gap: 6 }}>
                        <button
                          type="button"
                          onClick={() => setEditingId(editingId === u.id ? null : u.id)}
                          title={`Edit ${u.username}`}
                          style={{
                            display: 'flex', alignItems: 'center', gap: 4,
                            background: editingId === u.id ? 'var(--accent-glow)' : 'transparent',
                            border: '1px solid var(--border-default)', borderRadius: '3px',
                            color: 'var(--text-secondary)', fontSize: '11px', fontFamily: 'monospace',
                            padding: '2px 8px', cursor: 'pointer',
                          }}
                        >
                          <Pencil size={11} />
                          Edit
                        </button>
                        <button
                          type="button"
                          disabled={isSelf || deleteUser.isPending}
                          onClick={() => void handleDelete(u.id)}
                          title={isSelf ? 'Cannot delete your own account' : `Delete ${u.username}`}
                          style={{
                            display: 'flex',
                            alignItems: 'center',
                            gap: 4,
                            background: 'transparent',
                            border: '1px solid var(--border-default)',
                            borderRadius: '3px',
                            color: isSelf ? 'var(--text-muted)' : 'var(--status-error)',
                            fontSize: '11px',
                            fontFamily: 'monospace',
                            padding: '2px 8px',
                            cursor: isSelf ? 'not-allowed' : 'pointer',
                            opacity: isSelf ? 0.4 : 1,
                          }}
                        >
                          <Trash2 size={11} />
                          Delete
                        </button>
                        </div>
                        {deleteErrors[u.id] && (
                          <span style={{ fontSize: 10, color: 'var(--status-error)', fontFamily: 'monospace' }}>
                            {deleteErrors[u.id]}
                          </span>
                        )}
                      </div>
                    </td>
                  </tr>
                  {editingId === u.id && (
                    <tr>
                      <td colSpan={4} style={{ padding: 0, borderBottom: '1px solid var(--border-subtle)' }}>
                        <EditUserForm user={u} onDone={() => setEditingId(null)} />
                      </td>
                    </tr>
                  )}
                  </Fragment>
                )
              })}
            </tbody>
          </table>
        </div>
      )}
    </section>
  )
}

// ── Sessions section (any auth user) ─────────────────────────────────────────

// ChangePasswordSection lets the signed-in user change their own password.
// Available to every role.
function ChangePasswordSection() {
  const changePw = useChangePassword()
  const [current, setCurrent] = useState('')
  const [next, setNext] = useState('')
  const [confirm, setConfirm] = useState('')
  const [err, setErr] = useState('')
  const [done, setDone] = useState(false)

  const input: React.CSSProperties = {
    background: 'var(--bg-elevated)',
    border: '1px solid var(--border-default)',
    color: 'var(--text-primary)',
    borderRadius: '3px',
    fontSize: '12px',
    fontFamily: 'monospace',
    padding: '6px 8px',
    width: '100%',
    maxWidth: 320,
  }

  async function submit(e: React.FormEvent) {
    e.preventDefault()
    setErr('')
    setDone(false)
    if (next.length < 8) {
      setErr('New password must be at least 8 characters.')
      return
    }
    if (next !== confirm) {
      setErr('New passwords do not match.')
      return
    }
    try {
      await changePw.mutateAsync({ current_password: current, new_password: next })
      setDone(true)
      setCurrent('')
      setNext('')
      setConfirm('')
    } catch (e2) {
      setErr(errorMsg(e2, 'changepw') ?? 'Error')
    }
  }

  return (
    <section
      style={{
        backgroundColor: 'var(--bg-surface)',
        border: '1px solid var(--border-default)',
        borderRadius: '3px',
        padding: '20px',
      }}
    >
      <div style={{ display: 'flex', alignItems: 'center', gap: '10px', marginBottom: '16px' }}>
        <KeyRound size={16} style={{ color: 'var(--accent)' }} />
        <div>
          <h2 className="text-sm font-semibold" style={{ color: 'var(--text-primary)', margin: 0 }}>
            Change Password
          </h2>
          <p className="text-xs" style={{ color: 'var(--text-muted)', margin: 0 }}>
            Update the password for your own account
          </p>
        </div>
      </div>

      <form onSubmit={(e) => void submit(e)} style={{ display: 'flex', flexDirection: 'column', gap: 10 }}>
        <label style={{ display: 'flex', flexDirection: 'column', gap: 3 }}>
          <span style={{ fontSize: 11, color: 'var(--text-secondary)' }}>Current password</span>
          <input style={input} type="password" autoComplete="current-password" value={current} onChange={(e) => setCurrent(e.target.value)} required />
        </label>
        <label style={{ display: 'flex', flexDirection: 'column', gap: 3 }}>
          <span style={{ fontSize: 11, color: 'var(--text-secondary)' }}>New password</span>
          <input style={input} type="password" autoComplete="new-password" value={next} onChange={(e) => setNext(e.target.value)} required />
        </label>
        <label style={{ display: 'flex', flexDirection: 'column', gap: 3 }}>
          <span style={{ fontSize: 11, color: 'var(--text-secondary)' }}>Confirm new password</span>
          <input style={input} type="password" autoComplete="new-password" value={confirm} onChange={(e) => setConfirm(e.target.value)} required />
        </label>
        {err && <span style={{ fontSize: 11, color: 'var(--status-error)', fontFamily: 'monospace' }}>{err}</span>}
        {done && <span style={{ fontSize: 11, color: 'var(--status-ok)', fontFamily: 'monospace' }}>Password updated.</span>}
        <button
          type="submit"
          disabled={changePw.isPending}
          style={{
            display: 'flex', alignItems: 'center', gap: 6, alignSelf: 'flex-start',
            background: 'var(--accent)', color: '#fff', border: 'none',
            borderRadius: '3px', fontSize: 12, padding: '6px 14px', cursor: 'pointer',
            opacity: changePw.isPending ? 0.6 : 1,
          }}
        >
          <Save size={12} />
          {changePw.isPending ? 'Updating…' : 'Update password'}
        </button>
      </form>
    </section>
  )
}

function SessionsSection() {
  const { data, isLoading } = useSessions()
  const revoke = useRevokeSession()
  const [revokeErrors, setRevokeErrors] = useState<Record<string, string>>({})

  const sessions = data?.sessions ?? []

  async function handleRevoke(id: string) {
    setRevokeErrors(prev => ({ ...prev, [id]: '' }))
    try {
      await revoke.mutateAsync(id)
    } catch {
      setRevokeErrors(prev => ({ ...prev, [id]: 'Failed to revoke.' }))
    }
  }

  return (
    <section
      style={{
        backgroundColor: 'var(--bg-surface)',
        border: '1px solid var(--border-default)',
        borderRadius: '3px',
        padding: '20px',
      }}
    >
      <div style={{ display: 'flex', alignItems: 'center', gap: '10px', marginBottom: '16px' }}>
        <Monitor size={16} style={{ color: 'var(--text-muted)' }} />
        <div>
          <h2 className="text-sm font-semibold" style={{ color: 'var(--text-primary)', margin: 0 }}>
            Active Sessions
          </h2>
          <p className="text-xs" style={{ color: 'var(--text-muted)', margin: 0 }}>
            Your signed-in devices
          </p>
        </div>
      </div>

      {isLoading ? (
        <div style={{ display: 'flex', alignItems: 'center', gap: '8px', color: 'var(--text-muted)' }}>
          <Loader size={14} className="animate-spin" />
          <span className="text-xs">Loading sessions…</span>
        </div>
      ) : sessions.length === 0 ? (
        <p className="text-xs" style={{ color: 'var(--text-muted)' }}>No sessions found.</p>
      ) : (
        <div style={{ display: 'flex', flexDirection: 'column', gap: '8px' }}>
          {sessions.map(s => (
            <div
              key={s.id}
              style={{
                backgroundColor: 'var(--bg-elevated)',
                border: `1px solid ${s.current ? 'var(--accent)' : 'var(--border-subtle)'}`,
                borderRadius: '3px',
                padding: '10px 12px',
                display: 'flex',
                alignItems: 'center',
                gap: '10px',
              }}
            >
              <Shield
                size={14}
                style={{ color: s.current ? 'var(--accent)' : 'var(--text-muted)', flexShrink: 0 }}
              />
              <div style={{ flex: 1, minWidth: 0 }}>
                <div style={{ display: 'flex', alignItems: 'center', gap: '6px', marginBottom: 2 }}>
                  <span className="font-mono text-xs" style={{ color: 'var(--text-primary)', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                    {s.user_agent ?? 'Unknown client'}
                  </span>
                  {s.current && (
                    <span
                      style={{
                        fontSize: '0.6rem',
                        fontWeight: 600,
                        textTransform: 'uppercase',
                        letterSpacing: '0.06em',
                        color: 'var(--accent)',
                        border: '1px solid var(--accent)',
                        borderRadius: '3px',
                        padding: '0 4px',
                        flexShrink: 0,
                      }}
                    >
                      This device
                    </span>
                  )}
                  <span
                    style={{
                      fontSize: '0.6rem',
                      fontWeight: 600,
                      textTransform: 'uppercase',
                      letterSpacing: '0.06em',
                      color: s.active ? 'var(--status-ok)' : 'var(--text-muted)',
                      border: `1px solid ${s.active ? 'var(--status-ok)' : 'var(--border-subtle)'}`,
                      borderRadius: '3px',
                      padding: '0 4px',
                      flexShrink: 0,
                    }}
                  >
                    {s.active ? 'Active' : 'Expired'}
                  </span>
                </div>
                <div className="font-mono text-xs" style={{ color: 'var(--text-muted)' }}>
                  {s.ip && <span>{s.ip} · </span>}
                  Created {new Date(s.created_at).toLocaleString()}
                </div>
              </div>
              {!s.current && s.active && (
                <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'flex-end', gap: 3 }}>
                  <button
                    type="button"
                    disabled={revoke.isPending}
                    onClick={() => void handleRevoke(s.id)}
                    style={{
                      display: 'flex',
                      alignItems: 'center',
                      gap: 4,
                      background: 'transparent',
                      border: '1px solid var(--border-default)',
                      borderRadius: '3px',
                      color: 'var(--status-warn)',
                      fontSize: '11px',
                      fontFamily: 'monospace',
                      padding: '3px 10px',
                      cursor: revoke.isPending ? 'not-allowed' : 'pointer',
                    }}
                  >
                    <LogOut size={11} />
                    Revoke
                  </button>
                  {revokeErrors[s.id] && (
                    <span style={{ fontSize: 10, color: 'var(--status-error)', fontFamily: 'monospace' }}>
                      {revokeErrors[s.id]}
                    </span>
                  )}
                </div>
              )}
            </div>
          ))}
        </div>
      )}
    </section>
  )
}

// ── Features section (admin-only toggle; read-only for others) ────────────────

function FeaturesSection() {
  const { isAdmin } = useCan()
  const { data, isLoading } = useFeatures()
  const setFeature = useSetFeature()

  const features = data?.features ?? []

  return (
    <section
      style={{
        backgroundColor: 'var(--bg-surface)',
        border: '1px solid var(--border-default)',
        borderRadius: '3px',
        padding: '20px',
      }}
    >
      <div style={{ display: 'flex', alignItems: 'center', gap: '10px', marginBottom: '16px' }}>
        <Flag size={16} style={{ color: 'var(--accent)' }} />
        <div>
          <h2 className="text-sm font-semibold" style={{ color: 'var(--text-primary)', margin: 0 }}>
            Feature Flags
          </h2>
          <p className="text-xs" style={{ color: 'var(--text-muted)', margin: 0 }}>
            {isAdmin ? 'Enable or disable platform features' : 'Feature states (admin-controlled)'}
          </p>
        </div>
      </div>

      {isLoading ? (
        <div style={{ display: 'flex', alignItems: 'center', gap: '8px', color: 'var(--text-muted)' }}>
          <Loader size={14} className="animate-spin" />
          <span className="text-xs">Loading features…</span>
        </div>
      ) : features.length === 0 ? (
        <p className="text-xs" style={{ color: 'var(--text-muted)' }}>No feature flags registered.</p>
      ) : (
        <div style={{ display: 'flex', flexDirection: 'column', gap: '1px', border: '1px solid var(--border-subtle)', borderRadius: '3px', overflow: 'hidden' }}>
          {features.map((flag, idx) => {
            const isOverridden = flag.enabled !== flag.default
            const isPending = setFeature.isPending && (setFeature.variables as { key: string } | undefined)?.key === flag.key
            return (
              <div
                key={flag.key}
                style={{
                  display: 'flex',
                  alignItems: 'center',
                  gap: '12px',
                  padding: '10px 12px',
                  backgroundColor: idx % 2 === 0 ? 'var(--bg-elevated)' : 'var(--bg-surface)',
                  borderBottom: idx < features.length - 1 ? '1px solid var(--border-subtle)' : 'none',
                }}
              >
                <button
                  type="button"
                  disabled={!isAdmin || isPending}
                  onClick={() => {
                    if (isAdmin) {
                      void setFeature.mutate({ key: flag.key, enabled: !flag.enabled })
                    }
                  }}
                  title={isAdmin ? (flag.enabled ? 'Disable' : 'Enable') : 'Admin only'}
                  style={{
                    background: 'transparent',
                    border: 'none',
                    padding: 0,
                    cursor: isAdmin ? 'pointer' : 'not-allowed',
                    color: flag.enabled ? 'var(--status-ok)' : 'var(--text-muted)',
                    opacity: isPending ? 0.5 : 1,
                    display: 'flex',
                    alignItems: 'center',
                    flexShrink: 0,
                  }}
                  aria-label={flag.enabled ? `Disable ${flag.label}` : `Enable ${flag.label}`}
                >
                  {isPending ? (
                    <Loader size={18} className="animate-spin" style={{ color: 'var(--accent)' }} />
                  ) : flag.enabled ? (
                    <ToggleRight size={20} />
                  ) : (
                    <ToggleLeft size={20} />
                  )}
                </button>

                <div style={{ flex: 1, minWidth: 0 }}>
                  <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
                    <span className="text-xs font-medium" style={{ color: 'var(--text-primary)' }}>
                      {flag.label}
                    </span>
                    <code
                      className="font-mono text-xs"
                      style={{
                        color: 'var(--text-muted)',
                        backgroundColor: 'var(--bg-surface)',
                        border: '1px solid var(--border-subtle)',
                        borderRadius: '3px',
                        padding: '0 4px',
                        fontSize: '0.65rem',
                        letterSpacing: '0.03em',
                      }}
                    >
                      {flag.key}
                    </code>
                    {isOverridden && (
                      <span
                        style={{
                          fontSize: '0.6rem',
                          fontWeight: 600,
                          textTransform: 'uppercase',
                          letterSpacing: '0.06em',
                          color: 'var(--accent)',
                          border: '1px solid var(--accent)',
                          borderRadius: '3px',
                          padding: '0 4px',
                          flexShrink: 0,
                          backgroundColor: 'color-mix(in srgb, var(--accent) 10%, transparent)',
                        }}
                      >
                        overridden
                      </span>
                    )}
                  </div>
                  {flag.description && (
                    <p
                      className="text-xs"
                      style={{ color: 'var(--text-muted)', margin: '2px 0 0', lineHeight: 1.4 }}
                    >
                      {flag.description}
                    </p>
                  )}
                </div>

                <span
                  style={{
                    fontSize: '0.65rem',
                    fontWeight: 600,
                    padding: '2px 8px',
                    borderRadius: '3px',
                    textTransform: 'uppercase',
                    letterSpacing: '0.05em',
                    flexShrink: 0,
                    backgroundColor: flag.enabled
                      ? 'color-mix(in srgb, var(--status-ok) 15%, transparent)'
                      : 'var(--bg-elevated)',
                    color: flag.enabled ? 'var(--status-ok)' : 'var(--text-muted)',
                    border: `1px solid ${flag.enabled ? 'var(--status-ok)' : 'var(--border-subtle)'}`,
                  }}
                >
                  {flag.enabled ? 'on' : 'off'}
                </span>
              </div>
            )
          })}
        </div>
      )}
    </section>
  )
}

// ── Chat Integration section (admin-only) ────────────────────────────────────

function ChatIntegrationSection() {
  const { data, isLoading } = useChatConfig()
  const setConfig = useSetChatConfig()
  const isEnabled = useFeatureEnabled('feature.chat_integration')

  // Token state: 'keep' | 'replace' | 'clear'
  const [tokenMode, setTokenMode] = useState<'keep' | 'replace' | 'clear'>('keep')
  const [tokenValue, setTokenValue] = useState('')
  const [tokenFocused, setTokenFocused] = useState(false)

  // Allowed chat IDs
  const [chats, setChats] = useState<number[]>([])
  const [chatInput, setChatInput] = useState('')
  const [chatInputFocused, setChatInputFocused] = useState(false)

  // Sync chats from fetched data (only on first load)
  const [synced, setSynced] = useState(false)
  if (data && !synced) {
    setChats(data.allowed_chats)
    setSynced(true)
  }

  const [status, setStatus] = useState<'idle' | 'ok' | 'error'>('idle')
  const [errorMsg, setErrorMsg] = useState('')

  function addChat() {
    const n = parseInt(chatInput.trim(), 10)
    if (!Number.isFinite(n)) return
    if (chats.includes(n)) { setChatInput(''); return }
    setChats(prev => [...prev, n])
    setChatInput('')
  }

  function removeChat(id: number) {
    setChats(prev => prev.filter(c => c !== id))
  }

  async function handleSave() {
    setStatus('idle')
    setErrorMsg('')
    const req: Parameters<typeof setConfig.mutateAsync>[0] = {
      allowed_chats: chats,
    }
    if (tokenMode === 'replace' && tokenValue.trim()) {
      req.token = tokenValue.trim()
    } else if (tokenMode === 'clear') {
      req.token = ''
    }
    // tokenMode === 'keep': omit token field entirely
    try {
      await setConfig.mutateAsync(req)
      setSynced(false) // re-sync on next data arrival
      setTokenMode('keep')
      setTokenValue('')
      setStatus('ok')
    } catch {
      setStatus('error')
      setErrorMsg('Save failed. Check the bot token and try again.')
    }
  }

  const hasToken = data?.has_token ?? false

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
        <MessageSquare size={16} style={{ color: 'var(--accent)' }} />
        <div style={{ flex: 1 }}>
          <h2 className="text-sm font-semibold" style={{ color: 'var(--text-primary)', margin: 0 }}>
            Chat Integration
          </h2>
          <p className="text-xs" style={{ color: 'var(--text-muted)', margin: 0 }}>
            Telegram bot for read-only commands (/status, /nodes, /help)
          </p>
        </div>
      </div>

      {/* Feature-flag hint */}
      {!isEnabled && (
        <div
          style={{
            backgroundColor: 'var(--bg-elevated)',
            border: '1px solid var(--border-subtle)',
            borderRadius: '3px',
            padding: '8px 12px',
            marginBottom: '16px',
            display: 'flex',
            alignItems: 'center',
            gap: '8px',
          }}
        >
          <span className="font-mono text-xs" style={{ color: 'var(--text-muted)' }}>
            Enable "Chat Integration" in Features above to activate the bot. You can still configure it here.
          </span>
        </div>
      )}

      {isLoading ? (
        <div style={{ display: 'flex', alignItems: 'center', gap: '8px', color: 'var(--text-muted)' }}>
          <Loader size={14} className="animate-spin" />
          <span className="text-xs">Loading…</span>
        </div>
      ) : (
        <div style={{ display: 'flex', flexDirection: 'column', gap: '18px' }}>

          {/* Help line */}
          <p className="font-mono text-xs" style={{ color: 'var(--text-muted)', margin: 0, lineHeight: 1.5 }}>
            Message your bot, then add the chat ID shown by @userinfobot (or the chat's numeric ID).
            Only listed chats can run commands: /status, /nodes, /help.
          </p>

          {/* Bot token field */}
          <div style={{ display: 'flex', flexDirection: 'column', gap: '6px' }}>
            <label className="text-xs font-medium" style={{ color: 'var(--text-secondary)' }}>
              Bot Token
            </label>

            {/* Status / affordances row */}
            {tokenMode === 'keep' && (
              <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
                <span
                  className="font-mono text-xs"
                  style={{
                    color: hasToken ? 'var(--status-ok)' : 'var(--text-muted)',
                    backgroundColor: 'var(--bg-elevated)',
                    border: `1px solid ${hasToken ? 'var(--status-ok)' : 'var(--border-subtle)'}`,
                    borderRadius: '3px',
                    padding: '4px 10px',
                    flex: 1,
                  }}
                >
                  {hasToken ? 'token set' : 'no token stored'}
                </span>
                <button
                  type="button"
                  onClick={() => setTokenMode('replace')}
                  style={{
                    background: 'transparent',
                    border: '1px solid var(--border-default)',
                    borderRadius: '3px',
                    color: 'var(--text-secondary)',
                    fontSize: '0.7rem',
                    fontFamily: 'monospace',
                    padding: '4px 10px',
                    cursor: 'pointer',
                    whiteSpace: 'nowrap',
                  }}
                >
                  {hasToken ? 'Replace' : 'Set token'}
                </button>
                {hasToken && (
                  <button
                    type="button"
                    onClick={() => setTokenMode('clear')}
                    style={{
                      background: 'transparent',
                      border: '1px solid var(--border-default)',
                      borderRadius: '3px',
                      color: 'var(--status-error)',
                      fontSize: '0.7rem',
                      fontFamily: 'monospace',
                      padding: '4px 10px',
                      cursor: 'pointer',
                      whiteSpace: 'nowrap',
                    }}
                  >
                    Clear
                  </button>
                )}
              </div>
            )}

            {tokenMode === 'clear' && (
              <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
                <span
                  className="font-mono text-xs"
                  style={{
                    color: 'var(--status-warn)',
                    backgroundColor: 'var(--bg-elevated)',
                    border: '1px solid var(--status-warn)',
                    borderRadius: '3px',
                    padding: '4px 10px',
                    flex: 1,
                  }}
                >
                  Token will be cleared on save
                </span>
                <button
                  type="button"
                  onClick={() => setTokenMode('keep')}
                  style={{
                    background: 'transparent',
                    border: '1px solid var(--border-default)',
                    borderRadius: '3px',
                    color: 'var(--text-secondary)',
                    fontSize: '0.7rem',
                    fontFamily: 'monospace',
                    padding: '4px 10px',
                    cursor: 'pointer',
                  }}
                >
                  Cancel
                </button>
              </div>
            )}

            {tokenMode === 'replace' && (
              <div style={{ display: 'flex', flexDirection: 'column', gap: '4px' }}>
                <div style={{ display: 'flex', gap: '6px' }}>
                  <input
                    type="password"
                    placeholder="Paste bot token from @BotFather"
                    value={tokenValue}
                    onChange={e => setTokenValue(e.target.value)}
                    onFocus={() => setTokenFocused(true)}
                    onBlur={() => setTokenFocused(false)}
                    autoComplete="off"
                    style={{
                      backgroundColor: 'var(--bg-elevated)',
                      border: `1px solid ${tokenFocused ? 'var(--accent)' : 'var(--border-default)'}`,
                      color: 'var(--text-primary)',
                      borderRadius: '3px',
                      outline: 'none',
                      flex: 1,
                      padding: '6px 10px',
                      fontFamily: 'monospace',
                      fontSize: '0.75rem',
                    }}
                  />
                  <button
                    type="button"
                    onClick={() => { setTokenMode('keep'); setTokenValue('') }}
                    style={{
                      background: 'transparent',
                      border: '1px solid var(--border-default)',
                      borderRadius: '3px',
                      color: 'var(--text-secondary)',
                      fontSize: '0.7rem',
                      fontFamily: 'monospace',
                      padding: '4px 10px',
                      cursor: 'pointer',
                    }}
                  >
                    Cancel
                  </button>
                </div>
              </div>
            )}
          </div>

          {/* Allowed chat IDs */}
          <div style={{ display: 'flex', flexDirection: 'column', gap: '6px' }}>
            <label className="text-xs font-medium" style={{ color: 'var(--text-secondary)' }}>
              Allowed Chat IDs
            </label>

            {/* Add row */}
            <div style={{ display: 'flex', gap: '6px' }}>
              <input
                type="text"
                inputMode="numeric"
                placeholder="e.g. -1001234567890"
                value={chatInput}
                onChange={e => setChatInput(e.target.value.replace(/[^0-9-]/g, ''))}
                onFocus={() => setChatInputFocused(true)}
                onBlur={() => setChatInputFocused(false)}
                onKeyDown={e => { if (e.key === 'Enter') { e.preventDefault(); addChat() } }}
                style={{
                  backgroundColor: 'var(--bg-elevated)',
                  border: `1px solid ${chatInputFocused ? 'var(--accent)' : 'var(--border-default)'}`,
                  color: 'var(--text-primary)',
                  borderRadius: '3px',
                  outline: 'none',
                  flex: 1,
                  padding: '5px 8px',
                  fontFamily: 'monospace',
                  fontSize: '0.75rem',
                }}
              />
              <button
                type="button"
                onClick={addChat}
                disabled={!chatInput.trim()}
                style={{
                  display: 'flex',
                  alignItems: 'center',
                  gap: '4px',
                  backgroundColor: chatInput.trim() ? 'var(--accent)' : 'var(--bg-elevated)',
                  color: chatInput.trim() ? '#fff' : 'var(--text-muted)',
                  border: chatInput.trim() ? 'none' : '1px solid var(--border-default)',
                  borderRadius: '3px',
                  padding: '5px 12px',
                  fontSize: '0.7rem',
                  fontFamily: 'monospace',
                  cursor: chatInput.trim() ? 'pointer' : 'not-allowed',
                }}
              >
                <Plus size={13} />
                Add
              </button>
            </div>

            {/* Chat ID list */}
            {chats.length === 0 ? (
              <p className="font-mono text-xs" style={{ color: 'var(--text-muted)', margin: 0 }}>
                No chat IDs configured — no chats can run commands.
              </p>
            ) : (
              <div
                style={{
                  border: '1px solid var(--border-subtle)',
                  borderRadius: '3px',
                  overflow: 'hidden',
                }}
              >
                {chats.map((id, idx) => (
                  <div
                    key={id}
                    style={{
                      display: 'flex',
                      alignItems: 'center',
                      justifyContent: 'space-between',
                      padding: '6px 10px',
                      backgroundColor: idx % 2 === 0 ? 'var(--bg-elevated)' : 'var(--bg-surface)',
                      borderBottom: idx < chats.length - 1 ? '1px solid var(--border-subtle)' : 'none',
                    }}
                  >
                    <code className="font-mono text-xs" style={{ color: 'var(--text-primary)' }}>
                      {id}
                    </code>
                    <button
                      type="button"
                      onClick={() => removeChat(id)}
                      title="Remove"
                      style={{
                        background: 'transparent',
                        border: 'none',
                        cursor: 'pointer',
                        color: 'var(--text-muted)',
                        padding: '2px',
                        display: 'flex',
                        alignItems: 'center',
                        lineHeight: 1,
                      }}
                    >
                      <X size={13} />
                    </button>
                  </div>
                ))}
              </div>
            )}
          </div>

          {/* Save / status */}
          <div style={{ display: 'flex', alignItems: 'center', gap: '12px' }}>
            <button
              type="button"
              onClick={() => void handleSave()}
              disabled={setConfig.isPending}
              style={{
                display: 'flex',
                alignItems: 'center',
                gap: '6px',
                backgroundColor: 'var(--accent)',
                color: '#fff',
                border: 'none',
                borderRadius: '3px',
                padding: '6px 16px',
                fontSize: '0.75rem',
                fontWeight: 500,
                cursor: setConfig.isPending ? 'not-allowed' : 'pointer',
                opacity: setConfig.isPending ? 0.6 : 1,
              }}
            >
              {setConfig.isPending
                ? <Loader size={12} className="animate-spin" />
                : <Save size={13} />}
              Save
            </button>
            {status === 'ok' && (
              <span className="font-mono text-xs" style={{ color: 'var(--status-ok)' }}>
                Saved.
              </span>
            )}
            {status === 'error' && (
              <span className="font-mono text-xs" style={{ color: 'var(--status-error)' }}>
                {errorMsg}
              </span>
            )}
          </div>
        </div>
      )}
    </section>
  )
}

// ── Page ──────────────────────────────────────────────────────────────────────

export default function Settings() {
  const user = useAuthStore((s) => s.user)
  const { isAdmin } = useCan()

  return (
    <AppShell>
      <div style={{ maxWidth: '640px', display: 'flex', flexDirection: 'column', gap: '24px' }}>
        {/* Page header */}
        <div>
          <h1 className="text-base font-semibold" style={{ color: 'var(--text-primary)', margin: 0 }}>
            Settings
          </h1>
          {user && (
            <p className="text-xs" style={{ color: 'var(--text-muted)', marginTop: '2px' }}>
              Signed in as <span style={{ color: 'var(--text-secondary)' }}>{user.username}</span>
              {' · '}
              <RoleBadge role={user.role} />
            </p>
          )}
        </div>

        <TwoFactorSection />

        <ChangePasswordSection />

        <SessionsSection />

        {isAdmin && <UsersSection />}

        {isAdmin && <AISettingsSection />}

        <FeaturesSection />

        {isAdmin && <ChatIntegrationSection />}

        <MemoryPanel scope="global" />

        <RunbooksSection />
      </div>
    </AppShell>
  )
}
