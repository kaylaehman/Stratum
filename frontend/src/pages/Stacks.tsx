import { useState, useMemo } from 'react'
import { Layers, Loader, ChevronDown, ChevronRight, ExternalLink } from 'lucide-react'
import { useNavigate } from 'react-router-dom'
import { AppShell } from '../components/layout/AppShell'
import { useTree } from '../lib/api/tree'
import type { Container, TreeNode } from '../types/api'

// ── Types ─────────────────────────────────────────────────────────────────────

interface Stack {
  key: string
  projectName: string
  node: TreeNode
  containers: Container[]
}

// ── Helpers ───────────────────────────────────────────────────────────────────

const UNGROUPED = '__ungrouped__'

function statusColor(status: Container['status']): string {
  switch (status) {
    case 'running':
      return 'var(--status-ok)'
    case 'paused':
    case 'restarting':
      return 'var(--status-warn)'
    default:
      return 'var(--status-error)'
  }
}

function buildStacks(nodes: TreeNode[]): Stack[] {
  const stacks: Stack[] = []

  for (const node of nodes) {
    const byProject = new Map<string, Container[]>()

    for (const c of node.containers) {
      const key = c.compose_project ?? UNGROUPED
      const existing = byProject.get(key)
      if (existing) {
        existing.push(c)
      } else {
        byProject.set(key, [c])
      }
    }

    // Named projects first, then ungrouped
    for (const [project, containers] of byProject) {
      if (project === UNGROUPED) continue
      stacks.push({
        key: `${node.id}::${project}`,
        projectName: project,
        node,
        containers,
      })
    }

    const ungrouped = byProject.get(UNGROUPED)
    if (ungrouped && ungrouped.length > 0) {
      stacks.push({
        key: `${node.id}::${UNGROUPED}`,
        projectName: UNGROUPED,
        node,
        containers: ungrouped,
      })
    }
  }

  return stacks
}

// ── Sub-components ────────────────────────────────────────────────────────────

function StatusDot({ status }: { status: Container['status'] }) {
  return (
    <span
      style={{
        display: 'inline-block',
        width: '6px',
        height: '6px',
        borderRadius: '50%',
        backgroundColor: statusColor(status),
        flexShrink: 0,
      }}
    />
  )
}

function Stat({ label, value }: { label: string; value: number | string }) {
  return (
    <div className="flex items-baseline gap-1.5">
      <span className="font-mono text-sm" style={{ color: 'var(--text-primary)' }}>
        {value}
      </span>
      <span
        className="text-xs uppercase tracking-wider"
        style={{ color: 'var(--text-muted)', fontSize: '11px' }}
      >
        {label}
      </span>
    </div>
  )
}

interface StackCardProps {
  stack: Stack
  defaultOpen?: boolean
}

