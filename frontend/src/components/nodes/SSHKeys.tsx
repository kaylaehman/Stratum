import { useState } from 'react'
import { KeyRound, Trash2, Loader, User } from 'lucide-react'
import { useSSHKeys, useDeleteSSHKey } from '../../lib/api/sshkeys'
import { useMe } from '../../hooks/useMe'
import { ApiError } from '../../lib/api'
import type { SSHKey } from '../../types/api'

interface SSHKeysProps {
  nodeId: string
}

interface ConfirmDialogProps {
  keyEntry: SSHKey
  onConfirm: () => void
  onCancel: () => void
  isPending: boolean
  errorMsg: string | null
}

function ConfirmDialog({ keyEntry, onConfirm, onCancel, isPending, errorMsg }: ConfirmDialogProps) {
  return (
    <div
      className="flex flex-col gap-2 p-3 mt-2"
      style={{
        background: 'var(--bg-elevated)',
        border: '1px solid var(--status-error)',
        borderRadius: '3px',
      }}
    >
      <p className="font-mono text-xs" style={{ color: 'var(--text-primary)' }}>
        Remove this SSH key for <strong>{keyEntry.user}</strong>? They will lose access via this key.
      </p>
      <p className="font-mono text-xs truncate" style={{ color: 'var(--text-muted)' }} title={keyEntry.fingerprint}>
        {keyEntry.fingerprint}
      </p>
      {errorMsg && (
        <span className="font-mono text-xs" style={{ color: 'var(--status-error)' }}>
          {errorMsg}
        </span>
      )}
      <div className="flex items-center gap-2 mt-1">
        <button
          type="button"
          disabled={isPending}
          onClick={onConfirm}
          className="flex items-center gap-1.5 font-mono text-xs px-2.5 py-1"
          style={{
            background: 'rgba(232,64,64,0.15)',
            border: '1px solid var(--status-error)',
            color: isPending ? 'var(--text-muted)' : 'var(--status-error)',
            borderRadius: '3px',
            cursor: isPending ? 'not-allowed' : 'pointer',
            opacity: isPending ? 0.6 : 1,
          }}
        >
          {isPending ? <Loader size={12} className="animate-spin" /> : <Trash2 size={12} />}
          Remove
        </button>
        <button
          type="button"
          disabled={isPending}
          onClick={onCancel}
          className="font-mono text-xs px-2.5 py-1"
          style={{
            background: 'var(--bg-elevated)',
            border: '1px solid var(--border-default)',
            color: 'var(--text-secondary)',
            borderRadius: '3px',
            cursor: isPending ? 'not-allowed' : 'pointer',
          }}
        >
          Cancel
        </button>
      </div>
    </div>
  )
}

interface KeyRowProps {
  keyEntry: SSHKey
  nodeId: string
}

