import { useState, useCallback, useEffect } from 'react'
import { SquareTerminal, RotateCw } from 'lucide-react'
import { AppShell } from '../components/layout/AppShell'
import { TerminalView, type TerminalStatus } from '../components/terminal/TerminalView'
import { useNodes } from '../lib/api/nodes'
import { useCan } from '../lib/roles'

const statusMeta: Record<TerminalStatus, { color: string; label: string }> = {
  connecting: { color: 'var(--status-warn)', label: 'connecting' },
  connected: { color: 'var(--status-ok)', label: 'connected' },
  closed: { color: 'var(--text-muted)', label: 'disconnected' },
  error: { color: 'var(--status-error)', label: 'connection failed' },
}

export default function TerminalPage() {
  const { isAdmin } = useCan()
  const { data: nodes } = useNodes()
  const [nodeId, setNodeId] = useState<string | null>(null)
  const [reconnectKey, setReconnectKey] = useState(0)
  const [status, setStatus] = useState<TerminalStatus>('connecting')

  // Default to the first node once the list loads.
  useEffect(() => {
    if (!nodeId && nodes && nodes.length > 0) setNodeId(nodes[0].id)
  }, [nodes, nodeId])

  // Stable callback so TerminalView's effect doesn't re-run (reconnect) every render.
  const handleStatus = useCallback((s: TerminalStatus) => setStatus(s), [])

  const meta = statusMeta[status]

  return (
    <AppShell>
      <div className="flex flex-col flex-1 min-h-0 h-full w-full">
        {/* Header */}
        <div className="flex items-center justify-between gap-4 mb-3 shrink-0 flex-wrap">
          <div className="flex items-center gap-2">
            <SquareTerminal size={16} style={{ color: 'var(--text-secondary)' }} />
            <h1 className="text-sm font-medium uppercase tracking-wider" style={{ color: 'var(--text-primary)' }}>
              Terminal
            </h1>
          </div>

          {isAdmin && (
            <div className="flex items-center gap-3 flex-wrap">
              {/* Connection status */}
              <div className="flex items-center gap-1.5">
                <span style={{ width: 8, height: 8, borderRadius: '50%', backgroundColor: meta.color, flexShrink: 0 }} />
                <span className="font-mono text-xs" style={{ color: 'var(--text-muted)' }}>
                  {meta.label}
                </span>
              </div>

              {/* Clipboard hint */}
              <span className="font-mono text-xs" style={{ color: 'var(--text-muted)' }}>
                Ctrl+Shift+C&nbsp;/&nbsp;Ctrl+Shift+V — copy&nbsp;/&nbsp;paste
              </span>

              {/* Node selector */}
              <select
                value={nodeId ?? ''}
                onChange={(e) => setNodeId(e.target.value)}
                className="text-xs font-mono px-2 py-1"
                style={{
                  backgroundColor: 'var(--bg-elevated)',
                  border: '1px solid var(--border-default)',
                  color: 'var(--text-primary)',
                  borderRadius: '3px',
                  cursor: 'pointer',
                  outline: 'none',
                }}
              >
                {(nodes ?? []).map((n) => (
                  <option key={n.id} value={n.id}>
                    {n.name} ({n.host})
                  </option>
                ))}
              </select>

              {/* Reconnect */}
              <button
                type="button"
                onClick={() => setReconnectKey((k) => k + 1)}
                disabled={!nodeId}
                title="Reconnect"
                className="flex items-center gap-1 text-xs px-2 py-1"
                style={{
                  background: 'var(--bg-elevated)',
                  border: '1px solid var(--border-default)',
                  color: 'var(--text-secondary)',
                  borderRadius: '3px',
                  cursor: nodeId ? 'pointer' : 'not-allowed',
                }}
              >
                <RotateCw size={12} />
                Reconnect
              </button>
            </div>
          )}
        </div>

        {/* Body */}
        {!isAdmin ? (
          <div className="px-4 py-6 text-xs" style={{ color: 'var(--status-warn)' }}>
            The interactive terminal is available to administrators only.
          </div>
        ) : !nodes || nodes.length === 0 ? (
          <div className="px-4 py-6 text-xs" style={{ color: 'var(--text-muted)' }}>
            No nodes connected. Add a node to open a shell.
          </div>
        ) : (
          <div
            className="flex flex-col flex-1 min-h-0"
            style={{
              backgroundColor: 'var(--bg-base)',
              border: '1px solid var(--border-subtle)',
              borderRadius: '3px',
              overflow: 'hidden',
            }}
          >
            {nodeId && (
              <TerminalView nodeId={nodeId} reconnectKey={reconnectKey} onStatusChange={handleStatus} />
            )}
          </div>
        )}
      </div>
    </AppShell>
  )
}
