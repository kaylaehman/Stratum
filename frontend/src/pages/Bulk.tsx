import { useState, useMemo } from 'react'
import {
  Play,
  Square,
  RotateCw,
  Trash2,
  Loader,
  Check,
  X,
  Filter,
} from 'lucide-react'
import { AppShell } from '../components/layout/AppShell'
import { useMe } from '../hooks/useMe'
import { useTree } from '../lib/api/tree'
import { useBulkContainers } from '../lib/api/bulk'
import type { BulkAction, BulkResultItem } from '../types/api'

// ---- Types ----

interface FlatContainer {
  id: string
  name: string
  nodeId: string
  nodeName: string
  image: string
  status: string
  composeProject?: string
}

// ---- Helpers ----

function statusColor(status: string): string {
  if (status === 'running') return 'var(--accent)'
  if (status === 'exited') return 'var(--status-error)'
  if (status === 'paused') return 'var(--status-warn)'
  return 'var(--text-muted)'
}

function resultColor(result: BulkResultItem['result']): string {
  if (result === 'ok' || result === 'planned') return 'var(--accent)'
  if (result === 'error') return 'var(--status-error)'
  return 'var(--text-muted)'
}

function resultIcon(result: BulkResultItem['result'], isPending: boolean) {
  if (isPending) return <Loader size={12} style={{ animation: 'spin 1s linear infinite' }} />
  if (result === 'ok') return <Check size={12} />
  if (result === 'planned') return <Check size={12} />
  if (result === 'error') return <X size={12} />
  return <span style={{ fontSize: 10 }}>—</span>
}

function actionIcon(action: BulkAction) {
  if (action === 'start') return <Play size={12} />
  if (action === 'stop') return <Square size={12} />
  if (action === 'restart') return <RotateCw size={12} />
  return <Trash2 size={12} />
}

// ---- Sub-components ----

interface FilterBarProps {
  nodeOptions: string[]
  nodeNames: Record<string, string>
  statusOptions: string[]
  projectOptions: string[]
  nodeFilter: string
  statusFilter: string
  imageFilter: string
  projectFilter: string
  onNode: (v: string) => void
  onStatus: (v: string) => void
  onImage: (v: string) => void
  onProject: (v: string) => void
}

function FilterBar({
  nodeOptions,
  nodeNames,
  statusOptions,
  projectOptions,
  nodeFilter,
  statusFilter,
  imageFilter,
  projectFilter,
  onNode,
  onStatus,
  onImage,
  onProject,
}: FilterBarProps) {
  const selectStyle: React.CSSProperties = {
    background: 'var(--bg-elevated)',
    border: '1px solid var(--border-default)',
    color: 'var(--text-primary)',
    borderRadius: '3px',
    fontSize: '12px',
    fontFamily: 'monospace',
    padding: '4px 6px',
    minWidth: 120,
    cursor: 'pointer',
  }

  const inputStyle: React.CSSProperties = {
    ...selectStyle,
    minWidth: 160,
  }

  return (
    <div
      className="flex items-center gap-2 flex-wrap px-4 py-3"
      style={{ borderBottom: '1px solid var(--border-subtle)' }}
    >
      <Filter size={12} style={{ color: 'var(--text-muted)', flexShrink: 0 }} />
      <span style={{ color: 'var(--text-muted)', fontSize: 10, fontFamily: 'monospace', textTransform: 'uppercase', letterSpacing: '0.08em' }}>
        FILTER
      </span>

      <select style={selectStyle} value={nodeFilter} onChange={e => onNode(e.target.value)}>
        <option value="">All nodes</option>
        {nodeOptions.map(id => (
          <option key={id} value={id}>{nodeNames[id] ?? id}</option>
        ))}
      </select>

      <select style={selectStyle} value={statusFilter} onChange={e => onStatus(e.target.value)}>
        <option value="">Any status</option>
        {statusOptions.map(s => <option key={s} value={s}>{s}</option>)}
      </select>

      <input
        style={inputStyle}
        placeholder="image substring…"
        value={imageFilter}
        onChange={e => onImage(e.target.value)}
      />

      <select style={selectStyle} value={projectFilter} onChange={e => onProject(e.target.value)}>
        <option value="">All projects</option>
        {projectOptions.map(p => <option key={p} value={p}>{p}</option>)}
      </select>
    </div>
  )
}

