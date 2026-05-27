import { useState } from 'react'
import { History, RotateCcw, Save, Loader, Camera } from 'lucide-react'
import { useSnapshots, useSaveSnapshot, useRollback } from '../../lib/api/recreate'
import { ApiError } from '../../lib/api'
import type { Snapshot } from '../../types/api'

interface SnapshotsPanelProps {
  containerId: string
  containerName: string
}

// ---- Inline confirm box (matches SSHKeys ConfirmDialog pattern) ----

interface RollbackConfirmProps {
  containerName: string
  snapshot: Snapshot
  onConfirm: () => void
  onCancel: () => void
  isPending: boolean
  errorMsg: string | null
}

function RollbackConfirm({
  containerName,
  snapshot,
  onConfirm,
  onCancel,
  isPending,
  errorMsg,
}: RollbackConfirmProps) {
  const digest = snapshot.image_digest ? snapshot.image_digest.slice(0, 16) : null
  return (
    <div
      className="flex flex-col gap-2 p-3 mt-2"
      style={{
        background: 'var(--bg-elevated)',
        border: '1px solid var(--status-warn)',
        borderRadius: '3px',
      }}
    >
      <p className="font-mono text-xs" style={{ color: 'var(--text-primary)' }}>
        Roll back <strong>{containerName}</strong> to this snapshot?
      </p>
      <p className="text-xs" style={{ color: 'var(--text-muted)' }}>
        The container will be stopped and recreated from{' '}
        <span className="font-mono">{snapshot.image_ref}</span>
        {digest && (
          <span className="font-mono"> @{digest}…</span>
        )}.
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
            background: 'rgba(240,160,32,0.15)',
            border: '1px solid var(--status-warn)',
            color: isPending ? 'var(--text-muted)' : 'var(--status-warn)',
            borderRadius: '3px',
            cursor: isPending ? 'not-allowed' : 'pointer',
            opacity: isPending ? 0.6 : 1,
          }}
        >
          {isPending ? <Loader size={12} className="animate-spin" /> : <RotateCcw size={12} />}
          Roll back
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

// ---- Single snapshot row ----

interface SnapshotRowProps {
  snapshot: Snapshot
  containerId: string
  containerName: string
}

function SnapshotRow({ snapshot, containerId, containerName }: SnapshotRowProps) {
  const [confirming, setConfirming] = useState(false)
  const { mutate: rollback, isPending, error, reset } = useRollback()

  const errorMsg = error
    ? (error as ApiError).status === 404
      ? 'Snapshot not found'
      : 'Rollback failed — container could not be recreated'
    : null

  const digest = snapshot.image_digest ? snapshot.image_digest.slice(0, 12) : null

  function handleConfirm() {
    rollback(
      { containerId, snapshotId: snapshot.id },
      {
        onSuccess: () => setConfirming(false),
        onError: () => {/* stay open so errorMsg renders */},
      },
    )
  }

  return (
    <div
      className="flex flex-col"
      style={{ borderBottom: '1px solid var(--border-subtle)', paddingBottom: '8px', marginBottom: '8px' }}
    >
      <div className="flex items-start justify-between gap-3">
        <div className="flex flex-col gap-0.5 min-w-0">
          <div className="flex items-center gap-2">
            <span
              className="font-mono text-xs px-1 py-0.5 uppercase tracking-wider"
              style={{
                background: snapshot.reason === 'manual'
                  ? 'rgba(74,82,104,0.25)'
                  : 'rgba(64,120,200,0.12)',
                border: `1px solid ${snapshot.reason === 'manual' ? 'var(--border-subtle)' : 'rgba(64,120,200,0.3)'}`,
                color: snapshot.reason === 'manual' ? 'var(--text-muted)' : 'var(--accent)',
                borderRadius: '3px',
                fontSize: '9px',
              }}
            >
              {snapshot.reason}
            </span>
            <span className="text-xs" style={{ color: 'var(--text-muted)' }}>
              {new Date(snapshot.created_at).toLocaleString()}
            </span>
          </div>
          <span className="font-mono text-xs truncate" style={{ color: 'var(--text-secondary)' }} title={snapshot.image_ref}>
            {snapshot.image_ref}
            {digest && <span style={{ color: 'var(--text-muted)' }}> @{digest}…</span>}
          </span>
        </div>

        <button
          type="button"
          disabled={confirming || isPending}
          onClick={() => { reset(); setConfirming(true) }}
          className="flex items-center gap-1.5 font-mono text-xs px-2.5 py-1 shrink-0"
          style={{
            background: 'var(--bg-elevated)',
            border: '1px solid var(--border-default)',
            color: confirming || isPending ? 'var(--text-muted)' : 'var(--status-warn)',
            borderRadius: '3px',
            cursor: confirming || isPending ? 'default' : 'pointer',
            opacity: confirming || isPending ? 0.5 : 1,
          }}
        >
          <RotateCcw size={11} />
          Roll back
        </button>
      </div>

      {confirming && (
        <RollbackConfirm
          containerName={containerName}
          snapshot={snapshot}
          onConfirm={handleConfirm}
          onCancel={() => { reset(); setConfirming(false) }}
          isPending={isPending}
          errorMsg={errorMsg}
        />
      )}
    </div>
  )
}

