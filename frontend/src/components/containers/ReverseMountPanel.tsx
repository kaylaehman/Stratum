import { useState } from 'react'
import { Loader } from 'lucide-react'
import { useReverseMounts } from '../../lib/api/mounts'
import type { ReverseHit, MountRelation } from '../../types/api'

interface ReverseMountPanelProps {
  nodeId: string
  initialPath?: string
}

function relationLabel(r: MountRelation): string {
  switch (r) {
    case 'equal':
      return 'mounts exactly'
    case 'a_parent_of_b':
      return 'mounts child'
    case 'b_parent_of_a':
      return 'mounts parent'
  }
}

function HitRow({ hit }: { hit: ReverseHit }) {
  return (
    <div
      className="flex items-start gap-3 px-3 py-2.5"
      style={{ borderBottom: '1px solid var(--border-subtle)' }}
    >
      <span
        className="font-mono text-xs shrink-0"
        style={{ color: 'var(--text-secondary)', minWidth: '88px' }}
      >
        {hit.container_id.slice(0, 12)}
      </span>
      <div className="flex flex-col gap-0.5 flex-1 min-w-0">
        <span className="text-xs" style={{ color: 'var(--text-muted)' }}>
          {relationLabel(hit.relation)}{' '}
          <span className="font-mono" style={{ color: 'var(--text-primary)' }}>
            {hit.source}
          </span>
        </span>
        <span
          className="font-mono text-xs truncate"
          style={{ color: 'var(--text-secondary)' }}
          title={hit.destination}
        >
          → {hit.destination}
        </span>
      </div>
      <span
        className="text-xs px-1.5 py-0.5 font-mono shrink-0"
        style={{
          background: hit.rw ? 'rgba(34,201,122,0.12)' : 'rgba(74,82,104,0.25)',
          border: `1px solid ${hit.rw ? 'rgba(34,201,122,0.4)' : 'var(--border-default)'}`,
          color: hit.rw ? 'var(--status-ok)' : 'var(--text-muted)',
          borderRadius: '3px',
        }}
      >
        {hit.rw ? 'rw' : 'ro'}
      </span>
    </div>
  )
}

function Results({ nodeId, hostPath }: { nodeId: string; hostPath: string }) {
  const { data, isLoading, isError } = useReverseMounts(nodeId, hostPath)

  if (isLoading) {
    return (
      <div className="flex items-center gap-2 px-3 py-4">
        <Loader size={12} className="animate-spin" style={{ color: 'var(--accent)' }} />
        <span className="text-xs" style={{ color: 'var(--text-muted)' }}>Searching...</span>
      </div>
    )
  }

  if (isError) {
    return (
      <div className="px-3 py-3 text-xs" style={{ color: 'var(--status-error)' }}>
        Failed to query. Check that the path is absolute and the node has Docker capability.
      </div>
    )
  }

  if (!data || data.containers.length === 0) {
    return (
      <div className="px-3 py-3 text-xs" style={{ color: 'var(--text-muted)' }}>
        No containers mount this path or any parent/child of it.
      </div>
    )
  }

  return (
    <>
      {data.containers.map((hit, i) => (
        <HitRow key={`${hit.container_id}-${i}`} hit={hit} />
      ))}
    </>
  )
}

export function ReverseMountPanel({ nodeId, initialPath = '' }: ReverseMountPanelProps) {
  const [input, setInput] = useState(initialPath)
  const [submittedPath, setSubmittedPath] = useState(initialPath)

  return (
    <div
      style={{
        backgroundColor: 'var(--bg-surface)',
        border: '1px solid var(--border-subtle)',
        borderRadius: '3px',
        maxWidth: '640px',
      }}
    >
      <div
        className="px-3 py-2"
        style={{ borderBottom: '1px solid var(--border-subtle)' }}
      >
        <p
          className="text-xs font-medium uppercase tracking-wider mb-2"
          style={{ color: 'var(--text-muted)' }}
        >
          Reverse mount lookup
        </p>
        <p className="text-xs mb-2" style={{ color: 'var(--text-secondary)' }}>
          Enter a host path to find which containers mount it, a parent, or a child.
        </p>
        <form
          onSubmit={(e) => {
            e.preventDefault()
            const trimmed = input.trim()
            if (trimmed) setSubmittedPath(trimmed)
          }}
          className="flex items-center gap-2"
        >
          <input
            type="text"
            placeholder="/var/data"
            value={input}
            onChange={(e) => setInput(e.target.value)}
            className="font-mono text-xs px-2 py-1.5 flex-1"
            style={{
              background: 'var(--bg-elevated)',
              border: '1px solid var(--border-default)',
              color: 'var(--text-primary)',
              borderRadius: '3px',
              outline: 'none',
              maxWidth: '320px',
            }}
          />
          <button
            type="submit"
            className="text-xs px-3 py-1.5"
            style={{
              background: 'var(--accent-glow)',
              border: '1px solid var(--accent-dim)',
              color: 'var(--accent)',
              borderRadius: '3px',
              cursor: 'pointer',
            }}
          >
            Lookup
          </button>
        </form>
      </div>

      {submittedPath && <Results nodeId={nodeId} hostPath={submittedPath} />}
    </div>
  )
}
