import { useMemo, useState, useRef, useCallback } from 'react'
import type { MouseEvent as ReactMouseEvent } from 'react'
import type { IncidentEntry, IncidentSeverity, IncidentSource } from '../../lib/api/incidents'
import type { MetricsRange } from '../../lib/api/metrics'
import { LineChart } from '../metrics/LineChart'
import type { Series, SeriesPoint } from '../metrics/LineChart'
import { RangeSelector } from '../metrics/RangeSelector'

// Mirror the geometry constants from LineChart.tsx so the crosshair aligns.
const CHART_PAD = { top: 8, right: 12, bottom: 24, left: 52 }
const CHART_TOTAL_WIDTH = 520

const BUCKET_MS = 60_000

function fmtBucketTime(ms: number): string {
  const d = new Date(ms)
  const date = d.toLocaleDateString(undefined, { month: 'short', day: 'numeric' })
  const time = `${String(d.getHours()).padStart(2, '0')}:${String(d.getMinutes()).padStart(2, '0')}`
  return `${date} ${time}`
}

const SEV_LABEL_COLOR: Record<string, string> = {
  critical: 'var(--status-error)',
  warning: 'var(--status-warn)',
  info: 'var(--text-muted)',
}

interface BucketHover {
  /** CSS px x-position within the wrapper div, used for crosshair + tooltip placement. */
  px: number
  /** Bucket start timestamp (ms). */
  bucketT: number
  /** Counts per source at this bucket. */
  rows: { label: string; color: string; count: number }[]
  /** Up to 5 raw incident entries that fall in this bucket. */
  events: IncidentEntry[]
}

interface TimelineChartWithTooltipProps {
  series: Series[]
  entries: IncidentEntry[]
  height: number
  xDomainMs: [number, number]
}

