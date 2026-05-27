import type { TopologyResponse, TopologyNetwork, TopologyContainer } from '../../types/api'

// ── Layout constants ──────────────────────────────────────────────────────────

const NET_COL_X = 80
const CTR_COL_X = 380
const ISOLATED_COL_X = 560
const NODE_W = 160
const NET_H = 36
const CTR_H = 28
const ROW_GAP = 18
const TOP_PAD = 40

// ── Position helpers ──────────────────────────────────────────────────────────

function netCY(i: number): number {
  return TOP_PAD + i * (NET_H + ROW_GAP) + NET_H / 2
}

function ctrCY(i: number): number {
  return TOP_PAD + i * (CTR_H + ROW_GAP) + CTR_H / 2
}

function isolatedCY(i: number): number {
  return TOP_PAD + i * (CTR_H + ROW_GAP) + CTR_H / 2
}

function svgHeight(networks: TopologyNetwork[], regularCtrs: TopologyContainer[], isolatedCtrs: TopologyContainer[]): number {
  const netH = networks.length * (NET_H + ROW_GAP)
  const ctrH = regularCtrs.length * (CTR_H + ROW_GAP)
  const isoH = isolatedCtrs.length * (CTR_H + ROW_GAP)
  return TOP_PAD + Math.max(netH, ctrH, isoH) + 40
}

// ── Props ─────────────────────────────────────────────────────────────────────

export type GraphSelection =
  | { kind: 'network'; id: string }
  | { kind: 'container'; dockerId: string }
  | null

interface TopologyGraphProps {
  topology: TopologyResponse
  selection: GraphSelection
  onSelect: (sel: GraphSelection) => void
}

// ── Component ─────────────────────────────────────────────────────────────────

