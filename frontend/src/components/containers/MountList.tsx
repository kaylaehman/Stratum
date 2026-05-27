import { ExternalLink, Loader } from 'lucide-react'
import { useContainerMounts } from '../../lib/api/mounts'
import { useTreeStore } from '../../store/tree'
import type { MountView } from '../../types/api'

interface MountListProps {
  containerId: string
  nodeId: string
}

function RwChip({ rw }: { rw: boolean }) {
  return (
    <span
      className="text-xs px-1.5 py-0.5 font-mono shrink-0"
      style={{
        background: rw ? 'rgba(34,201,122,0.12)' : 'rgba(74,82,104,0.25)',
        border: `1px solid ${rw ? 'rgba(34,201,122,0.4)' : 'var(--border-default)'}`,
        color: rw ? 'var(--status-ok)' : 'var(--text-muted)',
        borderRadius: '3px',
      }}
    >
      {rw ? 'rw' : 'ro'}
    </span>
  )
}

function SharedBadge({ containerIds }: { containerIds?: string[] }) {
  const title = containerIds?.length
    ? `Mounted into ${containerIds.length} container(s): ${containerIds.join(', ')}`
    : 'Mounted into multiple containers simultaneously'
  return (
    <span
      className="text-xs px-1.5 py-0.5 shrink-0"
      title={title}
      style={{
        background: 'rgba(240,160,32,0.12)',
        border: '1px solid rgba(240,160,32,0.4)',
        color: 'var(--status-warn)',
        borderRadius: '3px',
        cursor: 'default',
      }}
    >
      SHARED
    </span>
  )
}

function VolumeChip({ name }: { name: string }) {
  return (
    <span
      className="text-xs px-1.5 py-0.5 font-mono shrink-0 truncate"
      title={name}
      style={{
        background: 'rgba(74,158,255,0.10)',
        border: '1px solid rgba(74,158,255,0.3)',
        color: 'var(--status-info)',
        borderRadius: '3px',
        maxWidth: '160px',
      }}
    >
      {name}
    </span>
  )
}

function MountRow({ mount, nodeId }: { mount: MountView; nodeId: string }) {
  const setSelected = useTreeStore((s) => s.setSelected)

  const canDeepLink =
    (mount.type === 'bind' || mount.traceable) &&
    mount.source.startsWith('/')

  function handleDeepLink() {
    setSelected({ kind: 'fs-root', nodeId })
  }

  return (
    <div
      className="flex items-start gap-3 px-3 py-2.5"
      style={{ borderBottom: '1px solid var(--border-subtle)' }}
    >
      {/* Source -> Destination */}
      <div className="flex flex-col gap-0.5 flex-1 min-w-0">
        <span
          className="font-mono text-xs truncate"
          style={{ color: 'var(--text-primary)' }}
          title={mount.source}
        >
          {mount.source || '(no source)'}
        </span>
        <span
          className="font-mono text-xs truncate"
          style={{ color: 'var(--text-secondary)' }}
          title={mount.destination}
        >
          {mount.destination}
        </span>
      </div>

      {/* Chips row */}
      <div className="flex items-center gap-1.5 shrink-0 flex-wrap justify-end">
        <RwChip rw={mount.rw} />
        {mount.shared && <SharedBadge />}
        {mount.type === 'volume' && mount.volume_name && (
          <VolumeChip name={mount.volume_name} />
        )}
        {canDeepLink && (
          <button
            type="button"
            onClick={handleDeepLink}
            title={`Open ${mount.source} in filesystem browser`}
            style={{
              background: 'none',
              border: 'none',
              color: 'var(--text-muted)',
              cursor: 'pointer',
              padding: '2px',
              display: 'flex',
              alignItems: 'center',
            }}
          >
            <ExternalLink size={11} />
          </button>
        )}
      </div>
    </div>
  )
}

export function MountList({ containerId, nodeId }: MountListProps) {
  const { data, isLoading, isError } = useContainerMounts(containerId)

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
        className="px-3 py-2 flex items-center justify-between"
        style={{ borderBottom: '1px solid var(--border-subtle)' }}
      >
        <p
          className="text-xs font-medium uppercase tracking-wider"
          style={{ color: 'var(--text-muted)' }}
        >
          Mounts
        </p>
        {data && (
          <span className="text-xs" style={{ color: 'var(--text-muted)' }}>
            {data.mounts.length}
          </span>
        )}
      </div>

      {isLoading && (
        <div className="flex items-center gap-2 px-3 py-4">
          <Loader size={12} className="animate-spin" style={{ color: 'var(--accent)' }} />
          <span className="text-xs" style={{ color: 'var(--text-muted)' }}>Loading mounts...</span>
        </div>
      )}

      {isError && (
        <div className="px-3 py-3 text-xs" style={{ color: 'var(--status-error)' }}>
          Failed to load mounts.
        </div>
      )}

      {data && data.mounts.length === 0 && (
        <div className="px-3 py-3 text-xs" style={{ color: 'var(--text-muted)' }}>
          No mounts.
        </div>
      )}

      {data?.mounts.map((m, i) => (
        <MountRow key={`${m.source}-${m.destination}-${i}`} mount={m} nodeId={nodeId} />
      ))}
    </div>
  )
}
