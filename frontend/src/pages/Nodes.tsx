import { useState } from 'react'
import { Server, Box, Terminal, Plus, RefreshCw, Pencil, Trash2, Check, X, Loader } from 'lucide-react'
import { AppShell } from '../components/layout/AppShell'
import { AddNodeWizard } from '../components/nodes/AddNodeWizard'
import { EditDockerModal } from '../components/nodes/EditDockerModal'
import { useNodes, useDeleteNode, useRenameNode, useReprobeNode } from '../lib/api/nodes'
import { ApiError } from '../lib/api'
import type { NodeView, NodeType, NodeStatus, Capabilities } from '../types/api'

// ---- type icons ----

function NodeTypeIcon({ type }: { type: NodeType }) {
  if (type === 'proxmox') return <Server size={13} style={{ color: 'var(--accent)' }} />
  if (type === 'standalone') return <Box size={13} style={{ color: 'var(--status-ok)' }} />
  return <Terminal size={13} style={{ color: 'var(--text-secondary)' }} />
}

// ---- status badge ----

const STATUS_COLOR: Record<NodeStatus, string> = {
  ok: 'var(--status-ok)',
  unreachable: 'var(--status-warn)',
  error: 'var(--status-error)',
  unknown: 'var(--status-muted)',
}

function StatusBadge({ status }: { status: NodeStatus }) {
  return (
    <span
      className="text-xs font-mono px-1.5 py-0.5"
      style={{
        color: STATUS_COLOR[status],
        backgroundColor: `color-mix(in srgb, ${STATUS_COLOR[status]} 12%, transparent)`,
        border: `1px solid color-mix(in srgb, ${STATUS_COLOR[status]} 30%, transparent)`,
        borderRadius: '3px',
      }}
    >
      {status}
    </span>
  )
}

// ---- capability chips ----

const CAP_KEYS = ['proxmox', 'docker', 'agent', 'systemd', 'cron'] as const

function CapChips({ caps }: { caps: Capabilities }) {
  const active = CAP_KEYS.filter((k) => caps[k])
  if (active.length === 0) return <span style={{ color: 'var(--text-muted)', fontSize: '12px' }}>none</span>
  return (
    <div className="flex flex-wrap gap-1">
      {active.map((k) => (
        <span
          key={k}
          className="text-xs font-mono px-1 py-0.5"
          style={{
            backgroundColor: 'var(--accent-glow)',
            border: '1px solid var(--accent-dim)',
            color: 'var(--accent)',
            borderRadius: '3px',
            fontSize: '12px',
          }}
        >
          {k}
        </span>
      ))}
    </div>
  )
}

// ---- inline rename ----

function RenameInline({
  node,
  onDone,
}: {
  node: NodeView
  onDone: () => void
}) {
  const [value, setValue] = useState(node.name)
  const [err, setErr] = useState<string | null>(null)
  const rename = useRenameNode()

  async function submit() {
    if (!value.trim() || value.trim() === node.name) {
      onDone()
      return
    }
    try {
      await rename.mutateAsync({ id: node.id, name: value.trim() })
      onDone()
    } catch (e) {
      if (e instanceof ApiError) {
        const b = e.body as { error?: string }
        setErr(b.error ?? 'Rename failed.')
      } else {
        setErr('Rename failed.')
      }
    }
  }

  return (
    <div className="flex items-center gap-1.5">
      <input
        autoFocus
        value={value}
        onChange={(e) => setValue(e.target.value)}
        onKeyDown={(e) => {
          if (e.key === 'Enter') void submit()
          if (e.key === 'Escape') onDone()
        }}
        className="font-mono text-xs px-1.5 py-0.5"
        style={{
          backgroundColor: 'var(--bg-overlay)',
          border: '1px solid var(--accent)',
          color: 'var(--text-primary)',
          borderRadius: '3px',
          outline: 'none',
          width: '160px',
        }}
      />
      <button
        onClick={() => void submit()}
        disabled={rename.isPending}
        style={{ background: 'transparent', border: 'none', cursor: 'pointer', color: 'var(--status-ok)', padding: '2px' }}
        aria-label="Confirm rename"
      >
        {rename.isPending ? <Loader size={12} className="animate-spin" /> : <Check size={12} />}
      </button>
      <button
        onClick={onDone}
        style={{ background: 'transparent', border: 'none', cursor: 'pointer', color: 'var(--text-muted)', padding: '2px' }}
        aria-label="Cancel rename"
      >
        <X size={12} />
      </button>
      {err && <span className="text-xs" style={{ color: 'var(--status-error)' }}>{err}</span>}
    </div>
  )
}

// ---- delete confirm ----

