import { useState, useMemo } from 'react'
import { Share2, Loader, AlertTriangle, Info } from 'lucide-react'
import { AppShell } from '../components/layout/AppShell'
import { TopologyGraph, type GraphSelection } from '../components/network/TopologyGraph'
import { useTree } from '../lib/api/tree'
import { useNodeTopology } from '../lib/api/topology'
import { ApiError } from '../lib/api'
import type { TopologyNetwork, TopologyContainer } from '../types/api'

// ── Helpers ───────────────────────────────────────────────────────────────────

function SectionLabel({ children }: { children: React.ReactNode }) {
  return (
    <span
      className="text-xs font-medium uppercase tracking-wider"
      style={{ color: 'var(--text-muted)' }}
    >
      {children}
    </span>
  )
}

// ── Network detail row ────────────────────────────────────────────────────────

function NetworkRow({ net, highlighted }: { net: TopologyNetwork; highlighted: boolean }) {
  const endpointCount = net.endpoints.length
  return (
    <div
      style={{
        padding: '8px 10px',
        borderBottom: '1px solid var(--border-subtle)',
        backgroundColor: highlighted ? 'var(--accent-glow)' : 'transparent',
        borderLeft: highlighted ? '2px solid var(--accent)' : '2px solid transparent',
      }}
    >
      <div className="flex items-center justify-between gap-2 mb-1">
        <span className="font-mono text-xs" style={{ color: 'var(--text-primary)' }}>
          {net.name}
        </span>
        <span
          className="font-mono uppercase text-xs"
          style={{
            color: 'var(--text-muted)',
            fontSize: '12px',
            letterSpacing: '0.06em',
            background: 'var(--bg-surface)',
            border: '1px solid var(--border-subtle)',
            borderRadius: '3px',
            padding: '0 4px',
          }}
        >
          {net.driver}
        </span>
      </div>
      <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '2px 8px' }}>
        {net.subnet && (
          <span className="font-mono text-xs" style={{ color: 'var(--text-muted)', fontSize: '12px' }}>
            subnet: {net.subnet}
          </span>
        )}
        {net.gateway && (
          <span className="font-mono text-xs" style={{ color: 'var(--text-muted)', fontSize: '12px' }}>
            gw: {net.gateway}
          </span>
        )}
        <span className="font-mono text-xs" style={{ color: 'var(--text-muted)', fontSize: '12px' }}>
          {endpointCount} container{endpointCount !== 1 ? 's' : ''}
        </span>
        {net.internal && (
          <span className="font-mono text-xs" style={{ color: 'var(--status-warn)', fontSize: '12px' }}>
            internal
          </span>
        )}
      </div>
    </div>
  )
}

// ── Isolated/host-network warning list ───────────────────────────────────────

function SpecialContainerList({ containers }: { containers: TopologyContainer[] }) {
  if (containers.length === 0) return null
  return (
    <div
      style={{
        marginTop: '12px',
        backgroundColor: 'rgba(232,64,64,0.07)',
        border: '1px solid rgba(232,64,64,0.25)',
        borderRadius: '3px',
        overflow: 'hidden',
      }}
    >
      <div
        className="flex items-center gap-1.5 px-2.5 py-1.5"
        style={{ borderBottom: '1px solid rgba(232,64,64,0.2)' }}
      >
        <AlertTriangle size={11} style={{ color: 'var(--status-error)', flexShrink: 0 }} />
        <span className="text-xs uppercase tracking-wider font-medium" style={{ color: 'var(--status-error)', fontSize: '12px' }}>
          Flagged containers
        </span>
      </div>
      {containers.map((c) => (
        <div
          key={c.docker_id}
          className="px-2.5 py-1.5 flex items-center justify-between gap-2"
          style={{ borderBottom: '1px solid rgba(232,64,64,0.1)' }}
        >
          <span className="font-mono text-xs" style={{ color: 'var(--text-primary)' }}>
            {c.name}
          </span>
          <span
            className="font-mono uppercase text-xs"
            style={{ color: 'var(--status-error)', fontSize: '12px', letterSpacing: '0.05em' }}
          >
            {c.isolated ? 'isolated' : 'host-net'}
          </span>
        </div>
      ))}
    </div>
  )
}

// ── Legend ────────────────────────────────────────────────────────────────────