function TimelineChartWithTooltip({
  series,
  entries,
  height,
  xDomainMs,
}: TimelineChartWithTooltipProps) {
  const wrapRef = useRef<HTMLDivElement>(null)
  const [hover, setHover] = useState<BucketHover | null>(null)

  const allTs = series.flatMap((s) => s.points.map((p) => p.t))
  const hasData = allTs.length > 0

  const [xMin, xMax] = xDomainMs
  const xRange = Math.max(xMax - xMin, 1)
  const plotW = CHART_TOTAL_WIDTH - CHART_PAD.left - CHART_PAD.right

  const handleMove = useCallback(
    (e: ReactMouseEvent<HTMLDivElement>) => {
      const el = wrapRef.current
      if (!el || !hasData) return
      const rect = el.getBoundingClientRect()
      if (rect.width === 0) return
      const cssX = e.clientX - rect.left
      const vbX = (cssX / rect.width) * CHART_TOTAL_WIDTH
      const frac = (vbX - CHART_PAD.left) / plotW
      const tHover = xMin + frac * xRange
      const bucketT = Math.floor(tHover / BUCKET_MS) * BUCKET_MS

      // Snap to the nearest bucket that actually has data.
      let bestT = allTs[0]
      let bestDist = Infinity
      for (const t of allTs) {
        const d = Math.abs(t - bucketT)
        if (d < bestDist) { bestDist = d; bestT = t }
      }
      const snappedBucket = Math.floor(bestT / BUCKET_MS) * BUCKET_MS

      const rows = series
        .map((s) => {
          const pt = s.points.find((p) => p.t === snappedBucket)
          if (!pt) return null
          return { label: s.label, color: s.color, count: Math.round(pt.v) }
        })
        .filter((r): r is { label: string; color: string; count: number } => r !== null)

      const events = entries
        .filter((ev) => {
          const t = new Date(ev.timestamp).getTime()
          return t >= snappedBucket && t < snappedBucket + BUCKET_MS
        })
        .slice(0, 5)

      const snappedVbX = CHART_PAD.left + ((snappedBucket - xMin) / xRange) * plotW
      const snappedCssX = (snappedVbX / CHART_TOTAL_WIDTH) * rect.width

      setHover({ px: snappedCssX, bucketT: snappedBucket, rows, events })
    },
    [series, entries, allTs, hasData, plotW, xMin, xRange],
  )

  const handleLeave = useCallback(() => setHover(null), [])

  const TIP_W = 220
  let tipLeft = hover ? hover.px + 12 : 0
  const wrapW = wrapRef.current?.getBoundingClientRect().width ?? 0
  if (hover && wrapW > 0 && tipLeft + TIP_W > wrapW) {
    tipLeft = Math.max(0, hover.px - TIP_W - 12)
  }

  return (
    <div
      ref={wrapRef}
      className="w-full overflow-hidden"
      style={{ position: 'relative' }}
      onMouseMove={handleMove}
      onMouseLeave={handleLeave}
    >
      <LineChart
        series={series}
        height={height}
        yFormatter={(v) => String(Math.round(v))}
        xDomainMs={xDomainMs}
      />

      {hover && (hover.rows.length > 0 || hover.events.length > 0) && (
        <>
          {/* Vertical crosshair */}
          <div
            style={{
              position: 'absolute',
              top: CHART_PAD.top,
              left: `${hover.px}px`,
              width: '1px',
              height: `${height - CHART_PAD.top - CHART_PAD.bottom}px`,
              backgroundColor: 'var(--accent)',
              opacity: 0.5,
              pointerEvents: 'none',
            }}
          />
          {/* Tooltip */}
          <div
            style={{
              position: 'absolute',
              top: '4px',
              left: `${tipLeft}px`,
              width: `${TIP_W}px`,
              backgroundColor: 'var(--bg-surface)',
              border: '1px solid var(--border-subtle)',
              borderRadius: '3px',
              padding: '6px 8px',
              pointerEvents: 'none',
              zIndex: 5,
              boxShadow: '0 2px 8px rgba(0,0,0,0.3)',
            }}
          >
            {/* Timestamp header */}
            <div
              className="font-mono"
              style={{ color: 'var(--text-secondary)', fontSize: '11px', marginBottom: '4px' }}
            >
              {fmtBucketTime(hover.bucketT)}
            </div>

            {/* Per-source counts */}
            {hover.rows.length > 0 && (
              <div style={{ marginBottom: hover.events.length > 0 ? '6px' : 0 }}>
                {hover.rows.map((r) => (
                  <div
                    key={r.label}
                    style={{ display: 'flex', alignItems: 'center', gap: '6px', marginTop: '2px' }}
                  >
                    <span
                      style={{
                        width: '8px',
                        height: '8px',
                        borderRadius: '1px',
                        backgroundColor: r.color,
                        display: 'inline-block',
                        flexShrink: 0,
                      }}
                    />
                    <span
                      className="font-mono"
                      style={{
                        color: 'var(--text-secondary)',
                        fontSize: '11px',
                        flex: 1,
                        overflow: 'hidden',
                        textOverflow: 'ellipsis',
                        whiteSpace: 'nowrap',
                      }}
                    >
                      {r.label}
                    </span>
                    <span className="font-mono" style={{ color: 'var(--text-primary)', fontSize: '11px' }}>
                      {r.count}
                    </span>
                  </div>
                ))}
              </div>
            )}

            {/* Individual events in this bucket (up to 5) */}
            {hover.events.length > 0 && (
              <div style={{ borderTop: '1px solid var(--border-subtle)', paddingTop: '5px' }}>
                {hover.events.map((ev, i) => (
                  <div
                    key={i}
                    style={{
                      display: 'flex',
                      alignItems: 'flex-start',
                      gap: '5px',
                      marginTop: i > 0 ? '3px' : 0,
                    }}
                  >
                    <span
                      style={{
                        width: '6px',
                        height: '6px',
                        borderRadius: '50%',
                        backgroundColor: SEV_LABEL_COLOR[ev.severity] ?? 'var(--text-muted)',
                        flexShrink: 0,
                        marginTop: '2px',
                      }}
                    />
                    <span
                      className="font-mono"
                      style={{
                        color: 'var(--text-secondary)',
                        fontSize: '10px',
                        lineHeight: '1.4',
                        overflow: 'hidden',
                        display: '-webkit-box',
                        WebkitLineClamp: 2,
                        WebkitBoxOrient: 'vertical',
                      }}
                    >
                      {ev.summary}
                    </span>
                  </div>
                ))}
              </div>
            )}
          </div>
        </>
      )}
    </div>
  )
}

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

      <TimelineChartWithTooltip
        series={series}
        entries={entries}
        height={140}
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
