import { useMemo } from 'react'
import type { IncidentEntry, IncidentSeverity, IncidentSource } from '../../lib/api/incidents'
import type { MetricsRange } from '../../lib/api/metrics'
import { LineChart } from '../metrics/LineChart'
import type { Series, SeriesPoint } from '../metrics/LineChart'
import { RangeSelector } from '../metrics/RangeSelector'

// Each source maps to a colour for the overlay chart.
const SOURCE_SERIES_COLOR: Record<IncidentSource, string> = {
  activity: 'var(--accent)',
  container: '#6c8ebf',
  metric: '#e84040',
  file_event: '#7b9e5f',
}

interface IncidentTimelineProps {
  entries: IncidentEntry[]
  range: MetricsRange
  onRangeChange: (r: MetricsRange) => void
  from: string
  to: string
}

/**
 * Aggregates incidents into per-minute counts per source, rendering one
 * chart series per source. Uses the existing LineChart and RangeSelector
 * components so the visual language matches the Metrics page.
 */
export function IncidentTimeline({ entries, range, onRangeChange, from, to }: IncidentTimelineProps) {
  const fromMs = useMemo(() => (from ? new Date(from).getTime() : Date.now() - 86_400_000), [from])
  const toMs = useMemo(() => (to ? new Date(to).getTime() : Date.now()), [to])

  // Bucket size: 1 minute.
  const BUCKET_MS = 60_000

  const series = useMemo<Series[]>(() => {
    const buckets = new Map<IncidentSource, Map<number, number>>()

    for (const e of entries) {
      const t = new Date(e.timestamp).getTime()
      const bucket = Math.floor(t / BUCKET_MS) * BUCKET_MS
      if (!buckets.has(e.source)) {
        buckets.set(e.source, new Map())
      }
      const m = buckets.get(e.source)!
      m.set(bucket, (m.get(bucket) ?? 0) + 1)
    }

    const result: Series[] = []
    for (const [src, m] of buckets) {
      const points: SeriesPoint[] = []
      for (const [t, v] of m) {
        points.push({ t, v })
      }
      points.sort((a, b) => a.t - b.t)
      result.push({
        label: src,
        color: SOURCE_SERIES_COLOR[src] ?? 'var(--text-muted)',
        points,
      })
    }
    return result
  }, [entries])

  return (
    <div className="flex flex-col gap-3">
      <div className="flex items-center justify-between gap-2">
        <span className="text-xs font-medium uppercase tracking-wider" style={{ color: 'var(--text-muted)' }}>
          Events per minute
        </span>
        <div className="flex items-center gap-3">
          {/* Legend */}
          <div className="flex items-center gap-3">
            {(Object.keys(SOURCE_SERIES_COLOR) as IncidentSource[]).map((src) => (
              <div key={src} className="flex items-center gap-1">
                <span
                  style={{
                    display: 'inline-block',
                    width: 10,
                    height: 3,
                    borderRadius: 1,
                    backgroundColor: SOURCE_SERIES_COLOR[src],
                  }}
                />
                <span className="text-xs font-mono" style={{ color: 'var(--text-muted)' }}>
                  {src}
                </span>
              </div>
            ))}
          </div>
          <RangeSelector value={range} onChange={onRangeChange} />
        </div>
      </div>

      <LineChart
        series={series}
        height={140}
        yFormatter={(v) => String(Math.round(v))}
        xDomainMs={[fromMs, toMs]}
      />
    </div>
  )
}

/**
 * IncidentSeverityBar renders a horizontal bar that visualises the severity
 * breakdown of events in the current window.
 */
interface SeverityBarProps {
  entries: IncidentEntry[]
}

const SEV_ORDER: IncidentSeverity[] = ['critical', 'warning', 'info']
const SEV_COLOR: Record<IncidentSeverity, string> = {
  critical: 'var(--status-error)',
  warning: 'var(--status-warn)',
  info: 'var(--text-muted)',
}

export function IncidentSeverityBar({ entries }: SeverityBarProps) {
  const counts = useMemo(() => {
    const m: Record<IncidentSeverity, number> = { critical: 0, warning: 0, info: 0 }
    for (const e of entries) {
      m[e.severity] = (m[e.severity] ?? 0) + 1
    }
    return m
  }, [entries])

  const total = entries.length
  if (total === 0) return null

  return (
    <div className="flex items-center gap-4">
      {SEV_ORDER.map((sev) => {
        const count = counts[sev]
        if (count === 0) return null
        return (
          <div key={sev} className="flex items-center gap-1.5">
            <span
              style={{ width: 8, height: 8, borderRadius: '50%', backgroundColor: SEV_COLOR[sev], flexShrink: 0 }}
            />
            <span className="text-xs font-mono" style={{ color: SEV_COLOR[sev] }}>
              {count}
            </span>
            <span className="text-xs" style={{ color: 'var(--text-muted)' }}>
              {sev}
            </span>
          </div>
        )
      })}
    </div>
  )
}
