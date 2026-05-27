import type { DepGraph, DepGraphNode, DepGraphEdge } from '../../types/api'

// ── Layout constants ──────────────────────────────────────────────────────────

const NET_COL_X = 40
const CTR_COL_X = 280
const VOL_COL_X = 520
const NODE_W = 180
const NODE_H = 32
const ROW_GAP = 16
const TOP_PAD = 40

// ── Color tokens per kind ─────────────────────────────────────────────────────

const KIND_STROKE: Record<string, string> = {
  container: 'var(--accent)',
  network: '#4a8abf',
  volume: '#bf8a4a',
}

const KIND_FILL: Record<string, string> = {
  container: 'rgba(100,210,172,0.08)',
  network: 'rgba(74,138,191,0.10)',
  volume: 'rgba(191,138,74,0.10)',
}

// ── Position helpers ──────────────────────────────────────────────────────────

function colCY(index: number): number {
  return TOP_PAD + index * (NODE_H + ROW_GAP) + NODE_H / 2
}

function svgHeight(counts: number[]): number {
  const rows = Math.max(...counts, 1)
  return TOP_PAD + rows * (NODE_H + ROW_GAP) + 40
}

// ── Props ─────────────────────────────────────────────────────────────────────

export interface DepGraphSelection {
  id: string
  kind: 'container' | 'network' | 'volume'
}

interface DependencyGraphProps {
  graph: DepGraph
  selectedProject: string | null
  selectedNodeId: string | null
  onSelectNode: (id: string | null) => void
}

// ── Helpers ───────────────────────────────────────────────────────────────────

function truncate(s: string, max: number): string {
  return s.length > max ? s.slice(0, max - 1) + '…' : s
}


// ── Component ─────────────────────────────────────────────────────────────────