// ---- Main panel ----

export function SnapshotsPanel({ containerId, containerName }: SnapshotsPanelProps) {
  const { data, isLoading } = useSnapshots(containerId)
  const { mutate: save, isPending: isSaving, isSuccess: savedOk, error: saveError, reset: resetSave } = useSaveSnapshot()

  const snapshots = data?.snapshots ?? []

  const saveErrorMsg = saveError
    ? 'Failed to save checkpoint'
    : null

  return (
    <div
      className="flex flex-col gap-3"
      style={{
        backgroundColor: 'var(--bg-surface)',
        border: '1px solid var(--border-subtle)',
        borderRadius: '3px',
        padding: '16px',
        maxWidth: '640px',
      }}
    >
      {/* Section header */}
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-2">
          <History size={13} style={{ color: 'var(--text-muted)' }} />
          <span
            className="text-xs font-medium uppercase tracking-wider"
            style={{ color: 'var(--text-muted)' }}
          >
            Snapshots &amp; Rollback
          </span>
        </div>

        {/* Save checkpoint */}
        <button
          type="button"
          disabled={isSaving}
          onClick={() => {
            resetSave()
            save(containerId, { onSuccess: () => { setTimeout(resetSave, 3000) } })
          }}
          className="flex items-center gap-1.5 font-mono text-xs px-2.5 py-1"
          style={{
            background: savedOk ? 'rgba(64,200,120,0.1)' : 'var(--bg-elevated)',
            border: `1px solid ${savedOk ? 'var(--status-ok, #40c878)' : 'var(--border-default)'}`,
            color: isSaving
              ? 'var(--text-muted)'
              : savedOk
              ? 'var(--status-ok, #40c878)'
              : 'var(--text-secondary)',
            borderRadius: '3px',
            cursor: isSaving ? 'not-allowed' : 'pointer',
            opacity: isSaving ? 0.6 : 1,
          }}
        >
          {isSaving ? (
            <Loader size={11} className="animate-spin" />
          ) : savedOk ? (
            <Camera size={11} />
          ) : (
            <Save size={11} />
          )}
          {savedOk ? 'Saved' : 'Save checkpoint'}
        </button>
      </div>

      {saveErrorMsg && (
        <span className="font-mono text-xs" style={{ color: 'var(--status-error)' }}>
          {saveErrorMsg}
        </span>
      )}

      {/* Snapshots list */}
      {isLoading && (
        <div className="flex items-center gap-2 py-2">
          <Loader size={12} className="animate-spin" style={{ color: 'var(--accent)' }} />
          <span className="text-xs" style={{ color: 'var(--text-muted)' }}>Loading snapshots…</span>
        </div>
      )}

      {!isLoading && snapshots.length === 0 && (
        <p className="text-xs" style={{ color: 'var(--text-muted)' }}>
          No snapshots yet — save a checkpoint or run an update.
        </p>
      )}

      {!isLoading && snapshots.length > 0 && (
        <div className="flex flex-col">
          {snapshots.map((snap) => (
            <SnapshotRow
              key={snap.id}
              snapshot={snap}
              containerId={containerId}
              containerName={containerName}
            />
          ))}
        </div>
      )}
    </div>
  )
}