function Legend() {
  const items: { color: string; label: string }[] = [
    { color: 'var(--accent)', label: 'Running container / highlighted edge' },
    { color: 'var(--text-muted)', label: 'Stopped container' },
    { color: 'var(--border-default)', label: 'Network node' },
    { color: 'rgba(74,82,104,0.5)', label: 'Internal network' },
    { color: 'var(--status-error)', label: 'Isolated / host-network container' },
  ]
  return (
    <div
      className="flex flex-wrap gap-x-4 gap-y-1.5 px-3 py-2"
      style={{
        backgroundColor: 'var(--bg-surface)',
        border: '1px solid var(--border-subtle)',
        borderRadius: '3px',
        marginTop: '12px',
      }}
    >
      <div className="flex items-center gap-1.5 w-full mb-0.5">
        <Info size={10} style={{ color: 'var(--text-muted)', flexShrink: 0 }} />
        <span className="text-xs uppercase tracking-wider" style={{ color: 'var(--text-muted)', fontSize: '12px' }}>
          Legend
        </span>
      </div>
      {items.map((item) => (
        <div key={item.label} className="flex items-center gap-1.5">
          <span
            style={{
              display: 'inline-block',
              width: '10px',
              height: '10px',
              borderRadius: '2px',
              backgroundColor: item.color,
              flexShrink: 0,
            }}
          />
          <span className="text-xs" style={{ color: 'var(--text-secondary)', fontSize: '12px' }}>
            {item.label}
          </span>
        </div>
      ))}
    </div>
  )
}

// ── Error banners ─────────────────────────────────────────────────────────────

function ErrorBanner({ message }: { message: string }) {
  return (
    <div
      className="flex items-center gap-2 px-3 py-2 text-xs"
      style={{
        backgroundColor: 'rgba(232,64,64,0.1)',
        border: '1px solid rgba(232,64,64,0.3)',
        borderRadius: '3px',
        color: 'var(--status-error)',
      }}
    >
      <AlertTriangle size={12} style={{ flexShrink: 0 }} />
      {message}
    </div>
  )
}

function WarnBanner({ message }: { message: string }) {
  return (
    <div
      className="flex items-center gap-2 px-3 py-2 text-xs"
      style={{
        backgroundColor: 'rgba(240,160,32,0.1)',
        border: '1px solid rgba(240,160,32,0.3)',
        borderRadius: '3px',
        color: 'var(--status-warn)',
      }}
    >
      <AlertTriangle size={12} style={{ flexShrink: 0 }} />
      {message}
    </div>
  )
}

// ── Main page ─────────────────────────────────────────────────────────────────