function StackCard({ stack, defaultOpen = false }: StackCardProps) {
  const [open, setOpen] = useState(defaultOpen)
  const navigate = useNavigate()

  const running = stack.containers.filter((c) => c.status === 'running').length
  const total = stack.containers.length
  const isUngrouped = stack.projectName === UNGROUPED
  const displayName = isUngrouped ? 'Ungrouped / standalone' : stack.projectName

  return (
    <div
      style={{
        backgroundColor: 'var(--bg-surface)',
        border: '1px solid var(--border-subtle)',
        borderRadius: '3px',
        overflow: 'hidden',
        // Keep natural height: the parent is a constrained flex column and these
        // cards set overflow:hidden, which zeroes their auto min-height — without
        // this they shrink and clip ("squished") instead of letting the list scroll.
        flexShrink: 0,
      }}
    >
      {/* Header row */}
      <button
        type="button"
        onClick={() => setOpen((o) => !o)}
        className="flex items-center gap-2.5 w-full px-3 py-2 text-left"
        style={{
          background: 'transparent',
          border: 'none',
          borderBottom: open ? '1px solid var(--border-subtle)' : 'none',
          cursor: 'pointer',
        }}
      >
        {open ? (
          <ChevronDown size={13} style={{ color: 'var(--text-muted)', flexShrink: 0 }} />
        ) : (
          <ChevronRight size={13} style={{ color: 'var(--text-muted)', flexShrink: 0 }} />
        )}

        <span
          className="font-mono text-xs flex-1 truncate"
          style={{ color: isUngrouped ? 'var(--text-muted)' : 'var(--text-primary)' }}
        >
          {displayName}
        </span>

        {/* Running / total pill */}
        <span
          className="font-mono text-xs px-1.5 py-0.5 shrink-0"
          style={{
            backgroundColor: 'var(--bg-elevated)',
            border: '1px solid var(--border-subtle)',
            borderRadius: '3px',
            color: running === total ? 'var(--status-ok)' : running === 0 ? 'var(--status-error)' : 'var(--status-warn)',
          }}
        >
          {running}/{total}
        </span>

        {/* Host badge */}
        <span
          className="text-xs truncate shrink-0"
          style={{
            color: 'var(--text-muted)',
            maxWidth: '120px',
          }}
        >
          {stack.node.name}
        </span>
      </button>

      {/* Container list */}
      {open && (
        <ul className="flex flex-col">
          {stack.containers.map((c) => (
            <li
              key={c.id}
              className="flex items-center gap-2 px-4 py-1.5"
              style={{ borderBottom: '1px solid var(--border-subtle)' }}
            >
              <StatusDot status={c.status} />
              <span
                className="font-mono text-xs flex-1 truncate"
                style={{ color: 'var(--text-primary)' }}
              >
                {c.name}
              </span>
              <span
                className="text-xs truncate"
                style={{ color: 'var(--text-muted)', maxWidth: '180px' }}
              >
                {c.image}
              </span>
              <button
                type="button"
                title="Open in Resources"
                onClick={() => navigate('/resources')}
                className="flex items-center justify-center shrink-0"
                style={{
                  background: 'transparent',
                  border: 'none',
                  cursor: 'pointer',
                  color: 'var(--text-muted)',
                  padding: '2px',
                  borderRadius: '3px',
                }}
              >
                <ExternalLink size={11} />
              </button>
            </li>
          ))}
        </ul>
      )}
    </div>
  )
}

// ── Main page ─────────────────────────────────────────────────────────────────

export default function Stacks() {
  const { data: tree, isLoading } = useTree()

  const nodes = useMemo(() => tree?.nodes ?? [], [tree])

  const stacks = useMemo(() => buildStacks(nodes), [nodes])

  const namedStacks = useMemo(
    () => stacks.filter((s) => s.projectName !== UNGROUPED),
    [stacks],
  )

  const totalContainers = useMemo(
    () => stacks.reduce((sum, s) => sum + s.containers.length, 0),
    [stacks],
  )

  return (
    <AppShell>
      <div
        className="flex flex-col flex-1 min-h-0 h-full w-full p-6"
        style={{ maxWidth: '900px', margin: '0 auto' }}
      >
        {/* Page header */}
        <div className="flex items-center justify-between gap-4 mb-5 flex-wrap shrink-0">
          <div className="flex items-center gap-2">
            <Layers size={16} style={{ color: 'var(--text-secondary)' }} />
            <h1
              className="text-sm font-medium uppercase tracking-wider"
              style={{ color: 'var(--text-primary)' }}
            >
              Stacks
            </h1>
          </div>

          {tree && (
            <div className="flex items-center gap-4 flex-wrap">
              <Stat label="stacks" value={namedStacks.length} />
              <Stat label="containers" value={totalContainers} />
            </div>
          )}
        </div>

        {/* Loading */}
        {isLoading && (
          <div className="flex items-center gap-2 py-8">
            <Loader size={13} className="animate-spin" style={{ color: 'var(--accent)' }} />
            <span className="text-xs" style={{ color: 'var(--text-muted)' }}>
              Loading stacks…
            </span>
          </div>
        )}

        {/* Empty */}
        {tree && stacks.length === 0 && (
          <p className="text-xs" style={{ color: 'var(--text-muted)' }}>
            No deployed stacks found.
          </p>
        )}

        {/* Stack list */}
        {stacks.length > 0 && (
          <div className="flex flex-col gap-2 overflow-auto flex-1 min-h-0">
            {stacks.map((stack) => (
              <StackCard key={stack.key} stack={stack} />
            ))}
          </div>
        )}
      </div>
    </AppShell>
  )
}
