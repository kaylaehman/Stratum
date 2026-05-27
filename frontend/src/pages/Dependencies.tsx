import { useState, useMemo } from 'react'
import { Workflow, Loader, AlertTriangle, Info } from 'lucide-react'
import { AppShell } from '../components/layout/AppShell'
import { DependencyGraph } from '../components/depgraph/DependencyGraph'
import { useTree } from '../lib/api/tree'
import { useNodeDepGraph } from '../lib/api/depgraph'
import { ApiError } from '../lib/api'
import type { DepGraphNode } from '../types/api'

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

function selectStyle() {
  return {
    backgroundColor: 'var(--bg-elevated)',
    border: '1px solid var(--border-default)',
    color: 'var(--text-primary)',
    borderRadius: '3px',
    cursor: 'pointer',
    outline: 'none',
  } as React.CSSProperties
}

// ── Error / warn banners ──────────────────────────────────────────────────────

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

// ── Legend ────────────────────────────────────────────────────────────────────

function Legend() {
  const nodeItems = [
    { color: 'var(--accent)', label: 'Container' },
    { color: '#4a8abf', label: 'Network' },
    { color: '#bf8a4a', label: 'Volume' },
  ]
  const edgeItems = [
    { color: '#4a8abf', dash: false, label: 'Network membership (solid)' },
    { color: '#bf8a4a', dash: true, label: 'Volume attachment (dashed)' },
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
        <span className="text-xs uppercase tracking-wider" style={{ color: 'var(--text-muted)', fontSize: '9px' }}>
          Legend
        </span>
      </div>
      {nodeItems.map((item) => (
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
          <span className="text-xs" style={{ color: 'var(--text-secondary)', fontSize: '10px' }}>
            {item.label}
          </span>
        </div>
      ))}
      {edgeItems.map((item) => (
        <div key={item.label} className="flex items-center gap-1.5">
          <svg width="20" height="10" style={{ flexShrink: 0 }}>
            <line
              x1={0} y1={5} x2={20} y2={5}
              stroke={item.color}
              strokeWidth={1.5}
              strokeDasharray={item.dash ? '4 3' : undefined}
            />
          </svg>
          <span className="text-xs" style={{ color: 'var(--text-secondary)', fontSize: '10px' }}>
            {item.label}
          </span>
        </div>
      ))}
    </div>
  )
}

// ── Detail panel ──────────────────────────────────────────────────────────────

interface DetailPanelProps {
  node: DepGraphNode
  allNodes: DepGraphNode[]
  edges: { source: string; target: string; kind: string }[]
}