export function TopologyGraph({ topology, selection, onSelect }: TopologyGraphProps) {
  const { networks, containers } = topology

  const regularCtrs = containers.filter((c) => !c.isolated && !c.host_network)
  const specialCtrs = containers.filter((c) => c.isolated || c.host_network)

  // Build lookup: network name → index
  const netIndexByName = new Map<string, number>()
  networks.forEach((n, i) => netIndexByName.set(n.name, i))

  // Build edges: { netIdx, ctrIdx }
  interface Edge { netIdx: number; ctrIdx: number }
  const edges: Edge[] = []
  regularCtrs.forEach((c, ci) => {
    for (const nname of c.networks) {
      const ni = netIndexByName.get(nname)
      if (ni !== undefined) edges.push({ netIdx: ni, ctrIdx: ci })
    }
  })

  // Determine if a node is highlighted
  function netHighlighted(ni: number): boolean {
    if (!selection) return true
    if (selection.kind === 'network') return networks[ni].id === selection.id
    if (selection.kind === 'container') {
      return edges.some((e) => e.netIdx === ni && regularCtrs[e.ctrIdx]?.docker_id === selection.dockerId)
    }
    return false
  }

  function ctrHighlighted(ci: number): boolean {
    if (!selection) return true
    if (selection.kind === 'container') return regularCtrs[ci].docker_id === selection.dockerId
    if (selection.kind === 'network') {
      return edges.some((e) => e.ctrIdx === ci && networks[e.netIdx]?.id === selection.id)
    }
    return false
  }

  function edgeHighlighted(e: Edge): boolean {
    if (!selection) return true
    if (selection.kind === 'network') return networks[e.netIdx]?.id === selection.id
    if (selection.kind === 'container') return regularCtrs[e.ctrIdx]?.docker_id === selection.dockerId
    return false
  }

  const totalH = svgHeight(networks, regularCtrs, specialCtrs)
  const needIsolatedCol = specialCtrs.length > 0
  const svgW = needIsolatedCol ? ISOLATED_COL_X + NODE_W + 40 : CTR_COL_X + NODE_W + 40

  // ── Render helpers ──────────────────────────────────────────────────────────

  function renderNetwork(net: TopologyNetwork, ni: number) {
    const cy = netCY(ni)
    const x = NET_COL_X
    const y = cy - NET_H / 2
    const isSelected = selection?.kind === 'network' && selection.id === net.id
    const dimmed = !netHighlighted(ni)
    const opacity = dimmed ? 0.25 : 1

    return (
      <g
        key={net.id}
        style={{ cursor: 'pointer', opacity }}
        onClick={(e) => {
          e.stopPropagation()
          onSelect(isSelected ? null : { kind: 'network', id: net.id })
        }}
      >
        <rect
          x={x}
          y={y}
          width={NODE_W}
          height={NET_H}
          rx={3}
          style={{
            fill: net.internal ? 'rgba(74,82,104,0.3)' : 'var(--bg-elevated)',
            stroke: isSelected ? 'var(--accent)' : 'var(--border-default)',
            strokeWidth: isSelected ? 1.5 : 1,
          }}
        />
        <text
          x={x + NODE_W / 2}
          y={y + 13}
          textAnchor="middle"
          style={{ fill: 'var(--text-primary)', fontSize: '11px', fontFamily: 'monospace' }}
        >
          {net.name.length > 18 ? net.name.slice(0, 17) + '…' : net.name}
        </text>
        <text
          x={x + NODE_W / 2}
          y={y + 26}
          textAnchor="middle"
          style={{ fill: 'var(--text-muted)', fontSize: '9px', fontFamily: 'monospace', textTransform: 'uppercase', letterSpacing: '0.06em' }}
        >
          {net.driver}{net.internal ? ' · internal' : ''}
        </text>
      </g>
    )
  }

  function renderContainer(ctr: TopologyContainer, ci: number, cx: number, cy: number, flag: boolean) {
    const x = cx
    const y = cy - CTR_H / 2
    const running = ctr.status === 'running'
    const isSelected = selection?.kind === 'container' && selection.dockerId === ctr.docker_id
    const dimmed = !ctrHighlighted(ci)
    const opacity = flag ? 1 : (dimmed ? 0.25 : 1)

    return (
      <g
        key={ctr.docker_id}
        style={{ cursor: 'pointer', opacity }}
        onClick={(e) => {
          e.stopPropagation()
          onSelect(isSelected ? null : { kind: 'container', dockerId: ctr.docker_id })
        }}
      >
        <rect
          x={x}
          y={y}
          width={NODE_W}
          height={CTR_H}
          rx={3}
          style={{
            fill: running ? 'rgba(100,210,172,0.08)' : 'var(--bg-surface)',
            stroke: flag
              ? 'var(--status-error)'
              : isSelected
              ? 'var(--accent)'
              : running
              ? 'rgba(100,210,172,0.4)'
              : 'var(--border-subtle)',
            strokeWidth: (flag || isSelected) ? 1.5 : 1,
          }}
        />
        <circle
          cx={x + 10}
          cy={cy}
          r={3.5}
          style={{ fill: running ? 'var(--accent)' : 'var(--text-muted)' }}
        />
        <text
          x={x + 20}
          y={cy + 4}
          style={{ fill: running ? 'var(--text-primary)' : 'var(--text-secondary)', fontSize: '11px', fontFamily: 'monospace' }}
        >
          {ctr.name.length > 17 ? ctr.name.slice(0, 16) + '…' : ctr.name}
        </text>
      </g>
    )
  }

  function renderEdge(e: Edge, idx: number) {
    const highlighted = edgeHighlighted(e)
    const x1 = NET_COL_X + NODE_W
    const y1 = netCY(e.netIdx)
    const x2 = CTR_COL_X
    const y2 = ctrCY(e.ctrIdx)
    const mx = (x1 + x2) / 2

    return (
      <path
        key={idx}
        d={`M${x1},${y1} C${mx},${y1} ${mx},${y2} ${x2},${y2}`}
        fill="none"
        style={{
          stroke: highlighted ? 'var(--accent)' : 'var(--border-subtle)',
          strokeWidth: highlighted ? 1.5 : 1,
          opacity: selection && !highlighted ? 0.15 : 0.6,
        }}
      />
    )
  }

  if (networks.length === 0 && containers.length === 0) {
    return (
      <div
        style={{
          color: 'var(--text-muted)',
          fontSize: '12px',
          padding: '32px 0',
          textAlign: 'center',
        }}
      >
        No Docker networks found on this node.
      </div>
    )
  }

  return (
    <svg
      width="100%"
      viewBox={`0 0 ${svgW} ${totalH}`}
      style={{ display: 'block', overflow: 'visible' }}
      onClick={() => onSelect(null)}
    >
      {/* Column labels */}
      <text x={NET_COL_X + NODE_W / 2} y={18} textAnchor="middle"
        style={{ fill: 'var(--text-muted)', fontSize: '9px', fontFamily: 'monospace', textTransform: 'uppercase', letterSpacing: '0.08em' }}>
        Networks
      </text>
      <text x={CTR_COL_X + NODE_W / 2} y={18} textAnchor="middle"
        style={{ fill: 'var(--text-muted)', fontSize: '9px', fontFamily: 'monospace', textTransform: 'uppercase', letterSpacing: '0.08em' }}>
        Containers
      </text>
      {needIsolatedCol && (
        <text x={ISOLATED_COL_X + NODE_W / 2} y={18} textAnchor="middle"
          style={{ fill: 'var(--status-error)', fontSize: '9px', fontFamily: 'monospace', textTransform: 'uppercase', letterSpacing: '0.08em', opacity: 0.8 }}>
          Isolated / Host-net
        </text>
      )}

      {/* Edges (behind nodes) */}
      {edges.map((e, i) => renderEdge(e, i))}

      {/* Network nodes */}
      {networks.map((net, ni) => renderNetwork(net, ni))}

      {/* Regular container nodes */}
      {regularCtrs.map((c, ci) => renderContainer(c, ci, CTR_COL_X, ctrCY(ci), false))}

      {/* Isolated / host-network containers */}
      {specialCtrs.map((c, si) =>
        renderContainer(c, si, ISOLATED_COL_X, isolatedCY(si), true)
      )}
    </svg>
  )
}
