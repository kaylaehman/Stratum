import { useState, useEffect, useRef } from 'react'
import {
  KeyRound,
  Plus,
  Eye,
  EyeOff,
  Trash2,
  Upload,
  Copy,
  Loader,
  ShieldAlert,
  AlertTriangle,
  Clock,
  ScanSearch,
  ChevronDown,
  ChevronRight,
  ExternalLink,
  CalendarClock,
  X,
} from 'lucide-react'
import { AppShell } from '../components/layout/AppShell'
import { useMe } from '../hooks/useMe'
import { useNodes } from '../lib/api/nodes'
import {
  useSecrets,
  useCreateGroup,
  useDeleteGroup,
  useSetSecret,
  useImportSecrets,
  useDeleteSecret,
  useRevealSecret,
  useSetExpiry,
  useExpiringSecrets,
  useScanNode,
} from '../lib/api/secrets'
import type {
  ExpiryStatus,
  SecretExpiryMeta,
  PlaintextFinding,
} from '../lib/api/secrets'
import type { SecretGroup, SecretKey } from '../types/api'

// ---- Helpers ----

const REVEAL_TIMEOUT_MS = 30_000

function formatDate(iso: string): string {
  return new Date(iso).toLocaleDateString(undefined, {
    year: 'numeric',
    month: 'short',
    day: 'numeric',
  })
}

function daysUntil(iso: string): number {
  return Math.ceil((new Date(iso).getTime() - Date.now()) / 86_400_000)
}

// ---- Expiry badge ----

const EXPIRY_BADGE: Record<
  ExpiryStatus,
  { label: string; bg: string; border: string; color: string }
> = {
  none: {
    label: 'No expiry',
    bg: 'rgba(74,82,104,0.15)',
    border: 'var(--border-default)',
    color: 'var(--text-muted)',
  },
  ok: {
    label: 'Valid',
    bg: 'rgba(74,200,80,0.10)',
    border: 'rgba(74,200,80,0.3)',
    color: 'var(--accent)',
  },
  warning: {
    label: 'Expiring soon',
    bg: 'rgba(240,160,32,0.12)',
    border: 'rgba(240,160,32,0.35)',
    color: 'var(--status-warn)',
  },
  expired: {
    label: 'Expired',
    bg: 'rgba(232,64,64,0.12)',
    border: 'rgba(232,64,64,0.35)',
    color: 'var(--status-error)',
  },
}

interface ExpiryBadgeProps {
  status: ExpiryStatus
  expiresAt?: string | null
}

function ExpiryBadge({ status, expiresAt }: ExpiryBadgeProps) {
  if (status === 'none') return null
  const cfg = EXPIRY_BADGE[status]
  const days = expiresAt ? daysUntil(expiresAt) : null
  const label =
    status === 'expired'
      ? `Expired ${expiresAt ? formatDate(expiresAt) : ''}`
      : status === 'warning' && days !== null
        ? `Expires in ${days}d`
        : status === 'ok' && expiresAt
          ? `Expires ${formatDate(expiresAt)}`
          : cfg.label

  return (
    <span
      className="inline-flex items-center gap-1 font-mono text-xs px-1.5 py-0.5"
      style={{
        backgroundColor: cfg.bg,
        border: `1px solid ${cfg.border}`,
        color: cfg.color,
        borderRadius: '3px',
        whiteSpace: 'nowrap',
      }}
    >
      <Clock size={9} />
      {label}
    </span>
  )
}

// ---- Set expiry form (inline) ----

interface SetExpiryFormProps {
  secretId: string
  currentExpiry: string | null
  onDone: () => void
}

function SetExpiryForm({ secretId, currentExpiry, onDone }: SetExpiryFormProps) {
  const [expiresAt, setExpiresAt] = useState<string>(
    currentExpiry ? currentExpiry.slice(0, 10) : '',
  )
  const { mutate: setExpiry, isPending, error } = useSetExpiry()

  function handleSave(e: React.FormEvent) {
    e.preventDefault()
    setExpiry(
      {
        secretId,
        body: { expires_at: expiresAt ? new Date(expiresAt).toISOString() : null },
      },
      { onSuccess: onDone },
    )
  }

  function handleClear() {
    setExpiry({ secretId, body: { expires_at: null } }, { onSuccess: onDone })
  }

  return (
    <form
      onSubmit={handleSave}
      className="flex items-center gap-1.5"
      style={{ display: 'inline-flex' }}
    >
      <input
        type="date"
        value={expiresAt}
        onChange={(e) => setExpiresAt(e.target.value)}
        className="font-mono text-xs px-1.5 py-0.5"
        style={{
          backgroundColor: 'var(--bg-surface)',
          border: '1px solid var(--border-default)',
          color: 'var(--text-primary)',
          borderRadius: '3px',
          outline: 'none',
        }}
      />
      <button
        type="submit"
        disabled={isPending}
        className="text-xs px-2 py-0.5"
        style={{
          backgroundColor: 'var(--bg-elevated)',
          border: '1px solid var(--border-default)',
          color: 'var(--text-secondary)',
          borderRadius: '3px',
          cursor: isPending ? 'default' : 'pointer',
          opacity: isPending ? 0.6 : 1,
        }}
      >
        {isPending ? <Loader size={9} className="animate-spin" /> : 'Set'}
      </button>
      {currentExpiry && (
        <button
          type="button"
          onClick={handleClear}
          disabled={isPending}
          className="text-xs px-2 py-0.5"
          style={{
            backgroundColor: 'var(--bg-elevated)',
            border: '1px solid var(--border-default)',
            color: 'var(--status-error)',
            borderRadius: '3px',
            cursor: isPending ? 'default' : 'pointer',
          }}
        >
          Clear
        </button>
      )}
      <button
        type="button"
        onClick={onDone}
        className="text-xs px-1.5 py-0.5"
        style={{
          background: 'transparent',
          border: 'none',
          color: 'var(--text-muted)',
          cursor: 'pointer',
        }}
      >
        <X size={10} />
      </button>
      {error && (
        <span className="text-xs" style={{ color: 'var(--status-error)' }}>
          Failed
        </span>
      )}
    </form>
  )
}

