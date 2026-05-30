import { useState, useMemo } from 'react'
import { AppShell } from '../components/layout/AppShell'
import { IncidentList } from '../components/incidents/IncidentList'
import { IncidentTimeline, IncidentSeverityBar } from '../components/incidents/IncidentTimeline'
import { useIncidentTimeline, type IncidentFilters, type IncidentSource, type IncidentSeverity } from '../lib/api/incidents'
import { useNodes } from '../lib/api/nodes'
import type { MetricsRange } from '../lib/api/metrics'

// Convert a MetricsRange string to a lookback duration in milliseconds.
function rangeToDuration(r: MetricsRange): number {
  switch (r) {
    case '6h': return 6 * 3_600_000
    case '24h': return 24 * 3_600_000
    case '7d': return 7 * 24 * 3_600_000
    default: return 3_600_000 // 1h
  }
}

const SOURCE_OPTIONS: { value: IncidentSource | ''; label: string }[] = [
  { value: '', label: 'All sources' },
  { value: 'activity', label: 'Activity' },
  { value: 'container', label: 'Container' },
  { value: 'metric', label: 'Metric spike' },
  { value: 'file_event', label: 'File change' },
]

const SEVERITY_OPTIONS: { value: IncidentSeverity | ''; label: string }[] = [
  { value: '', label: 'All severities' },
  { value: 'critical', label: 'Critical' },
  { value: 'warning', label: 'Warning' },
  { value: 'info', label: 'Info' },
]

function FilterSelect({
  value,
  onChange,
  options,
}: {
  value: string
  onChange: (v: string) => void
  options: { value: string; label: string }[]
}) {
  return (
    <select
      value={value}
      onChange={(e) => onChange(e.target.value)}
      className="font-mono text-xs"
      style={{
        padding: '4px 8px',
        backgroundColor: 'var(--bg-surface)',
        color: 'var(--text-secondary)',
        border: '1px solid var(--border-default)',
        borderRadius: '3px',
        cursor: 'pointer',
      }}
    >
      {options.map((o) => (
        <option key={o.value} value={o.value}>
          {o.label}
        </option>
      ))}
    </select>
  )
}

export default function Incidents() {
  const [range, setRange] = useState<MetricsRange>('24h')
  const [nodeFilter, setNodeFilter] = useState('')
  const [sourceFilter, setSourceFilter] = useState<IncidentSource | ''>('')
  const [severityFilter, setSeverityFilter] = useState<IncidentSeverity | ''>('')

  const { data: nodes } = useNodes()

  const to = useMemo(() => new Date().toISOString(), [])
  const from = useMemo(
    () => new Date(Date.now() - rangeToDuration(range)).toISOString(),
    [range],
  )

  const filters: IncidentFilters = {
    from,
    to,
    ...(nodeFilter ? { node_id: nodeFilter } : {}),
  }

  const { data, isLoading, isError } = useIncidentTimeline(filters)

  const entries = useMemo(() => {
    const raw = data?.entries ?? []
    return raw.filter((e) => {
      if (sourceFilter && e.source !== sourceFilter) return false
      if (severityFilter && e.severity !== severityFilter) return false
      return true
    })
  }, [data, sourceFilter, severityFilter])

  const nodeOptions = useMemo(
    () => [
      { value: '', label: 'All nodes' },
      ...(nodes ?? []).map((n) => ({ value: n.id, label: n.name })),
    ],
    [nodes],
  )

  return (
    <AppShell>
      <div className="flex flex-col gap-6">
        {/* Header */}
        <div>
          <h1 className="text-base font-semibold" style={{ color: 'var(--text-primary)' }}>
            Incident Timeline
          </h1>
          <p className="text-xs mt-1" style={{ color: 'var(--text-secondary)' }}>
            Merged view of activity mutations, container status changes, metric spikes, and file-change events.
          </p>
        </div>

        {/* Filter bar */}
        <div className="flex items-center gap-2 flex-wrap">
          <FilterSelect value={nodeFilter} onChange={setNodeFilter} options={nodeOptions} />
          <FilterSelect
            value={sourceFilter}
            onChange={(v) => setSourceFilter(v as IncidentSource | '')}
            options={SOURCE_OPTIONS}
          />
          <FilterSelect
            value={severityFilter}
            onChange={(v) => setSeverityFilter(v as IncidentSeverity | '')}
            options={SEVERITY_OPTIONS}
          />
          {entries.length > 0 && (
            <span className="text-xs font-mono ml-auto" style={{ color: 'var(--text-muted)' }}>
              {entries.length} event{entries.length !== 1 ? 's' : ''}
            </span>
          )}
        </div>

        {/* Chart panel */}
        <div
          className="rounded border p-4 flex flex-col gap-3"
          style={{
            backgroundColor: 'var(--bg-surface)',
            borderColor: 'var(--border-subtle)',
            borderRadius: '3px',
          }}
        >
          <div className="flex items-center justify-between gap-2">
            <IncidentSeverityBar entries={entries} />
          </div>
          <IncidentTimeline
            entries={entries}
            range={range}
            onRangeChange={setRange}
            from={data?.from ?? from}
            to={data?.to ?? to}
          />
        </div>

        {/* Event list panel */}
        <div
          className="rounded border flex flex-col"
          style={{
            backgroundColor: 'var(--bg-surface)',
            borderColor: 'var(--border-subtle)',
            borderRadius: '3px',
            overflow: 'hidden',
          }}
        >
          <div className="flex items-center justify-between gap-2 px-4 py-2.5">
            <p className="text-xs font-medium uppercase tracking-wider" style={{ color: 'var(--text-muted)' }}>
              Events
            </p>
            <span className="text-xs font-mono" style={{ color: 'var(--text-muted)' }}>
              newest first
            </span>
          </div>

          {isLoading && (
            <div
              className="px-4 py-6 text-xs"
              style={{ color: 'var(--text-muted)', borderTop: '1px solid var(--border-subtle)' }}
            >
              Loading…
            </div>
          )}
          {isError && !isLoading && (
            <div
              className="px-4 py-6 text-xs"
              style={{ color: 'var(--status-error)', borderTop: '1px solid var(--border-subtle)' }}
            >
              Failed to load timeline. Check that the backend is reachable.
            </div>
          )}
          {!isLoading && !isError && <IncidentList entries={entries} />}
        </div>
      </div>
    </AppShell>
  )
}
