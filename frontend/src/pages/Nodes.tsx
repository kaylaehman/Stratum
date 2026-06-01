import { useState, useEffect } from 'react'
import { Link } from 'react-router-dom'
import { Server, Box, Terminal, Plus, RefreshCw, Pencil, Trash2, Check, X, Loader, Layers } from 'lucide-react'
import { AppShell } from '../components/layout/AppShell'
import { AddNodeWizard } from '../components/nodes/AddNodeWizard'
import { EditDockerModal } from '../components/nodes/EditDockerModal'
import { useNodes, useDeleteNode, useRenameNode, useReprobeNode } from '../lib/api/nodes'
import { ApiError } from '../lib/api'
import { useDrainPlan, useDrainExecute } from '../lib/api/orchestration'
import { useCan } from '../lib/roles'
import type { NodeView, NodeType, NodeStatus, Capabilities } from '../types/api'
import type { OrchPlan, OrchStepResult } from '../lib/api/orchestration'

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

// ---- drain modal ----

type DrainPhase = 'plan' | 'confirm' | 'execute' | 'results'

interface DrainModalProps {
  nodeId: string
  nodeName: string
  onClose: () => void
}

function DrainModal({ nodeId, nodeName, onClose }: DrainModalProps) {
  const [phase, setPhase] = useState<DrainPhase>('plan')
  const [plan, setPlan] = useState<OrchPlan | null>(null)
  const [results, setResults] = useState<OrchStepResult[] | null>(null)
  const [err, setErr] = useState<string | null>(null)

  const drainPlan = useDrainPlan()
  const drainExecute = useDrainExecute()

  async function fetchPlan() {
    setErr(null)
    try {
      const p = await drainPlan.mutateAsync(nodeId)
      setPlan(p)
      setPhase('confirm')
    } catch (e) {
      setErr(e instanceof ApiError ? ((e.body as { error?: string }).error ?? 'Plan failed.') : 'Plan failed.')
    }
  }

  async function executeDrain() {
    setErr(null)
    setPhase('execute')
    try {
      const res = await drainExecute.mutateAsync(nodeId)
      setResults(res.results)
      setPhase('results')
    } catch (e) {
      if (e instanceof Error && e.message === 'step_up_cancelled') {
        setPhase('confirm')
        return
      }
      setErr(e instanceof ApiError ? ((e.body as { error?: string }).error ?? 'Drain failed.') : 'Drain failed.')
      setPhase('confirm')
    }
  }

  // Auto-fetch plan on mount
  // eslint-disable-next-line react-hooks/exhaustive-deps
  useEffect(() => { void fetchPlan() }, [])

  const overlay: React.CSSProperties = {
    position: 'fixed', inset: 0, zIndex: 50,
    backgroundColor: 'rgba(0,0,0,0.55)',
    display: 'flex', alignItems: 'center', justifyContent: 'center',
  }
  const modal: React.CSSProperties = {
    backgroundColor: 'var(--bg-surface)',
    border: '1px solid var(--border-default)',
    borderRadius: '4px',
    width: '440px',
    maxWidth: '95vw',
    maxHeight: '80vh',
    display: 'flex',
    flexDirection: 'column',
    overflow: 'hidden',
  }

  return (
    <div style={overlay} onClick={(e) => { if (e.target === e.currentTarget) onClose() }}>
      <div style={modal}>
        {/* Header */}
        <div style={{ padding: '12px 16px', borderBottom: '1px solid var(--border-subtle)', display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
          <span className="text-sm font-semibold" style={{ color: 'var(--text-primary)' }}>
            Drain node — {nodeName}
          </span>
          <button onClick={onClose} style={{ background: 'none', border: 'none', cursor: 'pointer', color: 'var(--text-muted)', padding: '2px' }} aria-label="Close">
            <X size={14} />
          </button>
        </div>

        {/* Body */}
        <div style={{ flex: 1, overflowY: 'auto', padding: '14px 16px' }}>
          {(phase === 'plan' || drainPlan.isPending) && (
            <div className="flex items-center gap-2 py-4 justify-center">
              <Loader size={14} className="animate-spin" style={{ color: 'var(--accent)' }} />
              <span className="text-xs" style={{ color: 'var(--text-muted)' }}>Building drain plan…</span>
            </div>
          )}

          {(phase === 'confirm' || phase === 'execute' || phase === 'results') && plan && (
            <div className="flex flex-col gap-3">
              <p className="text-xs" style={{ color: 'var(--text-secondary)' }}>
                The following guests and containers will be stopped in dependency order:
              </p>
              {plan.cycles.length > 0 && (
                <div style={{ backgroundColor: 'rgba(246,173,85,0.1)', border: '1px solid rgba(246,173,85,0.35)', borderRadius: '3px', padding: '8px 10px' }}>
                  <span className="text-xs" style={{ color: 'var(--status-warn)' }}>
                    Dependency cycles detected — order may be approximate.
                  </span>
                </div>
              )}
              <div style={{ display: 'flex', flexDirection: 'column', gap: '2px' }}>
                {plan.steps.map((step, i) => {
                  const res = results?.find((r) => r.step.id === step.id)
                  return (
                    <div key={step.id} style={{
                      display: 'flex', alignItems: 'center', gap: '8px',
                      padding: '5px 8px',
                      backgroundColor: 'var(--bg-elevated)',
                      border: '1px solid var(--border-subtle)',
                      borderRadius: '3px',
                    }}>
                      <span className="text-xs font-mono" style={{ color: 'var(--text-muted)', minWidth: '20px', textAlign: 'right' }}>
                        {i + 1}.
                      </span>
                      <span className="text-xs font-mono" style={{ color: 'var(--text-primary)', flex: 1 }}>
                        {step.name}
                      </span>
                      <span className="text-xs" style={{ color: 'var(--text-muted)' }}>{step.kind}</span>
                      {phase === 'execute' && !res && (
                        <Loader size={10} className="animate-spin" style={{ color: 'var(--accent)' }} />
                      )}
                      {res && (
                        <span style={{ color: res.ok ? 'var(--status-ok)' : 'var(--status-error)', fontSize: '11px', fontFamily: 'monospace' }}>
                          {res.ok ? 'ok' : 'err'}
                        </span>
                      )}
                    </div>
                  )
                })}
              </div>
              {phase === 'results' && results && (
                <div style={{
                  padding: '8px 10px',
                  backgroundColor: results.every((r) => r.ok) ? 'rgba(74,222,128,0.08)' : 'rgba(232,64,64,0.08)',
                  border: `1px solid ${results.every((r) => r.ok) ? 'rgba(74,222,128,0.3)' : 'rgba(232,64,64,0.3)'}`,
                  borderRadius: '3px',
                }}>
                  <span className="text-xs" style={{ color: results.every((r) => r.ok) ? 'var(--status-ok)' : 'var(--status-error)' }}>
                    {results.every((r) => r.ok)
                      ? `Drain complete — ${results.length} step(s) stopped.`
                      : `${results.filter((r) => !r.ok).length} step(s) failed.`}
                  </span>
                </div>
              )}
            </div>
          )}

          {err && (
            <p className="text-xs mt-2" style={{ color: 'var(--status-error)' }}>{err}</p>
          )}
        </div>

        {/* Footer */}
        <div style={{ padding: '10px 16px', borderTop: '1px solid var(--border-subtle)', display: 'flex', justifyContent: 'flex-end', gap: '8px' }}>
          {phase === 'results' ? (
            <button onClick={onClose} className="text-xs px-3 py-1.5" style={{ backgroundColor: 'var(--bg-elevated)', border: '1px solid var(--border-default)', color: 'var(--text-secondary)', borderRadius: '3px', cursor: 'pointer' }}>
              Close
            </button>
          ) : (
            <>
              <button onClick={onClose} disabled={phase === 'execute'} className="text-xs px-3 py-1.5" style={{ backgroundColor: 'transparent', border: '1px solid var(--border-default)', color: 'var(--text-secondary)', borderRadius: '3px', cursor: 'pointer' }}>
                Cancel
              </button>
              {phase === 'confirm' && (
                <button
                  onClick={() => void executeDrain()}
                  disabled={drainExecute.isPending}
                  className="text-xs px-3 py-1.5"
                  style={{ backgroundColor: 'rgba(232,64,64,0.15)', border: '1px solid rgba(232,64,64,0.4)', color: 'var(--status-error)', borderRadius: '3px', cursor: 'pointer' }}
                >
                  Drain node
                </button>
              )}
            </>
          )}
        </div>
      </div>
    </div>
  )
}

// ---- row actions ----

function RowActions({ node, onEditDocker, onDrain }: { node: NodeView; onEditDocker: () => void; onDrain: () => void }) {
  const [mode, setMode] = useState<'idle' | 'rename' | 'delete'>('idle')
  const reprobe = useReprobeNode()
  const { isAdmin } = useCan()

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
      {isAdmin && (
        <ActionButton label="Drain node (ordered stop)" danger onClick={onDrain}>
          <Layers size={12} />
        </ActionButton>
      )}
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

function NodeRow({ node, onEditDocker, onDrain }: { node: NodeView; onEditDocker: (node: NodeView) => void; onDrain: (node: NodeView) => void }) {
  return (
    <tr
      style={{
        borderBottom: '1px solid var(--border-subtle)',
      }}
    >
      {/* Type + Name — links to the node in the resource tree */}
      <td className="px-3 py-2 align-middle" style={{ whiteSpace: 'nowrap' }}>
        <Link
          to={`/resources?node=${node.id}`}
          className="flex items-center gap-2"
          style={{ textDecoration: 'none' }}
        >
          <NodeTypeIcon type={node.type} />
          <span className="text-xs font-mono" style={{ color: 'var(--text-primary)' }}>
            {node.name}
          </span>
        </Link>
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
        <RowActions node={node} onEditDocker={() => onEditDocker(node)} onDrain={() => onDrain(node)} />
      </td>
    </tr>
  )
}

// ---- page ----

export default function Nodes() {
  const { data: nodes, isLoading, isError, error } = useNodes()
  const [showWizard, setShowWizard] = useState(false)
  const [editDockerNode, setEditDockerNode] = useState<NodeView | null>(null)
  const [drainNode, setDrainNode] = useState<NodeView | null>(null)

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
                  <NodeRow key={node.id} node={node} onEditDocker={setEditDockerNode} onDrain={setDrainNode} />
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
      {drainNode && (
        <DrainModal nodeId={drainNode.id} nodeName={drainNode.name} onClose={() => setDrainNode(null)} />
      )}
    </AppShell>
  )
}
