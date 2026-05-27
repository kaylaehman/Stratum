import type { MetricSpike } from '../../types/api'

export interface SeriesPoint {
  t: number // ms epoch
  v: number
}

export interface Series {
  label: string
  color: string
  points: SeriesPoint[]
}

interface LineChartProps {
  series: Series[]
  height?: number
  yFormatter?: (v: number) => string
  spikes?: MetricSpike[]
  spikeToMs?: (iso: string) => number
  xDomainMs?: [number, number]
}

const PAD = { top: 8, right: 12, bottom: 24, left: 52 }

function buildPath(points: SeriesPoint[], xMin: number, xRange: number, yMin: number, yRange: number, W: number, H: number): string {
  if (points.length === 0) return ''
  return points
    .map((p, i) => {
      const x = PAD.left + ((p.t - xMin) / xRange) * W
      const y = PAD.top + H - ((p.v - yMin) / yRange) * H
      return `${i === 0 ? 'M' : 'L'}${x.toFixed(2)},${y.toFixed(2)}`
    })
    .join(' ')
}

export function LineChart({
  series,
  height = 160,
  yFormatter = (v) => String(v),
  spikes,
  spikeToMs,
  xDomainMs,
}: LineChartProps) {
  const totalWidth = 520
  const W = totalWidth - PAD.left - PAD.right
  const H = height - PAD.top - PAD.bottom

  const allPoints = series.flatMap((s) => s.points)

  if (allPoints.length === 0) {
    return (
      <div
        style={{
          height,
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'center',
          color: 'var(--text-muted)',
          fontSize: '11px',
          border: '1px dashed var(--border-subtle)',
          borderRadius: '3px',
        }}
      >
        No data
      </div>
    )
  }

  const allTs = allPoints.map((p) => p.t)
  const allVs = allPoints.map((p) => p.v)

  const xMin = xDomainMs ? xDomainMs[0] : Math.min(...allTs)
  const xMax = xDomainMs ? xDomainMs[1] : Math.max(...allTs)
  const xRange = xMax - xMin || 1

  const rawYMax = Math.max(...allVs)
  const yMin = 0
  const yMax = rawYMax === 0 ? 1 : rawYMax * 1.08
  const yRange = yMax - yMin

  // y gridlines
  const yTicks = [0, 0.25, 0.5, 0.75, 1].map((f) => yMin + f * yRange)

  // x label count
  const xTickCount = 5
  const xTicks = Array.from({ length: xTickCount }, (_, i) => xMin + (i / (xTickCount - 1)) * xRange)

  function xPx(t: number) {
    return PAD.left + ((t - xMin) / xRange) * W
  }
  function yPx(v: number) {
    return PAD.top + H - ((v - yMin) / yRange) * H
  }

  function fmtTime(ms: number) {
    const d = new Date(ms)
    return `${String(d.getHours()).padStart(2, '0')}:${String(d.getMinutes()).padStart(2, '0')}`
  }

  return (
    <svg
      viewBox={`0 0 ${totalWidth} ${height}`}
      width="100%"
      style={{ display: 'block', overflow: 'visible' }}
      aria-label="Line chart"
    >
      {/* Spike bands */}
      {spikes && spikeToMs &&
        spikes.map((spike, i) => {
          const fx = xPx(spikeToMs(spike.from))
          const tx = xPx(spikeToMs(spike.to))
          const bw = Math.max(tx - fx, 2)
          return (
            <rect
              key={i}
              x={fx}
              y={PAD.top}
              width={bw}
              height={H}
              fill="rgba(232,64,64,0.12)"
            />
          )
        })}

      {/* Y gridlines */}
      {yTicks.map((v) => (
        <line
          key={v}
          x1={PAD.left}
          x2={PAD.left + W}
          y1={yPx(v)}
          y2={yPx(v)}
          stroke="var(--border-subtle)"
          strokeWidth={0.8}
        />
      ))}

      {/* Y axis labels */}
      {yTicks.map((v) => (
        <text
          key={v}
          x={PAD.left - 4}
          y={yPx(v) + 4}
          textAnchor="end"
          fill="var(--text-muted)"
          style={{ fontSize: '9px', fontFamily: 'monospace' }}
        >
          {yFormatter(v)}
        </text>
      ))}

      {/* X axis labels */}
      {xTicks.map((t) => (
        <text
          key={t}
          x={xPx(t)}
          y={PAD.top + H + 14}
          textAnchor="middle"
          fill="var(--text-muted)"
          style={{ fontSize: '9px', fontFamily: 'monospace' }}
        >
          {fmtTime(t)}
        </text>
      ))}

      {/* Series lines */}
      {series.map((s) => {
        const d = buildPath(s.points, xMin, xRange, yMin, yRange, W, H)
        if (!d) return null
        return (
          <path
            key={s.label}
            d={d}
            fill="none"
            stroke={s.color}
            strokeWidth={1.5}
            strokeLinejoin="round"
            strokeLinecap="round"
          />
        )
      })}
    </svg>
  )
}
