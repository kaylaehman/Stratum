import { Loader } from 'lucide-react'
import { useSharedMounts } from '../../lib/api/mounts'
import type { SharedEntry } from '../../types/api'

interface SharedMountsViewProps {
  nodeId: string
}

function SharedRow({ entry }: { entry: SharedEntry }) {
  return (
    <div
      className="flex items-start gap-3 px-3 py-2.5"
      style={{ borderBottom: '1px solid var(--border-subtle)' }}
    >
      <span
        className="text-xs px-1.5 py-0.5 shrink-0"
        style={{
          background: entry.kind === 'bind'
            ? 'rgba(0,194,204,0.10)'
            : 'rgba(74,158,255,0.10)',
          border: `1px solid ${entry.kind === 'bind' ? 'var(--accent-dim)' : 'rgba(74,158,255,0.3)'}`,
          color: entry.kind === 'bind' ? 'var(--accent)' : 'var(--status-info)',
          borderRadius: '3px',
        }}
      >
        {entry.kind}
      </span>

      <span
        className="font-mono text-xs flex-1 truncate"
        style={{ color: 'var(--text-primary)' }}
        title={entry.key}
      >
        {entry.key}
      </span>

      <div className="flex flex-col items-end gap-1 shrink-0">
        <span
          className="text-xs"
          style={{ color: 'var(--status-warn)' }}
        >
          {entry.container_ids.length} container{entry.container_ids.length !== 1 ? 's' : ''}
        </span>
        <div className="flex flex-col gap-0.5">
          {entry.container_ids.map((id) => (
            <span
              key={id}
              className="font-mono text-xs"
              style={{ color: 'var(--text-muted)' }}
            >
              {id.slice(0, 12)}
            </span>
          ))}
        </div>
      </div>
    </div>
  )
}

export function SharedMountsView({ nodeId }: SharedMountsViewProps) {
  const { data, isLoading, isError } = useSharedMounts(nodeId)

  return (
    <div
      style={{
        backgroundColor: 'var(--bg-surface)',
        border: '1px solid var(--border-subtle)',
        borderRadius: '3px',
        maxWidth: '720px',
      }}
    >
      <div
        className="px-3 py-2 flex items-center justify-between"
        style={{ borderBottom: '1px solid var(--border-subtle)' }}
      >
        <p
          className="text-xs font-medium uppercase tracking-wider"
          style={{ color: 'var(--text-muted)' }}
        >
          Shared mounts
        </p>
        <span className="text-xs" style={{ color: 'var(--text-secondary)' }}>
          host paths and volumes used by multiple containers
        </span>
      </div>

      {isLoading && (
        <div className="flex items-center gap-2 px-3 py-4">
          <Loader size={12} className="animate-spin" style={{ color: 'var(--accent)' }} />
          <span className="text-xs" style={{ color: 'var(--text-muted)' }}>Loading...</span>
        </div>
      )}

      {isError && (
        <div className="px-3 py-3 text-xs" style={{ color: 'var(--status-error)' }}>
          Failed to load shared mounts. The node may lack Docker capability.
        </div>
      )}

      {data && data.shared.length === 0 && (
        <div className="px-3 py-3 text-xs" style={{ color: 'var(--text-muted)' }}>
          No shared mounts detected on this node.
        </div>
      )}

      {data?.shared.map((entry, i) => (
        <SharedRow key={`${entry.kind}-${entry.key}-${i}`} entry={entry} />
      ))}
    </div>
  )
}
