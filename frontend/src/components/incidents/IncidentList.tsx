import { Link } from 'react-router-dom'
import type { IncidentEntry, IncidentSeverity, IncidentSource } from '../../lib/api/incidents'

// ── Severity badge ─────────────────────────────────────────────────────────────

const SEVERITY_COLOR: Record<IncidentSeverity, string> = {
  critical: 'var(--status-error)',
  warning: 'var(--status-warn)',
  info: 'var(--text-muted)',
}

const SEVERITY_LABEL: Record<IncidentSeverity, string> = {
  critical: 'critical',
  warning: 'warning',
  info: 'info',
}

// ── Source badge ───────────────────────────────────────────────────────────────

const SOURCE_LABEL: Record<IncidentSource, string> = {
  activity: 'activity',
  container: 'container',
  metric: 'metric',
  file_event: 'file',
}

const SOURCE_COLOR: Record<IncidentSource, string> = {
  activity: 'var(--accent)',
  container: '#6c8ebf',
  metric: '#d48d00',
  file_event: '#7b9e5f',
}

// ── Entry row ──────────────────────────────────────────────────────────────────

interface EntryRowProps {
  entry: IncidentEntry
}

function EntryRow({ entry }: EntryRowProps) {
  const sevColor = SEVERITY_COLOR[entry.severity] ?? 'var(--text-muted)'
  const srcColor = SOURCE_COLOR[entry.source] ?? 'var(--text-muted)'
  const srcLabel = SOURCE_LABEL[entry.source] ?? entry.source
  const sevLabel = SEVERITY_LABEL[entry.severity] ?? entry.severity

  const ts = new Date(entry.timestamp)
  const timeStr = ts.toLocaleTimeString(undefined, { hour: '2-digit', minute: '2-digit', second: '2-digit' })
  const dateStr = ts.toLocaleDateString(undefined, { month: 'short', day: 'numeric' })

  const inner = (
    <div
      className="flex items-center gap-3 px-4 py-2.5"
      style={{ borderTop: '1px solid var(--border-subtle)', textDecoration: 'none' }}
    >
      {/* Severity dot */}
      <span
        title={sevLabel}
        style={{ width: 8, height: 8, borderRadius: '50%', backgroundColor: sevColor, flexShrink: 0 }}
      />

      {/* Timestamp */}
      <div className="shrink-0" style={{ minWidth: 88 }}>
        <span className="text-xs font-mono" style={{ color: 'var(--text-secondary)' }}>
          {timeStr}
        </span>
        <span className="text-xs font-mono ml-1" style={{ color: 'var(--text-muted)' }}>
          {dateStr}
        </span>
      </div>

      {/* Source badge */}
      <span
        className="font-mono text-xs px-1.5 py-0.5 shrink-0"
        style={{
          color: srcColor,
          backgroundColor: 'var(--bg-elevated)',
          border: `1px solid ${srcColor}`,
          borderRadius: '3px',
          opacity: 0.9,
        }}
      >
        {srcLabel}
      </span>

      {/* Summary */}
      <span
        className="text-xs truncate flex-1"
        style={{ color: 'var(--text-primary)' }}
        title={entry.summary}
      >
        {entry.summary}
      </span>

      {/* Severity label (right-aligned) */}
      <span
        className="font-mono text-xs shrink-0"
        style={{ color: sevColor, minWidth: 52, textAlign: 'right' }}
      >
        {sevLabel}
      </span>
    </div>
  )

  if (entry.deep_link) {
    return (
      <Link to={entry.deep_link} style={{ textDecoration: 'none', display: 'block' }}>
        {inner}
      </Link>
    )
  }
  return <div>{inner}</div>
}

// ── List ───────────────────────────────────────────────────────────────────────

interface IncidentListProps {
  entries: IncidentEntry[]
}

export function IncidentList({ entries }: IncidentListProps) {
  if (entries.length === 0) {
    return (
      <div
        className="flex items-center justify-center px-4 py-12 text-xs"
        style={{ color: 'var(--text-muted)', borderTop: '1px solid var(--border-subtle)' }}
      >
        No incidents found in this time range.
      </div>
    )
  }

  return (
    <div style={{ overflowY: 'auto', maxHeight: 560 }}>
      {entries.map((e, i) => (
        <EntryRow key={`${e.timestamp}-${e.source}-${i}`} entry={e} />
      ))}
    </div>
  )
}
