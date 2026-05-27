import { useState } from 'react'
import { Loader, AlertTriangle } from 'lucide-react'
import { useUidAnalysis } from '../../lib/api/permissions'
import { UserTable } from './UserTable'
import { ExportButton } from './ExportButton'
import type { UidRow } from '../../types/api'

interface LegendItemProps {
  color: string
  bg: string
  label: string
  count: number
}

function LegendItem({ color, bg, label, count }: LegendItemProps) {
  return (
    <div
      className="flex items-center gap-2 px-2 py-1"
      style={{
        background: bg,
        border: `1px solid ${color}`,
        borderRadius: '3px',
      }}
    >
      <span
        className="inline-block shrink-0"
        style={{
          width: 8,
          height: 8,
          borderRadius: '50%',
          backgroundColor: color,
        }}
      />
      <span className="text-xs" style={{ color: 'var(--text-secondary)' }}>
        {label}
      </span>
      <span className="font-mono text-xs" style={{ color }}>
        {count}
      </span>
    </div>
  )
}

interface UidGidVisualizerProps {
  containerId: string
}

export function UidGidVisualizer({ containerId }: UidGidVisualizerProps) {
  const [mode, setMode] = useState<'uid' | 'gid'>('uid')
  const { data, isLoading, isError } = useUidAnalysis(containerId)

  const rows: UidRow[] = data ? (mode === 'uid' ? data.uid_rows : data.gid_rows) : []
  const legend = data?.legend ?? { match: 0, mismatch: 0, unresolvable: 0 }

  return (
    <div
      className="flex flex-col gap-0"
      style={{
        backgroundColor: 'var(--bg-surface)',
        border: '1px solid var(--border-subtle)',
        borderRadius: '3px',
      }}
    >
      {/* Header */}
      <div
        className="flex flex-col gap-1.5 px-4 py-3 shrink-0"
        style={{ borderBottom: '1px solid var(--border-subtle)' }}
      >
        <div className="flex items-center justify-between gap-3 flex-wrap">
          <p
            className="text-xs font-medium uppercase tracking-wider"
            style={{ color: 'var(--text-muted)' }}
          >
            UID / GID Conflict Visualizer
          </p>
          {data && <ExportButton containerId={containerId} />}
        </div>
        <p className="text-xs" style={{ color: 'var(--text-secondary)', maxWidth: '560px' }}>
          Same numeric UID mapping to different usernames across the host/container
          boundary is the primary cause of silent bind-mount permission failures.
        </p>

        {/* UID / GID toggle */}
        <div className="flex items-center gap-1">
          {(['uid', 'gid'] as const).map((m) => (
            <button
              key={m}
              type="button"
              onClick={() => setMode(m)}
              className="text-xs px-3 py-1 font-mono uppercase"
              style={{
                background: mode === m ? 'var(--accent-glow)' : 'var(--bg-elevated)',
                border: `1px solid ${mode === m ? 'var(--accent-dim)' : 'var(--border-default)'}`,
                color: mode === m ? 'var(--accent)' : 'var(--text-secondary)',
                borderRadius: '3px',
                cursor: 'pointer',
              }}
            >
              {m.toUpperCase()}
            </button>
          ))}
        </div>
      </div>

      {/* Loading / error states */}
      {isLoading && (
        <div className="flex items-center gap-2 px-4 py-5">
          <Loader size={13} className="animate-spin" style={{ color: 'var(--accent)' }} />
          <span className="text-xs" style={{ color: 'var(--text-muted)' }}>
            Resolving UIDs...
          </span>
        </div>
      )}

      {isError && (
        <div className="flex items-center gap-2 px-4 py-4">
          <AlertTriangle size={13} style={{ color: 'var(--status-warn)' }} />
          <span className="text-xs" style={{ color: 'var(--status-warn)' }}>
            Unable to load UID analysis. The container may not be running or the
            agent is unavailable.
          </span>
        </div>
      )}

      {data && (
        <>
          {/* Legend */}
          <div
            className="flex items-center gap-2 px-4 py-2 flex-wrap shrink-0"
            style={{ borderBottom: '1px solid var(--border-subtle)' }}
          >
            <LegendItem
              color="var(--status-ok)"
              bg="rgba(34,201,122,0.07)"
              label="Match"
              count={legend.match}
            />
            <LegendItem
              color="var(--status-warn)"
              bg="rgba(240,160,32,0.10)"
              label="Mismatch"
              count={legend.mismatch}
            />
            <LegendItem
              color="var(--status-error)"
              bg="rgba(232,64,64,0.08)"
              label="Unresolvable"
              count={legend.unresolvable}
            />
            {legend.mismatch > 0 && (
              <span
                className="text-xs ml-2"
                style={{ color: 'var(--status-warn)' }}
              >
                {legend.mismatch} mismatch{legend.mismatch !== 1 ? 'es' : ''} — same{' '}
                {mode.toUpperCase()}, different identity
              </span>
            )}
          </div>

          {/* Two-column aligned table */}
          <div
            className="flex flex-1 min-h-0"
            style={{ minHeight: '200px', maxHeight: '340px' }}
          >
            <UserTable rows={rows} side="host" tinted />
            <div style={{ width: '1px', backgroundColor: 'var(--border-default)', flexShrink: 0 }} />
            <UserTable rows={rows} side="container" tinted />
          </div>
        </>
      )}
    </div>
  )
}
