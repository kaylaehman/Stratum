import type { RichLogLine } from '../../store/logs'

interface LogLineRowProps {
  line: RichLogLine
  color: string
  containerName: string
}

const LEVEL_STYLES: Record<string, { bg: string; color: string }> = {
  error: { bg: 'rgba(232,64,64,0.18)', color: '#e84040' },
  fatal: { bg: 'rgba(232,64,64,0.25)', color: '#e84040' },
  warn:  { bg: 'rgba(240,160,32,0.18)', color: '#f0a020' },
  warning: { bg: 'rgba(240,160,32,0.18)', color: '#f0a020' },
  info:  { bg: 'rgba(74,158,255,0.15)', color: '#4a9eff' },
  debug: { bg: 'rgba(139,147,168,0.15)', color: '#8b93a8' },
  trace: { bg: 'rgba(139,147,168,0.10)', color: '#4a5268' },
}

function formatTs(ts: string): string {
  if (!ts) return ''
  try {
    const d = new Date(ts)
    if (isNaN(d.getTime())) return ''
    return d.toLocaleTimeString(undefined, {
      hour: '2-digit',
      minute: '2-digit',
      second: '2-digit',
      hour12: false,
    })
  } catch {
    return ''
  }
}

export function LogLineRow({ line, color, containerName }: LogLineRowProps) {
  const ts = formatTs(line.ts)
  const levelStyle = line.level ? (LEVEL_STYLES[line.level] ?? null) : null
  const isStderr = line.stream === 'stderr'
  const text = line.truncated
    ? `${line.text} [TRUNCATED]`
    : line.text

  return (
    <div
      className="flex items-start font-mono text-xs leading-5 hover:bg-white/[0.02]"
      style={{ minHeight: '20px' }}
    >
      {/* Left color bar */}
      <div
        className="shrink-0 self-stretch"
        style={{ width: '3px', backgroundColor: color, opacity: 0.85 }}
      />

      {/* Timestamp */}
      {ts && (
        <span
          className="shrink-0 pl-2 pr-1 select-none"
          style={{ color: 'var(--text-muted)', minWidth: '64px' }}
        >
          {ts}
        </span>
      )}

      {/* Level badge */}
      {levelStyle && (
        <span
          className="shrink-0 mr-1.5 px-1 rounded-sm text-[10px] leading-4 self-center"
          style={{
            background: levelStyle.bg,
            color: levelStyle.color,
            border: `1px solid ${levelStyle.color}30`,
            fontVariantNumeric: 'tabular-nums',
          }}
        >
          {(line.level ?? '').toUpperCase().slice(0, 5)}
        </span>
      )}

      {/* Container short-name */}
      <span
        className="shrink-0 mr-2 self-center"
        style={{ color, opacity: 0.7, maxWidth: '80px', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}
        title={containerName}
      >
        {containerName}
      </span>

      {/* Log text */}
      <span
        className="break-all whitespace-pre-wrap flex-1 pr-2"
        style={{
          color: isStderr ? '#f87171' : 'var(--text-primary)',
        }}
      >
        {text}
      </span>
    </div>
  )
}
