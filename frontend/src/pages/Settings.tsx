import { useState } from 'react'
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
} from '../lib/api/users'
import { useCan } from '../lib/roles'
import { ApiError } from '../lib/api'
import type { TwoFASetupResponse, UserRole } from '../types/api'
import { AISettingsSection } from '../components/ai/AISettingsSection'

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

function errorMsg(err: unknown, field: 'username' | 'password' | 'role' | 'delete'): string | null {
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

function UsersSection() {
  const { data, isLoading } = useUsers()
  const updateRole = useUpdateUserRole()
  const deleteUser = useDeleteUser()
  const currentUser = useAuthStore((s) => s.user)
  const [showCreate, setShowCreate] = useState(false)
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
                  <tr
                    key={u.id}
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
                        {deleteErrors[u.id] && (
                          <span style={{ fontSize: 10, color: 'var(--status-error)', fontFamily: 'monospace' }}>
                            {deleteErrors[u.id]}
                          </span>
                        )}
                      </div>
                    </td>
                  </tr>
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

// ── Page ──────────────────────────────────────────────────────────────────────

export default function Settings() {
  const user = useAuthStore((s) => s.user)
  const { isAdmin } = useCan()

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
              {' · '}
              <RoleBadge role={user.role} />
            </p>
          )}
        </div>

        <TwoFactorSection />

        <SessionsSection />

        {isAdmin && <UsersSection />}

        {isAdmin && <AISettingsSection />}
      </div>
    </div>
  )
}