export default function Network() {
  const { data: tree } = useTree()

  // Show ALL registered nodes in the selector — Docker capability is checked
  // per-node by the backend and communicated back via topology.docker_error.
  // Filtering to `capabilities.docker` here was the root cause of SSH-only and
  // Proxmox nodes being silently excluded from the view.
  const allNodes = useMemo(() => tree?.nodes ?? [], [tree])

  const [selectedNodeId, setSelectedNodeId] = useState<string | null>(null)
  const [graphSelection, setGraphSelection] = useState<GraphSelection>(null)

  // Auto-select first node once tree loads
  const activeNodeId = selectedNodeId ?? allNodes[0]?.id ?? null

  const { data: topology, isLoading, error } = useNodeTopology(activeNodeId)

  const specialCtrs = topology?.containers.filter((c) => c.isolated || c.host_network) ?? []

  // Determine which networks are highlighted in the detail panel
  function isNetHighlighted(net: TopologyNetwork): boolean {
    if (!graphSelection) return false
    if (graphSelection.kind === 'network') return net.id === graphSelection.id
    if (graphSelection.kind === 'container') {
      return (topology?.containers.find((c) => c.docker_id === graphSelection.dockerId)
        ?.networks ?? []).includes(net.name)
    }
    return false
  }

  // Derive a user-visible error message. With the updated backend, 502 is only
  // returned for genuine store errors, not for Docker transport failures.
  // Node-level unreachability is communicated via topology.node_status, and
  // Docker-specific failures via topology.docker_error — both at HTTP 200.
  function errorMessage(): string | null {
    if (!error) return null
    if (error instanceof ApiError) {
      if (error.status === 404) return 'Node not found.'
      if (error.status === 500) return 'Server error loading topology.'
    }
    return 'Failed to load network topology.'
  }

  // Warn when the topology loaded but the node is not reachable (poller-set).
  function reachabilityWarning(): string | null {
    if (!topology) return null
    if (topology.node_status === 'unreachable') {
      return 'Node is unreachable. Topology data may be stale.'
    }
    if (topology.docker_error) {
      return 'Docker daemon is unavailable on this node. Network topology cannot be loaded.'
    }
    return null
  }

  const errMsg = errorMessage()
  const reachWarn = reachabilityWarning()

  return (
    <AppShell>
      <div
        className="flex flex-col flex-1 min-h-0 h-full w-full p-6"
        style={{ maxWidth: '1200px', margin: '0 auto' }}
      >
        {/* Page header */}
        <div className="flex items-center justify-between gap-4 mb-6 flex-wrap">
          <div className="flex items-center gap-2">
            <Share2 size={16} style={{ color: 'var(--text-secondary)' }} />
            <h1
              className="text-sm font-medium uppercase tracking-wider"
              style={{ color: 'var(--text-primary)' }}
            >
              Network Topology
            </h1>
          </div>

          {/* Node selector */}
          {allNodes.length > 0 && (
            <div className="flex items-center gap-2">
              <SectionLabel>Node</SectionLabel>
              <select
                value={activeNodeId ?? ''}
                onChange={(e) => {
                  setSelectedNodeId(e.target.value || null)
                  setGraphSelection(null)
                }}
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
                {allNodes.map((n) => (
                  <option key={n.id} value={n.id}>
                    {n.name}
                  </option>
                ))}
              </select>
            </div>
          )}
        </div>

        {/* No nodes registered */}
        {allNodes.length === 0 && !isLoading && (
          <WarnBanner message="No nodes registered. Add a standalone, Proxmox, or SSH node to get started." />
        )}

        {/* Loading */}
        {isLoading && activeNodeId && (
          <div className="flex items-center gap-2 py-8">
            <Loader size={13} className="animate-spin" style={{ color: 'var(--accent)' }} />
            <span className="text-xs" style={{ color: 'var(--text-muted)' }}>
              Loading topology...
            </span>
          </div>
        )}

        {/* Hard API error (node not found / server error) */}
        {errMsg && <ErrorBanner message={errMsg} />}

        {/* Soft reachability / Docker warning — topology response loaded but
            the node is down or Docker is unavailable. Show the warning above
            the (empty) graph so the user understands why it's blank. */}
        {reachWarn && <WarnBanner message={reachWarn} />}

        {/* Main layout: graph + detail panel */}
        {topology && !errMsg && (
          <div className="flex gap-4 flex-1 min-h-0" style={{ alignItems: 'flex-start' }}>
            {/* Graph area */}
            <div
              style={{
                flex: '1 1 0',
                minWidth: 0,
                backgroundColor: 'var(--bg-surface)',
                border: '1px solid var(--border-subtle)',
                borderRadius: '3px',
                padding: '12px',
                overflowX: 'auto',
              }}
            >
              <TopologyGraph
                topology={topology}
                selection={graphSelection}
                onSelect={setGraphSelection}
              />
              <Legend />
            </div>

            {/* Detail panel */}
            <div
              style={{
                width: '240px',
                flexShrink: 0,
                display: 'flex',
                flexDirection: 'column',
                gap: '8px',
              }}
            >
              {/* Networks list */}
              <div
                style={{
                  backgroundColor: 'var(--bg-surface)',
                  border: '1px solid var(--border-subtle)',
                  borderRadius: '3px',
                  overflow: 'hidden',
                }}
              >
                <div
                  className="px-2.5 py-1.5"
                  style={{ borderBottom: '1px solid var(--border-subtle)', backgroundColor: 'var(--bg-elevated)' }}
                >
                  <SectionLabel>Networks ({topology.networks.length})</SectionLabel>
                </div>
                {topology.networks.length === 0 ? (
                  <div className="px-3 py-3 text-xs" style={{ color: 'var(--text-muted)' }}>
                    None
                  </div>
                ) : (
                  topology.networks.map((net) => (
                    <NetworkRow
                      key={net.id}
                      net={net}
                      highlighted={isNetHighlighted(net)}
                    />
                  ))
                )}
              </div>

              {/* Stats */}
              <div
                className="px-2.5 py-2"
                style={{
                  backgroundColor: 'var(--bg-surface)',
                  border: '1px solid var(--border-subtle)',
                  borderRadius: '3px',
                  display: 'grid',
                  gridTemplateColumns: '1fr 1fr',
                  gap: '6px',
                }}
              >
                <div>
                  <div className="text-xs uppercase tracking-wider" style={{ color: 'var(--text-muted)', fontSize: '12px' }}>Containers</div>
                  <div className="font-mono text-sm" style={{ color: 'var(--text-primary)' }}>
                    {topology.containers.length}
                  </div>
                </div>
                <div>
                  <div className="text-xs uppercase tracking-wider" style={{ color: 'var(--text-muted)', fontSize: '12px' }}>Flagged</div>
                  <div
                    className="font-mono text-sm"
                    style={{ color: specialCtrs.length > 0 ? 'var(--status-error)' : 'var(--text-muted)' }}
                  >
                    {specialCtrs.length}
                  </div>
                </div>
              </div>

              {/* Flagged containers */}
              <SpecialContainerList containers={specialCtrs} />
            </div>
          </div>
        )}
      </div>
    </AppShell>
  )
}
