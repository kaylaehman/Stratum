import type { NodeStatus, ContainerStatus } from '../../types/api'

type BadgeStatus = NodeStatus | ContainerStatus | string

function resolveColor(status: BadgeStatus): string {
  switch (status) {
    case 'ok':
    case 'running':
      return 'var(--status-ok)'
    case 'unreachable':
    case 'paused':
    case 'restarting':
      return 'var(--status-warn)'
    case 'error':
    case 'dead':
      return 'var(--status-error)'
    case 'exited':
    case 'created':
    case 'unknown':
    default:
      return 'var(--status-muted)'
  }
}

function isErrorStatus(status: BadgeStatus): boolean {
  return status === 'error' || status === 'unreachable'
}

interface StatusBadgeProps {
  status: BadgeStatus
  stale?: boolean
  dot?: boolean
}

export function StatusBadge({ status, stale, dot }: StatusBadgeProps) {
  const effectiveStatus = stale ? 'unreachable' : status
  const color = resolveColor(effectiveStatus)
  const pulse = isErrorStatus(effectiveStatus)

  if (dot) {
    return (
      <span
        aria-label={effectiveStatus}
        style={{
          display: 'inline-block',
          width: '6px',
          height: '6px',
          borderRadius: '50%',
          backgroundColor: color,
          flexShrink: 0,
          animation: pulse ? 'pulse 2s cubic-bezier(0.4,0,0.6,1) infinite' : undefined,
        }}
      />
    )
  }

  return (
    <span
      className="text-xs font-mono px-1 py-0"
      style={{
        color,
        backgroundColor: `color-mix(in srgb, ${color} 10%, transparent)`,
        border: `1px solid color-mix(in srgb, ${color} 25%, transparent)`,
        borderRadius: '3px',
        fontSize: '10px',
        lineHeight: '16px',
        animation: pulse ? 'pulse 2s cubic-bezier(0.4,0,0.6,1) infinite' : undefined,
      }}
    >
      {effectiveStatus}
    </span>
  )
}
