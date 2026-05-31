import { useState } from 'react'
import { Activity, Plus, Pencil, Trash2, Loader, ChevronDown, ChevronUp, Clock } from 'lucide-react'
import { AppShell } from '../components/layout/AppShell'
import { StatusPill } from '../components/uptime/StatusPill'
import { MonitorForm } from '../components/uptime/MonitorForm'
import { LineChart } from '../components/metrics/LineChart'
import type { Series, SeriesPoint } from '../components/metrics/LineChart'
import {
  useUptimeMonitors,
  useUptimeHistory,
  useCreateUptimeMonitor,
  useUpdateUptimeMonitor,
  useDeleteUptimeMonitor,
} from '../lib/api/uptime'
import type { UptimeMonitor, UptimeMonitorRequest, UptimeHistoryRange } from '../lib/api/uptime'

// ---- Helpers ----

function fmtPct(v: number | undefined): string {
  if (v == null) return '—'
  return `${v.toFixed(1)}%`
}

function fmtMs(v: number | undefined): string {
  if (v == null || v <= 0) return '—'
  return `${Math.round(v)} ms`
}

function fmtInterval(s: number): string {
  if (s < 60) return `${s}s`
  return `${s / 60}m`
}

// ---- Monitor detail with response-time chart ----

function MonitorDetail({ monitor }: { monitor: UptimeMonitor }) {
  const [range, setRange] = useState<UptimeHistoryRange>('24h')
  const { data, isLoading } = useUptimeHistory(monitor.id, range)

  const points: SeriesPoint[] = (data?.results ?? []).map((r) => ({
    t: new Date(r.checked_at).getTime(),
    v: r.response_time_ms,
  }))

  const series: Series[] = [
    {
      label: 'Response time (ms)',
      color: 'var(--accent)',
      points,
    },
  ]

  const recentResults = (data?.results ?? []).slice(-20).reverse()

  return (
    <div
      className="mt-3 flex flex-col gap-4 rounded p-4"
      style={{ background: 'var(--bg-base)', border: '1px solid var(--border-subtle)' }}
    >
      {/* Uptime % row */}
      <div className="flex gap-6 text-sm">
        {(
          [
            ['24 h', monitor.uptime_24h],
            ['7 d', monitor.uptime_7d],
            ['30 d', monitor.uptime_30d],
          ] as [string, number | undefined][]
        ).map(([label, pct]) => (
          <div key={label} className="flex flex-col gap-0.5">
            <span style={{ color: 'var(--text-muted)', fontSize: '11px' }}>{label} uptime</span>
            <span
              style={{
                color:
                  (pct ?? 100) >= 99
                    ? 'var(--color-green)'
                    : (pct ?? 100) >= 95
                      ? 'var(--color-amber)'
                      : 'var(--color-red)',
                fontWeight: 600,
              }}
            >
              {fmtPct(pct)}
            </span>
          </div>
        ))}
        <div className="flex flex-col gap-0.5">
          <span style={{ color: 'var(--text-muted)', fontSize: '11px' }}>Avg resp</span>
          <span style={{ color: 'var(--text-secondary)' }}>{fmtMs(monitor.avg_response_ms)}</span>
        </div>
        <div className="flex flex-col gap-0.5">
          <span style={{ color: 'var(--text-muted)', fontSize: '11px' }}>Interval</span>
          <span style={{ color: 'var(--text-secondary)' }}>{fmtInterval(monitor.interval_seconds)}</span>
        </div>
      </div>

      {/* Range selector */}
      <div className="flex gap-1">
        {(['24h', '7d', '30d'] as UptimeHistoryRange[]).map((r) => (
          <button
            key={r}
            type="button"
            onClick={() => setRange(r)}
            className="px-2 py-0.5 rounded text-xs"
            style={{
              background: range === r ? 'var(--accent-dim)' : 'transparent',
              border: `1px solid ${range === r ? 'var(--accent-glow)' : 'var(--border-subtle)'}`,
              color: range === r ? 'var(--accent)' : 'var(--text-muted)',
              cursor: 'pointer',
            }}
          >
            {r}
          </button>
        ))}
      </div>

      {/* Chart */}
      {isLoading ? (
        <div className="flex items-center gap-2" style={{ color: 'var(--text-muted)' }}>
          <Loader size={14} className="animate-spin" />
          <span className="text-xs">Loading…</span>
        </div>
      ) : (
        <LineChart
          series={series}
          height={120}
          yFormatter={(v) => `${Math.round(v)} ms`}
        />
      )}

      {/* Recent events */}
      {recentResults.length > 0 && (
        <div>
          <p className="text-xs mb-2" style={{ color: 'var(--text-muted)' }}>
            Recent checks
          </p>
          <div className="flex flex-col gap-0.5">
            {recentResults.map((r) => (
              <div key={r.id} className="flex items-center gap-3 text-xs font-mono">
                <StatusPill status={r.status as 'up' | 'down' | 'degraded'} />
                <span style={{ color: 'var(--text-muted)' }}>
                  {new Date(r.checked_at).toLocaleTimeString()}
                </span>
                <span style={{ color: 'var(--text-secondary)' }}>{r.response_time_ms} ms</span>
                {r.error && (
                  <span
                    className="truncate"
                    style={{ color: 'var(--color-red)', maxWidth: '300px' }}
                    title={r.error}
                  >
                    {r.error}
                  </span>
                )}
              </div>
            ))}
          </div>
        </div>
      )}
    </div>
  )
}

