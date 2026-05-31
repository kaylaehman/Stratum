interface StatusPillProps {
  status: 'up' | 'down' | 'degraded' | undefined
}

const config = {
  up: { label: 'UP', cls: 'text-green-400 border-green-700 bg-green-900/20' },
  down: { label: 'DOWN', cls: 'text-red-400 border-red-700 bg-red-900/20' },
  degraded: { label: 'DEGRADED', cls: 'text-amber-400 border-amber-700 bg-amber-900/20' },
} as const

export function StatusPill({ status }: StatusPillProps) {
  const s = status ?? 'down'
  const { label, cls } = config[s] ?? config.down
  return (
    <span
      className={`font-mono text-xs px-1.5 py-0.5 border rounded shrink-0 uppercase tracking-wider ${cls}`}
      style={{ fontSize: '11px' }}
    >
      {label}
    </span>
  )
}
