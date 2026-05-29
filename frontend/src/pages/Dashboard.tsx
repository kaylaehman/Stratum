import { Link } from 'react-router-dom'
import { AppShell } from '../components/layout/AppShell'
import { useMe } from '../hooks/useMe'
import { useNodes } from '../lib/api/nodes'
import { Activity, HardDrive, Container, Shield } from 'lucide-react'
import type { NodeStatus, NodeView } from '../types/api'

interface StatCardProps {
  icon: React.ReactNode
  label: string
  value: string
  status?: 'ok' | 'warn' | 'muted'
}

function StatCard({ icon, label, value, status = 'muted' }: StatCardProps) {
  const statusColor = {
    ok: 'var(--status-ok)',
    warn: 'var(--status-warn)',
    muted: 'var(--text-muted)',
  }[status]

  return (
    <div
      className="flex items-center gap-4 p-4 rounded border"
      style={{
        backgroundColor: 'var(--bg-surface)',
        borderColor: 'var(--border-default)',
        borderRadius: '3px',
      }}
    >
      <div style={{ color: statusColor }}>{icon}</div>
      <div>
        <p className="text-xs" style={{ color: 'var(--text-secondary)' }}>
          {label}
        </p>
        <p className="text-sm font-semibold font-mono" style={{ color: 'var(--text-primary)' }}>
          {value}
        </p>
      </div>
    </div>
  )
}

// statusMeta maps a node status to a dot color + label.
const statusMeta: Record<NodeStatus, { color: string; label: string }> = {
  ok: { color: 'var(--status-ok)', label: 'online' },
  unreachable: { color: 'var(--status-error)', label: 'unreachable' },
  error: { color: 'var(--status-error)', label: 'error' },
  unknown: { color: 'var(--text-muted)', label: 'unknown' },
}

function NodeRow({ node }: { node: NodeView }) {
  const meta = statusMeta[node.status] ?? statusMeta.unknown
  return (
    <Link
      to="/resources"
      className="flex items-center gap-3 px-4 py-2.5"
      style={{ borderTop: '1px solid var(--border-subtle)', textDecoration: 'none' }}
    >
      <span
        title={meta.label}
        style={{
          width: 8,
          height: 8,
          borderRadius: '50%',
          backgroundColor: meta.color,
          flexShrink: 0,
        }}
      />
      <span className="text-xs font-medium" style={{ color: 'var(--text-primary)' }}>
        {node.name}
      </span>
      <span className="text-xs font-mono" style={{ color: 'var(--text-muted)' }}>
        {node.host}
      </span>
      <span
        className="text-xs font-mono ml-auto px-1.5 py-0.5"
        style={{
          color: 'var(--text-secondary)',
          backgroundColor: 'var(--bg-elevated)',
          border: '1px solid var(--border-subtle)',
          borderRadius: '3px',
        }}
      >
        {node.type}
      </span>
      <span className="text-xs font-mono" style={{ color: meta.color, minWidth: 84, textAlign: 'right' }}>
        {meta.label}
      </span>
    </Link>
  )
}

export default function Dashboard() {
  const { data: me, isLoading: meLoading, isError: meError } = useMe()
  const { data: nodes, isLoading: nodesLoading, isError: nodesError } = useNodes()

  const total = nodes?.length ?? 0
  const online = nodes?.filter((n) => n.status === 'ok').length ?? 0
  const unhealthy = nodes?.filter((n) => n.status === 'error' || n.status === 'unreachable').length ?? 0

  return (
    <AppShell>
      <div className="flex flex-col gap-6">
        {/* Header */}
        <div>
          <h1 className="text-base font-semibold" style={{ color: 'var(--text-primary)' }}>
            Dashboard
          </h1>
          {meLoading && (
            <p className="text-xs mt-1" style={{ color: 'var(--text-muted)' }}>
              Loading...
            </p>
          )}
          {meError && (
            <p className="text-xs mt-1" style={{ color: 'var(--status-error)' }}>
              Failed to load user info.
            </p>
          )}
          {me && (
            <p className="text-xs mt-1" style={{ color: 'var(--text-secondary)' }}>
              Signed in as{' '}
              <span className="font-mono" style={{ color: 'var(--accent)' }}>
                {me.username}
              </span>{' '}
              &middot; role:{' '}
              <span className="font-mono" style={{ color: 'var(--text-primary)' }}>
                {me.role}
              </span>
            </p>
          )}
        </div>

        {/* Stat cards */}
        <div className="grid grid-cols-2 gap-3" style={{ gridTemplateColumns: 'repeat(auto-fill, minmax(200px, 1fr))' }}>
          <StatCard
            icon={<HardDrive size={18} />}
            label="Connected nodes"
            value={nodesLoading ? '…' : `${online}/${total}`}
            status={total > 0 && unhealthy === 0 ? 'ok' : unhealthy > 0 ? 'warn' : 'muted'}
          />
          <StatCard icon={<Container size={18} />} label="Running containers" value="—" status="muted" />
          <StatCard icon={<Shield size={18} />} label="Critical CVEs" value="—" status="muted" />
          <StatCard
            icon={<Activity size={18} />}
            label="Unhealthy nodes"
            value={nodesLoading ? '…' : String(unhealthy)}
            status={unhealthy > 0 ? 'warn' : 'muted'}
          />
        </div>

        {/* Nodes list */}
        <div
          className="rounded border"
          style={{
            backgroundColor: 'var(--bg-surface)',
            borderColor: 'var(--border-subtle)',
            borderRadius: '3px',
            overflow: 'hidden',
          }}
        >
          <div className="px-4 py-2.5">
            <p
              className="text-xs font-medium uppercase tracking-wider"
              style={{ color: 'var(--text-muted)' }}
            >
              Connected hosts
            </p>
          </div>

          {nodesLoading && (
            <p className="px-4 py-6 text-xs" style={{ color: 'var(--text-muted)', borderTop: '1px solid var(--border-subtle)' }}>
              Loading nodes…
            </p>
          )}
          {nodesError && (
            <p className="px-4 py-6 text-xs" style={{ color: 'var(--status-error)', borderTop: '1px solid var(--border-subtle)' }}>
              Failed to load nodes. Check that the backend is reachable.
            </p>
          )}
          {!nodesLoading && !nodesError && total === 0 && (
            <p className="px-4 py-6 text-xs text-center" style={{ color: 'var(--text-muted)', borderTop: '1px solid var(--border-subtle)' }}>
              No nodes connected.{' '}
              <Link to="/nodes" style={{ color: 'var(--accent)' }}>
                Add a node
              </Link>{' '}
              to get started.
            </p>
          )}
          {!nodesLoading && !nodesError && nodes?.map((n) => <NodeRow key={n.id} node={n} />)}
        </div>
      </div>
    </AppShell>
  )
}