function KeyRow({ keyEntry, nodeId }: KeyRowProps) {
  const [confirming, setConfirming] = useState(false)
  const { mutate: deleteKey, isPending, error, reset } = useDeleteSSHKey()

  const apiErr = error as ApiError | null
  const errCode = apiErr ? (apiErr.body as { error?: string })?.error : null
  const errorMsg =
    errCode === 'key_not_found'
      ? 'Key not found (already removed?)'
      : errCode === 'delete_failed'
        ? 'SSH/delete failed'
        : apiErr
          ? 'Delete failed'
          : null

  function handleConfirm() {
    deleteKey(
      { nodeId, request: { path: keyEntry.path, fingerprint: keyEntry.fingerprint } },
      { onSuccess: () => setConfirming(false) },
    )
  }

  function handleCancel() {
    reset()
    setConfirming(false)
  }

  return (
    <div
      className="flex flex-col gap-1 py-2"
      style={{ borderBottom: '1px solid var(--border-subtle)' }}
    >
      <div className="flex items-start justify-between gap-2">
        <div className="flex flex-col gap-0.5 min-w-0">
          <div className="flex items-center gap-1.5">
            <User size={11} style={{ color: 'var(--text-muted)', flexShrink: 0 }} />
            <span className="font-mono text-xs" style={{ color: 'var(--text-secondary)' }}>
              {keyEntry.user}
            </span>
            <span
              className="font-mono text-xs px-1"
              style={{
                background: 'var(--bg-elevated)',
                border: '1px solid var(--border-subtle)',
                color: 'var(--text-muted)',
                borderRadius: '3px',
              }}
            >
              {keyEntry.type}
            </span>
          </div>
          {keyEntry.comment && (
            <span className="font-mono text-xs truncate" style={{ color: 'var(--text-muted)', paddingLeft: '19px' }}>
              {keyEntry.comment}
            </span>
          )}
          <span
            className="font-mono text-xs truncate"
            style={{ color: 'var(--text-primary)', paddingLeft: '19px' }}
            title={keyEntry.fingerprint}
          >
            {keyEntry.fingerprint}
          </span>
        </div>
        <button
          type="button"
          disabled={confirming || isPending}
          onClick={() => setConfirming(true)}
          title="Delete key"
          className="flex items-center gap-1 font-mono text-xs px-2 py-1 shrink-0"
          style={{
            background: 'var(--bg-elevated)',
            border: '1px solid var(--border-default)',
            color: confirming ? 'var(--text-muted)' : 'var(--status-error)',
            borderRadius: '3px',
            cursor: confirming || isPending ? 'not-allowed' : 'pointer',
            opacity: confirming || isPending ? 0.5 : 1,
          }}
        >
          <Trash2 size={11} />
        </button>
      </div>
      {confirming && (
        <ConfirmDialog
          keyEntry={keyEntry}
          onConfirm={handleConfirm}
          onCancel={handleCancel}
          isPending={isPending}
          errorMsg={errorMsg}
        />
      )}
    </div>
  )
}

export function SSHKeys({ nodeId }: SSHKeysProps) {
  const { data: me } = useMe()
  const { data, isLoading, isError } = useSSHKeys(nodeId)

  if (me?.role !== 'admin') return null

  const sectionStyle: React.CSSProperties = {
    borderTop: '1px solid var(--border-subtle)',
    paddingTop: '8px',
    marginTop: '8px',
  }

  const labelStyle: React.CSSProperties = {
    fontSize: '12px',
    color: 'var(--text-muted)',
  }

  if (isLoading) {
    return (
      <div style={sectionStyle}>
        <div className="flex items-center gap-1.5">
          <KeyRound size={12} style={{ color: 'var(--text-muted)' }} />
          <span className="text-xs font-medium uppercase tracking-wider" style={labelStyle}>
            SSH Keys
          </span>
        </div>
        <Loader size={12} className="animate-spin mt-2" style={{ color: 'var(--text-muted)' }} />
      </div>
    )
  }

  if (isError) {
    return (
      <div style={sectionStyle}>
        <div className="flex items-center gap-1.5">
          <KeyRound size={12} style={{ color: 'var(--text-muted)' }} />
          <span className="text-xs font-medium uppercase tracking-wider" style={labelStyle}>
            SSH Keys
          </span>
        </div>
        <p className="font-mono text-xs mt-2" style={{ color: 'var(--status-error)' }}>
          Could not read keys — node unreachable over SSH
        </p>
      </div>
    )
  }

  const keys = data?.keys ?? []

  return (
    <div style={sectionStyle}>
      <div className="flex items-center gap-1.5 mb-2">
        <KeyRound size={12} style={{ color: 'var(--text-muted)' }} />
        <span className="text-xs font-medium uppercase tracking-wider" style={labelStyle}>
          SSH Keys
        </span>
        {keys.length > 0 && (
          <span
            className="font-mono text-xs px-1"
            style={{
              background: 'var(--bg-elevated)',
              border: '1px solid var(--border-subtle)',
              color: 'var(--text-muted)',
              borderRadius: '3px',
            }}
          >
            {keys.length}
          </span>
        )}
      </div>
      {keys.length === 0 ? (
        <p className="font-mono text-xs" style={{ color: 'var(--text-muted)' }}>
          No authorized_keys found across users.
        </p>
      ) : (
        <div className="flex flex-col">
          {keys.map((k) => (
            <KeyRow key={`${k.path}:${k.fingerprint}`} keyEntry={k} nodeId={nodeId} />
          ))}
        </div>
      )}
    </div>
  )
}