// ---- Admin gate ----

function AdminRequired() {
  return (
    <div
      className="flex flex-col items-center justify-center gap-3 py-16"
      style={{ color: 'var(--text-muted)' }}
    >
      <ShieldAlert size={28} style={{ color: 'var(--text-muted)' }} />
      <span className="text-xs uppercase tracking-wider">Admin access required</span>
      <span className="text-xs" style={{ color: 'var(--text-muted)' }}>
        The secrets manager is only accessible to administrators.
      </span>
    </div>
  )
}

// ---- Confirm dialog ----

interface ConfirmDialogProps {
  title: string
  message: React.ReactNode
  confirmLabel: string
  isPending: boolean
  onConfirm: () => void
  onCancel: () => void
}

function ConfirmDialog({
  title,
  message,
  confirmLabel,
  isPending,
  onConfirm,
  onCancel,
}: ConfirmDialogProps) {
  return (
    <div
      style={{
        position: 'fixed',
        inset: 0,
        backgroundColor: 'rgba(0,0,0,0.55)',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        zIndex: 50,
      }}
    >
      <div
        style={{
          backgroundColor: 'var(--bg-elevated)',
          border: '1px solid var(--border-default)',
          borderRadius: '3px',
          padding: '20px 24px',
          maxWidth: '420px',
          width: '100%',
        }}
      >
        <div className="flex items-center gap-2 mb-3">
          <Trash2 size={14} style={{ color: 'var(--status-error)', flexShrink: 0 }} />
          <span
            className="text-xs font-medium uppercase tracking-wider"
            style={{ color: 'var(--text-primary)' }}
          >
            {title}
          </span>
        </div>
        <p className="text-xs mb-4" style={{ color: 'var(--text-secondary)', lineHeight: '1.6' }}>
          {message}
        </p>
        <div className="flex items-center gap-2 justify-end">
          <button
            type="button"
            onClick={onCancel}
            disabled={isPending}
            className="text-xs px-3 py-1.5"
            style={{
              backgroundColor: 'var(--bg-surface)',
              border: '1px solid var(--border-default)',
              color: 'var(--text-secondary)',
              borderRadius: '3px',
              cursor: 'pointer',
            }}
          >
            Cancel
          </button>
          <button
            type="button"
            onClick={onConfirm}
            disabled={isPending}
            className="text-xs px-3 py-1.5"
            style={{
              backgroundColor: 'rgba(232,64,64,0.15)',
              border: '1px solid rgba(232,64,64,0.4)',
              color: 'var(--status-error)',
              borderRadius: '3px',
              cursor: isPending ? 'default' : 'pointer',
              opacity: isPending ? 0.6 : 1,
            }}
          >
            {isPending ? 'Deleting...' : confirmLabel}
          </button>
        </div>
      </div>
    </div>
  )
}

// ---- Secret row ----

interface SecretRowProps {
  secret: SecretKey
  isAdmin: boolean
  expiryMeta?: SecretExpiryMeta
}