function DetailPanel({ node, allNodes, edges }: DetailPanelProps) {
  const nodeById = useMemo(() => {
    const m = new Map<string, DepGraphNode>()
    for (const n of allNodes) m.set(n.id, n)
    return m
  }, [allNodes])

  function connectedLabels(ids: string[]) {
    return ids.map((id) => nodeById.get(id)?.label ?? id)
  }

  const header = (
    <div
      className="px-2.5 py-1.5"
      style={{ borderBottom: '1px solid var(--border-subtle)', backgroundColor: 'var(--bg-elevated)' }}
    >
      <div className="flex items-center justify-between gap-2">
        <span className="font-mono text-xs" style={{ color: 'var(--text-primary)' }}>
          {node.label}
        </span>
        <span
          className="font-mono uppercase text-xs"
          style={{
            color: 'var(--text-muted)',
            fontSize: '9px',
            letterSpacing: '0.06em',
            background: 'var(--bg-surface)',
            border: '1px solid var(--border-subtle)',
            borderRadius: '3px',
            padding: '0 4px',
          }}
        >
          {node.kind}
        </span>
      </div>
    </div>
  )

  if (node.kind === 'container') {
    const netIds = edges.filter((e) => e.source === node.id && e.kind === 'network').map((e) => e.target)
    const volIds = edges.filter((e) => e.source === node.id && e.kind === 'volume').map((e) => e.target)
    return (
      <div style={{ backgroundColor: 'var(--bg-surface)', border: '1px solid var(--border-subtle)', borderRadius: '3px', overflow: 'hidden' }}>
        {header}
        <div className="px-2.5 py-2" style={{ display: 'flex', flexDirection: 'column', gap: '8px' }}>
          {node.status && (
            <Row label="Status" value={node.status} accent={node.status === 'running'} />
          )}
          {node.compose_project && (
            <Row label="Project" value={node.compose_project} />
          )}
          {netIds.length > 0 && (
            <div>
              <div className="text-xs uppercase tracking-wider mb-1" style={{ color: 'var(--text-muted)', fontSize: '9px' }}>
                Networks ({netIds.length})
              </div>
              {connectedLabels(netIds).map((l) => (
                <div key={l} className="font-mono text-xs" style={{ color: '#4a8abf', paddingLeft: '4px', marginBottom: '2px' }}>
                  {l}
                </div>
              ))}
            </div>
          )}
          {volIds.length > 0 && (
            <div>
              <div className="text-xs uppercase tracking-wider mb-1" style={{ color: 'var(--text-muted)', fontSize: '9px' }}>
                Volumes ({volIds.length})
              </div>
              {connectedLabels(volIds).map((l) => (
                <div key={l} className="font-mono text-xs" style={{ color: '#bf8a4a', paddingLeft: '4px', marginBottom: '2px' }}>
                  {l}
                </div>
              ))}
            </div>
          )}
        </div>
      </div>
    )
  }

  if (node.kind === 'network') {
    const ctrIds = edges.filter((e) => e.target === node.id && e.kind === 'network').map((e) => e.source)
    return (
      <div style={{ backgroundColor: 'var(--bg-surface)', border: '1px solid var(--border-subtle)', borderRadius: '3px', overflow: 'hidden' }}>
        {header}
        <div className="px-2.5 py-2" style={{ display: 'flex', flexDirection: 'column', gap: '8px' }}>
          {node.driver && <Row label="Driver" value={node.driver} />}
          {ctrIds.length > 0 && (
            <div>
              <div className="text-xs uppercase tracking-wider mb-1" style={{ color: 'var(--text-muted)', fontSize: '9px' }}>
                Containers ({ctrIds.length})
              </div>
              {connectedLabels(ctrIds).map((l) => (
                <div key={l} className="font-mono text-xs" style={{ color: 'var(--accent)', paddingLeft: '4px', marginBottom: '2px' }}>
                  {l}
                </div>
              ))}
            </div>
          )}
        </div>
      </div>
    )
  }

  // volume
  const ctrIds = edges.filter((e) => e.target === node.id && e.kind === 'volume').map((e) => e.source)
  return (
    <div style={{ backgroundColor: 'var(--bg-surface)', border: '1px solid var(--border-subtle)', borderRadius: '3px', overflow: 'hidden' }}>
      {header}
      <div className="px-2.5 py-2" style={{ display: 'flex', flexDirection: 'column', gap: '8px' }}>
        {ctrIds.length > 0 && (
          <div>
            <div className="text-xs uppercase tracking-wider mb-1" style={{ color: 'var(--text-muted)', fontSize: '9px' }}>
              Containers ({ctrIds.length})
            </div>
            {connectedLabels(ctrIds).map((l) => (
              <div key={l} className="font-mono text-xs" style={{ color: 'var(--accent)', paddingLeft: '4px', marginBottom: '2px' }}>
                {l}
              </div>
            ))}
          </div>
        )}
        {ctrIds.length === 0 && (
          <span className="text-xs" style={{ color: 'var(--text-muted)' }}>No containers attached</span>
        )}
      </div>
    </div>
  )
}

function Row({ label, value, accent }: { label: string; value: string; accent?: boolean }) {
  return (
    <div className="flex items-center justify-between gap-2">
      <span className="text-xs uppercase tracking-wider" style={{ color: 'var(--text-muted)', fontSize: '9px' }}>
        {label}
      </span>
      <span className="font-mono text-xs" style={{ color: accent ? 'var(--accent)' : 'var(--text-secondary)' }}>
        {value}
      </span>
    </div>
  )
}

// ── Main page ─────────────────────────────────────────────────────────────────

