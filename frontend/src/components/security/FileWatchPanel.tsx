import { useState } from 'react'
import { Eye, ScanLine, Plus, X, Loader, FileWarning } from 'lucide-react'
import { useWatches, useAddWatch, useDeleteWatch, useScanWatches, useFileEvents } from '../../lib/api/filewatch'
import { useCan } from '../../lib/roles'
import { ApiError } from '../../lib/api'
import type { FileEventType } from '../../types/api'

interface FileWatchPanelProps {
  nodeId: string
}

const sectionStyle: React.CSSProperties = {
  borderTop: '1px solid var(--border-subtle)',
  paddingTop: '8px',
  marginTop: '8px',
}

const labelStyle: React.CSSProperties = {
  fontSize: '12px',
  color: 'var(--text-muted)',
}

function eventBadgeColor(type: FileEventType): string {
  switch (type) {
    case 'create': return 'var(--status-ok)'
    case 'delete': return 'var(--status-error)'
    case 'chmod': return 'var(--accent)'
    default: return 'var(--status-warn)'
  }
}

function AddWatchForm({ nodeId }: { nodeId: string }) {
  const [path, setPath] = useState('')
  const [recursive, setRecursive] = useState(false)
  const { mutate, isPending, error, reset } = useAddWatch(nodeId)

  const apiErr = error as ApiError | null
  const errCode = apiErr ? (apiErr.body as { error?: string })?.error : null
  const errorMsg =
    errCode === 'path_required' ? 'Path is required'
    : errCode === 'invalid_path' ? 'Path must be absolute (starts with /)'
    : errCode === 'watch_exists_or_failed' ? 'Watch already exists or could not be added'
    : apiErr ? 'Failed to add watch'
    : null

  function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    const trimmed = path.trim()
    if (!trimmed) return
    mutate(
      { path: trimmed, recursive },
      {
        onSuccess: () => {
          setPath('')
          setRecursive(false)
          reset()
        },
      },
    )
  }

  return (
    <form onSubmit={handleSubmit} className="flex flex-col gap-1.5 mt-2">
      <div className="flex items-center gap-2">
        <input
          type="text"
          placeholder="/etc"
          value={path}
          onChange={(e) => { setPath(e.target.value); reset() }}
          className="font-mono text-xs px-2 py-1.5 flex-1"
          style={{
            background: 'var(--bg-elevated)',
            border: `1px solid ${errorMsg ? 'var(--status-error)' : 'var(--border-default)'}`,
            color: 'var(--text-primary)',
            borderRadius: '3px',
            outline: 'none',
            minWidth: 0,
          }}
        />
        <label className="flex items-center gap-1 font-mono text-xs shrink-0 cursor-pointer"
          style={{ color: 'var(--text-secondary)' }}>
          <input
            type="checkbox"
            checked={recursive}
            onChange={(e) => setRecursive(e.target.checked)}
            style={{ accentColor: 'var(--accent)', cursor: 'pointer' }}
          />
          recursive
        </label>
        <button
          type="submit"
          disabled={isPending}
          className="flex items-center gap-1 font-mono text-xs px-2.5 py-1 shrink-0"
          style={{
            background: 'var(--accent-glow)',
            border: '1px solid var(--accent-dim)',
            color: isPending ? 'var(--text-muted)' : 'var(--accent)',
            borderRadius: '3px',
            cursor: isPending ? 'not-allowed' : 'pointer',
            opacity: isPending ? 0.6 : 1,
          }}
        >
          {isPending ? <Loader size={11} className="animate-spin" /> : <Plus size={11} />}
          Add
        </button>
      </div>
      <p className="font-mono text-xs" style={{ color: 'var(--text-muted)' }}>
        Common paths: /etc, /root/.ssh, /home/*/.ssh
      </p>
      {errorMsg && (
        <span className="font-mono text-xs" style={{ color: 'var(--status-error)' }}>
          {errorMsg}
        </span>
      )}
    </form>
  )
}

function WatchRow({ watchId, path, recursive, nodeId }: {
  watchId: string
  path: string
  recursive: boolean
  nodeId: string
}) {
  const { mutate: del, isPending } = useDeleteWatch(nodeId)

  return (
    <div
      className="flex items-center justify-between gap-2 py-1.5"
      style={{ borderBottom: '1px solid var(--border-subtle)' }}
    >
      <div className="flex items-center gap-1.5 min-w-0">
        <span className="font-mono text-xs truncate" style={{ color: 'var(--text-primary)' }} title={path}>
          {path}
        </span>
        {recursive && (
          <span
            className="font-mono text-xs px-1 shrink-0"
            style={{
              background: 'var(--bg-elevated)',
              border: '1px solid var(--border-subtle)',
              color: 'var(--text-muted)',
              borderRadius: '3px',
            }}
          >
            recursive
          </span>
        )}
      </div>
      <button
        type="button"
        disabled={isPending}
        onClick={() => del(watchId)}
        title="Remove watch"
        className="flex items-center font-mono text-xs px-1.5 py-1 shrink-0"
        style={{
          background: 'var(--bg-elevated)',
          border: '1px solid var(--border-default)',
          color: isPending ? 'var(--text-muted)' : 'var(--status-error)',
          borderRadius: '3px',
          cursor: isPending ? 'not-allowed' : 'pointer',
          opacity: isPending ? 0.5 : 1,
        }}
      >
        {isPending ? <Loader size={11} className="animate-spin" /> : <X size={11} />}
      </button>
    </div>
  )
}

function ScanButton({ nodeId }: { nodeId: string }) {
  const { mutate: scan, isPending, data, error, reset } = useScanWatches(nodeId)

  const apiErr = error as ApiError | null
  const errCode = apiErr ? (apiErr.body as { error?: string })?.error : null
  const errorMsg = errCode === 'scan_failed' ? "Couldn't reach host" : apiErr ? 'Scan failed' : null

  return (
    <div className="flex flex-col gap-1">
      <button
        type="button"
        disabled={isPending}
        onClick={() => { reset(); scan() }}
        className="flex items-center gap-1.5 font-mono text-xs px-2.5 py-1 self-start"
        style={{
          background: 'var(--bg-elevated)',
          border: '1px solid var(--border-default)',
          color: isPending ? 'var(--text-muted)' : 'var(--text-secondary)',
          borderRadius: '3px',
          cursor: isPending ? 'not-allowed' : 'pointer',
          opacity: isPending ? 0.7 : 1,
        }}
      >
        {isPending ? <Loader size={11} className="animate-spin" /> : <ScanLine size={11} />}
        {isPending ? 'Scanning…' : 'Scan now'}
      </button>
      {data !== undefined && !isPending && !errorMsg && (
        <span className="font-mono text-xs" style={{ color: data.detected > 0 ? 'var(--status-warn)' : 'var(--status-ok)' }}>
          {data.detected} change{data.detected !== 1 ? 's' : ''} detected
        </span>
      )}
      {errorMsg && (
        <span className="font-mono text-xs" style={{ color: 'var(--status-error)' }}>
          {errorMsg}
        </span>
      )}
    </div>
  )
}

function EventsList({ nodeId }: { nodeId: string }) {
  const { data, isLoading, isError } = useFileEvents(nodeId)
  const events = data?.events ?? []

  if (isLoading) {
    return <Loader size={12} className="animate-spin mt-1" style={{ color: 'var(--text-muted)' }} />
  }

  if (isError) {
    return (
      <p className="font-mono text-xs mt-1" style={{ color: 'var(--status-error)' }}>
        Could not load events.
      </p>
    )
  }

  if (events.length === 0) {
    return (
      <p className="font-mono text-xs mt-1" style={{ color: 'var(--text-muted)' }}>
        No changes detected yet. Add a watch path and run a scan.
      </p>
    )
  }

  return (
    <div className="flex flex-col mt-1">
      {events.map((ev) => (
        <div
          key={ev.id}
          className="flex items-center gap-2 py-1.5"
          style={{ borderBottom: '1px solid var(--border-subtle)' }}
        >
          <span
            className="font-mono text-xs px-1 shrink-0"
            style={{
              background: 'var(--bg-elevated)',
              border: `1px solid ${eventBadgeColor(ev.event_type)}`,
              color: eventBadgeColor(ev.event_type),
              borderRadius: '3px',
              minWidth: '48px',
              textAlign: 'center',
            }}
          >
            {ev.event_type}
          </span>
          <span className="font-mono text-xs truncate flex-1" style={{ color: 'var(--text-primary)' }} title={ev.path}>
            {ev.path}
          </span>
          <span className="font-mono text-xs shrink-0" style={{ color: 'var(--text-muted)' }}>
            {new Date(ev.detected_at).toLocaleString()}
          </span>
        </div>
      ))}
    </div>
  )
}

export function FileWatchPanel({ nodeId }: FileWatchPanelProps) {
  const { isAdmin } = useCan()
  const { data, isLoading } = useWatches(nodeId)
  const watches = data?.watches ?? []

  if (!isAdmin) return null

  return (
    <div style={sectionStyle}>
      {/* Header */}
      <div className="flex items-center justify-between gap-2 mb-2">
        <div className="flex items-center gap-1.5">
          <FileWarning size={12} style={{ color: 'var(--text-muted)' }} />
          <span className="text-xs font-medium uppercase tracking-wider" style={labelStyle}>
            File Change Detection
          </span>
          {watches.length > 0 && (
            <span
              className="font-mono text-xs px-1"
              style={{
                background: 'var(--bg-elevated)',
                border: '1px solid var(--border-subtle)',
                color: 'var(--text-muted)',
                borderRadius: '3px',
              }}
            >
              {watches.length}
            </span>
          )}
        </div>
        <ScanButton nodeId={nodeId} />
      </div>

      {/* Watched paths */}
      {isLoading ? (
        <Loader size={12} className="animate-spin" style={{ color: 'var(--text-muted)' }} />
      ) : watches.length === 0 ? (
        <p className="font-mono text-xs" style={{ color: 'var(--text-muted)' }}>
          No watch paths configured.
        </p>
      ) : (
        <div className="flex flex-col">
          {watches.map((w) => (
            <WatchRow
              key={w.id}
              watchId={w.id}
              path={w.path}
              recursive={w.recursive}
              nodeId={nodeId}
            />
          ))}
        </div>
      )}

      {/* Add form */}
      <AddWatchForm nodeId={nodeId} />

      {/* Events */}
      <div className="mt-3">
        <div className="flex items-center gap-1.5 mb-1">
          <Eye size={11} style={{ color: 'var(--text-muted)' }} />
          <span className="text-xs font-medium uppercase tracking-wider" style={labelStyle}>
            Recent Events
          </span>
        </div>
        <p className="font-mono text-xs mb-1" style={{ color: 'var(--text-muted)' }}>
          Near-real-time via scan; install the agent for instant inotify alerts.
        </p>
        <EventsList nodeId={nodeId} />
      </div>
    </div>
  )
}