// ---- Monitor row ----

interface MonitorRowProps {
  monitor: UptimeMonitor
  onEdit: (m: UptimeMonitor) => void
  onDelete: (id: string) => void
}

function MonitorRow({ monitor, onEdit, onDelete }: MonitorRowProps) {
  const [expanded, setExpanded] = useState(false)

  return (
    <div
      className="rounded"
      style={{ border: '1px solid var(--border-subtle)', background: 'var(--bg-surface)' }}
    >
      <div className="flex items-center gap-3 px-4 py-3">
        <button
          type="button"
          onClick={() => setExpanded((o) => !o)}
          className="flex items-center gap-3 flex-1 text-left min-w-0"
          style={{ background: 'transparent', border: 'none', cursor: 'pointer', padding: 0 }}
        >
          <StatusPill status={monitor.current_status} />
          <span className="text-sm font-medium truncate" style={{ color: 'var(--text-primary)' }}>
            {monitor.name}
          </span>
          <span
            className="font-mono text-xs shrink-0"
            style={{ color: 'var(--text-muted)' }}
          >
            {monitor.type.toUpperCase()}
          </span>
          <span
            className="font-mono text-xs truncate"
            style={{ color: 'var(--text-secondary)', maxWidth: '280px' }}
          >
            {monitor.target}
          </span>
          <span className="ml-auto shrink-0 text-xs" style={{ color: 'var(--text-muted)' }}>
            {fmtPct(monitor.uptime_24h)} / 24 h
          </span>
          {expanded ? (
            <ChevronUp size={13} style={{ color: 'var(--text-muted)', flexShrink: 0 }} />
          ) : (
            <ChevronDown size={13} style={{ color: 'var(--text-muted)', flexShrink: 0 }} />
          )}
        </button>

        <button
          type="button"
          onClick={() => onEdit(monitor)}
          className="flex items-center justify-center p-1.5 rounded"
          style={{
            background: 'transparent',
            border: 'none',
            color: 'var(--text-muted)',
            cursor: 'pointer',
          }}
          title="Edit monitor"
        >
          <Pencil size={13} />
        </button>
        <button
          type="button"
          onClick={() => onDelete(monitor.id)}
          className="flex items-center justify-center p-1.5 rounded"
          style={{
            background: 'transparent',
            border: 'none',
            color: 'var(--text-muted)',
            cursor: 'pointer',
          }}
          title="Delete monitor"
        >
          <Trash2 size={13} />
        </button>
      </div>

      {expanded && (
        <div className="px-4 pb-3">
          <MonitorDetail monitor={monitor} />
        </div>
      )}
    </div>
  )
}

// ---- Main page ----