export function DependencyGraph({
  graph,
  selectedProject,
  selectedNodeId,
  onSelectNode,
}: DependencyGraphProps) {
  const { nodes, edges } = graph

  // Determine container IDs that belong to selected project's reachable nets/vols
  const projectContainerIds = new Set<string>(
    selectedProject
      ? nodes.filter((n) => n.kind === 'container' && n.compose_project === selectedProject).map((n) => n.id)
      : [],
  )

  // For project filter: build reachable net/vol ids via edges from project containers
  const reachableFromProject = new Set<string>()
  if (selectedProject) {
    for (const e of edges) {
      if (projectContainerIds.has(e.source)) reachableFromProject.add(e.target)
    }
  }

  function isVisible(node: DepGraphNode): boolean {
    if (!selectedProject) return true
    if (node.kind === 'container') return node.compose_project === selectedProject
    return reachableFromProject.has(node.id)
  }

  // Partition nodes by kind, respecting project filter
  const networkNodes = nodes.filter((n) => n.kind === 'network' && isVisible(n))
  const containerNodes = nodes.filter((n) => n.kind === 'container' && isVisible(n))
  const volumeNodes = nodes.filter((n) => n.kind === 'volume' && isVisible(n))

  // Build index maps for position lookup
  const netIndex = new Map(networkNodes.map((n, i) => [n.id, i]))
  const ctrIndex = new Map(containerNodes.map((n, i) => [n.id, i]))
  const volIndex = new Map(volumeNodes.map((n, i) => [n.id, i]))

  // 1-hop neighbor sets for selection highlight
  const connectedToSelected = new Set<string>()
  const highlightedEdges = new Set<number>()
  if (selectedNodeId) {
    edges.forEach((e, i) => {
      if (e.source === selectedNodeId || e.target === selectedNodeId) {
        connectedToSelected.add(e.source === selectedNodeId ? e.target : e.source)
        highlightedEdges.add(i)
      }
    })
  }

  function isDimmed(id: string): boolean {
    if (!selectedNodeId) return false
    return id !== selectedNodeId && !connectedToSelected.has(id)
  }

  function edgeDimmed(edgeIdx: number): boolean {
    if (!selectedNodeId) return false
    return !highlightedEdges.has(edgeIdx)
  }

  const totalH = svgHeight([networkNodes.length, containerNodes.length, volumeNodes.length])
  const svgW = VOL_COL_X + NODE_W + 40

  // Filter edges to visible nodes only
  const visibleEdges = edges.filter((e) => {
    const srcVisible = containerNodes.some((n) => n.id === e.source)
    const tgtVisible =
      (e.kind === 'network' ? networkNodes : volumeNodes).some((n) => n.id === e.target)
    return srcVisible && tgtVisible
  })

  function renderNode(node: DepGraphNode, x: number, index: number) {
    const cy = colCY(index)
    const y = cy - NODE_H / 2
    const isSelected = node.id === selectedNodeId
    const dimmed = isDimmed(node.id)
    const running = node.status === 'running'
    const stroke = isSelected ? 'var(--accent)' : KIND_STROKE[node.kind]

    return (
      <g
        key={node.id}
        style={{ cursor: 'pointer', opacity: dimmed ? 0.25 : 1 }}
        onClick={(ev) => {
          ev.stopPropagation()
          onSelectNode(isSelected ? null : node.id)
        }}
      >
        <rect
          x={x}
          y={y}
          width={NODE_W}
          height={NODE_H}
          rx={3}
          style={{
            fill: KIND_FILL[node.kind],
            stroke,
            strokeWidth: isSelected ? 1.5 : 1,
          }}
        />
        {node.kind === 'container' && (
          <circle
            cx={x + 10}
            cy={cy}
            r={3.5}
            style={{ fill: running ? 'var(--accent)' : 'var(--text-muted)' }}
          />
        )}
        <text
          x={x + (node.kind === 'container' ? 20 : NODE_W / 2)}
          y={cy - 4}
          textAnchor={node.kind === 'container' ? 'start' : 'middle'}
          style={{
            fill: 'var(--text-primary)',
            fontSize: '11px',
            fontFamily: 'monospace',
          }}
        >
          {truncate(node.label, node.kind === 'container' ? 20 : 22)}
        </text>
        {(node.compose_project || node.driver) && (
          <text
            x={x + (node.kind === 'container' ? 20 : NODE_W / 2)}
            y={cy + 9}
            textAnchor={node.kind === 'container' ? 'start' : 'middle'}
            style={{
              fill: 'var(--text-muted)',
              fontSize: '9px',
              fontFamily: 'monospace',
              textTransform: 'uppercase',
              letterSpacing: '0.05em',
            }}
          >
            {node.kind === 'container'
              ? node.compose_project ?? ''
              : node.driver ?? ''}
          </text>
        )}
      </g>
    )
  }

  function renderEdge(edge: DepGraphEdge, edgeIdx: number) {
    const ctrIdx = ctrIndex.get(edge.source)
    if (ctrIdx === undefined) return null

    const isNetwork = edge.kind === 'network'
    const tgtIndex = isNetwork ? netIndex.get(edge.target) : volIndex.get(edge.target)
    if (tgtIndex === undefined) return null

    const ctrCY = colCY(ctrIdx)
    const tgtCY = colCY(tgtIndex)
    const dimmed = edgeDimmed(edgeIdx)
    const highlighted = !dimmed && !!selectedNodeId

    if (isNetwork) {
      // network: container (right edge) → network (left edge) going left
      const x1 = CTR_COL_X
      const y1 = ctrCY
      const x2 = NET_COL_X + NODE_W
      const y2 = tgtCY
      const mx = (x1 + x2) / 2
      return (
        <path
          key={edgeIdx}
          d={`M${x1},${y1} C${mx},${y1} ${mx},${y2} ${x2},${y2}`}
          fill="none"
          style={{
            stroke: highlighted ? '#4a8abf' : 'var(--border-subtle)',
            strokeWidth: highlighted ? 1.5 : 1,
            opacity: selectedNodeId && dimmed ? 0.12 : 0.55,
            strokeDasharray: undefined,
          }}
        />
      )
    } else {
      // volume: container (right edge) → volume (left edge) going right
      const x1 = CTR_COL_X + NODE_W
      const y1 = ctrCY
      const x2 = VOL_COL_X
      const y2 = tgtCY
      const mx = (x1 + x2) / 2
      return (
        <path
          key={edgeIdx}
          d={`M${x1},${y1} C${mx},${y1} ${mx},${y2} ${x2},${y2}`}
          fill="none"
          style={{
            stroke: highlighted ? '#bf8a4a' : 'var(--border-subtle)',
            strokeWidth: highlighted ? 1.5 : 1,
            opacity: selectedNodeId && dimmed ? 0.12 : 0.55,
            strokeDasharray: '4 3',
          }}
        />
      )
    }
  }

  const containers = nodes.filter((n) => n.kind === 'container')
  if (containers.length === 0) {
    return (
      <div
        style={{
          color: 'var(--text-muted)',
          fontSize: '12px',
          padding: '32px 0',
          textAlign: 'center',
        }}
      >
        No containers found on this node.
      </div>
    )
  }

  return (
    <svg
      width="100%"
      viewBox={`0 0 ${svgW} ${totalH}`}
      style={{ display: 'block', overflow: 'visible' }}
      onClick={() => onSelectNode(null)}
    >
      {/* Column labels */}
      <text
        x={NET_COL_X + NODE_W / 2}
        y={18}
        textAnchor="middle"
        style={{ fill: '#4a8abf', fontSize: '9px', fontFamily: 'monospace', textTransform: 'uppercase', letterSpacing: '0.08em' }}
      >
        Networks
      </text>
      <text
        x={CTR_COL_X + NODE_W / 2}
        y={18}
        textAnchor="middle"
        style={{ fill: 'var(--text-muted)', fontSize: '9px', fontFamily: 'monospace', textTransform: 'uppercase', letterSpacing: '0.08em' }}
      >
        Containers
      </text>
      <text
        x={VOL_COL_X + NODE_W / 2}
        y={18}
        textAnchor="middle"
        style={{ fill: '#bf8a4a', fontSize: '9px', fontFamily: 'monospace', textTransform: 'uppercase', letterSpacing: '0.08em' }}
      >
        Volumes
      </text>

      {/* Edges (behind nodes) */}
      {visibleEdges.map((e, i) => renderEdge(e, i))}

      {/* Network nodes */}
      {networkNodes.map((n, i) => renderNode(n, NET_COL_X, i))}

      {/* Container nodes */}
      {containerNodes.map((n, i) => renderNode(n, CTR_COL_X, i))}

      {/* Volume nodes */}
      {volumeNodes.map((n, i) => renderNode(n, VOL_COL_X, i))}
    </svg>
  )
}