function DeleteConfirm({ nodeId, onDone }: { nodeId: string; onDone: () => void }) {
  const del = useDeleteNode()
  const [err, setErr] = useState<string | null>(null)

  async function confirm() {
    try {
      await del.mutateAsync(nodeId)
      onDone()
    } catch {
      setErr('Delete failed.')
    }
  }

  return (
    <div className="flex items-center gap-1.5">
      <span className="text-xs" style={{ color: 'var(--status-error)' }}>Remove?</span>
      <button
        onClick={() => void confirm()}
        disabled={del.isPending}
        className="px-2 py-0.5 text-xs"
        style={{
          backgroundColor: 'rgba(232,64,64,0.15)',
          border: '1px solid rgba(232,64,64,0.4)',
          color: 'var(--status-error)',
          borderRadius: '3px',
          cursor: 'pointer',
        }}
      >
        {del.isPending ? 'Removing...' : 'Yes, remove'}
      </button>
      <button
        onClick={onDone}
        className="text-xs px-2 py-0.5"
        style={{
          backgroundColor: 'transparent',
          border: '1px solid var(--border-default)',
          color: 'var(--text-secondary)',
          borderRadius: '3px',
          cursor: 'pointer',
        }}
      >
        Cancel
      </button>
      {err && <span className="text-xs" style={{ color: 'var(--status-error)' }}>{err}</span>}
    </div>
  )
}

// ---- row actions ----

function RowActions({ node, onEditDocker }: { node: NodeView; onEditDocker: () => void }) {
  const [mode, setMode] = useState<'idle' | 'rename' | 'delete'>('idle')
  const reprobe = useReprobeNode()

  if (mode === 'rename') {
    return <RenameInline node={node} onDone={() => setMode('idle')} />
  }
  if (mode === 'delete') {
    return <DeleteConfirm nodeId={node.id} onDone={() => setMode('idle')} />
  }

  return (
    <div className="flex items-center gap-1">
      <ActionButton
        label="Re-probe"
        loading={reprobe.isPending && reprobe.variables === node.id}
        onClick={() => void reprobe.mutateAsync(node.id)}
      >
        <RefreshCw size={12} />
      </ActionButton>
      <ActionButton label="Docker config" onClick={onEditDocker}>
        <Box size={12} />
      </ActionButton>
      <ActionButton label="Rename" onClick={() => setMode('rename')}>
        <Pencil size={12} />
      </ActionButton>
      <ActionButton label="Remove" danger onClick={() => setMode('delete')}>
        <Trash2 size={12} />
      </ActionButton>
    </div>
  )
}

function ActionButton({
  children,
  label,
  onClick,
  loading,
  danger,
}: {
  children: React.ReactNode
  label: string
  onClick: () => void
  loading?: boolean
  danger?: boolean
}) {
  return (
    <button
      onClick={onClick}
      disabled={loading}
      title={label}
      aria-label={label}
      className="flex items-center justify-center p-1.5"
      style={{
        background: 'transparent',
        border: '1px solid var(--border-subtle)',
        color: danger ? 'var(--status-error)' : 'var(--text-muted)',
        borderRadius: '3px',
        cursor: loading ? 'not-allowed' : 'pointer',
        opacity: loading ? 0.5 : 1,
      }}
    >
      {loading ? <Loader size={12} className="animate-spin" /> : children}
    </button>
  )
}

// ---- last seen ----

function formatLastSeen(ts?: string): string {
  if (!ts) return 'never'
  const d = new Date(ts)
  const now = Date.now()
  const diff = Math.floor((now - d.getTime()) / 1000)
  if (diff < 60) return `${diff}s ago`
  if (diff < 3600) return `${Math.floor(diff / 60)}m ago`
  if (diff < 86400) return `${Math.floor(diff / 3600)}h ago`
  return d.toLocaleDateString()
}

// ---- node row ----

function NodeRow({ node, onEditDocker }: { node: NodeView; onEditDocker: (node: NodeView) => void }) {
  return (
    <tr
      style={{
        borderBottom: '1px solid var(--border-subtle)',
      }}
    >
      {/* Type + Name */}
      <td className="px-3 py-2 align-middle" style={{ whiteSpace: 'nowrap' }}>
        <div className="flex items-center gap-2">
          <NodeTypeIcon type={node.type} />
          <span className="text-xs font-mono" style={{ color: 'var(--text-primary)' }}>
            {node.name}
          </span>
        </div>
        <div className="text-xs font-mono mt-0.5" style={{ color: 'var(--text-muted)' }}>
          {node.type}
        </div>
      </td>

      {/* Host */}
      <td className="px-3 py-2 align-middle">
        <span className="text-xs font-mono" style={{ color: 'var(--text-secondary)' }}>
          {node.host}:{node.port}
        </span>
      </td>

      {/* OS */}
      <td className="px-3 py-2 align-middle">
        <span className="text-xs font-mono" style={{ color: 'var(--text-muted)' }}>
          {node.os_type || '—'}
        </span>
      </td>

      {/* Capabilities */}
      <td className="px-3 py-2 align-middle">
        <CapChips caps={node.capabilities} />
      </td>

      {/* Status */}
      <td className="px-3 py-2 align-middle">
        <StatusBadge status={node.status} />
        {node.last_error && (
          <div
            className="text-xs mt-0.5 max-w-xs truncate"
            style={{ color: 'var(--status-error)' }}
            title={node.last_error}
          >
            {node.last_error}
          </div>
        )}
      </td>

      {/* Last seen */}
      <td className="px-3 py-2 align-middle">
        <span className="text-xs font-mono" style={{ color: 'var(--text-muted)' }}>
          {formatLastSeen(node.last_seen)}
        </span>
      </td>

      {/* Actions */}
      <td className="px-3 py-2 align-middle">
        <RowActions node={node} onEditDocker={() => onEditDocker(node)} />
      </td>
    </tr>
  )
}