interface ContainerTableProps {
  containers: FlatContainer[]
  selected: Set<string>
  onToggle: (id: string) => void
  onToggleAll: () => void
}

function ContainerTable({ containers, selected, onToggle, onToggleAll }: ContainerTableProps) {
  const allSelected = containers.length > 0 && containers.every(c => selected.has(c.id))

  const headerCell: React.CSSProperties = {
    fontSize: 10,
    fontFamily: 'monospace',
    textTransform: 'uppercase',
    letterSpacing: '0.08em',
    color: 'var(--text-muted)',
    padding: '6px 8px',
    fontWeight: 600,
    whiteSpace: 'nowrap',
  }

  const cell: React.CSSProperties = {
    padding: '6px 8px',
    fontSize: 11,
    fontFamily: 'monospace',
    color: 'var(--text-secondary)',
    whiteSpace: 'nowrap',
    overflow: 'hidden',
    textOverflow: 'ellipsis',
    maxWidth: 220,
  }

  if (containers.length === 0) {
    return (
      <div style={{ padding: '24px', textAlign: 'center', color: 'var(--text-muted)', fontSize: 12 }}>
        No containers match the current filters.
      </div>
    )
  }

  return (
    <div style={{ overflowX: 'auto' }}>
      <table style={{ width: '100%', borderCollapse: 'collapse', tableLayout: 'fixed' }}>
        <colgroup>
          <col style={{ width: 36 }} />
          <col style={{ width: '22%' }} />
          <col style={{ width: '16%' }} />
          <col style={{ width: '28%' }} />
          <col style={{ width: '14%' }} />
          <col style={{ width: '20%' }} />
        </colgroup>
        <thead>
          <tr style={{ borderBottom: '1px solid var(--border-subtle)' }}>
            <th style={{ ...headerCell, textAlign: 'center' }}>
              <input
                type="checkbox"
                checked={allSelected}
                onChange={onToggleAll}
                style={{ cursor: 'pointer', accentColor: 'var(--accent)' }}
              />
            </th>
            <th style={{ ...headerCell, textAlign: 'left' }}>Name</th>
            <th style={{ ...headerCell, textAlign: 'left' }}>Node</th>
            <th style={{ ...headerCell, textAlign: 'left' }}>Image</th>
            <th style={{ ...headerCell, textAlign: 'left' }}>Status</th>
            <th style={{ ...headerCell, textAlign: 'left' }}>Project</th>
          </tr>
        </thead>
        <tbody>
          {containers.map((c, i) => (
            <tr
              key={c.id}
              onClick={() => onToggle(c.id)}
              style={{
                borderBottom: '1px solid var(--border-subtle)',
                background: selected.has(c.id) ? 'var(--accent-glow)' : (i % 2 === 1 ? 'rgba(255,255,255,0.015)' : 'transparent'),
                cursor: 'pointer',
              }}
            >
              <td style={{ ...cell, textAlign: 'center' }}>
                <input
                  type="checkbox"
                  checked={selected.has(c.id)}
                  onChange={() => onToggle(c.id)}
                  onClick={e => e.stopPropagation()}
                  style={{ cursor: 'pointer', accentColor: 'var(--accent)' }}
                />
              </td>
              <td style={{ ...cell, color: 'var(--text-primary)', fontWeight: 500 }}>{c.name}</td>
              <td style={cell}>{c.nodeName}</td>
              <td style={{ ...cell, color: 'var(--text-muted)' }}>{c.image}</td>
              <td style={{ ...cell, color: statusColor(c.status) }}>{c.status}</td>
              <td style={{ ...cell, color: 'var(--text-muted)' }}>{c.composeProject ?? '—'}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}

interface ResultsTableProps {
  results: BulkResultItem[]
  isDryRun: boolean
  isPending: boolean
  nodeNames: Record<string, string>
}

function ResultsTable({ results, isDryRun, isPending, nodeNames }: ResultsTableProps) {
  const cell: React.CSSProperties = {
    padding: '6px 8px',
    fontSize: 11,
    fontFamily: 'monospace',
    color: 'var(--text-secondary)',
    whiteSpace: 'nowrap',
    overflow: 'hidden',
    textOverflow: 'ellipsis',
    maxWidth: 200,
  }

  const headerCell: React.CSSProperties = {
    fontSize: 10,
    fontFamily: 'monospace',
    textTransform: 'uppercase',
    letterSpacing: '0.08em',
    color: 'var(--text-muted)',
    padding: '6px 8px',
    fontWeight: 600,
  }

  return (
    <div
      style={{
        margin: '0 16px 16px',
        border: '1px solid var(--border-default)',
        borderRadius: '3px',
        background: 'var(--bg-elevated)',
      }}
    >
      <div
        style={{
          padding: '8px 12px',
          borderBottom: '1px solid var(--border-subtle)',
          display: 'flex',
          alignItems: 'center',
          gap: 8,
        }}
      >
        <span style={{ fontSize: 10, fontFamily: 'monospace', textTransform: 'uppercase', letterSpacing: '0.08em', color: isDryRun ? 'var(--status-warn)' : 'var(--accent)', fontWeight: 600 }}>
          {isDryRun ? 'DRY RUN PREVIEW' : 'EXECUTION RESULTS'}
        </span>
        {isPending && <Loader size={12} style={{ color: 'var(--text-muted)', animation: 'spin 1s linear infinite' }} />}
      </div>
      <div style={{ overflowX: 'auto' }}>
        <table style={{ width: '100%', borderCollapse: 'collapse' }}>
          <thead>
            <tr style={{ borderBottom: '1px solid var(--border-subtle)' }}>
              <th style={{ ...headerCell, textAlign: 'left' }}>Container</th>
              <th style={{ ...headerCell, textAlign: 'left' }}>Node</th>
              <th style={{ ...headerCell, textAlign: 'left' }}>Status</th>
              <th style={{ ...headerCell, textAlign: 'left' }}>Result</th>
              <th style={{ ...headerCell, textAlign: 'left' }}>Detail</th>
            </tr>
          </thead>
          <tbody>
            {results.map(r => (
              <tr key={r.container_id} style={{ borderBottom: '1px solid var(--border-subtle)' }}>
                <td style={{ ...cell, color: 'var(--text-primary)' }}>{r.name}</td>
                <td style={cell}>{nodeNames[r.node_id] ?? r.node_id}</td>
                <td style={cell}>{r.status}</td>
                <td style={{ ...cell }}>
                  <span style={{ display: 'flex', alignItems: 'center', gap: 4, color: isPending ? 'var(--text-muted)' : resultColor(r.result) }}>
                    {resultIcon(r.result, isPending)}
                    <span>{isPending ? 'running…' : r.result}</span>
                  </span>
                </td>
                <td style={{ ...cell, color: 'var(--text-muted)' }}>
                  {r.skip_reason ?? r.error ?? '—'}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  )
}

// ---- Main page ----

export default function Bulk() {
  const { data: me, isLoading: meLoading } = useMe()
  const { data: tree, isLoading: treeLoading } = useTree()
  const bulk = useBulkContainers()

  // Filter state
  const [nodeFilter, setNodeFilter] = useState('')
  const [statusFilter, setStatusFilter] = useState('')
  const [imageFilter, setImageFilter] = useState('')
  const [projectFilter, setProjectFilter] = useState('')

  // Selection state
  const [selected, setSelected] = useState<Set<string>>(new Set())

  // Action state
  const [action, setAction] = useState<BulkAction>('stop')
  const [showConfirm, setShowConfirm] = useState(false)

  // Results state
  const [results, setResults] = useState<BulkResultItem[] | null>(null)
  const [lastWasDryRun, setLastWasDryRun] = useState(false)

  // Flatten tree into container list
  const allContainers = useMemo<FlatContainer[]>(() => {
    if (!tree) return []
    const flat: FlatContainer[] = []
    for (const node of tree.nodes) {
      for (const c of node.containers) {
        flat.push({
          id: c.id,
          name: c.name,
          nodeId: node.id,
          nodeName: node.name,
          image: c.image,
          status: c.status,
          composeProject: c.compose_project,
        })
      }
    }
    return flat
  }, [tree])

  // Derive filter options
  const nodeOptions = useMemo(() => tree?.nodes.map(n => n.id) ?? [], [tree])
  const nodeNames = useMemo<Record<string, string>>(() => {
    if (!tree) return {}
    return Object.fromEntries(tree.nodes.map(n => [n.id, n.name]))
  }, [tree])
  const statusOptions = useMemo(() => {
    const s = new Set(allContainers.map(c => c.status))
    return [...s].sort()
  }, [allContainers])
  const projectOptions = useMemo(() => {
    const s = new Set(allContainers.flatMap(c => c.composeProject ? [c.composeProject] : []))
    return [...s].sort()
  }, [allContainers])

  // Filtered list
  const filtered = useMemo(() => {
    return allContainers.filter(c => {
      if (nodeFilter && c.nodeId !== nodeFilter) return false
      if (statusFilter && c.status !== statusFilter) return false
      if (imageFilter && !c.image.toLowerCase().includes(imageFilter.toLowerCase())) return false
      if (projectFilter && c.composeProject !== projectFilter) return false
      return true
    })
  }, [allContainers, nodeFilter, statusFilter, imageFilter, projectFilter])

  function toggleOne(id: string) {
    setSelected(prev => {
      const next = new Set(prev)
      if (next.has(id)) next.delete(id)
      else next.add(id)
      return next
    })
  }

  function toggleAll() {
    const filteredIds = filtered.map(c => c.id)
    const allIn = filteredIds.every(id => selected.has(id))
    setSelected(prev => {
      const next = new Set(prev)
      if (allIn) filteredIds.forEach(id => next.delete(id))
      else filteredIds.forEach(id => next.add(id))
      return next
    })
  }

  async function runBulk(dryRun: boolean) {
    const ids = [...selected]
    setLastWasDryRun(dryRun)
    const res = await bulk.mutateAsync({
      action,
      container_ids: ids,
      dry_run: dryRun,
    })
    setResults(res.results)
  }

  function handleDryRun() {
    void runBulk(true)
  }

  function handleExecute() {
    if (action === 'remove') {
      setShowConfirm(true)
    } else {
      void runBulk(false)
    }
  }

  function confirmRemove() {
    setShowConfirm(false)
    void runBulk(false)
  }

  // Admin gate
  if (!meLoading && me && me.role !== 'admin') {
    return (
      <AppShell>
        <div style={{ padding: 32, display: 'flex', flexDirection: 'column', alignItems: 'center', gap: 12 }}>
          <span style={{ color: 'var(--status-error)', fontSize: 13, fontFamily: 'monospace' }}>
            Admin access required to use Bulk Operations.
          </span>
        </div>
      </AppShell>
    )
  }

  const selectedCount = selected.size
  const actionsDisabled = selectedCount === 0 || bulk.isPending

  const btnBase: React.CSSProperties = {
    fontFamily: 'monospace',
    fontSize: 11,
    textTransform: 'uppercase',
    letterSpacing: '0.08em',
    borderRadius: '3px',
    border: '1px solid var(--border-default)',
    padding: '5px 12px',
    cursor: actionsDisabled ? 'not-allowed' : 'pointer',
    display: 'flex',
    alignItems: 'center',
    gap: 6,
    opacity: actionsDisabled ? 0.5 : 1,
    transition: 'opacity 0.15s',
  }

  const btnPrimary: React.CSSProperties = {
    ...btnBase,
    background: 'var(--accent)',
    border: '1px solid var(--accent)',
    color: '#0a0f14',
    fontWeight: 600,
  }

  const btnGhost: React.CSSProperties = {
    ...btnBase,
    background: 'transparent',
    color: 'var(--text-secondary)',
  }

  const btnDanger: React.CSSProperties = {
    ...btnBase,
    background: action === 'remove' ? 'rgba(232,64,64,0.12)' : 'transparent',
    border: action === 'remove' ? '1px solid rgba(232,64,64,0.4)' : '1px solid var(--border-default)',
    color: action === 'remove' ? 'var(--status-error)' : 'var(--text-secondary)',
  }

  const selectStyle: React.CSSProperties = {
    background: 'var(--bg-elevated)',
    border: '1px solid var(--border-default)',
    color: 'var(--text-primary)',
    borderRadius: '3px',
    fontSize: '12px',
    fontFamily: 'monospace',
    padding: '5px 8px',
    cursor: 'pointer',
  }

  return (
    <AppShell>
      <style>{`@keyframes spin { from { transform: rotate(0deg); } to { transform: rotate(360deg); } }`}</style>

      {/* Header */}
      <div
        style={{
          padding: '16px 20px 12px',
          borderBottom: '1px solid var(--border-subtle)',
          display: 'flex',
          alignItems: 'center',
          gap: 10,
        }}
      >
        <span
          style={{
            fontFamily: 'monospace',
            fontSize: 13,
            fontWeight: 600,
            color: 'var(--text-primary)',
            textTransform: 'uppercase',
            letterSpacing: '0.06em',
          }}
        >
          Bulk Operations
        </span>
        {selectedCount > 0 && (
          <span
            style={{
              fontFamily: 'monospace',
              fontSize: 10,
              background: 'var(--accent-glow)',
              color: 'var(--accent)',
              border: '1px solid var(--accent)',
              borderRadius: '3px',
              padding: '1px 6px',
            }}
          >
            {selectedCount} selected
          </span>
        )}
      </div>

      {/* Filter bar */}
      <FilterBar
        nodeOptions={nodeOptions}
        nodeNames={nodeNames}
        statusOptions={statusOptions}
        projectOptions={projectOptions}
        nodeFilter={nodeFilter}
        statusFilter={statusFilter}
        imageFilter={imageFilter}
        projectFilter={projectFilter}
        onNode={setNodeFilter}
        onStatus={setStatusFilter}
        onImage={setImageFilter}
        onProject={setProjectFilter}
      />

      {/* Container table */}
      <div style={{ flex: 1, overflow: 'auto', minHeight: 0 }}>
        {treeLoading ? (
          <div style={{ padding: 24, textAlign: 'center', color: 'var(--text-muted)', fontSize: 12 }}>
            <Loader size={16} style={{ animation: 'spin 1s linear infinite', display: 'inline-block' }} />
          </div>
        ) : (
          <ContainerTable
            containers={filtered}
            selected={selected}
            onToggle={toggleOne}
            onToggleAll={toggleAll}
          />
        )}
      </div>

      {/* Action bar — pinned to the bottom so Action/Dry Run/Execute stay
          visible when a long selection scrolls the list. */}
      <div
        style={{
          position: 'sticky',
          bottom: 0,
          zIndex: 10,
          padding: '10px 16px',
          borderTop: '1px solid var(--border-subtle)',
          display: 'flex',
          alignItems: 'center',
          gap: 10,
          flexWrap: 'wrap',
          background: 'var(--bg-surface)',
        }}
      >
        <span style={{ fontSize: 10, fontFamily: 'monospace', color: 'var(--text-muted)', textTransform: 'uppercase', letterSpacing: '0.08em', marginRight: 2 }}>
          ACTION
        </span>

        <select
          style={selectStyle}
          value={action}
          onChange={e => setAction(e.target.value as BulkAction)}
        >
          <option value="start">Start</option>
          <option value="stop">Stop</option>
          <option value="restart">Restart</option>
          <option value="remove">Remove</option>
        </select>

        <button
          style={btnGhost}
          disabled={actionsDisabled}
          onClick={handleDryRun}
          title="Preview what would happen (nothing is executed)"
        >
          {bulk.isPending && lastWasDryRun ? <Loader size={12} style={{ animation: 'spin 1s linear infinite' }} /> : <Filter size={12} />}
          Dry Run
        </button>

        <button
          style={action === 'remove' ? { ...btnBase, ...btnDanger, cursor: actionsDisabled ? 'not-allowed' : 'pointer', opacity: actionsDisabled ? 0.5 : 1 } : btnPrimary}
          disabled={actionsDisabled}
          onClick={handleExecute}
          title={selectedCount === 0 ? 'Select containers first' : `Execute ${action} on ${selectedCount} container(s)`}
        >
          {bulk.isPending && !lastWasDryRun ? (
            <Loader size={12} style={{ animation: 'spin 1s linear infinite' }} />
          ) : (
            actionIcon(action)
          )}
          {bulk.isPending && !lastWasDryRun ? 'Running…' : `Execute (${selectedCount})`}
        </button>

        {bulk.isError && (
          <span style={{ fontSize: 11, fontFamily: 'monospace', color: 'var(--status-error)' }}>
            Request failed
          </span>
        )}
      </div>

      {/* Results table */}
      {results && (
        <ResultsTable
          results={results}
          isDryRun={lastWasDryRun}
          isPending={bulk.isPending}
          nodeNames={nodeNames}
        />
      )}

      {/* Confirm dialog for remove */}
      {showConfirm && (
        <div
          style={{
            position: 'fixed',
            inset: 0,
            background: 'rgba(0,0,0,0.6)',
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            zIndex: 1000,
          }}
          onClick={() => setShowConfirm(false)}
        >
          <div
            style={{
              background: 'var(--bg-elevated)',
              border: '1px solid var(--border-default)',
              borderRadius: '3px',
              padding: '24px 28px',
              maxWidth: 380,
              width: '90%',
            }}
            onClick={e => e.stopPropagation()}
          >
            <div style={{ fontFamily: 'monospace', fontSize: 13, fontWeight: 600, color: 'var(--status-error)', textTransform: 'uppercase', letterSpacing: '0.06em', marginBottom: 10 }}>
              Confirm Remove
            </div>
            <p style={{ fontFamily: 'monospace', fontSize: 12, color: 'var(--text-secondary)', lineHeight: 1.6, marginBottom: 20 }}>
              Remove {selectedCount} container{selectedCount !== 1 ? 's' : ''}? This cannot be undone. Running containers will be skipped unless stopped first.
            </p>
            <div style={{ display: 'flex', gap: 10, justifyContent: 'flex-end' }}>
              <button
                style={{ ...btnGhost, opacity: 1, cursor: 'pointer' }}
                onClick={() => setShowConfirm(false)}
              >
                Cancel
              </button>
              <button
                style={{
                  fontFamily: 'monospace',
                  fontSize: 11,
                  textTransform: 'uppercase',
                  letterSpacing: '0.08em',
                  borderRadius: '3px',
                  border: '1px solid rgba(232,64,64,0.6)',
                  padding: '5px 14px',
                  cursor: 'pointer',
                  background: 'rgba(232,64,64,0.2)',
                  color: 'var(--status-error)',
                  display: 'flex',
                  alignItems: 'center',
                  gap: 6,
                }}
                onClick={confirmRemove}
              >
                <Trash2 size={12} />
                Remove
              </button>
            </div>
          </div>
        </div>
      )}
    </AppShell>
  )
}