function SecretRow({ secret, isAdmin, expiryMeta }: SecretRowProps) {
  const [revealedValue, setRevealedValue] = useState<string | null>(null)
  const [copied, setCopied] = useState(false)
  const [confirmingDelete, setConfirmingDelete] = useState(false)
  const [editingExpiry, setEditingExpiry] = useState(false)
  const timeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null)
  const { mutate: reveal, isPending: revealing } = useRevealSecret()
  const { mutate: deleteSecret, isPending: deleting } = useDeleteSecret()

  useEffect(() => {
    if (revealedValue !== null) {
      timeoutRef.current = setTimeout(() => setRevealedValue(null), REVEAL_TIMEOUT_MS)
    }
    return () => {
      if (timeoutRef.current) clearTimeout(timeoutRef.current)
    }
  }, [revealedValue])

  function handleReveal() {
    reveal(secret.id, {
      onSuccess: (data) => {
        setRevealedValue(data.value)
      },
    })
  }

  function handleHide() {
    setRevealedValue(null)
    if (timeoutRef.current) clearTimeout(timeoutRef.current)
  }

  async function handleCopy() {
    if (!revealedValue) return
    await navigator.clipboard.writeText(revealedValue)
    setCopied(true)
    setTimeout(() => setCopied(false), 1500)
  }

  function handleConfirmDelete() {
    deleteSecret(secret.id, { onSuccess: () => setConfirmingDelete(false) })
  }

  const expiryStatus = expiryMeta?.status ?? 'none'

  return (
    <>
      <div
        className="flex flex-col gap-1 px-3 py-2"
        style={{ borderBottom: '1px solid var(--border-subtle)' }}
      >
        <div className="flex items-center gap-3">
          {/* Key name */}
          <span
            className="font-mono text-xs flex-1 min-w-0 truncate"
            style={{ color: 'var(--text-primary)' }}
          >
            {secret.key}
          </span>

          {/* Expiry badge */}
          {expiryMeta && (
            <ExpiryBadge status={expiryStatus} expiresAt={expiryMeta.expires_at} />
          )}

          {/* Value display */}
          <span
            className="font-mono text-xs"
            style={{
              color: revealedValue ? 'var(--accent)' : 'var(--text-muted)',
              letterSpacing: revealedValue ? undefined : '0.15em',
              userSelect: revealedValue ? 'text' : 'none',
              maxWidth: '220px',
              overflow: 'hidden',
              textOverflow: 'ellipsis',
              whiteSpace: 'nowrap',
            }}
          >
            {revealedValue ?? '••••••••'}
          </span>

          {/* Actions */}
          <div className="flex items-center gap-1 shrink-0">
            {revealedValue ? (
              <>
                <button
                  type="button"
                  onClick={() => void handleCopy()}
                  title="Copy value"
                  className="flex items-center gap-1 text-xs px-1.5 py-1"
                  style={{
                    backgroundColor: 'var(--bg-elevated)',
                    border: '1px solid var(--border-default)',
                    color: copied ? 'var(--accent)' : 'var(--text-muted)',
                    borderRadius: '3px',
                    cursor: 'pointer',
                  }}
                >
                  <Copy size={10} />
                  {copied ? 'Copied' : 'Copy'}
                </button>
                <button
                  type="button"
                  onClick={handleHide}
                  title="Hide value"
                  className="flex items-center gap-1 text-xs px-1.5 py-1"
                  style={{
                    backgroundColor: 'var(--bg-elevated)',
                    border: '1px solid var(--border-default)',
                    color: 'var(--text-muted)',
                    borderRadius: '3px',
                    cursor: 'pointer',
                  }}
                >
                  <EyeOff size={10} />
                  Hide
                </button>
              </>
            ) : (
              <button
                type="button"
                onClick={handleReveal}
                disabled={revealing}
                title="Reveal value"
                className="flex items-center gap-1 text-xs px-1.5 py-1"
                style={{
                  backgroundColor: 'var(--bg-elevated)',
                  border: '1px solid var(--border-default)',
                  color: 'var(--text-secondary)',
                  borderRadius: '3px',
                  cursor: revealing ? 'default' : 'pointer',
                  opacity: revealing ? 0.6 : 1,
                }}
              >
                {revealing ? <Loader size={10} className="animate-spin" /> : <Eye size={10} />}
                Reveal
              </button>
            )}

            {/* Admin: set expiry */}
            {isAdmin && !editingExpiry && (
              <button
                type="button"
                onClick={() => setEditingExpiry(true)}
                title="Set expiry"
                className="flex items-center gap-1 text-xs px-1.5 py-1"
                style={{
                  backgroundColor: 'var(--bg-elevated)',
                  border: '1px solid var(--border-default)',
                  color: 'var(--text-muted)',
                  borderRadius: '3px',
                  cursor: 'pointer',
                }}
              >
                <CalendarClock size={10} />
              </button>
            )}

            <button
              type="button"
              onClick={() => setConfirmingDelete(true)}
              title={`Delete ${secret.key}`}
              className="flex items-center gap-1 text-xs px-1.5 py-1"
              style={{
                backgroundColor: 'var(--bg-elevated)',
                border: '1px solid var(--border-default)',
                color: 'var(--status-error)',
                borderRadius: '3px',
                cursor: 'pointer',
              }}
            >
              <Trash2 size={10} />
            </button>
          </div>
        </div>

        {/* Inline expiry editor */}
        {editingExpiry && (
          <div className="pl-0 pt-1">
            <SetExpiryForm
              secretId={secret.id}
              currentExpiry={expiryMeta?.expires_at ?? null}
              onDone={() => setEditingExpiry(false)}
            />
          </div>
        )}
      </div>

      {confirmingDelete && (
        <ConfirmDialog
          title="Delete Secret"
          message={
            <>
              Delete secret{' '}
              <strong className="font-mono" style={{ color: 'var(--text-primary)' }}>
                {secret.key}
              </strong>
              ? This cannot be undone.
            </>
          }
          confirmLabel="Delete"
          isPending={deleting}
          onConfirm={handleConfirmDelete}
          onCancel={() => setConfirmingDelete(false)}
        />
      )}
    </>
  )
}

// ---- Add secret form ----

interface AddSecretFormProps {
  groupId: string
  onDone: () => void
}