// ---- page ----

export default function Nodes() {
  const { data: nodes, isLoading, isError, error } = useNodes()
  const [showWizard, setShowWizard] = useState(false)
  const [editDockerNode, setEditDockerNode] = useState<NodeView | null>(null)

  return (
    <AppShell>
      <div className="flex flex-col gap-5">
        {/* Header */}
        <div className="flex items-center justify-between">
          <div>
            <h1 className="text-base font-semibold" style={{ color: 'var(--text-primary)' }}>
              Nodes
            </h1>
            <p className="text-xs mt-0.5" style={{ color: 'var(--text-muted)' }}>
              Registered infrastructure hosts
            </p>
          </div>
          <button
            onClick={() => setShowWizard(true)}
            className="flex items-center gap-1.5 px-3 py-1.5 text-xs font-medium"
            style={{
              backgroundColor: 'var(--accent)',
              color: 'var(--text-inverse)',
              border: 'none',
              borderRadius: '3px',
              cursor: 'pointer',
            }}
          >
            <Plus size={13} />
            Add node
          </button>
        </div>

        {/* Loading */}
        {isLoading && (
          <div className="flex items-center gap-2 py-8 justify-center">
            <Loader size={16} className="animate-spin" style={{ color: 'var(--accent)' }} />
            <span className="text-xs" style={{ color: 'var(--text-muted)' }}>
              Loading nodes...
            </span>
          </div>
        )}

        {/* Error */}
        {isError && (
          <div
            className="px-4 py-3"
            style={{
              backgroundColor: 'rgba(232,64,64,0.08)',
              border: '1px solid rgba(232,64,64,0.3)',
              borderRadius: '3px',
            }}
          >
            <p className="text-xs" style={{ color: 'var(--status-error)' }}>
              {error instanceof ApiError
                ? `Failed to load nodes: ${(error.body as { error?: string }).error ?? error.message}`
                : 'Failed to load nodes.'}
            </p>
          </div>
        )}

        {/* Empty state */}
        {!isLoading && !isError && nodes && nodes.length === 0 && (
          <div
            className="flex flex-col items-center gap-4 py-14"
            style={{
              backgroundColor: 'var(--bg-surface)',
              border: '1px solid var(--border-subtle)',
              borderRadius: '3px',
            }}
          >
            <Server size={28} style={{ color: 'var(--text-muted)' }} />
            <div className="text-center">
              <p className="text-sm font-medium" style={{ color: 'var(--text-primary)' }}>
                No nodes registered
              </p>
              <p className="text-xs mt-1" style={{ color: 'var(--text-muted)' }}>
                Add a Linux host, Proxmox node, or Docker host to get started.
              </p>
            </div>
            <button
              onClick={() => setShowWizard(true)}
              className="flex items-center gap-1.5 px-3 py-1.5 text-xs font-medium"
              style={{
                backgroundColor: 'var(--accent)',
                color: 'var(--text-inverse)',
                border: 'none',
                borderRadius: '3px',
                cursor: 'pointer',
              }}
            >
              <Plus size={13} />
              Add node
            </button>
          </div>
        )}

        {/* Table */}
        {!isLoading && !isError && nodes && nodes.length > 0 && (
          <div
            style={{
              backgroundColor: 'var(--bg-surface)',
              border: '1px solid var(--border-subtle)',
              borderRadius: '3px',
              overflow: 'hidden',
            }}
          >
            <table
              className="w-full"
              style={{ borderCollapse: 'collapse', tableLayout: 'auto' }}
            >
              <thead>
                <tr
                  style={{
                    borderBottom: '1px solid var(--border-default)',
                    backgroundColor: 'var(--bg-elevated)',
                  }}
                >
                  {['Node', 'Host', 'OS', 'Capabilities', 'Status', 'Last seen', ''].map((h) => (
                    <th
                      key={h}
                      className="px-3 py-2 text-left text-xs font-medium"
                      style={{ color: 'var(--text-muted)', whiteSpace: 'nowrap' }}
                    >
                      {h}
                    </th>
                  ))}
                </tr>
              </thead>
              <tbody>
                {nodes.map((node) => (
                  <NodeRow key={node.id} node={node} onEditDocker={setEditDockerNode} />
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>

      {showWizard && <AddNodeWizard onClose={() => setShowWizard(false)} />}
      {editDockerNode && (
        <EditDockerModal node={editDockerNode} onClose={() => setEditDockerNode(null)} />
      )}
    </AppShell>
  )
}
