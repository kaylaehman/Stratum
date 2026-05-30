import { useState } from 'react'
import { Archive, Play, Loader, CheckCircle, XCircle, AlertTriangle } from 'lucide-react'
import { AppShell } from '../components/layout/AppShell'
import { useMe } from '../hooks/useMe'
import { useTree } from '../lib/api/tree'
import { useVolumes } from '../lib/api/volumes'
import { useBackups, useStartBackup } from '../lib/api/backups'
import { ApiError } from '../lib/api'
import type { Backup, BackupStatus } from '../types/api'

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

// ---- History table row ----

interface BackupRowProps {
  backup: Backup
  nodeName: string
}

function BackupRow({ backup, nodeName }: BackupRowProps) {
  return (
    <tr>
      <td className="px-3 py-2 text-xs" style={{ color: 'var(--text-secondary)', borderBottom: '1px solid var(--border-subtle)' }}>
        {nodeName}
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
    </tr>
  )
}

// ---- Trigger form ----

interface TriggerFormProps {
  nodeOptions: Array<{ id: string; name: string }>
  volumeOptions: string[]
  onNodeChange: (id: string) => void
}

function TriggerForm({ nodeOptions, volumeOptions, onNodeChange }: TriggerFormProps) {
  const [selectedNode, setSelectedNode] = useState(nodeOptions[0]?.id ?? '')
  const [volume, setVolume] = useState('')
  const [destDir, setDestDir] = useState('/mnt/backups')
  const [feedback, setFeedback] = useState<{ kind: 'ok' | 'error'; msg: string } | null>(null)

  const { mutate: startBackup, isPending } = useStartBackup()

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
          const msg = err instanceof ApiError
            ? errorLabel(err.body)
            : 'Failed to start backup.'
          setFeedback({ kind: 'error', msg })
        },
      },
    )
  }

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

  return (
    <form onSubmit={handleSubmit}>
      <div
        style={{
          backgroundColor: 'var(--bg-elevated)',
          border: '1px solid var(--border-default)',
          borderRadius: '3px',
          padding: '16px 20px',
          marginBottom: '24px',
        }}
      >
        <div className="flex items-center gap-2 mb-4">
          <Archive size={13} style={{ color: 'var(--text-secondary)' }} />
          <span className="text-xs font-medium uppercase tracking-wider" style={{ color: 'var(--text-primary)' }}>
            Trigger Backup
          </span>
        </div>

        <div className="flex flex-wrap gap-3 items-end">
          {/* Node selector */}
          <div style={{ minWidth: '160px', flex: '1' }}>
            <label className="block text-xs mb-1" style={{ color: 'var(--text-muted)' }}>Node</label>
            <select
              value={selectedNode}
              onChange={(e) => handleNodeChange(e.target.value)}
              style={inputStyle}
            >
              {nodeOptions.length === 0 && (
                <option value="">No docker nodes available</option>
              )}
              {nodeOptions.map((n) => (
                <option key={n.id} value={n.id}>{n.name}</option>
              ))}
            </select>
          </div>

          {/* Volume */}
          <div style={{ minWidth: '160px', flex: '1' }}>
            <label className="block text-xs mb-1" style={{ color: 'var(--text-muted)' }}>Volume</label>
            {volumeOptions.length > 0 ? (
              <select
                value={volume}
                onChange={(e) => setVolume(e.target.value)}
                style={inputStyle}
              >
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

          {/* Destination directory */}
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

          {/* Submit */}
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

        {/* Feedback */}
        {feedback && (
          <div
            className="flex items-center gap-2 mt-3 px-2 py-1.5 text-xs"
            style={{
              backgroundColor: feedback.kind === 'ok'
                ? 'rgba(64,200,120,0.08)'
                : 'rgba(232,64,64,0.08)',
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
      </div>
    </form>
  )
}

// ---- Main page ----

const TABLE_COLS = ['Node', 'Volume', 'Status', 'Size', 'Started', 'Finished', 'Destination']

export default function Backups() {
  const { data: me } = useMe()
  const isAdmin = me?.role === 'admin'

  const { data: tree } = useTree()
  const nodes = tree?.nodes ?? []
  const dockerNodes = nodes.filter((n) => n.capabilities.docker)

  const { data: volumesData } = useVolumes()
  const allVolumes = volumesData?.volumes ?? []

  const { data, isLoading } = useBackups()
  const backups = data?.backups ?? []

  const [selectedNodeId, setSelectedNodeId] = useState<string>('')

  const activeNodeId = selectedNodeId || dockerNodes[0]?.id || ''
  const volumeOptions = allVolumes
    .filter((v) => v.node_id === activeNodeId)
    .map((v) => v.name)

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
        </div>

        {/* Trigger form */}
        <TriggerForm
          nodeOptions={dockerNodes.map((n) => ({ id: n.id, name: n.name }))}
          volumeOptions={volumeOptions}
          onNodeChange={setSelectedNodeId}
        />

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
                {backups.map((b) => (
                  <BackupRow key={b.id} backup={b} nodeName={nodeName(b.node_id)} />
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>
    </AppShell>
  )
}