function AddSecretForm({ groupId, onDone }: AddSecretFormProps) {
  const [key, setKey] = useState('')
  const [value, setValue] = useState('')
  const [error, setError] = useState<string | null>(null)
  const { mutate: setSecret, isPending } = useSetSecret()

  function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    if (!key.trim()) { setError('Key is required.'); return }
    setError(null)
    setSecret(
      { groupId, body: { key: key.trim(), value } },
      {
        onSuccess: () => {
          setKey('')
          setValue('')
          onDone()
        },
        onError: () => setError('Failed to save secret.'),
      },
    )
  }

  return (
    <form
      onSubmit={handleSubmit}
      className="flex flex-col gap-2 px-3 py-3"
      style={{ borderTop: '1px solid var(--border-subtle)', backgroundColor: 'var(--bg-elevated)' }}
    >
      <span
        className="text-xs uppercase tracking-wider font-medium"
        style={{ color: 'var(--text-muted)' }}
      >
        Add Secret
      </span>
      <div className="flex items-center gap-2">
        <input
          type="text"
          placeholder="KEY_NAME"
          value={key}
          onChange={(e) => setKey(e.target.value)}
          className="font-mono text-xs px-2 py-1.5 flex-1"
          style={{
            backgroundColor: 'var(--bg-surface)',
            border: '1px solid var(--border-default)',
            color: 'var(--text-primary)',
            borderRadius: '3px',
            outline: 'none',
          }}
          autoCapitalize="none"
          autoCorrect="off"
        />
        <input
          type="password"
          placeholder="value"
          value={value}
          onChange={(e) => setValue(e.target.value)}
          className="font-mono text-xs px-2 py-1.5 flex-1"
          style={{
            backgroundColor: 'var(--bg-surface)',
            border: '1px solid var(--border-default)',
            color: 'var(--text-primary)',
            borderRadius: '3px',
            outline: 'none',
          }}
        />
        <button
          type="submit"
          disabled={isPending}
          className="flex items-center gap-1.5 text-xs px-3 py-1.5 shrink-0"
          style={{
            backgroundColor: 'var(--bg-elevated)',
            border: '1px solid var(--border-default)',
            color: 'var(--text-secondary)',
            borderRadius: '3px',
            cursor: isPending ? 'default' : 'pointer',
            opacity: isPending ? 0.6 : 1,
          }}
        >
          {isPending ? <Loader size={10} className="animate-spin" /> : <Plus size={10} />}
          Save
        </button>
        <button
          type="button"
          onClick={onDone}
          className="text-xs px-2 py-1.5"
          style={{
            background: 'transparent',
            border: 'none',
            color: 'var(--text-muted)',
            cursor: 'pointer',
          }}
        >
          Cancel
        </button>
      </div>
      {error && (
        <span className="text-xs" style={{ color: 'var(--status-error)' }}>
          {error}
        </span>
      )}
    </form>
  )
}

// ---- Import form ----

interface ImportFormProps {
  groupId: string
  onDone: (count: number) => void
  onCancel: () => void
}

function ImportForm({ groupId, onDone, onCancel }: ImportFormProps) {
  const [envText, setEnvText] = useState('')
  const [error, setError] = useState<string | null>(null)
  const { mutate: importSecrets, isPending } = useImportSecrets()

  function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    if (!envText.trim()) { setError('Paste .env content first.'); return }
    setError(null)
    importSecrets(
      { groupId, body: { env: envText } },
      {
        onSuccess: (data) => {
          setEnvText('')
          onDone(data.imported)
        },
        onError: () => setError('Import failed.'),
      },
    )
  }

  return (
    <form
      onSubmit={handleSubmit}
      className="flex flex-col gap-2 px-3 py-3"
      style={{ borderTop: '1px solid var(--border-subtle)', backgroundColor: 'var(--bg-elevated)' }}
    >
      <span
        className="text-xs uppercase tracking-wider font-medium"
        style={{ color: 'var(--text-muted)' }}
      >
        Import from .env
      </span>
      <textarea
        placeholder={'KEY=value\nOTHER_KEY=another_value'}
        value={envText}
        onChange={(e) => setEnvText(e.target.value)}
        rows={4}
        className="font-mono text-xs px-2 py-1.5 resize-y"
        style={{
          backgroundColor: 'var(--bg-surface)',
          border: '1px solid var(--border-default)',
          color: 'var(--text-primary)',
          borderRadius: '3px',
          outline: 'none',
          width: '100%',
          boxSizing: 'border-box',
        }}
      />
      <div className="flex items-center gap-2">
        <button
          type="submit"
          disabled={isPending}
          className="flex items-center gap-1.5 text-xs px-3 py-1.5"
          style={{
            backgroundColor: 'var(--bg-elevated)',
            border: '1px solid var(--border-default)',
            color: 'var(--text-secondary)',
            borderRadius: '3px',
            cursor: isPending ? 'default' : 'pointer',
            opacity: isPending ? 0.6 : 1,
          }}
        >
          {isPending ? <Loader size={10} className="animate-spin" /> : <Upload size={10} />}
          Import
        </button>
        <button
          type="button"
          onClick={onCancel}
          className="text-xs px-2 py-1.5"
          style={{
            background: 'transparent',
            border: 'none',
            color: 'var(--text-muted)',
            cursor: 'pointer',
          }}
        >
          Cancel
        </button>
      </div>
      {error && (
        <span className="text-xs" style={{ color: 'var(--status-error)' }}>
          {error}
        </span>
      )}
    </form>
  )
}

// ---- Group card ----

interface GroupCardProps {
  group: SecretGroup
  isAdmin: boolean
  expiryMap: Map<string, SecretExpiryMeta>
}

type PanelMode = 'none' | 'add' | 'import'