export default function Dependencies() {
  const { data: tree } = useTree()
  const dockerNodes = useMemo(
    () => (tree?.nodes ?? []).filter((n) => n.capabilities.docker),
    [tree],
  )

  const [selectedNodeId, setSelectedNodeId] = useState<string | null>(null)
  const [selectedGraphNodeId, setSelectedGraphNodeId] = useState<string | null>(null)
  const [selectedProject, setSelectedProject] = useState<string | null>(null)

  const activeNodeId = selectedNodeId ?? dockerNodes[0]?.id ?? null

  const { data: graph, isLoading, error } = useNodeDepGraph(activeNodeId)

  // Distinct compose projects from container nodes
  const projects = useMemo(() => {
    if (!graph) return []
    const set = new Set<string>()
    for (const n of graph.nodes) {
      if (n.kind === 'container' && n.compose_project) set.add(n.compose_project)
    }
    return [...set].sort()
  }, [graph])

  // Selected graph node object
  const selectedGraphNode = useMemo(
    () => (graph && selectedGraphNodeId ? graph.nodes.find((n) => n.id === selectedGraphNodeId) ?? null : null),
    [graph, selectedGraphNodeId],
  )

  function handleNodeChange(id: string) {
    setSelectedNodeId(id)
    setSelectedGraphNodeId(null)
    setSelectedProject(null)
  }

  function handleProjectChange(proj: string) {
    setSelectedProject(proj === '' ? null : proj)
    setSelectedGraphNodeId(null)
  }

  function errorMessage(): string | null {
    if (!error) return null
    if (error instanceof ApiError) {
      if (error.status === 409) return 'No Docker available on this node.'
      if (error.status === 502) return 'Node is unreachable.'
      if (error.status === 404) return 'Node not found.'
    }
    return 'Failed to load dependency graph.'
  }

  const errMsg = errorMessage()

  return (
    <AppShell>
      <div
        className="flex flex-col flex-1 min-h-0 h-full w-full p-6"
        style={{ maxWidth: '1200px', margin: '0 auto' }}
      >
        {/* Page header */}
        <div className="flex items-center justify-between gap-4 mb-6 flex-wrap">
          <div className="flex items-center gap-2">
            <Workflow size={16} style={{ color: 'var(--text-secondary)' }} />
            <h1
              className="text-sm font-medium uppercase tracking-wider"
              style={{ color: 'var(--text-primary)' }}
            >
              Dependency Graph
            </h1>
          </div>

          <div className="flex items-center gap-4 flex-wrap">
            {/* Node selector */}
            {dockerNodes.length > 0 && (
              <div className="flex items-center gap-2">
                <SectionLabel>Node</SectionLabel>
                <select
                  value={activeNodeId ?? ''}
                  onChange={(e) => handleNodeChange(e.target.value)}
                  className="text-xs font-mono px-2 py-1"
                  style={selectStyle()}
                >
                  {dockerNodes.map((n) => (
                    <option key={n.id} value={n.id}>
                      {n.name}
                    </option>
                  ))}
                </select>
              </div>
            )}

            {/* Compose project filter */}
            {projects.length > 0 && (
              <div className="flex items-center gap-2">
                <SectionLabel>Project</SectionLabel>
                <select
                  value={selectedProject ?? ''}
                  onChange={(e) => handleProjectChange(e.target.value)}
                  className="text-xs font-mono px-2 py-1"
                  style={selectStyle()}
                >
                  <option value="">All projects</option>
                  {projects.map((p) => (
                    <option key={p} value={p}>
                      {p}
                    </option>
                  ))}
                </select>
              </div>
            )}
          </div>
        </div>

        {/* No docker nodes */}
        {dockerNodes.length === 0 && !isLoading && (
          <WarnBanner message="No Docker-capable nodes registered. Add a standalone or Proxmox node with Docker enabled." />
        )}

        {/* Loading */}
        {isLoading && activeNodeId && (
          <div className="flex items-center gap-2 py-8">
            <Loader size={13} className="animate-spin" style={{ color: 'var(--accent)' }} />
            <span className="text-xs" style={{ color: 'var(--text-muted)' }}>
              Loading dependency graph...
            </span>
          </div>
        )}

        {/* API error */}
        {errMsg && <ErrorBanner message={errMsg} />}

        {/* Main layout */}
        {graph && !errMsg && (
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
              <DependencyGraph
                graph={graph}
                selectedProject={selectedProject}
                selectedNodeId={selectedGraphNodeId}
                onSelectNode={setSelectedGraphNodeId}
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
              {selectedGraphNode ? (
                <DetailPanel
                  node={selectedGraphNode}
                  allNodes={graph.nodes}
                  edges={graph.edges}
                />
              ) : (
                <div
                  className="px-3 py-4 text-xs text-center"
                  style={{
                    backgroundColor: 'var(--bg-surface)',
                    border: '1px solid var(--border-subtle)',
                    borderRadius: '3px',
                    color: 'var(--text-muted)',
                  }}
                >
                  Click any node to see details
                </div>
              )}

              {/* Stats summary */}
              <div
                className="px-2.5 py-2"
                style={{
                  backgroundColor: 'var(--bg-surface)',
                  border: '1px solid var(--border-subtle)',
                  borderRadius: '3px',
                  display: 'grid',
                  gridTemplateColumns: '1fr 1fr 1fr',
                  gap: '6px',
                }}
              >
                <Stat
                  label="Containers"
                  value={graph.nodes.filter((n) => n.kind === 'container').length}
                />
                <Stat
                  label="Networks"
                  value={graph.nodes.filter((n) => n.kind === 'network').length}
                />
                <Stat
                  label="Volumes"
                  value={graph.nodes.filter((n) => n.kind === 'volume').length}
                />
              </div>
            </div>
          </div>
        )}
      </div>
    </AppShell>
  )
}

function Stat({ label, value }: { label: string; value: number }) {
  return (
    <div>
      <div className="text-xs uppercase tracking-wider" style={{ color: 'var(--text-muted)', fontSize: '9px' }}>
        {label}
      </div>
      <div className="font-mono text-sm" style={{ color: 'var(--text-primary)' }}>
        {value}
      </div>
    </div>
  )
}
