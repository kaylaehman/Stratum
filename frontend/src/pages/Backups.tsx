import { useState } from 'react'
import {
  Archive,
  Play,
  Loader,
  CheckCircle,
  XCircle,
  AlertTriangle,
  RotateCcw,
  ShieldCheck,
  Download,
  ChevronDown,
} from 'lucide-react'
import { AppShell } from '../components/layout/AppShell'
import { useMe } from '../hooks/useMe'
import { useTree } from '../lib/api/tree'
import { useVolumes } from '../lib/api/volumes'
import {
  useBackups,
  useStartBackup,
  useStartGuestBackup,
  useRestoreDocker,
  useRestoreGuest,
  useVerifyBackup,
  useVerifyResults,
} from '../lib/api/backups'
import { downloadDRExport } from '../lib/api/drexport'
import type { DRExportFormat } from '../lib/api/drexport'
import type { VerifyResult } from '../lib/api/backups'
import { ApiError } from '../lib/api'
import type { Backup, BackupStatus, TreeNode, VM } from '../types/api'

// ---- Helpers ----

function humanBytes(bytes: number): string {
  if (bytes <= 0) return '—'
  if (bytes < 1024) return `${bytes} B`
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`
  if (bytes < 1024 * 1024 * 1024) return `${(bytes / (1024 * 1024)).toFixed(1)} MB`
  return `${(bytes / (1024 * 1024 * 1024)).toFixed(2)} GB`
}

function fmtTime(iso: string | undefined): string {
  if (!iso) return '—'
  return new Date(iso).toLocaleString()
}

function errorLabel(body: unknown): string {
  if (body && typeof body === 'object' && 'error' in body) {
    const code = (body as { error: string }).error
    if (code === 'invalid_volume_or_dest') return 'Invalid volume name or destination path.'
    if (code === 'docker_not_available') return 'Docker is not available on this node.'
    if (code === 'node_not_found') return 'Node not found.'
    if (code === 'vm_not_found') return 'VM not found on this node.'
    if (code === 'invalid_vmid') return 'Invalid VMID.'
    if (code === 'backup_failed') return 'Backup failed to start.'
  }
  return 'An unexpected error occurred.'
}

// ---- Status chip ----

function StatusChip({ status, error }: { status: BackupStatus; error?: string }) {
  if (status === 'running') {
    return (
      <span
        className="flex items-center gap-1 font-mono text-xs px-1.5 py-0.5 uppercase tracking-wider"
        style={{
          color: 'var(--status-warn)',
          background: 'rgba(240,160,32,0.12)',
          border: '1px solid rgba(240,160,32,0.4)',
          borderRadius: '3px',
          fontSize: '12px',
        }}
      >
        <Loader size={9} className="animate-spin" />
        running
      </span>
    )
  }
  if (status === 'ok') {
    return (
      <span
        className="flex items-center gap-1 font-mono text-xs px-1.5 py-0.5 uppercase tracking-wider"
        style={{
          color: 'var(--status-ok, #40c878)',
          background: 'rgba(64,200,120,0.1)',
          border: '1px solid rgba(64,200,120,0.35)',
          borderRadius: '3px',
          fontSize: '12px',
        }}
        title={error}
      >
        <CheckCircle size={9} />
        ok
      </span>
    )
  }
  return (
    <span
      className="flex items-center gap-1 font-mono text-xs px-1.5 py-0.5 uppercase tracking-wider"
      style={{
        color: 'var(--status-error)',
        background: 'rgba(232,64,64,0.1)',
        border: '1px solid rgba(232,64,64,0.3)',
        borderRadius: '3px',
        fontSize: '12px',
        maxWidth: '200px',
      }}
      title={error}
    >
      <XCircle size={9} style={{ flexShrink: 0 }} />
      <span className="truncate">{error ?? 'error'}</span>
    </span>
  )
}

// ---- Restore confirm dialog ----

interface RestoreDialogProps {
  backup: Backup
  nodeName: string
  onClose: () => void
}

function RestoreDialog({ backup, nodeName, onClose }: RestoreDialogProps) {
  const isGuest = backup.kind === 'proxmox-guest'

  // Docker-specific fields
  const [targetPath, setTargetPath] = useState('')

  // Proxmox-specific fields
  const [pveNode, setPveNode] = useState('')
  const [targetStorage, setTargetStorage] = useState('local')
  const [targetVmid, setTargetVmid] = useState('')

  const [result, setResult] = useState<{ output: string } | null>(null)
  const [err, setErr] = useState<string | null>(null)

  const { mutate: restoreDocker, isPending: pendingDocker } = useRestoreDocker()
  const { mutate: restoreGuest, isPending: pendingGuest } = useRestoreGuest()
  const isPending = pendingDocker || pendingGuest

  const inputStyle: React.CSSProperties = {
    backgroundColor: 'var(--bg-surface)',
    border: '1px solid var(--border-default)',
    color: 'var(--text-primary)',
    borderRadius: '3px',
    padding: '5px 8px',
    fontSize: '12px',
    fontFamily: 'inherit',
    width: '100%',
  }

  function handleRestore() {
    setErr(null)
    setResult(null)

    if (isGuest) {
      if (!pveNode.trim() || !targetStorage.trim()) {
        setErr('PVE node and target storage are required.')
        return
      }
      restoreGuest(
        {
          nodeId: backup.node_id,
          pveNode: pveNode.trim(),
          archivePath: backup.dest_path,
          targetStorage: targetStorage.trim(),
          targetVmid: targetVmid.trim() !== '' ? parseInt(targetVmid.trim(), 10) : undefined,
        },
        {
          onSuccess: (res) => setResult({ output: res.output }),
          onError: (e) => setErr(e instanceof ApiError ? errorLabel(e.body) : 'Restore failed.'),
        },
      )
    } else {
      if (!targetPath.trim()) {
        setErr('Target path is required.')
        return
      }
      restoreDocker(
        {
          nodeId: backup.node_id,
          archivePath: backup.dest_path,
          targetPath: targetPath.trim(),
        },
        {
          onSuccess: (res) => setResult({ output: res.output }),
          onError: (e) => setErr(e instanceof ApiError ? errorLabel(e.body) : 'Restore failed.'),
        },
      )
    }
  }

  return (
    <div
      style={{
        position: 'fixed',
        inset: 0,
        zIndex: 200,
        background: 'rgba(0,0,0,0.6)',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
      }}
      onClick={(e) => { if (e.target === e.currentTarget) onClose() }}
    >
      <div
        style={{
          backgroundColor: 'var(--bg-elevated)',
          border: '1px solid var(--border-default)',
          borderRadius: '4px',
          padding: '24px',
          width: '480px',
          maxWidth: '95vw',
          maxHeight: '80vh',
          overflowY: 'auto',
        }}
      >
        {/* Header */}
        <div className="flex items-center gap-2 mb-4">
          <RotateCcw size={14} style={{ color: 'var(--status-error)' }} />
          <span className="text-sm font-medium uppercase tracking-wider" style={{ color: 'var(--text-primary)' }}>
            Restore Backup
          </span>
        </div>

        {/* Destructive warning */}
        <div
          className="flex items-start gap-2 mb-4 px-3 py-2 text-xs"
          style={{
            backgroundColor: 'rgba(232,64,64,0.08)',
            border: '1px solid rgba(232,64,64,0.3)',
            borderRadius: '3px',
            color: 'var(--status-error)',
          }}
        >
          <AlertTriangle size={12} style={{ flexShrink: 0, marginTop: '1px' }} />
          <div>
            <strong>Destructive operation.</strong> This will overwrite the target{' '}
            {isGuest ? 'guest' : 'path'} with the contents of this archive. This cannot be undone.
          </div>
        </div>

        {/* Archive info */}
        <div className="mb-4 text-xs" style={{ color: 'var(--text-secondary)' }}>
          <div><span style={{ color: 'var(--text-muted)' }}>Node:</span> {nodeName}</div>
          <div className="mt-1 font-mono" style={{ color: 'var(--text-primary)', wordBreak: 'break-all' }}>
            <span style={{ color: 'var(--text-muted)', fontFamily: 'inherit' }}>Archive:</span>{' '}
            {backup.dest_path}
          </div>
          <div className="mt-1">
            <span style={{ color: 'var(--text-muted)' }}>Size:</span> {humanBytes(backup.size_bytes)}
            {' '}&middot; backed up {fmtTime(backup.finished_at)}
          </div>
        </div>

        {/* Fields */}
        {!result && (
          <div className="flex flex-col gap-3 mb-4">
            {isGuest ? (
              <>
                <div>
                  <label className="block text-xs mb-1" style={{ color: 'var(--text-muted)' }}>
                    PVE Node <span style={{ color: 'var(--status-error)' }}>*</span>
                  </label>
                  <input
                    type="text"
                    placeholder="pve"
                    value={pveNode}
                    onChange={(e) => setPveNode(e.target.value)}
                    style={inputStyle}
                    className="font-mono"
                  />
                </div>
                <div>
                  <label className="block text-xs mb-1" style={{ color: 'var(--text-muted)' }}>
                    Target Storage <span style={{ color: 'var(--status-error)' }}>*</span>
                  </label>
                  <input
                    type="text"
                    placeholder="local"
                    value={targetStorage}
                    onChange={(e) => setTargetStorage(e.target.value)}
                    style={inputStyle}
                    className="font-mono"
                  />
                </div>
                <div>
                  <label className="block text-xs mb-1" style={{ color: 'var(--text-muted)' }}>
                    Target VMID <span style={{ color: 'var(--text-muted)' }}>(leave blank to auto-assign)</span>
                  </label>
                  <input
                    type="number"
                    placeholder="optional"
                    value={targetVmid}
                    onChange={(e) => setTargetVmid(e.target.value)}
                    style={inputStyle}
                    className="font-mono"
                  />
                </div>
              </>
            ) : (
              <div>
                <label className="block text-xs mb-1" style={{ color: 'var(--text-muted)' }}>
                  Restore to path <span style={{ color: 'var(--status-error)' }}>*</span>
                </label>
                <input
                  type="text"
                  placeholder="/var/lib/docker/volumes/myvolume/_data"
                  value={targetPath}
                  onChange={(e) => setTargetPath(e.target.value)}
                  style={inputStyle}
                  className="font-mono"
                />
                <p className="mt-1 text-xs" style={{ color: 'var(--text-muted)' }}>
                  Contents of the archive will be extracted into this directory.
                </p>
              </div>
            )}
          </div>
        )}

        {/* Error */}
        {err && (
          <div
            className="flex items-center gap-2 mb-3 px-2 py-1.5 text-xs"
            style={{
              backgroundColor: 'rgba(232,64,64,0.08)',
              border: '1px solid rgba(232,64,64,0.3)',
              borderRadius: '3px',
              color: 'var(--status-error)',
            }}
          >
            <AlertTriangle size={11} style={{ flexShrink: 0 }} />
            {err}
          </div>
        )}

        {/* Success: output */}
        {result && (
          <div
            className="mb-4"
            style={{
              backgroundColor: 'rgba(64,200,120,0.06)',
              border: '1px solid rgba(64,200,120,0.25)',
              borderRadius: '3px',
              padding: '10px 12px',
            }}
          >
            <div className="flex items-center gap-1.5 mb-2 text-xs font-medium" style={{ color: 'var(--status-ok, #40c878)' }}>
              <CheckCircle size={12} />
              Restore completed
            </div>
            <pre
              className="text-xs font-mono overflow-auto"
              style={{
                color: 'var(--text-secondary)',
                whiteSpace: 'pre-wrap',
                wordBreak: 'break-all',
                maxHeight: '200px',
              }}
            >
              {result.output || '(no output)'}
            </pre>
          </div>
        )}

        {/* Actions */}
        <div className="flex gap-2 justify-end mt-2">
          <button
            onClick={onClose}
            className="text-xs px-3 py-1.5"
            style={{
              backgroundColor: 'transparent',
              border: '1px solid var(--border-default)',
              color: 'var(--text-secondary)',
              borderRadius: '3px',
              cursor: 'pointer',
            }}
          >
            {result ? 'Close' : 'Cancel'}
          </button>

          {!result && (
            <button
              onClick={handleRestore}
              disabled={isPending}
              className="flex items-center gap-1.5 text-xs px-3 py-1.5"
              style={{
                backgroundColor: 'rgba(232,64,64,0.12)',
                border: '1px solid rgba(232,64,64,0.5)',
                color: 'var(--status-error)',
                borderRadius: '3px',
                cursor: isPending ? 'default' : 'pointer',
                opacity: isPending ? 0.6 : 1,
              }}
            >
              {isPending
                ? <><Loader size={11} className="animate-spin" />Restoring…</>
                : <><RotateCcw size={11} />Restore</>
              }
            </button>
          )}
        </div>
      </div>
    </div>
  )
}

// ---- Verify panel ----

interface VerifyPanelProps {
  nodeId: string
  isOperator: boolean
}

function VerifyPanel({ nodeId, isOperator }: VerifyPanelProps) {
  const [expanded, setExpanded] = useState(false)
  const { mutate: runVerify, isPending, data: latestResult, error } = useVerifyBackup(nodeId)
  const { data: history } = useVerifyResults(nodeId, expanded)

  const verifyResults: VerifyResult[] = history?.results ?? []

  const lastRun = latestResult ?? verifyResults[0] ?? null

  function chipFor(passed: boolean) {
    return passed
      ? (
        <span
          className="flex items-center gap-1 font-mono text-xs px-1.5 py-0.5 uppercase tracking-wider"
          style={{
            color: 'var(--status-ok, #40c878)',
            background: 'rgba(64,200,120,0.1)',
            border: '1px solid rgba(64,200,120,0.35)',
            borderRadius: '3px',
            fontSize: '11px',
          }}
        >
          <CheckCircle size={9} />passed
        </span>
      )
      : (
        <span
          className="flex items-center gap-1 font-mono text-xs px-1.5 py-0.5 uppercase tracking-wider"
          style={{
            color: 'var(--status-error)',
            background: 'rgba(232,64,64,0.1)',
            border: '1px solid rgba(232,64,64,0.3)',
            borderRadius: '3px',
            fontSize: '11px',
          }}
        >
          <XCircle size={9} />failed
        </span>
      )
  }

  return (
    <div
      style={{
        backgroundColor: 'var(--bg-elevated)',
        border: '1px solid var(--border-default)',
        borderRadius: '3px',
        padding: '14px 18px',
        marginBottom: '16px',
      }}
    >
      <div className="flex items-center gap-2 mb-3">
        <ShieldCheck size={13} style={{ color: 'var(--text-secondary)' }} />
        <span className="text-xs font-medium uppercase tracking-wider" style={{ color: 'var(--text-primary)' }}>
          Backup Verification
        </span>
        <button
          onClick={() => setExpanded((v) => !v)}
          className="ml-auto flex items-center gap-1 text-xs"
          style={{ color: 'var(--text-muted)', background: 'none', border: 'none', cursor: 'pointer' }}
        >
          {expanded ? 'Hide history' : 'Show history'}
          <ChevronDown
            size={11}
            style={{
              transform: expanded ? 'rotate(180deg)' : 'none',
              transition: 'transform 0.15s',
            }}
          />
        </button>
      </div>

      <div className="flex flex-wrap items-center gap-3">
        {isOperator && (
          <button
            onClick={() => { runVerify() }}
            disabled={isPending}
            className="flex items-center gap-1.5 text-xs px-3 py-1.5"
            style={{
              backgroundColor: 'var(--accent-glow)',
              border: '1px solid var(--accent)',
              color: 'var(--accent)',
              borderRadius: '3px',
              cursor: isPending ? 'default' : 'pointer',
              opacity: isPending ? 0.6 : 1,
            }}
          >
            {isPending
              ? <><Loader size={11} className="animate-spin" />Verifying…</>
              : <><ShieldCheck size={11} />Verify latest</>
            }
          </button>
        )}

        {lastRun && (
          <div className="flex items-center gap-2 text-xs" style={{ color: 'var(--text-secondary)' }}>
            {chipFor(lastRun.passed)}
            <span className="font-mono">{lastRun.file_count} files</span>
            <span style={{ color: 'var(--text-muted)' }}>&middot;</span>
            <span className="font-mono">{humanBytes(lastRun.total_bytes)}</span>
            <span style={{ color: 'var(--text-muted)' }}>&middot;</span>
            <span style={{ color: 'var(--text-muted)', fontSize: '11px' }}>{fmtTime(lastRun.checked_at)}</span>
          </div>
        )}

        {error && (
          <span className="flex items-center gap-1 text-xs" style={{ color: 'var(--status-error)' }}>
            <AlertTriangle size={11} />
            {error instanceof ApiError ? errorLabel(error.body) : 'Verify failed.'}
          </span>
        )}
      </div>

      {expanded && verifyResults.length > 0 && (
        <div
          style={{
            marginTop: '12px',
            borderTop: '1px solid var(--border-subtle)',
            paddingTop: '10px',
          }}
        >
          <table style={{ width: '100%', borderCollapse: 'collapse' }}>
            <thead>
              <tr>
                {['Result', 'Files', 'Size', 'Archive', 'Checked At'].map((col) => (
                  <th
                    key={col}
                    className="px-2 py-1 text-left text-xs uppercase tracking-wider font-medium"
                    style={{ color: 'var(--text-muted)', borderBottom: '1px solid var(--border-subtle)', whiteSpace: 'nowrap' }}
                  >
                    {col}
                  </th>
                ))}
              </tr>
            </thead>
            <tbody>
              {verifyResults.map((r, i) => (
                <tr key={i}>
                  <td className="px-2 py-1.5" style={{ borderBottom: '1px solid var(--border-subtle)' }}>
                    {chipFor(r.passed)}
                  </td>
                  <td className="px-2 py-1.5 font-mono text-xs" style={{ color: 'var(--text-secondary)', borderBottom: '1px solid var(--border-subtle)' }}>
                    {r.file_count}
                  </td>
                  <td className="px-2 py-1.5 font-mono text-xs" style={{ color: 'var(--text-secondary)', borderBottom: '1px solid var(--border-subtle)' }}>
                    {humanBytes(r.total_bytes)}
                  </td>
                  <td
                    className="px-2 py-1.5 font-mono text-xs"
                    style={{ color: 'var(--text-muted)', borderBottom: '1px solid var(--border-subtle)', maxWidth: '200px' }}
                  >
                    <span className="truncate block" title={r.archive_path}>{r.archive_path}</span>
                  </td>
                  <td className="px-2 py-1.5 font-mono text-xs" style={{ color: 'var(--text-muted)', borderBottom: '1px solid var(--border-subtle)', fontSize: '11px' }}>
                    {fmtTime(r.checked_at)}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {expanded && verifyResults.length === 0 && (
        <p className="text-xs mt-3" style={{ color: 'var(--text-muted)' }}>
          No verify runs recorded yet.
        </p>
      )}
    </div>
  )
}

// ---- DR Export dropdown ----

function DRExportDropdown() {
  const [open, setOpen] = useState(false)
  const [busy, setBusy] = useState(false)
  const [err, setErr] = useState<string | null>(null)

  async function handleExport(fmt: DRExportFormat) {
    setOpen(false)
    setBusy(true)
    setErr(null)
    try {
      await downloadDRExport(fmt)
    } catch {
      setErr('DR export failed.')
    } finally {
      setBusy(false)
    }
  }

  return (
    <div style={{ position: 'relative' }}>
      <button
        onClick={() => setOpen((v) => !v)}
        disabled={busy}
        className="flex items-center gap-1.5 text-xs px-3 py-1.5"
        style={{
          backgroundColor: 'var(--bg-surface)',
          border: '1px solid var(--border-default)',
          color: 'var(--text-secondary)',
          borderRadius: '3px',
          cursor: busy ? 'default' : 'pointer',
          opacity: busy ? 0.6 : 1,
        }}
      >
        {busy
          ? <><Loader size={11} className="animate-spin" />Exporting…</>
          : <><Download size={11} />Export DR manifest<ChevronDown size={10} /></>
        }
      </button>

      {open && (
        <>
          {/* backdrop */}
          <div
            style={{ position: 'fixed', inset: 0, zIndex: 99 }}
            onClick={() => setOpen(false)}
          />
          <div
            style={{
              position: 'absolute',
              right: 0,
              top: 'calc(100% + 4px)',
              zIndex: 100,
              backgroundColor: 'var(--bg-elevated)',
              border: '1px solid var(--border-default)',
              borderRadius: '3px',
              minWidth: '140px',
              boxShadow: '0 4px 12px rgba(0,0,0,0.3)',
            }}
          >
            {(['json', 'yaml', 'md'] as const).map((fmt) => (
              <button
                key={fmt}
                onClick={() => { void handleExport(fmt) }}
                className="flex items-center gap-2 w-full text-left px-3 py-2 text-xs"
                style={{
                  background: 'none',
                  border: 'none',
                  color: 'var(--text-primary)',
                  cursor: 'pointer',
                  fontFamily: 'inherit',
                }}
                onMouseEnter={(e) => {
                  (e.currentTarget as HTMLButtonElement).style.backgroundColor = 'var(--bg-surface)'
                }}
                onMouseLeave={(e) => {
                  (e.currentTarget as HTMLButtonElement).style.backgroundColor = 'transparent'
                }}
              >
                <Download size={10} style={{ color: 'var(--text-muted)' }} />
                <span className="font-mono uppercase">{fmt}</span>
              </button>
            ))}
          </div>
        </>
      )}

      {err && (
        <span
          className="absolute text-xs mt-1"
          style={{ color: 'var(--status-error)', top: '100%', right: 0, whiteSpace: 'nowrap' }}
        >
          {err}
        </span>
      )}
    </div>
  )
}

// ---- History table row ----

interface BackupRowProps {
  backup: Backup
  nodeName: string
  isAdmin: boolean
  onRestore: (backup: Backup) => void
}

function BackupRow({ backup, nodeName, isAdmin, onRestore }: BackupRowProps) {
  return (
    <tr>
      <td className="px-3 py-2 text-xs" style={{ color: 'var(--text-secondary)', borderBottom: '1px solid var(--border-subtle)' }}>
        {nodeName}
      </td>
      <td className="px-3 py-2 font-mono text-xs" style={{ color: 'var(--text-muted)', borderBottom: '1px solid var(--border-subtle)', textTransform: 'uppercase', letterSpacing: '0.05em', fontSize: '11px' }}>
        {backup.kind}
      </td>
      <td className="px-3 py-2 font-mono text-xs" style={{ color: 'var(--text-primary)', borderBottom: '1px solid var(--border-subtle)' }}>
        {backup.target}
      </td>
      <td className="px-3 py-2" style={{ borderBottom: '1px solid var(--border-subtle)' }}>
        <StatusChip status={backup.status} error={backup.error} />
      </td>
      <td className="px-3 py-2 font-mono text-xs" style={{ color: 'var(--text-secondary)', borderBottom: '1px solid var(--border-subtle)' }}>
        {backup.status === 'running' ? '—' : humanBytes(backup.size_bytes)}
      </td>
      <td className="px-3 py-2 font-mono text-xs" style={{ color: 'var(--text-muted)', borderBottom: '1px solid var(--border-subtle)', fontSize: '12px' }}>
        {fmtTime(backup.started_at)}
      </td>
      <td className="px-3 py-2 font-mono text-xs" style={{ color: 'var(--text-muted)', borderBottom: '1px solid var(--border-subtle)', fontSize: '12px' }}>
        {fmtTime(backup.finished_at)}
      </td>
      <td className="px-3 py-2 font-mono text-xs" style={{ color: 'var(--text-muted)', borderBottom: '1px solid var(--border-subtle)', maxWidth: '200px' }}>
        <span className="truncate block" title={backup.dest_path}>{backup.dest_path}</span>
      </td>
      <td className="px-3 py-2" style={{ borderBottom: '1px solid var(--border-subtle)' }}>
        {isAdmin && backup.status === 'ok' && backup.dest_path && (
          <button
            onClick={() => onRestore(backup)}
            className="flex items-center gap-1 text-xs px-2 py-1"
            style={{
              backgroundColor: 'rgba(232,64,64,0.08)',
              border: '1px solid rgba(232,64,64,0.3)',
              color: 'var(--status-error)',
              borderRadius: '3px',
              cursor: 'pointer',
              whiteSpace: 'nowrap',
            }}
          >
            <RotateCcw size={10} />
            Restore
          </button>
        )}
      </td>
    </tr>
  )
}

// ---- Docker volume trigger form ----

interface DockerTriggerProps {
  nodeOptions: Array<{ id: string; name: string }>
  volumeOptions: string[]
  onNodeChange: (id: string) => void
}

function DockerTriggerFields({ nodeOptions, volumeOptions, onNodeChange }: DockerTriggerProps) {
  const [selectedNode, setSelectedNode] = useState(nodeOptions[0]?.id ?? '')
  const [volume, setVolume] = useState('')
  const [destDir, setDestDir] = useState('/mnt/backups')
  const [feedback, setFeedback] = useState<{ kind: 'ok' | 'error'; msg: string } | null>(null)

  const { mutate: startBackup, isPending } = useStartBackup()

  const inputStyle: React.CSSProperties = {
    backgroundColor: 'var(--bg-surface)',
    border: '1px solid var(--border-default)',
    color: 'var(--text-primary)',
    borderRadius: '3px',
    padding: '5px 8px',
    fontSize: '12px',
    fontFamily: 'inherit',
    width: '100%',
  }

  function handleNodeChange(id: string) {
    setSelectedNode(id)
    setVolume('')
    onNodeChange(id)
  }

  function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    setFeedback(null)
    if (!selectedNode || !volume.trim() || !destDir.trim()) return

    startBackup(
      { nodeId: selectedNode, volume: volume.trim(), destDir: destDir.trim() },
      {
        onSuccess: (res) => {
          setFeedback({ kind: 'ok', msg: `Backup started (ID: ${res.backup_id})` })
        },
        onError: (err) => {
          const msg = err instanceof ApiError ? errorLabel(err.body) : 'Failed to start backup.'
          setFeedback({ kind: 'error', msg })
        },
      },
    )
  }

  return (
    <form onSubmit={handleSubmit}>
      <div className="flex flex-wrap gap-3 items-end">
        <div style={{ minWidth: '160px', flex: '1' }}>
          <label className="block text-xs mb-1" style={{ color: 'var(--text-muted)' }}>Node</label>
          <select value={selectedNode} onChange={(e) => handleNodeChange(e.target.value)} style={inputStyle}>
            {nodeOptions.length === 0 && <option value="">No Docker nodes available</option>}
            {nodeOptions.map((n) => (
              <option key={n.id} value={n.id}>{n.name}</option>
            ))}
          </select>
        </div>

        <div style={{ minWidth: '160px', flex: '1' }}>
          <label className="block text-xs mb-1" style={{ color: 'var(--text-muted)' }}>Volume</label>
          {volumeOptions.length > 0 ? (
            <select value={volume} onChange={(e) => setVolume(e.target.value)} style={inputStyle}>
              <option value="">Select volume…</option>
              {volumeOptions.map((v) => (
                <option key={v} value={v}>{v}</option>
              ))}
            </select>
          ) : (
            <input
              type="text"
              placeholder="volume name"
              value={volume}
              onChange={(e) => setVolume(e.target.value)}
              style={inputStyle}
              className="font-mono"
            />
          )}
        </div>

        <div style={{ minWidth: '180px', flex: '2' }}>
          <label className="block text-xs mb-1" style={{ color: 'var(--text-muted)' }}>Destination directory</label>
          <input
            type="text"
            value={destDir}
            onChange={(e) => setDestDir(e.target.value)}
            style={inputStyle}
            className="font-mono"
          />
        </div>

        <div style={{ flexShrink: 0 }}>
          <button
            type="submit"
            disabled={isPending || !selectedNode || !volume.trim() || !destDir.trim()}
            className="flex items-center gap-1.5 text-xs px-3 py-1.5"
            style={{
              backgroundColor: 'var(--accent-glow)',
              border: '1px solid var(--accent)',
              color: 'var(--accent)',
              borderRadius: '3px',
              cursor: isPending || !selectedNode || !volume.trim() || !destDir.trim() ? 'default' : 'pointer',
              opacity: isPending || !selectedNode || !volume.trim() || !destDir.trim() ? 0.55 : 1,
            }}
          >
            {isPending
              ? <><Loader size={11} className="animate-spin" />Starting…</>
              : <><Play size={11} />Start backup</>
            }
          </button>
        </div>
      </div>

      {feedback && (
        <div
          className="flex items-center gap-2 mt-3 px-2 py-1.5 text-xs"
          style={{
            backgroundColor: feedback.kind === 'ok' ? 'rgba(64,200,120,0.08)' : 'rgba(232,64,64,0.08)',
            border: `1px solid ${feedback.kind === 'ok' ? 'rgba(64,200,120,0.3)' : 'rgba(232,64,64,0.3)'}`,
            borderRadius: '3px',
            color: feedback.kind === 'ok' ? 'var(--status-ok, #40c878)' : 'var(--status-error)',
          }}
        >
          {feedback.kind === 'ok'
            ? <CheckCircle size={11} style={{ flexShrink: 0 }} />
            : <AlertTriangle size={11} style={{ flexShrink: 0 }} />
          }
          {feedback.msg}
        </div>
      )}
    </form>
  )
}

// ---- Proxmox vzdump trigger form ----

interface ProxmoxTriggerProps {
  nodeOptions: Array<{ id: string; name: string; vms: VM[] }>
}

function ProxmoxTriggerFields({ nodeOptions }: ProxmoxTriggerProps) {
  const [selectedNode, setSelectedNode] = useState(nodeOptions[0]?.id ?? '')
  const [selectedVmid, setSelectedVmid] = useState<number | ''>(
    nodeOptions[0]?.vms[0]?.proxmox_vmid ?? '',
  )
  const [storage, setStorage] = useState('local')
  const [feedback, setFeedback] = useState<{ kind: 'ok' | 'error'; msg: string } | null>(null)

  const { mutate: startGuestBackup, isPending } = useStartGuestBackup()

  const inputStyle: React.CSSProperties = {
    backgroundColor: 'var(--bg-surface)',
    border: '1px solid var(--border-default)',
    color: 'var(--text-primary)',
    borderRadius: '3px',
    padding: '5px 8px',
    fontSize: '12px',
    fontFamily: 'inherit',
    width: '100%',
  }

  const selectedNodeObj = nodeOptions.find((n) => n.id === selectedNode)
  const vms = selectedNodeObj?.vms ?? []

  function handleNodeChange(id: string) {
    setSelectedNode(id)
    const node = nodeOptions.find((n) => n.id === id)
    setSelectedVmid(node?.vms[0]?.proxmox_vmid ?? '')
    setFeedback(null)
  }

  function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    setFeedback(null)
    if (!selectedNode || selectedVmid === '') return

    startGuestBackup(
      { nodeId: selectedNode, vmid: selectedVmid as number, storage: storage.trim() },
      {
        onSuccess: (res) => {
          setFeedback({ kind: 'ok', msg: `Backup started (ID: ${res.backup_id})` })
        },
        onError: (err) => {
          const msg = err instanceof ApiError ? errorLabel(err.body) : 'Failed to start backup.'
          setFeedback({ kind: 'error', msg })
        },
      },
    )
  }

  const canSubmit = !isPending && !!selectedNode && selectedVmid !== '' && storage.trim() !== ''

  return (
    <form onSubmit={handleSubmit}>
      <div className="flex flex-wrap gap-3 items-end">
        <div style={{ minWidth: '160px', flex: '1' }}>
          <label className="block text-xs mb-1" style={{ color: 'var(--text-muted)' }}>Node</label>
          <select value={selectedNode} onChange={(e) => handleNodeChange(e.target.value)} style={inputStyle}>
            {nodeOptions.length === 0 && <option value="">No Proxmox nodes available</option>}
            {nodeOptions.map((n) => (
              <option key={n.id} value={n.id}>{n.name}</option>
            ))}
          </select>
        </div>

        <div style={{ minWidth: '160px', flex: '1' }}>
          <label className="block text-xs mb-1" style={{ color: 'var(--text-muted)' }}>VM / LXC</label>
          {vms.length > 0 ? (
            <select
              value={selectedVmid === '' ? '' : String(selectedVmid)}
              onChange={(e) => setSelectedVmid(e.target.value === '' ? '' : Number(e.target.value))}
              style={inputStyle}
            >
              <option value="">Select guest…</option>
              {vms.map((v) => (
                <option key={v.proxmox_vmid} value={String(v.proxmox_vmid)}>
                  {v.name ? `${v.name} (${v.proxmox_vmid})` : String(v.proxmox_vmid)} — {v.kind.toUpperCase()}
                </option>
              ))}
            </select>
          ) : (
            <div
              className="text-xs px-2 py-1.5"
              style={{
                border: '1px solid var(--border-subtle)',
                borderRadius: '3px',
                color: 'var(--text-muted)',
                backgroundColor: 'var(--bg-surface)',
              }}
            >
              No guests enumerated yet
            </div>
          )}
        </div>

        <div style={{ minWidth: '140px', flex: '1' }}>
          <label className="block text-xs mb-1" style={{ color: 'var(--text-muted)' }}>Storage</label>
          <input
            type="text"
            placeholder="local"
            value={storage}
            onChange={(e) => setStorage(e.target.value)}
            style={inputStyle}
            className="font-mono"
          />
        </div>

        <div style={{ flexShrink: 0 }}>
          <button
            type="submit"
            disabled={!canSubmit}
            className="flex items-center gap-1.5 text-xs px-3 py-1.5"
            style={{
              backgroundColor: 'var(--accent-glow)',
              border: '1px solid var(--accent)',
              color: 'var(--accent)',
              borderRadius: '3px',
              cursor: canSubmit ? 'pointer' : 'default',
              opacity: canSubmit ? 1 : 0.55,
            }}
          >
            {isPending
              ? <><Loader size={11} className="animate-spin" />Starting…</>
              : <><Play size={11} />Start backup</>
            }
          </button>
        </div>
      </div>

      {feedback && (
        <div
          className="flex items-center gap-2 mt-3 px-2 py-1.5 text-xs"
          style={{
            backgroundColor: feedback.kind === 'ok' ? 'rgba(64,200,120,0.08)' : 'rgba(232,64,64,0.08)',
            border: `1px solid ${feedback.kind === 'ok' ? 'rgba(64,200,120,0.3)' : 'rgba(232,64,64,0.3)'}`,
            borderRadius: '3px',
            color: feedback.kind === 'ok' ? 'var(--status-ok, #40c878)' : 'var(--status-error)',
          }}
        >
          {feedback.kind === 'ok'
            ? <CheckCircle size={11} style={{ flexShrink: 0 }} />
            : <AlertTriangle size={11} style={{ flexShrink: 0 }} />
          }
          {feedback.msg}
        </div>
      )}
    </form>
  )
}

// ---- Combined trigger panel ----

type BackupKind = 'docker' | 'proxmox'

interface TriggerPanelProps {
  dockerNodes: TreeNode[]
  proxmoxNodes: TreeNode[]
  volumeOptions: string[]
  onDockerNodeChange: (id: string) => void
}

function TriggerPanel({ dockerNodes, proxmoxNodes, volumeOptions, onDockerNodeChange }: TriggerPanelProps) {
  const hasDocker = dockerNodes.length > 0
  const hasProxmox = proxmoxNodes.length > 0
  const defaultKind: BackupKind = hasProxmox ? 'proxmox' : 'docker'
  const [kind, setKind] = useState<BackupKind>(defaultKind)

  const tabStyle = (active: boolean): React.CSSProperties => ({
    padding: '4px 10px',
    fontSize: '11px',
    fontFamily: 'inherit',
    border: 'none',
    borderBottom: active ? '2px solid var(--accent)' : '2px solid transparent',
    background: 'transparent',
    color: active ? 'var(--accent)' : 'var(--text-muted)',
    cursor: 'pointer',
    fontWeight: active ? 600 : 400,
    letterSpacing: '0.04em',
    textTransform: 'uppercase' as const,
  })

  return (
    <div
      style={{
        backgroundColor: 'var(--bg-elevated)',
        border: '1px solid var(--border-default)',
        borderRadius: '3px',
        padding: '16px 20px',
        marginBottom: '16px',
      }}
    >
      <div className="flex items-center gap-2 mb-3">
        <Archive size={13} style={{ color: 'var(--text-secondary)' }} />
        <span className="text-xs font-medium uppercase tracking-wider" style={{ color: 'var(--text-primary)' }}>
          Trigger Backup
        </span>
      </div>

      {/* Kind tabs — only show if both types are available */}
      {hasDocker && hasProxmox && (
        <div className="flex gap-0 mb-4" style={{ borderBottom: '1px solid var(--border-subtle)' }}>
          <button style={tabStyle(kind === 'proxmox')} onClick={() => setKind('proxmox')} type="button">
            Proxmox (vzdump)
          </button>
          <button style={tabStyle(kind === 'docker')} onClick={() => setKind('docker')} type="button">
            Docker volume
          </button>
        </div>
      )}

      {(!hasDocker && !hasProxmox) && (
        <div className="text-xs" style={{ color: 'var(--text-muted)' }}>
          No backup-capable nodes registered.
        </div>
      )}

      {(kind === 'docker' || !hasProxmox) && hasDocker && (
        <DockerTriggerFields
          nodeOptions={dockerNodes.map((n) => ({ id: n.id, name: n.name }))}
          volumeOptions={volumeOptions}
          onNodeChange={onDockerNodeChange}
        />
      )}

      {(kind === 'proxmox' || !hasDocker) && hasProxmox && (
        <ProxmoxTriggerFields
          nodeOptions={proxmoxNodes.map((n) => ({ id: n.id, name: n.name, vms: n.vms }))}
        />
      )}
    </div>
  )
}

// ---- Main page ----

const TABLE_COLS = ['Node', 'Kind', 'Target', 'Status', 'Size', 'Started', 'Finished', 'Destination', '']

export default function Backups() {
  const { data: me } = useMe()
  const isAdmin = me?.role === 'admin'
  const isOperator = me?.role === 'admin' || me?.role === 'operator'

  const { data: tree } = useTree()
  const nodes = tree?.nodes ?? []
  const dockerNodes = [...nodes.filter((n) => n.capabilities.docker)].sort((a, b) =>
    a.name.localeCompare(b.name, undefined, { sensitivity: 'base' }),
  )
  const proxmoxNodes = [
    ...nodes.filter((n) => n.capabilities.proxmox && n.proxmox_auth_status === 'confirmed'),
  ].sort((a, b) => a.name.localeCompare(b.name, undefined, { sensitivity: 'base' }))

  const { data: volumesData } = useVolumes()
  const allVolumes = volumesData?.volumes ?? []

  const { data, isLoading } = useBackups()
  const backups = data?.backups ?? []

  const [selectedDockerNodeId, setSelectedDockerNodeId] = useState<string>('')
  const [restoreTarget, setRestoreTarget] = useState<Backup | null>(null)

  const activeDockerNodeId = selectedDockerNodeId || dockerNodes[0]?.id || ''
  const volumeOptions = allVolumes
    .filter((v) => v.node_id === activeDockerNodeId)
    .map((v) => v.name)

  // Use first docker node for verify panel; per-node in future
  const verifyNodeId = dockerNodes[0]?.id ?? proxmoxNodes[0]?.id ?? ''

  function nodeName(nodeId: string): string {
    return nodes.find((n) => n.id === nodeId)?.name ?? nodeId
  }

  if (!isAdmin) {
    return (
      <AppShell>
        <div className="flex items-center justify-center h-full">
          <div
            className="flex items-center gap-2 px-4 py-3 text-xs"
            style={{
              backgroundColor: 'rgba(232,64,64,0.08)',
              border: '1px solid rgba(232,64,64,0.25)',
              borderRadius: '3px',
              color: 'var(--status-error)',
            }}
          >
            <AlertTriangle size={13} />
            Backups — admin access required.
          </div>
        </div>
      </AppShell>
    )
  }

  return (
    <AppShell>
      <div
        className="flex flex-col flex-1 min-h-0 h-full w-full p-6"
        style={{ maxWidth: '1100px', margin: '0 auto' }}
      >
        {/* Page header */}
        <div className="flex items-center gap-2 mb-6">
          <Archive size={16} style={{ color: 'var(--text-secondary)' }} />
          <h1
            className="text-sm font-medium uppercase tracking-wider"
            style={{ color: 'var(--text-primary)' }}
          >
            Backup Orchestration
          </h1>
          <div className="ml-auto">
            <DRExportDropdown />
          </div>
        </div>

        {/* Trigger panel */}
        <TriggerPanel
          dockerNodes={dockerNodes}
          proxmoxNodes={proxmoxNodes}
          volumeOptions={volumeOptions}
          onDockerNodeChange={setSelectedDockerNodeId}
        />

        {/* Verify panel */}
        {verifyNodeId && (
          <VerifyPanel nodeId={verifyNodeId} isOperator={isOperator} />
        )}

        {/* History header */}
        <div
          className="flex items-center gap-2 mb-3"
          style={{ borderBottom: '1px solid var(--border-subtle)', paddingBottom: '8px' }}
        >
          <span className="text-xs font-medium uppercase tracking-wider" style={{ color: 'var(--text-muted)' }}>
            Backup History
          </span>
          {backups.length > 0 && (
            <span className="text-xs font-mono" style={{ color: 'var(--text-muted)' }}>
              ({backups.length})
            </span>
          )}
          <span
            className="ml-auto text-xs font-mono"
            style={{ color: 'var(--text-muted)', fontSize: '12px' }}
          >
            auto-refreshes every 5s
          </span>
        </div>

        {/* Loading */}
        {isLoading && (
          <div className="flex items-center gap-2 py-8">
            <Loader size={13} className="animate-spin" style={{ color: 'var(--accent)' }} />
            <span className="text-xs" style={{ color: 'var(--text-muted)' }}>
              Loading backups…
            </span>
          </div>
        )}

        {/* Empty */}
        {!isLoading && backups.length === 0 && (
          <div
            className="px-3 py-4 text-xs"
            style={{
              color: 'var(--text-muted)',
              backgroundColor: 'var(--bg-surface)',
              border: '1px solid var(--border-subtle)',
              borderRadius: '3px',
            }}
          >
            No backups yet. Use the form above to trigger one.
          </div>
        )}

        {/* Table */}
        {!isLoading && backups.length > 0 && (
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
                  {TABLE_COLS.map((col, i) => (
                    <th
                      key={i}
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
                {backups.map((b) => (
                  <BackupRow
                    key={b.id}
                    backup={b}
                    nodeName={nodeName(b.node_id)}
                    isAdmin={isAdmin}
                    onRestore={setRestoreTarget}
                  />
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>

      {/* Restore dialog */}
      {restoreTarget && (
        <RestoreDialog
          backup={restoreTarget}
          nodeName={nodeName(restoreTarget.node_id)}
          onClose={() => setRestoreTarget(null)}
        />
      )}
    </AppShell>
  )
}
