import { useState, useCallback, useRef } from 'react'
import { TrendingUp, Loader, Download, AlertTriangle } from 'lucide-react'
import { AppShell } from '../components/layout/AppShell'
import { ContainerPicker, containerColor } from '../components/metrics/ContainerPicker'
import { RangeSelector } from '../components/metrics/RangeSelector'
import { LineChart } from '../components/metrics/LineChart'
import { useContainerMetrics, exportMetricsCsv } from '../lib/api/metrics'
import { useTree } from '../lib/api/tree'
import type { MetricsRange } from '../lib/api/metrics'
import type { MetricSample, MetricSpike } from '../types/api'
import type { SeriesPoint, Series } from '../components/metrics/LineChart'

// ---- Helpers ----

function humanBytes(bytes: number): string {
  if (bytes <= 0) return '0 B'
  if (bytes < 1024) return `${bytes.toFixed(0)} B`
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`
  if (bytes < 1024 * 1024 * 1024) return `${(bytes / (1024 * 1024)).toFixed(1)} MB`
  return `${(bytes / (1024 * 1024 * 1024)).toFixed(2)} GB`
}

function humanBytesPerSec(v: number): string {
  return `${humanBytes(v)}/s`
}

function fmtCpuPct(v: number): string {
  return `${v.toFixed(0)}%`
}

function isoToMs(iso: string): number {
  return new Date(iso).getTime()
}

/** Derive per-interval disk I/O rates (bytes/sec) from cumulative byte counters. */
function diskRateSeries(
  samples: MetricSample[],
  field: 'disk_read_bytes' | 'disk_write_bytes',
): SeriesPoint[] {
  if (samples.length < 2) return []
  const out: SeriesPoint[] = []
  for (let i = 1; i < samples.length; i++) {
    const prev = samples[i - 1]
    const curr = samples[i]
    const dtMs = isoToMs(curr.sampled_at) - isoToMs(prev.sampled_at)
    if (dtMs <= 0) continue
    const dtSec = dtMs / 1000
    const delta = curr[field] - prev[field]
    // Guard against counter resets (container restart resets cumulative counter)
    const rate = delta >= 0 ? delta / dtSec : 0
    out.push({ t: isoToMs(curr.sampled_at), v: rate })
  }
  return out
}

// ---- Per-container data collector ----
// Renders nothing; calls useContainerMetrics (hook must be at component top level)
// and reports data up via onData.

interface ContainerSeriesData {
  cpuPoints: SeriesPoint[]
  memPoints: SeriesPoint[]
  memLimitBytes: number
  diskReadPoints: SeriesPoint[]
  diskWritePoints: SeriesPoint[]
  spikes: MetricSpike[]
  isLoading: boolean
  isError: boolean
}

interface ContainerSeriesProps {
  containerId: string
  range: MetricsRange
  onData: (id: string, data: ContainerSeriesData) => void
}

function ContainerSeries({ containerId, range, onData }: ContainerSeriesProps) {
  const { data, isLoading, isError } = useContainerMetrics(containerId, range)

  const lastKeyRef = useRef<string>('')
  const key = `${containerId}:${range}:${isLoading}:${isError}:${data?.samples.length ?? 0}`

  if (key !== lastKeyRef.current) {
    lastKeyRef.current = key
    const samples = data?.samples ?? []
    const spikes = data?.spikes ?? []

    const cpuPoints: SeriesPoint[] = samples.map((s) => ({ t: isoToMs(s.sampled_at), v: s.cpu_pct }))
    const memPoints: SeriesPoint[] = samples.map((s) => ({ t: isoToMs(s.sampled_at), v: s.mem_bytes }))
    const memLimitBytes = samples.length > 0 ? samples[samples.length - 1].mem_limit_bytes : 0
    const diskReadPoints = diskRateSeries(samples, 'disk_read_bytes')
    const diskWritePoints = diskRateSeries(samples, 'disk_write_bytes')

    onData(containerId, {
      cpuPoints,
      memPoints,
      memLimitBytes,
      diskReadPoints,
      diskWritePoints,
      spikes,
      isLoading,
      isError,
    })
  }

  return null
}

// ---- Chart section wrapper ----

interface ChartSectionProps {
  title: string
  children: React.ReactNode
}

function ChartSection({ title, children }: ChartSectionProps) {
  return (
    <div
      style={{
        backgroundColor: 'var(--bg-surface)',
        border: '1px solid var(--border-subtle)',
        borderRadius: '3px',
        padding: '16px',
        marginBottom: '16px',
      }}
    >
      <div
        className="text-xs font-medium uppercase tracking-wider mb-3"
        style={{ color: 'var(--text-muted)' }}
      >
        {title}
      </div>
      {children}
    </div>
  )
}

// ---- Legend row ----

interface LegendRowProps {
  entries: { label: string; color: string; last?: string; min?: string; max?: string }[]
}

function LegendRow({ entries }: LegendRowProps) {
  if (entries.length === 0) return null
  return (
    <div style={{ display: 'flex', flexWrap: 'wrap', gap: '12px', marginTop: '8px' }}>
      {entries.map((e) => (
        <div key={e.label} style={{ display: 'flex', alignItems: 'center', gap: '6px' }}>
          <span
            style={{
              width: '20px',
              height: '2px',
              backgroundColor: e.color,
              display: 'inline-block',
              borderRadius: '1px',
              flexShrink: 0,
            }}
          />
          <span className="font-mono text-xs" style={{ color: 'var(--text-secondary)' }}>
            {e.label}
          </span>
          {e.last !== undefined && (
            <span className="font-mono text-xs" style={{ color: 'var(--text-muted)', fontSize: '12px' }}>
              last: {e.last}
            </span>
          )}
        </div>
      ))}
    </div>
  )
}

// ---- Main Metrics page ----

export default function Metrics() {
  const [selectedIds, setSelectedIds] = useState<string[]>([])
  const [range, setRange] = useState<MetricsRange>('1h')
  const [seriesMap, setSeriesMap] = useState<Record<string, ContainerSeriesData>>({})
  const [exportingId, setExportingId] = useState<string | null>(null)

  const { data: tree } = useTree()

  const handleData = useCallback((id: string, data: ContainerSeriesData) => {
    setSeriesMap((prev) => ({ ...prev, [id]: data }))
  }, [])

  function containerName(id: string): string {
    for (const node of tree?.nodes ?? []) {
      const c = node.containers.find((x) => x.id === id)
      if (c) return c.name
    }
    return id.slice(0, 12)
  }

  // Build flat ordered list for stable color assignment
  const allContainerIds = (tree?.nodes ?? []).flatMap((n) => n.containers.map((c) => c.id))

  function colorFor(id: string): string {
    const idx = allContainerIds.indexOf(id)
    return containerColor(idx >= 0 ? idx : 0)
  }

  const anyLoading = selectedIds.some((id) => seriesMap[id]?.isLoading)

  // Aggregate series across selected containers
  const cpuSeriesList: Series[] = selectedIds
    .filter((id) => seriesMap[id] && seriesMap[id].cpuPoints.length > 0)
    .map((id) => ({ label: containerName(id), color: colorFor(id), points: seriesMap[id].cpuPoints }))

  const cpuSpikes: MetricSpike[] = selectedIds.flatMap(
    (id) => seriesMap[id]?.spikes.filter((s) => s.metric === 'cpu') ?? [],
  )

  const memSeriesList: Series[] = selectedIds
    .filter((id) => seriesMap[id] && seriesMap[id].memPoints.length > 0)
    .map((id) => ({ label: containerName(id), color: colorFor(id), points: seriesMap[id].memPoints }))

  const memSpikes: MetricSpike[] = selectedIds.flatMap(
    (id) => seriesMap[id]?.spikes.filter((s) => s.metric === 'mem') ?? [],
  )

  // Mem limit reference lines — one dashed line per selected container that has a limit
  const memLimitSeries: Series[] = selectedIds
    .filter((id) => seriesMap[id] && seriesMap[id].memLimitBytes > 0 && seriesMap[id].memPoints.length > 0)
    .map((id) => {
      const limit = seriesMap[id].memLimitBytes
      const pts = seriesMap[id].memPoints
      return {
        label: `${containerName(id)} (limit)`,
        color: colorFor(id),
        // Flat horizontal line at the limit value across the full time domain
        points: [
          { t: pts[0].t, v: limit },
          { t: pts[pts.length - 1].t, v: limit },
        ],
      }
    })

  const diskReadSeries: Series[] = selectedIds
    .filter((id) => seriesMap[id] && seriesMap[id].diskReadPoints.length > 0)
    .map((id) => ({
      label: `${containerName(id)} read`,
      color: colorFor(id),
      points: seriesMap[id].diskReadPoints,
    }))

  const diskWriteSeries: Series[] = selectedIds
    .filter((id) => seriesMap[id] && seriesMap[id].diskWritePoints.length > 0)
    .map((id) => ({
      label: `${containerName(id)} write`,
      // Slightly desaturated version for write vs read
      color: colorFor(id) + 'aa',
      points: seriesMap[id].diskWritePoints,
    }))

  async function handleExport(id: string) {
    setExportingId(id)
    try {
      await exportMetricsCsv(id, range)
    } catch {
      // silently swallow — could add toast in future
    } finally {
      setExportingId(null)
    }
  }

  return (
    <AppShell>
      {/* Hidden data collectors — one per selected container */}
      {selectedIds.map((id) => (
        <ContainerSeries key={`${id}:${range}`} containerId={id} range={range} onData={handleData} />
      ))}

      <div
        className="flex flex-col flex-1 min-h-0 h-full w-full p-6"
        style={{ maxWidth: '1200px', margin: '0 auto' }}
      >
        {/* Page header */}
        <div className="flex items-center justify-between mb-6 gap-4 flex-wrap">
          <div className="flex items-center gap-2">
            <TrendingUp size={16} style={{ color: 'var(--text-secondary)' }} />
            <h1
              className="text-sm font-medium uppercase tracking-wider"
              style={{ color: 'var(--text-primary)' }}
            >
              Resource Timeline
            </h1>
            {anyLoading && (
              <Loader size={12} className="animate-spin" style={{ color: 'var(--accent)', marginLeft: '4px' }} />
            )}
          </div>
          <RangeSelector value={range} onChange={setRange} />
        </div>

        <div style={{ display: 'flex', gap: '20px', alignItems: 'flex-start' }}>
          {/* Left sidebar: container picker */}
          <div
            style={{
              width: '220px',
              flexShrink: 0,
              backgroundColor: 'var(--bg-surface)',
              border: '1px solid var(--border-subtle)',
              borderRadius: '3px',
              padding: '12px',
            }}
          >
            <div
              className="text-xs font-medium uppercase tracking-wider mb-3"
              style={{ color: 'var(--text-muted)' }}
            >
              Containers
            </div>
            <ContainerPicker selectedIds={selectedIds} onChange={setSelectedIds} />
          </div>

          {/* Main chart area */}
          <div style={{ flex: 1, minWidth: 0 }}>
            {selectedIds.length === 0 ? (
              <div
                style={{
                  display: 'flex',
                  flexDirection: 'column',
                  alignItems: 'center',
                  justifyContent: 'center',
                  gap: '8px',
                  minHeight: '260px',
                  backgroundColor: 'var(--bg-surface)',
                  border: '1px dashed var(--border-subtle)',
                  borderRadius: '3px',
                  color: 'var(--text-muted)',
                  fontSize: '12px',
                }}
              >
                <TrendingUp size={22} style={{ opacity: 0.3 }} />
                Select one or more containers to view their resource timeline.
              </div>
            ) : (
              <>
                {/* Per-container export buttons */}
                <div style={{ display: 'flex', flexWrap: 'wrap', gap: '8px', marginBottom: '14px' }}>
                  {selectedIds.map((id) => (
                    <button
                      key={id}
                      type="button"
                      onClick={() => void handleExport(id)}
                      disabled={exportingId === id}
                      className="flex items-center gap-1.5 text-xs px-2.5 py-1"
                      style={{
                        backgroundColor: 'var(--bg-elevated)',
                        border: '1px solid var(--border-default)',
                        borderLeft: `3px solid ${colorFor(id)}`,
                        color: 'var(--text-secondary)',
                        borderRadius: '3px',
                        cursor: exportingId === id ? 'default' : 'pointer',
                        opacity: exportingId === id ? 0.6 : 1,
                      }}
                    >
                      <Download size={10} />
                      {containerName(id)}
                      <span style={{ color: 'var(--text-muted)', fontSize: '12px' }}>CSV</span>
                    </button>
                  ))}
                </div>

                {/* Spike warning banner */}
                {(cpuSpikes.length > 0 || memSpikes.length > 0) && (
                  <div
                    className="flex items-center gap-2 px-3 py-2 text-xs mb-3"
                    style={{
                      backgroundColor: 'rgba(232,64,64,0.08)',
                      border: '1px solid rgba(232,64,64,0.25)',
                      borderRadius: '3px',
                      color: 'var(--status-error)',
                    }}
                  >
                    <AlertTriangle size={11} style={{ flexShrink: 0 }} />
                    {cpuSpikes.length > 0 && `${cpuSpikes.length} CPU spike${cpuSpikes.length > 1 ? 's' : ''} detected`}
                    {cpuSpikes.length > 0 && memSpikes.length > 0 && ' · '}
                    {memSpikes.length > 0 && `${memSpikes.length} memory spike${memSpikes.length > 1 ? 's' : ''} detected`}
                    <span style={{ color: 'var(--text-muted)', marginLeft: '4px' }}>
                      — shaded red bands on charts
                    </span>
                  </div>
                )}

                {/* CPU chart */}
                <ChartSection title="CPU %">
                  <LineChart
                    series={cpuSeriesList}
                    height={160}
                    yFormatter={fmtCpuPct}
                    spikes={cpuSpikes}
                    spikeToMs={isoToMs}
                  />
                  <LegendRow
                    entries={cpuSeriesList.map((s) => {
                      const last = s.points[s.points.length - 1]?.v
                      return {
                        label: s.label,
                        color: s.color,
                        last: last !== undefined ? fmtCpuPct(last) : undefined,
                      }
                    })}
                  />
                </ChartSection>

                {/* Memory chart */}
                <ChartSection title="Memory">
                  <LineChart
                    series={[...memSeriesList, ...memLimitSeries]}
                    height={160}
                    yFormatter={humanBytes}
                    spikes={memSpikes}
                    spikeToMs={isoToMs}
                  />
                  <LegendRow
                    entries={memSeriesList.map((s) => {
                      const last = s.points[s.points.length - 1]?.v
                      return {
                        label: s.label,
                        color: s.color,
                        last: last !== undefined ? humanBytes(last) : undefined,
                      }
                    })}
                  />
                  {memLimitSeries.length > 0 && (
                    <div
                      className="text-xs mt-1.5"
                      style={{ color: 'var(--text-muted)', fontSize: '12px' }}
                    >
                      Dashed lines indicate memory limit per container.
                    </div>
                  )}
                </ChartSection>

                {/* Disk I/O chart */}
                <ChartSection title="Disk I/O (rate)">
                  <LineChart
                    series={[...diskReadSeries, ...diskWriteSeries]}
                    height={160}
                    yFormatter={humanBytesPerSec}
                  />
                  <LegendRow
                    entries={[...diskReadSeries, ...diskWriteSeries].map((s) => {
                      const last = s.points[s.points.length - 1]?.v
                      return {
                        label: s.label,
                        color: s.color,
                        last: last !== undefined ? humanBytesPerSec(last) : undefined,
                      }
                    })}
                  />
                  <div
                    className="text-xs mt-1.5"
                    style={{ color: 'var(--text-muted)', fontSize: '12px' }}
                  >
                    Rate computed from consecutive sample deltas (bytes/sec). Counter resets on container restart are clamped to 0.
                  </div>
                </ChartSection>
              </>
            )}
          </div>
        </div>
      </div>
    </AppShell>
  )
}