function GroupCard({ group, isAdmin, expiryMap }: GroupCardProps) {
  const [panel, setPanel] = useState<PanelMode>('none')
  const [importMessage, setImportMessage] = useState<string | null>(null)
  const [confirmingDelete, setConfirmingDelete] = useState(false)
  const { mutate: deleteGroup, isPending: deletingGroup } = useDeleteGroup()

  function handleImportDone(count: number) {
    setPanel('none')
    setImportMessage(`Imported ${count} secret${count !== 1 ? 's' : ''}.`)
    setTimeout(() => setImportMessage(null), 4000)
  }

  function handleConfirmDelete() {
    deleteGroup(group.id, { onSuccess: () => setConfirmingDelete(false) })
  }

  return (
    <>
      <div
        style={{
          backgroundColor: 'var(--bg-surface)',
          border: '1px solid var(--border-subtle)',
          borderRadius: '3px',
          marginBottom: '16px',
        }}
      >
        {/* Group header */}
        <div
          className="flex items-center gap-3 px-3 py-2.5"
          style={{ borderBottom: '1px solid var(--border-subtle)' }}
        >
          <KeyRound size={13} style={{ color: 'var(--accent)', flexShrink: 0 }} />
          <div className="flex flex-col min-w-0 flex-1">
            <span
              className="font-mono text-xs font-medium"
              style={{ color: 'var(--text-primary)' }}
            >
              {group.name}
            </span>
            {group.description && (
              <span className="text-xs truncate" style={{ color: 'var(--text-muted)' }}>
                {group.description}
              </span>
            )}
          </div>
          <span
            className="font-mono text-xs px-1.5 py-0.5 uppercase tracking-wider shrink-0"
            style={{
              background: 'rgba(74,82,104,0.2)',
              border: '1px solid var(--border-default)',
              color: 'var(--text-muted)',
              borderRadius: '3px',
              fontSize: '12px',
            }}
          >
            {group.secrets.length} {group.secrets.length === 1 ? 'secret' : 'secrets'}
          </span>

          {/* Header actions */}
          <div className="flex items-center gap-1 shrink-0">
            <button
              type="button"
              onClick={() => setPanel(panel === 'add' ? 'none' : 'add')}
              title="Add secret"
              className="flex items-center gap-1 text-xs px-2 py-1"
              style={{
                backgroundColor: 'var(--bg-elevated)',
                border: '1px solid var(--border-default)',
                color: panel === 'add' ? 'var(--accent)' : 'var(--text-secondary)',
                borderRadius: '3px',
                cursor: 'pointer',
              }}
            >
              <Plus size={10} />
              Add
            </button>
            <button
              type="button"
              onClick={() => setPanel(panel === 'import' ? 'none' : 'import')}
              title="Import .env"
              className="flex items-center gap-1 text-xs px-2 py-1"
              style={{
                backgroundColor: 'var(--bg-elevated)',
                border: '1px solid var(--border-default)',
                color: panel === 'import' ? 'var(--accent)' : 'var(--text-secondary)',
                borderRadius: '3px',
                cursor: 'pointer',
              }}
            >
              <Upload size={10} />
              Import
            </button>
            <button
              type="button"
              onClick={() => setConfirmingDelete(true)}
              title="Delete group"
              className="flex items-center gap-1 text-xs px-2 py-1"
              style={{
                backgroundColor: 'var(--bg-elevated)',
                border: '1px solid var(--border-default)',
                color: 'var(--status-error)',
                borderRadius: '3px',
                cursor: 'pointer',
              }}
            >
              <Trash2 size={10} />
            </button>
          </div>
        </div>

        {/* Import success message */}
        {importMessage && (
          <div
            className="px-3 py-1.5 text-xs"
            style={{
              backgroundColor: 'rgba(74,200,80,0.08)',
              borderBottom: '1px solid rgba(74,200,80,0.2)',
              color: 'var(--accent)',
            }}
          >
            {importMessage}
          </div>
        )}

        {/* Secrets list */}
        {group.secrets.length === 0 && panel === 'none' ? (
          <div
            className="px-3 py-3 text-xs"
            style={{ color: 'var(--text-muted)' }}
          >
            No secrets in this group.
          </div>
        ) : (
          group.secrets.map((secret) => (
            <SecretRow
              key={secret.id}
              secret={secret}
              isAdmin={isAdmin}
              expiryMeta={expiryMap.get(secret.id)}
            />
          ))
        )}

        {/* Inline forms */}
        {panel === 'add' && (
          <AddSecretForm groupId={group.id} onDone={() => setPanel('none')} />
        )}
        {panel === 'import' && (
          <ImportForm
            groupId={group.id}
            onDone={handleImportDone}
            onCancel={() => setPanel('none')}
          />
        )}
      </div>

      {confirmingDelete && (
        <ConfirmDialog
          title="Delete Group"
          message={
            <>
              Delete group{' '}
              <strong className="font-mono" style={{ color: 'var(--text-primary)' }}>
                {group.name}
              </strong>
              ? This will permanently delete all {group.secrets.length} secret
              {group.secrets.length !== 1 ? 's' : ''} inside it.
            </>
          }
          confirmLabel="Delete Group"
          isPending={deletingGroup}
          onConfirm={handleConfirmDelete}
          onCancel={() => setConfirmingDelete(false)}
        />
      )}
    </>
  )
}

// ---- New group form ----

