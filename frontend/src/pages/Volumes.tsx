import { useState } from 'react'
import { Link } from 'react-router-dom'
import { HardDrive, Trash2, Loader, AlertTriangle, Database, Filter } from 'lucide-react'
import { AppShell } from '../components/layout/AppShell'
import { useMe } from '../hooks/useMe'
import { useTree } from '../lib/api/tree'
import { useVolumes, useRemoveVolume, usePruneUnusedVolumes } from '../lib/api/volumes'
import { ApiError } from '../lib/api'
import type { VolumeView, VolumeSamplePoint } from '../types/api'
import type { PruneUnusedResult } from '../lib/api/volumes'

// ---- Helpers ----

function humanBytes(bytes: number): string {
  if (bytes < 0) return '—'
  if (bytes < 1024) return `${bytes} B`
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`
  if (bytes < 1024 * 1024 * 1024) return `${(bytes / (1024 * 1024)).toFixed(1)} MB`
  if (bytes < 1024 * 1024 * 1024 * 1024) return `${(bytes / (1024 * 1024 * 1024)).toFixed(2)} GB`
  return `${(bytes / (1024 * 1024 * 1024 * 1024)).toFixed(2)} TB`
}

// ---- Sparkline ----

interface SparklineProps {
  samples: VolumeSamplePoint[]
}

function Sparkline({ samples }: SparklineProps) {
  if (samples.length < 2) {
    return <span style={{ color: 'var(--text-muted)', fontSize: '12px' }}>—</span>
  }

  const W = 60
  const H = 20
  const values = samples.map((s) => s.size_bytes)
  const min = Math.min(...values)
  const max = Math.max(...values)
  const range = max - min || 1

  const points = values
    .map((v, i) => {
      const x = (i / (values.length - 1)) * W
      const y = H - ((v - min) / range) * (H - 2) - 1
      return `${x.toFixed(1)},${y.toFixed(1)}`
    })
    .join(' ')

  return (
    <svg
      width={W}
      height={H}
      viewBox={`0 0 ${W} ${H}`}
      style={{ display: 'block', overflow: 'visible' }}
    >
      <polyline
        points={points}
        fill="none"
        stroke="var(--accent)"
        strokeWidth="1.5"
        strokeLinejoin="round"
        strokeLinecap="round"
      />
    </svg>
  )
}

// ---- Status chip ----

function StatusChip({ status }: { status: VolumeView['status'] }) {
  let color: string
  let bg: string
  let border: string
  let label: string

  if (status === 'attached') {
    color = 'var(--text-muted)'
    bg = 'rgba(74,82,104,0.15)'
    border = 'var(--border-default)'
    label = 'attached'
  } else if (status === 'unused') {
    color = 'var(--status-warn)'
    bg = 'rgba(240,160,32,0.12)'
    border = 'rgba(240,160,32,0.4)'
    label = 'unused'
  } else {
    color = 'var(--text-muted)'
    bg = 'transparent'
    border = 'var(--border-subtle)'
    label = 'unknown'
  }

  return (
    <span
      className="font-mono text-xs px-1.5 py-0.5 uppercase tracking-wider"
      style={{ color, background: bg, border: `1px solid ${border}`, borderRadius: '3px', fontSize: '12px' }}
    >
      {label}
    </span>
  )
}

// ---- Confirm dialog ----

interface ConfirmDialogProps {
  volumeName: string
  nodeName: string
  onConfirm: () => void
  onCancel: () => void
  isPending: boolean
  inlineError: string | null
}

function ConfirmDialog({ volumeName, nodeName, onConfirm, onCancel, isPending, inlineError }: ConfirmDialogProps) {
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
          <span className="text-xs font-medium uppercase tracking-wider" style={{ color: 'var(--text-primary)' }}>
            Remove Volume
          </span>
        </div>
        <p className="text-xs mb-4" style={{ color: 'var(--text-secondary)', lineHeight: '1.6' }}>
          Remove volume{' '}
          <strong className="font-mono" style={{ color: 'var(--text-primary)' }}>
            {volumeName}
          </strong>{' '}
          on <strong style={{ color: 'var(--text-primary)' }}>{nodeName}</strong>? This cannot be undone.
        </p>
        {inlineError && (
          <div
            className="flex items-center gap-2 mb-3 px-2 py-1.5 text-xs"
            style={{
              backgroundColor: 'rgba(232,64,64,0.1)',
              border: '1px solid rgba(232,64,64,0.3)',
              borderRadius: '3px',
              color: 'var(--status-error)',
            }}
          >
            <AlertTriangle size={11} style={{ flexShrink: 0 }} />
            {inlineError}
          </div>
        )}
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
            {isPending ? 'Removing...' : 'Remove'}
          </button>
        </div>
      </div>
    </div>
  )
}

// ---- Prune confirm dialog ----

interface PruneConfirmDialogProps {
  unusedVolumes: VolumeView[]
  nodeNameFn: (id: string) => string
  onConfirm: () => void
  onCancel: () => void
  isPending: boolean
  inlineError: string | null
  results: PruneUnusedResult[] | null
}

function PruneConfirmDialog({
  unusedVolumes,
  nodeNameFn,
  onConfirm,
  onCancel,
  isPending,
  inlineError,
  results,
}: PruneConfirmDialogProps) {
  const settled = results !== null

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
          maxWidth: '520px',
          width: '100%',
          maxHeight: '80vh',
          display: 'flex',
          flexDirection: 'column',
        }}
      >
        <div className="flex items-center gap-2 mb-3">
          <Trash2 size={14} style={{ color: 'var(--status-error)', flexShrink: 0 }} />
          <span className="text-xs font-medium uppercase tracking-wider" style={{ color: 'var(--text-primary)' }}>
            {settled ? 'Prune Results' : `Remove All Unused Volumes (${unusedVolumes.length})`}
          </span>
        </div>

        {!settled && (
          <p className="text-xs mb-3" style={{ color: 'var(--text-secondary)', lineHeight: '1.6' }}>
            The following unused volumes will be permanently removed. This cannot be undone.
          </p>
        )}

        {/* Volume list / results */}
        <div
          style={{
            overflowY: 'auto',
            flex: '1 1 auto',
            marginBottom: '12px',
            border: '1px solid var(--border-subtle)',
            borderRadius: '3px',
          }}
        >
          {!settled &&
            unusedVolumes.map((vol) => (
              <div
                key={`${vol.node_id}:${vol.name}`}
                className="flex items-center justify-between px-3 py-1.5 text-xs"
                style={{
                  borderBottom: '1px solid var(--border-subtle)',
                  gap: '8px',
                }}
              >
                <span className="font-mono truncate" style={{ color: 'var(--text-primary)' }}>
                  {vol.name}
                </span>
                <span style={{ color: 'var(--text-muted)', flexShrink: 0 }}>{nodeNameFn(vol.node_id)}</span>
              </div>
            ))}

          {settled &&
            results!.map((r) => (
              <div
                key={`${r.node_id}:${r.name}`}
                className="flex items-center justify-between px-3 py-1.5 text-xs"
                style={{ borderBottom: '1px solid var(--border-subtle)', gap: '8px' }}
              >
                <span className="font-mono truncate" style={{ color: 'var(--text-primary)' }}>
                  {r.name}
                </span>
                <span style={{ flexShrink: 0, color: r.ok ? 'var(--status-ok)' : 'var(--status-error)' }}>
                  {r.ok ? 'removed' : (r.error ?? 'failed')}
                </span>
              </div>
            ))}
        </div>

        {inlineError && (
          <div
            className="flex items-center gap-2 mb-3 px-2 py-1.5 text-xs"
            style={{
              backgroundColor: 'rgba(232,64,64,0.1)',
              border: '1px solid rgba(232,64,64,0.3)',
              borderRadius: '3px',
              color: 'var(--status-error)',
            }}
          >
            <AlertTriangle size={11} style={{ flexShrink: 0 }} />
            {inlineError}
          </div>
        )}

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
            {settled ? 'Close' : 'Cancel'}
          </button>
          {!settled && (
            <button
              type="button"
              onClick={onConfirm}
              disabled={isPending || unusedVolumes.length === 0}
              className="text-xs px-3 py-1.5"
              style={{
                backgroundColor: 'rgba(232,64,64,0.15)',
                border: '1px solid rgba(232,64,64,0.4)',
                color: 'var(--status-error)',
                borderRadius: '3px',
                cursor: isPending || unusedVolumes.length === 0 ? 'default' : 'pointer',
                opacity: isPending || unusedVolumes.length === 0 ? 0.6 : 1,
              }}
            >
              {isPending ? 'Removing...' : `Remove ${unusedVolumes.length}`}
            </button>
          )}
        </div>
      </div>
    </div>
  )
}

// ---- Volume row ----

interface VolumeRowProps {
  volume: VolumeView
  nodeName: string
  nodeId: string
  isAdmin: boolean
}

function VolumeRow({ volume, nodeName, nodeId, isAdmin }: VolumeRowProps) {
  const [confirming, setConfirming] = useState(false)
  const [inlineError, setInlineError] = useState<string | null>(null)
  const { mutate: removeVolume, isPending } = useRemoveVolume()

  const canRemove = isAdmin && volume.status === 'unused'

  function handleConfirm() {
    setInlineError(null)
    removeVolume(
      { nodeId: volume.node_id, name: volume.name },
      {
        onSuccess: () => setConfirming(false),
        onError: (err) => {
          if (err instanceof ApiError && err.status === 409) {
            setInlineError('Volume is in use — detach all containers first.')
          } else {
            setInlineError('Removal failed. Please try again.')
          }
        },
      },
    )
  }

  const rowStyle: React.CSSProperties = volume.over_threshold
    ? { borderLeft: '3px solid var(--status-warn)', paddingLeft: '9px' }
    : { borderLeft: '3px solid transparent', paddingLeft: '9px' }

  return (
    <>
      <tr style={rowStyle}>
        <td
          className="px-3 py-2 font-mono text-xs"
          style={{ color: 'var(--text-primary)', borderBottom: '1px solid var(--border-subtle)', maxWidth: '180px' }}
        >
          <div className="flex items-center gap-1.5 min-w-0">
            <HardDrive size={11} style={{ color: 'var(--text-muted)', flexShrink: 0 }} />
            <span className="truncate">{volume.name}</span>
            {volume.over_threshold && (
              <AlertTriangle size={10} style={{ color: 'var(--status-warn)', flexShrink: 0 }} aria-label="Above size threshold" />
            )}
          </div>
        </td>
        <td
          className="px-3 py-2 text-xs"
          style={{ borderBottom: '1px solid var(--border-subtle)' }}
        >
          <Link
            to={`/resources?node=${nodeId}`}
            style={{ color: 'var(--text-secondary)', textDecoration: 'none' }}
          >
            {nodeName}
          </Link>
        </td>
        <td
          className="px-3 py-2 font-mono text-xs uppercase"
          style={{ color: 'var(--text-muted)', borderBottom: '1px solid var(--border-subtle)', fontSize: '12px' }}
        >
          {volume.driver}
        </td>
        <td
          className="px-3 py-2 font-mono text-xs"
          style={{ color: 'var(--text-secondary)', borderBottom: '1px solid var(--border-subtle)' }}
        >
          {humanBytes(volume.size_bytes)}
        </td>
        <td
          className="px-3 py-2"
          style={{ borderBottom: '1px solid var(--border-subtle)' }}
        >
          <StatusChip status={volume.status} />
        </td>
        <td
          className="px-3 py-2 text-xs"
          style={{ color: 'var(--text-muted)', borderBottom: '1px solid var(--border-subtle)', maxWidth: '200px' }}
        >
          {(volume.attached_containers ?? []).length > 0
            ? (volume.attached_containers ?? []).join(', ')
            : '—'}
        </td>
        <td
          className="px-3 py-2"
          style={{ borderBottom: '1px solid var(--border-subtle)' }}
        >
          <Sparkline samples={volume.samples ?? []} />
        </td>
        <td
          className="px-3 py-2"
          style={{ borderBottom: '1px solid var(--border-subtle)', width: '48px' }}
        >
          {canRemove && (
            <button
              type="button"
              onClick={() => { setInlineError(null); setConfirming(true) }}
              title={`Remove ${volume.name}`}
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
              Remove
            </button>
          )}
        </td>
      </tr>
      {confirming && (
        <ConfirmDialog
          volumeName={volume.name}
          nodeName={nodeName}
          onConfirm={handleConfirm}
          onCancel={() => { setConfirming(false); setInlineError(null) }}
          isPending={isPending}
          inlineError={inlineError}
        />
      )}
    </>
  )
}

// ---- Section header ----

function SectionHeader({ label, count }: { label: string; count?: number }) {
  return (
    <div
      className="px-0 py-2 mb-3 flex items-center gap-2"
      style={{ borderBottom: '1px solid var(--border-subtle)' }}
    >
      <span className="text-xs font-medium uppercase tracking-wider" style={{ color: 'var(--text-muted)' }}>
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

// ---- Main page ----

export default function Volumes() {
  const { data: me } = useMe()
  const isAdmin = me?.role === 'admin'

  const { data: tree } = useTree()
  const nodes = tree?.nodes ?? []

  const { data, isLoading } = useVolumes()
  const volumes = data?.volumes ?? []

  // ---- Filter state ----
  const [unusedOnly, setUnusedOnly] = useState(false)

  // ---- Prune dialog state ----
  const [pruneOpen, setPruneOpen] = useState(false)
  const [pruneError, setPruneError] = useState<string | null>(null)
  const [pruneResults, setPruneResults] = useState<PruneUnusedResult[] | null>(null)

  const { mutate: pruneUnused, isPending: pruneIsPending } = usePruneUnusedVolumes()

  function nodeName(nodeId: string): string {
    return nodes.find((n) => n.id === nodeId)?.name ?? nodeId
  }

  const unusedVolumes = volumes.filter((v) => v.status === 'unused')
  const displayedVolumes = unusedOnly ? unusedVolumes : volumes

  function handlePruneOpen() {
    setPruneError(null)
    setPruneResults(null)
    setPruneOpen(true)
  }

  function handlePruneConfirm() {
    setPruneError(null)
    pruneUnused(
      {},
      {
        onSuccess: (resp) => {
          setPruneResults(resp.results)
        },
        onError: (err) => {
          if (err instanceof ApiError && err.status === 428) {
            // apiFetch already handled the step-up modal and retry;
            // if we still get here it means the user cancelled the 2FA prompt.
            setPruneError('Step-up authentication was cancelled.')
          } else {
            setPruneError('Bulk removal failed. Please try again.')
          }
        },
      },
    )
  }

  function handlePruneClose() {
    setPruneOpen(false)
    setPruneResults(null)
    setPruneError(null)
  }

  const TABLE_COLS = ['Name', 'Node', 'Driver', 'Size', 'Status', 'Attached Containers', 'Trend', '']

  return (
    <AppShell>
      {/* max-w-full prevents page-level horizontal overflow on narrow viewports */}
      <div
        className="flex flex-col flex-1 min-h-0 h-full w-full max-w-full p-6"
        style={{ maxWidth: '1100px', margin: '0 auto' }}
      >
        {/* Page header */}
        <div className="flex items-center justify-between gap-4 mb-6">
          <div className="flex items-center gap-2">
            <Database size={16} style={{ color: 'var(--text-secondary)' }} />
            <h1
              className="text-sm font-medium uppercase tracking-wider"
              style={{ color: 'var(--text-primary)' }}
            >
              Volume Health
            </h1>
          </div>

          {/* Toolbar — filter + bulk prune; flex-wrap so buttons stack on narrow screens */}
          <div className="flex items-center gap-2 flex-wrap">
            {/* Unused-only toggle */}
            <button
              type="button"
              onClick={() => setUnusedOnly((v) => !v)}
              className="flex items-center gap-1.5 text-xs px-2.5 py-1.5"
              style={{
                backgroundColor: unusedOnly ? 'rgba(240,160,32,0.15)' : 'var(--bg-surface)',
                border: `1px solid ${unusedOnly ? 'rgba(240,160,32,0.45)' : 'var(--border-default)'}`,
                color: unusedOnly ? 'var(--status-warn)' : 'var(--text-secondary)',
                borderRadius: '3px',
                cursor: 'pointer',
                transition: 'background 0.1s, border-color 0.1s, color 0.1s',
              }}
              aria-pressed={unusedOnly}
            >
              <Filter size={10} />
              Unused only
              {unusedVolumes.length > 0 && (
                <span
                  className="font-mono ml-0.5"
                  style={{
                    backgroundColor: unusedOnly ? 'rgba(240,160,32,0.25)' : 'rgba(74,82,104,0.2)',
                    borderRadius: '3px',
                    padding: '0 4px',
                    fontSize: '11px',
                  }}
                >
                  {unusedVolumes.length}
                </span>
              )}
            </button>

            {/* Remove all unused — admin only */}
            {isAdmin && unusedVolumes.length > 0 && (
              <button
                type="button"
                onClick={handlePruneOpen}
                className="flex items-center gap-1.5 text-xs px-2.5 py-1.5"
                style={{
                  backgroundColor: 'rgba(232,64,64,0.1)',
                  border: '1px solid rgba(232,64,64,0.35)',
                  color: 'var(--status-error)',
                  borderRadius: '3px',
                  cursor: 'pointer',
                }}
              >
                <Trash2 size={10} />
                Remove all unused ({unusedVolumes.length})
              </button>
            )}
          </div>
        </div>

        {/* Loading */}
        {isLoading && (
          <div className="flex items-center gap-2 py-8">
            <Loader size={13} className="animate-spin" style={{ color: 'var(--accent)' }} />
            <span className="text-xs" style={{ color: 'var(--text-muted)' }}>
              Loading volumes...
            </span>
          </div>
        )}

        {/* Content */}
        {!isLoading && (
          <>
            <SectionHeader
              label={unusedOnly ? 'Unused Volumes' : 'All Volumes'}
              count={displayedVolumes.length}
            />
            {displayedVolumes.length === 0 ? (
              <div
                className="px-3 py-4 text-xs"
                style={{
                  color: 'var(--text-muted)',
                  backgroundColor: 'var(--bg-surface)',
                  border: '1px solid var(--border-subtle)',
                  borderRadius: '3px',
                }}
              >
                {unusedOnly ? 'No unused volumes found.' : 'No volumes found.'}
              </div>
            ) : (
              <div
                style={{
                  backgroundColor: 'var(--bg-surface)',
                  border: '1px solid var(--border-subtle)',
                  borderRadius: '3px',
                  overflowX: 'auto',
                }}
              >
                <table style={{ width: '100%', borderCollapse: 'collapse' }}>
                  <thead>
                    <tr>
                      {TABLE_COLS.map((col) => (
                        <th
                          key={col}
                          className="px-3 py-2 text-left text-xs uppercase tracking-wider font-medium"
                          style={{
                            color: 'var(--text-muted)',
                            borderBottom: '1px solid var(--border-subtle)',
                            whiteSpace: 'nowrap',
                          }}
                        >
                          {col}
                        </th>
                      ))}
                    </tr>
                  </thead>
                  <tbody>
                    {displayedVolumes.map((vol) => (
                      <VolumeRow
                        key={`${vol.node_id}:${vol.name}`}
                        volume={vol}
                        nodeName={nodeName(vol.node_id)}
                        nodeId={vol.node_id}
                        isAdmin={isAdmin}
                      />
                    ))}
                  </tbody>
                </table>
              </div>
            )}

            {/* Legend */}
            <div
              className="flex items-start gap-2 mt-5 px-3 py-2.5 text-xs"
              style={{
                backgroundColor: 'rgba(240,160,32,0.07)',
                border: '1px solid rgba(240,160,32,0.2)',
                borderRadius: '3px',
                color: 'var(--text-secondary)',
              }}
            >
              <AlertTriangle size={12} style={{ color: 'var(--status-warn)', flexShrink: 0, marginTop: '1px' }} />
              <span>
                Volumes marked <strong style={{ color: 'var(--status-warn)' }}>unused</strong> have no
                attached containers and may be safe to remove. Rows with an orange left border exceed
                the configured size threshold.
              </span>
            </div>
          </>
        )}

        {/* Prune confirm / results dialog */}
        {pruneOpen && (
          <PruneConfirmDialog
            unusedVolumes={unusedVolumes}
            nodeNameFn={nodeName}
            onConfirm={handlePruneConfirm}
            onCancel={handlePruneClose}
            isPending={pruneIsPending}
            inlineError={pruneError}
            results={pruneResults}
          />
        )}
      </div>
    </AppShell>
  )
}