export default function Uptime() {
  const { data, isLoading, isError } = useUptimeMonitors()
  const { mutate: create, isPending: isCreating } = useCreateUptimeMonitor()
  const { mutate: update, isPending: isUpdating } = useUpdateUptimeMonitor()
  const { mutate: remove } = useDeleteUptimeMonitor()

  const [showForm, setShowForm] = useState(false)
  const [editTarget, setEditTarget] = useState<UptimeMonitor | null>(null)
  const [deleteConfirm, setDeleteConfirm] = useState<string | null>(null)

  const monitors = data?.monitors ?? []

  function handleCreate(body: UptimeMonitorRequest) {
    create(body, { onSuccess: () => setShowForm(false) })
  }

  function handleUpdate(body: UptimeMonitorRequest) {
    if (!editTarget) return
    update(
      { id: editTarget.id, body },
      { onSuccess: () => setEditTarget(null) },
    )
  }

  function handleDelete(id: string) {
    if (deleteConfirm === id) {
      remove(id)
      setDeleteConfirm(null)
    } else {
      setDeleteConfirm(id)
      // Auto-cancel after 3 s
      setTimeout(() => setDeleteConfirm((c) => (c === id ? null : c)), 3000)
    }
  }

  const totalUp = monitors.filter((m) => m.current_status === 'up').length
  const totalDown = monitors.filter((m) => m.current_status === 'down').length

  return (
    <AppShell>
      <div className="flex flex-col gap-6 p-6 max-w-5xl mx-auto w-full">
        {/* Header */}
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-2">
            <Activity size={18} style={{ color: 'var(--accent)' }} />
            <h1 className="text-base font-semibold" style={{ color: 'var(--text-primary)' }}>
              Uptime Monitors
            </h1>
            {monitors.length > 0 && (
              <div className="flex items-center gap-2 ml-2">
                <span
                  className="text-xs px-1.5 py-0.5 rounded font-mono"
                  style={{ background: 'rgba(74,222,128,0.1)', color: '#4ade80' }}
                >
                  {totalUp} up
                </span>
                {totalDown > 0 && (
                  <span
                    className="text-xs px-1.5 py-0.5 rounded font-mono"
                    style={{ background: 'rgba(248,113,113,0.1)', color: '#f87171' }}
                  >
                    {totalDown} down
                  </span>
                )}
              </div>
            )}
          </div>
          <button
            type="button"
            onClick={() => {
              setShowForm(true)
              setEditTarget(null)
            }}
            className="flex items-center gap-1.5 px-3 py-1.5 rounded text-sm"
            style={{
              background: 'var(--accent)',
              border: 'none',
              color: '#fff',
              cursor: 'pointer',
            }}
          >
            <Plus size={14} />
            Add monitor
          </button>
        </div>

        {/* Create form */}
        {showForm && !editTarget && (
          <div
            className="rounded p-4"
            style={{ background: 'var(--bg-surface)', border: '1px solid var(--border-subtle)' }}
          >
            <h2 className="text-sm font-medium mb-3" style={{ color: 'var(--text-primary)' }}>
              New monitor
            </h2>
            <MonitorForm
              onSubmit={handleCreate}
              onCancel={() => setShowForm(false)}
              isPending={isCreating}
            />
          </div>
        )}

        {/* Edit form */}
        {editTarget && (
          <div
            className="rounded p-4"
            style={{ background: 'var(--bg-surface)', border: '1px solid var(--border-subtle)' }}
          >
            <h2 className="text-sm font-medium mb-3" style={{ color: 'var(--text-primary)' }}>
              Edit monitor
            </h2>
            <MonitorForm
              initial={editTarget}
              onSubmit={handleUpdate}
              onCancel={() => setEditTarget(null)}
              isPending={isUpdating}
            />
          </div>
        )}

        {/* Loading / error states */}
        {isLoading && (
          <div className="flex items-center gap-2" style={{ color: 'var(--text-muted)' }}>
            <Loader size={14} className="animate-spin" />
            <span className="text-sm">Loading monitors…</span>
          </div>
        )}

        {isError && (
          <div
            className="rounded p-3 text-sm"
            style={{ background: 'rgba(248,113,113,0.1)', border: '1px solid rgba(248,113,113,0.3)', color: '#f87171' }}
          >
            Failed to load monitors.
          </div>
        )}

        {!isLoading && !isError && monitors.length === 0 && (
          <div
            className="rounded p-6 text-center"
            style={{ border: '1px dashed var(--border-subtle)', color: 'var(--text-muted)' }}
          >
            <Clock size={24} className="mx-auto mb-2 opacity-50" />
            <p className="text-sm">No monitors configured yet.</p>
            <p className="text-xs mt-1">Add one to start tracking endpoint uptime.</p>
          </div>
        )}

        {/* Monitor list */}
        <div className="flex flex-col gap-2">
          {monitors.map((m) => (
            <div key={m.id}>
              {deleteConfirm === m.id ? (
                <div
                  className="flex items-center justify-between gap-3 px-4 py-3 rounded"
                  style={{
                    border: '1px solid rgba(248,113,113,0.4)',
                    background: 'rgba(248,113,113,0.08)',
                  }}
                >
                  <span className="text-sm" style={{ color: 'var(--text-secondary)' }}>
                    Delete <strong style={{ color: 'var(--text-primary)' }}>{m.name}</strong> and all its history?
                  </span>
                  <div className="flex gap-2">
                    <button
                      type="button"
                      onClick={() => setDeleteConfirm(null)}
                      className="px-2 py-1 rounded text-xs"
                      style={{
                        background: 'var(--bg-input)',
                        border: '1px solid var(--border-subtle)',
                        color: 'var(--text-secondary)',
                        cursor: 'pointer',
                      }}
                    >
                      Cancel
                    </button>
                    <button
                      type="button"
                      onClick={() => handleDelete(m.id)}
                      className="px-2 py-1 rounded text-xs"
                      style={{
                        background: 'rgba(248,113,113,0.2)',
                        border: '1px solid rgba(248,113,113,0.4)',
                        color: '#f87171',
                        cursor: 'pointer',
                      }}
                    >
                      Delete
                    </button>
                  </div>
                </div>
              ) : (
                <MonitorRow
                  monitor={m}
                  onEdit={(mon) => {
                    setEditTarget(mon)
                    setShowForm(false)
                  }}
                  onDelete={(id) => handleDelete(id)}
                />
              )}
            </div>
          ))}
        </div>
      </div>
    </AppShell>
  )
}
