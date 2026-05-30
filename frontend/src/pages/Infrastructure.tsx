import { useMemo } from 'react'
import { Network, Loader } from 'lucide-react'
import { AppShell } from '../components/layout/AppShell'
import { InfraMap, infraStats } from '../components/network/InfraMap'
import { useTree } from '../lib/api/tree'
import { useNodeGuestLinks } from '../hooks/useNodeGuestLinks'

function Stat({ label, value }: { label: string; value: number }) {
  return (
    <div className="flex items-baseline gap-1.5">
      <span className="font-mono text-sm" style={{ color: 'var(--text-primary)' }}>
        {value}
      </span>
      <span className="text-xs uppercase tracking-wider" style={{ color: 'var(--text-muted)', fontSize: '11px' }}>
        {label}
      </span>
    </div>
  )
}

export default function Infrastructure() {
  const { data: tree, isLoading } = useTree()
  const nodes = useMemo(() => tree?.nodes ?? [], [tree])
  const correlation = useNodeGuestLinks(nodes)
  const stats = useMemo(() => infraStats(nodes, correlation), [nodes, correlation])

  return (
    <AppShell>
      <div className="flex flex-col flex-1 min-h-0 h-full w-full p-6" style={{ maxWidth: '1100px', margin: '0 auto' }}>
        {/* Header */}
        <div className="flex items-center justify-between gap-4 mb-5 flex-wrap shrink-0">
          <div className="flex items-center gap-2">
            <Network size={16} style={{ color: 'var(--text-secondary)' }} />
            <h1 className="text-sm font-medium uppercase tracking-wider" style={{ color: 'var(--text-primary)' }}>
              Infrastructure
            </h1>
          </div>
          {tree && (
            <div className="flex items-center gap-4 flex-wrap">
              <Stat label="hosts" value={stats.hosts} />
              <Stat label="VMs" value={stats.vms} />
              <Stat label="LXCs" value={stats.lxcs} />
              <Stat label="containers" value={stats.containers} />
            </div>
          )}
        </div>

        {isLoading && (
          <div className="flex items-center gap-2 py-8">
            <Loader size={13} className="animate-spin" style={{ color: 'var(--accent)' }} />
            <span className="text-xs" style={{ color: 'var(--text-muted)' }}>
              Loading infrastructure…
            </span>
          </div>
        )}

        {tree && nodes.length === 0 && (
          <p className="text-xs" style={{ color: 'var(--text-muted)' }}>
            No hosts connected yet.
          </p>
        )}

        {/* Map */}
        {tree && nodes.length > 0 && (
          <div className="flex-1 min-h-0 overflow-auto">
            <InfraMap nodes={nodes} correlation={correlation} />
          </div>
        )}
      </div>
    </AppShell>
  )
}
