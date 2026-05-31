import { useMemo } from 'react'
import { Link } from 'react-router-dom'
import { AppShell } from '../components/layout/AppShell'
import { useMe } from '../hooks/useMe'
import { useNodes } from '../lib/api/nodes'
import { useTree } from '../lib/api/tree'
import { useCVEScans } from '../lib/api/cve'
import { usePrivileged } from '../lib/api/security'
import {
  Activity,
  HardDrive,
  Container,
  Shield,
  AlertTriangle,
  Boxes,
  CheckCircle2,
  ShieldAlert,
} from 'lucide-react'
import type { NodeStatus, NodeView, ContainerStatus, Container as TreeContainer } from '../types/api'

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

// containerStatusMeta maps a container status to a dot color + label.
const containerStatusMeta: Record<ContainerStatus, { color: string; label: string }> = {
  running: { color: 'var(--status-ok)', label: 'running' },
  exited: { color: 'var(--text-muted)', label: 'exited' },
  paused: { color: 'var(--status-warn)', label: 'paused' },
  restarting: { color: 'var(--status-warn)', label: 'restarting' },
  dead: { color: 'var(--status-error)', label: 'dead' },
  created: { color: 'var(--text-muted)', label: 'created' },
}

function NodeRow({ node }: { node: NodeView }) {
  const meta = statusMeta[node.status] ?? statusMeta.unknown
  return (
    <Link
      to={`/resources?node=${node.id}`}
      className="flex items-center gap-3 px-4 py-2.5"
      style={{ borderTop: '1px solid var(--border-subtle)', textDecoration: 'none' }}
    >
      <span
        title={meta.label}
        style={{ width: 8, height: 8, borderRadius: '50%', backgroundColor: meta.color, flexShrink: 0 }}
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

// ── Correlation ────────────────────────────────────────────────────────────────

interface AttentionItem {
  key: string
  severity: 'error' | 'warn'
  title: string
  subtitle?: string
  detail: string
  to: string
}

function PanelHeader({ children, right }: { children: React.ReactNode; right?: React.ReactNode }) {
  return (
    <div className="flex items-center justify-between gap-2 px-4 py-2.5">
      <p className="text-xs font-medium uppercase tracking-wider" style={{ color: 'var(--text-muted)' }}>
        {children}
      </p>
      {right}
    </div>
  )
}

const ATTENTION_LIMIT = 12

// ── Page ───────────────────────────────────────────────────────────────────────

export default function Dashboard() {
  const { data: me, isLoading: meLoading, isError: meError } = useMe()
  const { data: nodes, isLoading: nodesLoading, isError: nodesError } = useNodes()
  const isAdmin = me?.role === 'admin'
  const { data: tree } = useTree()
  // CVE + privileged scans are admin-scoped; only query when allowed (avoids 403s).
  const { data: cve } = useCVEScans(isAdmin)
  const { data: priv } = usePrivileged(isAdmin)

  const total = nodes?.length ?? 0
  const online = nodes?.filter((n) => n.status === 'ok').length ?? 0
  const unhealthy = nodes?.filter((n) => n.status === 'error' || n.status === 'unreachable').length ?? 0

  const allContainers = useMemo(() => tree?.nodes.flatMap((n) => n.containers) ?? [], [tree])
  const runningContainers = allContainers.filter((c) => c.status === 'running').length
  const criticalCves = cve?.scans?.reduce((sum, s) => sum + (s.critical ?? 0), 0) ?? 0

  // image name -> total Critical CVEs, used to correlate scans with running containers.
  const criticalByImage = useMemo(() => {
    const m = new Map<string, number>()
    for (const s of cve?.scans ?? []) {
      if (s.critical > 0) m.set(s.image, (m.get(s.image) ?? 0) + s.critical)
    }
    return m
  }, [cve])

  // docker IDs of containers flagged with elevated privileges.
  const privilegedIds = useMemo(
    () => new Set((priv?.containers ?? []).map((c) => c.container_id)),
    [priv],
  )

  // Correlate node health, container state, CVEs, and privilege flags into one
  // prioritized "needs attention" list (errors first).
  const attention = useMemo<AttentionItem[]>(() => {
    const items: AttentionItem[] = []

    for (const n of nodes ?? []) {
      if (n.status === 'error' || n.status === 'unreachable') {
        items.push({
          key: `node-${n.id}`,
          severity: 'error',
          title: n.name,
          subtitle: n.host,
          detail: statusMeta[n.status].label,
          to: '/nodes',
        })
      }
    }

    for (const tn of tree?.nodes ?? []) {
      for (const c of tn.containers) {
        const issues: { text: string; sev: 'error' | 'warn' }[] = []
        if (c.status === 'dead') issues.push({ text: 'dead', sev: 'error' })
        else if (c.status === 'exited' || c.status === 'restarting' || c.status === 'paused') {
          issues.push({ text: c.status, sev: 'warn' })
        }
        if (c.status === 'running') {
          const crit = criticalByImage.get(c.image) ?? 0
          if (crit > 0) issues.push({ text: `${crit} critical CVE${crit === 1 ? '' : 's'}`, sev: 'error' })
        }
        if (privilegedIds.has(c.docker_id)) issues.push({ text: 'privileged', sev: 'warn' })

        if (issues.length > 0) {
          items.push({
            key: `ctr-${c.id}`,
            severity: issues.some((i) => i.sev === 'error') ? 'error' : 'warn',
            title: c.name,
            subtitle: tn.name,
            detail: issues.map((i) => i.text).join(' · '),
            to: `/resources?node=${tn.id}&container=${c.id}`,
          })
        }
      }
    }

    return items.sort((a, b) => (a.severity === 'error' ? 0 : 1) - (b.severity === 'error' ? 0 : 1))
  }, [nodes, tree, criticalByImage, privilegedIds])

  // Containers grouped by node for the overview panel.
  const nodesWithContainers = useMemo(
    () => (tree?.nodes ?? []).filter((n) => n.containers.length > 0),
    [tree],
  )

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
          <StatCard
            icon={<Container size={18} />}
            label="Running containers"
            value={tree ? `${runningContainers}/${allContainers.length}` : '—'}
            status={runningContainers > 0 ? 'ok' : 'muted'}
          />
          <StatCard
            icon={<Shield size={18} />}
            label="Critical CVEs"
            value={!isAdmin ? '—' : cve ? String(criticalCves) : '…'}
            status={criticalCves > 0 ? 'warn' : 'muted'}
          />
          <StatCard
            icon={<Activity size={18} />}
            label="Unhealthy nodes"
            value={nodesLoading ? '…' : String(unhealthy)}
            status={unhealthy > 0 ? 'warn' : 'muted'}
          />
        </div>

        {/* Correlation + containers */}
        <div className="grid gap-4" style={{ gridTemplateColumns: 'repeat(auto-fit, minmax(360px, 1fr))' }}>
          {/* Needs attention (correlated signals) */}
          <div
            className="rounded border flex flex-col"
            style={{ backgroundColor: 'var(--bg-surface)', borderColor: 'var(--border-subtle)', borderRadius: '3px', overflow: 'hidden' }}
          >
            <PanelHeader
              right={
                attention.length > 0 ? (
                  <span
                    className="font-mono text-xs px-1.5 py-0.5"
                    style={{
                      color: 'var(--status-warn)',
                      background: 'var(--bg-elevated)',
                      border: '1px solid var(--border-subtle)',
                      borderRadius: '3px',
                    }}
                  >
                    {attention.length}
                  </span>
                ) : undefined
              }
            >
              Needs attention
            </PanelHeader>

            {attention.length === 0 ? (
              <div
                className="flex items-center gap-2 px-4 py-6"
                style={{ borderTop: '1px solid var(--border-subtle)', color: 'var(--status-ok)' }}
              >
                <CheckCircle2 size={14} />
                <span className="text-xs" style={{ color: 'var(--text-secondary)' }}>
                  {isAdmin
                    ? 'All clear — no node, container, CVE, or privilege issues detected.'
                    : 'All clear — no node or container issues detected.'}
                </span>
              </div>
            ) : (
              <>
                {attention.slice(0, ATTENTION_LIMIT).map((it) => {
                  const color = it.severity === 'error' ? 'var(--status-error)' : 'var(--status-warn)'
                  return (
                    <Link
                      key={it.key}
                      to={it.to}
                      className="flex items-center gap-3 px-4 py-2"
                      style={{ borderTop: '1px solid var(--border-subtle)', textDecoration: 'none' }}
                    >
                      {it.detail.includes('CVE') || it.detail === 'privileged' ? (
                        <ShieldAlert size={13} style={{ color, flexShrink: 0 }} />
                      ) : (
                        <AlertTriangle size={13} style={{ color, flexShrink: 0 }} />
                      )}
                      <span className="text-xs font-medium truncate" style={{ color: 'var(--text-primary)' }}>
                        {it.title}
                      </span>
                      {it.subtitle && (
                        <span className="text-xs font-mono truncate" style={{ color: 'var(--text-muted)' }}>
                          {it.subtitle}
                        </span>
                      )}
                      <span className="text-xs font-mono ml-auto shrink-0" style={{ color }}>
                        {it.detail}
                      </span>
                    </Link>
                  )
                })}
                {attention.length > ATTENTION_LIMIT && (
                  <Link
                    to="/security"
                    className="px-4 py-2 text-xs"
                    style={{ borderTop: '1px solid var(--border-subtle)', color: 'var(--accent)', textDecoration: 'none' }}
                  >
                    +{attention.length - ATTENTION_LIMIT} more…
                  </Link>
                )}
              </>
            )}
          </div>

          {/* Containers overview, grouped by node */}
          <div
            className="rounded border flex flex-col"
            style={{ backgroundColor: 'var(--bg-surface)', borderColor: 'var(--border-subtle)', borderRadius: '3px', overflow: 'hidden' }}
          >
            <PanelHeader
              right={
                tree ? (
                  <span className="font-mono text-xs" style={{ color: 'var(--text-muted)' }}>
                    {runningContainers}/{allContainers.length} running
                  </span>
                ) : undefined
              }
            >
              Containers
            </PanelHeader>

            {!tree ? (
              <p className="px-4 py-6 text-xs" style={{ color: 'var(--text-muted)', borderTop: '1px solid var(--border-subtle)' }}>
                Loading containers…
              </p>
            ) : nodesWithContainers.length === 0 ? (
              <p className="px-4 py-6 text-xs" style={{ color: 'var(--text-muted)', borderTop: '1px solid var(--border-subtle)' }}>
                No containers across connected hosts.
              </p>
            ) : (
              <div style={{ maxHeight: '320px', overflowY: 'auto' }}>
                {nodesWithContainers.map((tn) => {
                  const running = tn.containers.filter((c) => c.status === 'running').length
                  return (
                    <div key={tn.id}>
                      <div
                        className="flex items-center gap-2 px-4 py-1.5"
                        style={{ borderTop: '1px solid var(--border-subtle)', backgroundColor: 'var(--bg-elevated)' }}
                      >
                        <Boxes size={12} style={{ color: 'var(--text-muted)' }} />
                        <span className="text-xs font-medium" style={{ color: 'var(--text-secondary)' }}>
                          {tn.name}
                        </span>
                        <span className="text-xs font-mono ml-auto" style={{ color: 'var(--text-muted)' }}>
                          {running}/{tn.containers.length}
                        </span>
                      </div>
                      {tn.containers.map((c) => (
                        <ContainerRow key={c.id} c={c} nodeId={tn.id} />
                      ))}
                    </div>
                  )
                })}
              </div>
            )}
          </div>
        </div>

        {/* Nodes list */}
        <div
          className="rounded border"
          style={{ backgroundColor: 'var(--bg-surface)', borderColor: 'var(--border-subtle)', borderRadius: '3px', overflow: 'hidden' }}
        >
          <PanelHeader>Connected hosts</PanelHeader>

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

function ContainerRow({ c, nodeId }: { c: TreeContainer; nodeId: string }) {
  const meta = containerStatusMeta[c.status] ?? { color: 'var(--text-muted)', label: c.status }
  return (
    <Link
      to={`/resources?node=${nodeId}&container=${c.id}`}
      className="flex items-center gap-3 px-4 py-1.5"
      style={{ borderTop: '1px solid var(--border-subtle)', textDecoration: 'none' }}
    >
      <span
        title={meta.label}
        style={{ width: 7, height: 7, borderRadius: '50%', backgroundColor: meta.color, flexShrink: 0 }}
      />
      <span className="text-xs font-medium truncate" style={{ color: 'var(--text-primary)', maxWidth: '40%' }}>
        {c.name}
      </span>
      <span className="text-xs font-mono truncate" style={{ color: 'var(--text-muted)' }}>
        {c.image}
      </span>
      {c.compose_project && (
        <span
          className="text-xs font-mono ml-auto px-1.5 py-0.5 truncate shrink-0"
          style={{
            color: 'var(--text-secondary)',
            backgroundColor: 'var(--bg-elevated)',
            border: '1px solid var(--border-subtle)',
            borderRadius: '3px',
            maxWidth: '120px',
          }}
          title={c.compose_project}
        >
          {c.compose_project}
        </span>
      )}
    </Link>
  )
}