function NewGroupForm({ onDone }: { onDone: () => void }) {
  const [name, setName] = useState('')
  const [description, setDescription] = useState('')
  const [error, setError] = useState<string | null>(null)
  const { mutate: createGroup, isPending } = useCreateGroup()

  function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    if (!name.trim()) { setError('Name is required.'); return }
    setError(null)
    createGroup(
      { name: name.trim(), description: description.trim() || undefined },
      {
        onSuccess: () => {
          setName('')
          setDescription('')
          onDone()
        },
        onError: () => setError('Failed to create group.'),
      },
    )
  }

  return (
    <form
      onSubmit={handleSubmit}
      className="flex flex-col gap-2 p-3 mb-4"
      style={{
        backgroundColor: 'var(--bg-surface)',
        border: '1px solid var(--border-default)',
        borderRadius: '3px',
      }}
    >
      <span
        className="text-xs uppercase tracking-wider font-medium"
        style={{ color: 'var(--text-muted)' }}
      >
        New Group
      </span>
      <div className="flex items-center gap-2">
        <input
          type="text"
          placeholder="group-name"
          value={name}
          onChange={(e) => setName(e.target.value)}
          className="font-mono text-xs px-2 py-1.5 flex-1"
          style={{
            backgroundColor: 'var(--bg-elevated)',
            border: '1px solid var(--border-default)',
            color: 'var(--text-primary)',
            borderRadius: '3px',
            outline: 'none',
          }}
          autoFocus
        />
        <input
          type="text"
          placeholder="description (optional)"
          value={description}
          onChange={(e) => setDescription(e.target.value)}
          className="text-xs px-2 py-1.5 flex-1"
          style={{
            backgroundColor: 'var(--bg-elevated)',
            border: '1px solid var(--border-default)',
            color: 'var(--text-primary)',
            borderRadius: '3px',
            outline: 'none',
          }}
        />
        <button
          type="submit"
          disabled={isPending}
          className="flex items-center gap-1.5 text-xs px-3 py-1.5 shrink-0"
          style={{
            backgroundColor: 'var(--bg-elevated)',
            border: '1px solid var(--border-default)',
            color: 'var(--text-secondary)',
            borderRadius: '3px',
            cursor: isPending ? 'default' : 'pointer',
            opacity: isPending ? 0.6 : 1,
          }}
        >
          {isPending ? <Loader size={10} className="animate-spin" /> : <Plus size={10} />}
          Create
        </button>
        <button
          type="button"
          onClick={onDone}
          className="text-xs px-2 py-1.5"
          style={{
            background: 'transparent',
            border: 'none',
            color: 'var(--text-muted)',
            cursor: 'pointer',
          }}
        >
          Cancel
        </button>
      </div>
      {error && (
        <span className="text-xs" style={{ color: 'var(--status-error)' }}>
          {error}
        </span>
      )}
    </form>
  )
}

// ---- Section header ----

function SectionHeader({ label, count }: { label: string; count?: number }) {
  return (
    <div
      className="px-0 py-2 mb-3 flex items-center gap-2"
      style={{ borderBottom: '1px solid var(--border-subtle)' }}
    >
      <span
        className="text-xs font-medium uppercase tracking-wider"
        style={{ color: 'var(--text-muted)' }}
      >
        {label}
      </span>
      {count !== undefined && (
        <span className="text-xs font-mono" style={{ color: 'var(--text-muted)' }}>
          ({count})
        </span>
      )}
    </div>
  )
}

// ---- Expiring summary banner ----

interface ExpiringSummaryProps {
  secrets: SecretExpiryMeta[]
}

function ExpiringSummary({ secrets }: ExpiringSummaryProps) {
  const [expanded, setExpanded] = useState(false)
  const expired = secrets.filter((s) => s.status === 'expired')
  const warning = secrets.filter((s) => s.status === 'warning')

  if (expired.length === 0 && warning.length === 0) return null

  const totalUrgent = expired.length + warning.length

  return (
    <div
      className="mb-4"
      style={{
        backgroundColor: expired.length > 0 ? 'rgba(232,64,64,0.07)' : 'rgba(240,160,32,0.07)',
        border: `1px solid ${expired.length > 0 ? 'rgba(232,64,64,0.25)' : 'rgba(240,160,32,0.25)'}`,
        borderRadius: '3px',
      }}
    >
      <button
        type="button"
        className="w-full flex items-center gap-2 px-3 py-2.5 text-left"
        onClick={() => setExpanded((v) => !v)}
        style={{ background: 'transparent', border: 'none', cursor: 'pointer' }}
      >
        <Clock
          size={12}
          style={{
            color: expired.length > 0 ? 'var(--status-error)' : 'var(--status-warn)',
            flexShrink: 0,
          }}
        />
        <span
          className="text-xs font-medium flex-1"
          style={{
            color: expired.length > 0 ? 'var(--status-error)' : 'var(--status-warn)',
          }}
        >
          {expired.length > 0
            ? `${expired.length} expired secret${expired.length !== 1 ? 's' : ''}`
            : ''}
          {expired.length > 0 && warning.length > 0 ? ', ' : ''}
          {warning.length > 0
            ? `${warning.length} expiring soon`
            : ''}
          {' '}— action required
        </span>
        <span className="text-xs" style={{ color: 'var(--text-muted)' }}>
          {totalUrgent} total
        </span>
        {expanded
          ? <ChevronDown size={11} style={{ color: 'var(--text-muted)' }} />
          : <ChevronRight size={11} style={{ color: 'var(--text-muted)' }} />
        }
      </button>

      {expanded && (
        <div style={{ borderTop: '1px solid rgba(255,255,255,0.06)' }}>
          {[...expired, ...warning].map((s) => (
            <div
              key={s.id}
              className="flex items-center gap-2 px-3 py-1.5"
              style={{ borderBottom: '1px solid rgba(255,255,255,0.04)' }}
            >
              <ExpiryBadge status={s.status} expiresAt={s.expires_at} />
              <span
                className="font-mono text-xs flex-1 truncate"
                style={{ color: 'var(--text-primary)' }}
              >
                {s.key}
              </span>
              <span className="text-xs" style={{ color: 'var(--text-muted)' }}>
                {s.group_name}
              </span>
            </div>
          ))}
        </div>
      )}
    </div>
  )
}

// ---- Plaintext scanner panel ----

