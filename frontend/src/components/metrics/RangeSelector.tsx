import type { MetricsRange } from '../../lib/api/metrics'

const RANGES: { value: MetricsRange; label: string }[] = [
  { value: '1h', label: '1h' },
  { value: '6h', label: '6h' },
  { value: '24h', label: '24h' },
  { value: '7d', label: '7d' },
]

interface RangeSelectorProps {
  value: MetricsRange
  onChange: (range: MetricsRange) => void
}

export function RangeSelector({ value, onChange }: RangeSelectorProps) {
  return (
    <div
      style={{
        display: 'inline-flex',
        border: '1px solid var(--border-default)',
        borderRadius: '3px',
        overflow: 'hidden',
      }}
    >
      {RANGES.map((r, i) => {
        const active = r.value === value
        return (
          <button
            key={r.value}
            type="button"
            onClick={() => onChange(r.value)}
            className="font-mono text-xs uppercase tracking-wider"
            style={{
              padding: '4px 10px',
              backgroundColor: active ? 'var(--accent-glow)' : 'var(--bg-surface)',
              color: active ? 'var(--accent)' : 'var(--text-secondary)',
              border: 'none',
              borderRight: i < RANGES.length - 1 ? '1px solid var(--border-default)' : 'none',
              cursor: 'pointer',
              fontSize: '10px',
              transition: 'background-color 0.1s, color 0.1s',
            }}
          >
            {r.label}
          </button>
        )
      })}
    </div>
  )
}