interface ScannerPanelProps {
  nodeId: string
  nodeName: string
  findings: PlaintextFinding[]
  onClose: () => void
}

function FindingRow({ finding }: { finding: PlaintextFinding }) {
  return (
    <div
      className="flex flex-col gap-0.5 px-3 py-2"
      style={{ borderBottom: '1px solid var(--border-subtle)' }}
    >
      <div className="flex items-center gap-2">
        <span
          className="font-mono text-xs"
          style={{ color: 'var(--status-warn)', fontWeight: 500 }}
        >
          {finding.key_name}
        </span>
        <span className="text-xs" style={{ color: 'var(--text-muted)' }}>
          line {finding.line}
        </span>
        <span
          className="text-xs px-1.5 py-0.5"
          style={{
            backgroundColor: 'rgba(240,160,32,0.10)',
            border: '1px solid rgba(240,160,32,0.25)',
            color: 'var(--status-warn)',
            borderRadius: '3px',
          }}
        >
          {finding.reason}
        </span>
      </div>
      <div className="flex items-center gap-2">
        <span
          className="font-mono text-xs truncate flex-1"
          style={{ color: 'var(--text-secondary)' }}
        >
          {finding.path}
        </span>
        <span
          className="text-xs flex items-center gap-1 shrink-0"
          style={{ color: 'var(--accent)', cursor: 'pointer' }}
          title="Move this secret to the vault"
        >
          <ExternalLink size={9} />
          Move to vault
        </span>
      </div>
    </div>
  )
}

function ScannerPanel({ nodeName, findings, onClose }: ScannerPanelProps) {
  return (
    <div
      style={{
        backgroundColor: 'var(--bg-surface)',
        border: '1px solid var(--border-subtle)',
        borderRadius: '3px',
        marginBottom: '16px',
      }}
    >
      <div
        className="flex items-center gap-2 px-3 py-2.5"
        style={{ borderBottom: '1px solid var(--border-subtle)' }}
      >
        <ScanSearch size={13} style={{ color: 'var(--status-warn)', flexShrink: 0 }} />
        <span
          className="text-xs font-medium flex-1"
          style={{ color: 'var(--text-primary)' }}
        >
          Plaintext secrets found on{' '}
          <span className="font-mono">{nodeName}</span>
        </span>
        <span
          className="font-mono text-xs px-1.5 py-0.5"
          style={{
            backgroundColor: 'rgba(240,160,32,0.12)',
            border: '1px solid rgba(240,160,32,0.3)',
            color: 'var(--status-warn)',
            borderRadius: '3px',
          }}
        >
          {findings.length} {findings.length === 1 ? 'finding' : 'findings'}
        </span>
        <button
          type="button"
          onClick={onClose}
          className="text-xs px-1.5 py-0.5"
          style={{
            background: 'transparent',
            border: 'none',
            color: 'var(--text-muted)',
            cursor: 'pointer',
          }}
        >
          <X size={11} />
        </button>
      </div>

      {findings.length === 0 ? (
        <div
          className="px-3 py-3 text-xs"
          style={{
            color: 'var(--accent)',
            backgroundColor: 'rgba(74,200,80,0.06)',
          }}
        >
          No plaintext secrets detected. All clear.
        </div>
      ) : (
        <>
          <div
            className="px-3 py-2 text-xs"
            style={{
              color: 'var(--text-secondary)',
              backgroundColor: 'rgba(240,160,32,0.05)',
              borderBottom: '1px solid var(--border-subtle)',
            }}
          >
            These keys appear to be stored in plaintext on disk. No values are shown.
            Consider moving them to the vault.
          </div>
          {findings.map((f, i) => (
            <FindingRow key={`${f.path}:${f.line}:${i}`} finding={f} />
          ))}
        </>
      )}
    </div>
  )
}

// ---- Scanner trigger control ----

interface ScanTriggerProps {
  onResult: (nodeId: string, nodeName: string, findings: PlaintextFinding[]) => void
}

function ScanTrigger({ onResult }: ScanTriggerProps) {
  const { data: nodes } = useNodes()
  const [selectedNodeId, setSelectedNodeId] = useState<string>('')
  const { mutate: scan, isPending } = useScanNode()

  const nodeList = nodes ?? []

  function handleScan() {
    if (!selectedNodeId) return
    const node = nodeList.find((n) => n.id === selectedNodeId)
    scan(selectedNodeId, {
      onSuccess: (data) => {
        onResult(selectedNodeId, node?.name ?? selectedNodeId, data.findings)
      },
    })
  }

  return (
    <div
      className="flex items-center gap-2 p-3 mb-4"
      style={{
        backgroundColor: 'var(--bg-surface)',
        border: '1px solid var(--border-subtle)',
        borderRadius: '3px',
      }}
    >
      <ScanSearch size={13} style={{ color: 'var(--text-muted)', flexShrink: 0 }} />
      <span className="text-xs" style={{ color: 'var(--text-secondary)' }}>
        Scan node for plaintext secrets:
      </span>
      <select
        value={selectedNodeId}
        onChange={(e) => setSelectedNodeId(e.target.value)}
        className="text-xs px-2 py-1.5 flex-1"
        style={{
          backgroundColor: 'var(--bg-elevated)',
          border: '1px solid var(--border-default)',
          color: selectedNodeId ? 'var(--text-primary)' : 'var(--text-muted)',
          borderRadius: '3px',
          outline: 'none',
        }}
      >
        <option value="">Select a node...</option>
        {nodeList.map((n) => (
          <option key={n.id} value={n.id}>
            {n.name} ({n.host})
          </option>
        ))}
      </select>
      <button
        type="button"
        onClick={handleScan}
        disabled={!selectedNodeId || isPending}
        className="flex items-center gap-1.5 text-xs px-3 py-1.5 shrink-0"
        style={{
          backgroundColor: 'var(--bg-elevated)',
          border: '1px solid var(--border-default)',
          color: !selectedNodeId || isPending ? 'var(--text-muted)' : 'var(--text-secondary)',
          borderRadius: '3px',
          cursor: !selectedNodeId || isPending ? 'default' : 'pointer',
          opacity: !selectedNodeId || isPending ? 0.6 : 1,
        }}
      >
        {isPending ? <Loader size={10} className="animate-spin" /> : <ScanSearch size={10} />}
        {isPending ? 'Scanning...' : 'Scan'}
      </button>
    </div>
  )
}

// ---- Main Secrets page ----

interface ScanResult {
  nodeId: string
  nodeName: string
  findings: PlaintextFinding[]
}

export default function Secrets() {
  const { data: me, isLoading: meLoading } = useMe()
  const isAdmin = me?.role === 'admin'

  const { data, isLoading: secretsLoading } = useSecrets()
  const groups: SecretGroup[] = data?.groups ?? []

  const { data: expiringData } = useExpiringSecrets()
  const expiringSecrets = expiringData?.secrets ?? []

  // Build a lookup map: secretId → expiry meta
  const expiryMap = new Map<string, SecretExpiryMeta>(
    expiringSecrets.map((s) => [s.id, s]),
  )

  const [showNewGroup, setShowNewGroup] = useState(false)
  const [scanResult, setScanResult] = useState<ScanResult | null>(null)

  const isLoading = meLoading || secretsLoading

  function handleScanResult(nodeId: string, nodeName: string, findings: PlaintextFinding[]) {
    setScanResult({ nodeId, nodeName, findings })
  }

  return (
    <AppShell>
      <div
        className="flex flex-col flex-1 min-h-0 h-full w-full p-6"
        style={{ maxWidth: '860px', margin: '0 auto' }}
      >
        {/* Page header */}
        <div className="flex items-center justify-between mb-6">
          <div className="flex items-center gap-2">
            <KeyRound size={16} style={{ color: 'var(--text-secondary)' }} />
            <h1
              className="text-sm font-medium uppercase tracking-wider"
              style={{ color: 'var(--text-primary)' }}
            >
              Secrets Manager
            </h1>
          </div>
          {isAdmin && (
            <button
              type="button"
              onClick={() => setShowNewGroup((v) => !v)}
              className="flex items-center gap-1.5 text-xs px-3 py-1.5"
              style={{
                backgroundColor: 'var(--bg-elevated)',
                border: '1px solid var(--border-default)',
                color: showNewGroup ? 'var(--accent)' : 'var(--text-secondary)',
                borderRadius: '3px',
                cursor: 'pointer',
              }}
            >
              <Plus size={11} />
              New Group
            </button>
          )}
        </div>

        {/* Loading */}
        {isLoading && (
          <div className="flex items-center gap-2 py-8">
            <Loader size={13} className="animate-spin" style={{ color: 'var(--accent)' }} />
            <span className="text-xs" style={{ color: 'var(--text-muted)' }}>
              Loading secrets...
            </span>
          </div>
        )}

        {/* Admin gate */}
        {!meLoading && !isAdmin && <AdminRequired />}

        {/* Content */}
        {isAdmin && !isLoading && (
          <>
            {showNewGroup && (
              <NewGroupForm onDone={() => setShowNewGroup(false)} />
            )}

            {/* Expiring secrets summary */}
            <ExpiringSummary secrets={expiringSecrets} />

            {/* Scanner trigger */}
            <ScanTrigger onResult={handleScanResult} />

            {/* Scanner results */}
            {scanResult && (
              <ScannerPanel
                nodeId={scanResult.nodeId}
                nodeName={scanResult.nodeName}
                findings={scanResult.findings}
                onClose={() => setScanResult(null)}
              />
            )}

            <SectionHeader label="Secret Groups" count={groups.length} />

            {groups.length === 0 ? (
              <div
                className="px-3 py-4 text-xs"
                style={{
                  color: 'var(--text-muted)',
                  backgroundColor: 'var(--bg-surface)',
                  border: '1px solid var(--border-subtle)',
                  borderRadius: '3px',
                }}
              >
                No secret groups yet. Create one above.
              </div>
            ) : (
              groups.map((group) => (
                <GroupCard
                  key={group.id}
                  group={group}
                  isAdmin={isAdmin}
                  expiryMap={expiryMap}
                />
              ))
            )}

            {/* Security notice */}
            <div
              className="flex items-start gap-2 mt-4 px-3 py-2.5 text-xs"
              style={{
                backgroundColor: 'rgba(240,160,32,0.07)',
                border: '1px solid rgba(240,160,32,0.2)',
                borderRadius: '3px',
                color: 'var(--text-secondary)',
              }}
            >
              <AlertTriangle
                size={12}
                style={{ color: 'var(--status-warn)', flexShrink: 0, marginTop: '1px' }}
              />
              <span>
                Secret values are <strong style={{ color: 'var(--status-warn)' }}>encrypted at rest</strong>.
                Values are never shown by default — each reveal is audited server-side.
                Revealed values are masked automatically after 30 seconds.
              </span>
            </div>
          </>
        )}
      </div>
    </AppShell>
  )
}
