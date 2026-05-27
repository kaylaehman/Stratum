import { useState } from 'react'
import {
  HeartPulse,
  CheckCircle,
  XCircle,
  Clock,
  MinusCircle,
  Loader,
} from 'lucide-react'
import { useContainerHealth } from '../../lib/api/health'
import type { HealthStatus, HealthLogEntry } from '../../types/api'

interface HealthCheckProps {
  containerId: string
}

function fmtSeconds(s: number): string {
  return `${s}s`
}

function formatTestCommand(test: string[]): string {
  if (test.length === 0) return '(empty)'
  const [type, ...rest] = test
  if (type === 'CMD-SHELL') return rest.join(' ')
  if (type === 'CMD') return rest.join(' ')
  if (type === 'NONE') return 'NONE'
  return test.join(' ')
}

interface StatusBadgeProps {
  status: HealthStatus
  failingStreak: number
}

function StatusBadge({ status, failingStreak }: StatusBadgeProps) {
  const configs: Record<
    HealthStatus,
    { icon: React.ReactNode; label: string; color: string }
  > = {
    healthy: {
      icon: <CheckCircle size={13} />,
      label: 'Healthy',
      color: 'var(--status-ok, #40c878)',
    },
    unhealthy: {
      icon: <XCircle size={13} />,
      label: failingStreak > 0 ? `Unhealthy (${failingStreak} failed)` : 'Unhealthy',
      color: 'var(--status-error)',
    },
    starting: {
      icon: <Clock size={13} />,
      label: 'Starting',
      color: 'var(--status-warn)',
    },
    none: {
      icon: <MinusCircle size={13} />,
      label: 'No healthcheck configured',
      color: 'var(--text-muted)',
    },
  }

  const cfg = configs[status]

  return (
    <span
      className="flex items-center gap-1.5 font-mono text-xs"
      style={{ color: cfg.color }}
    >
      {cfg.icon}
      {cfg.label}
    </span>
  )
}

function LogRow({ entry }: { entry: HealthLogEntry }) {
  const [expanded, setExpanded] = useState(false)
  const ok = entry.exit_code === 0
  const ts = new Date(entry.end).toLocaleTimeString()
  const trimmed = entry.output.trim()
  const isLong = trimmed.length > 120

  return (
    <div
      className="flex items-start gap-3 px-3 py-2"
      style={{ borderBottom: '1px solid var(--border-subtle)' }}
    >
      <span
        className="font-mono text-xs shrink-0 w-16"
        style={{ color: 'var(--text-muted)' }}
      >
        {ts}
      </span>
      <span
        className="font-mono text-xs shrink-0 w-6 text-center"
        style={{ color: ok ? 'var(--status-ok, #40c878)' : 'var(--status-error)' }}
        title={`Exit code ${entry.exit_code}`}
      >
        {entry.exit_code}
      </span>
      <span
        className="font-mono text-xs flex-1 min-w-0"
        style={{
          color: 'var(--text-secondary)',
          whiteSpace: expanded ? 'pre-wrap' : 'nowrap',
          overflow: 'hidden',
          textOverflow: expanded ? 'unset' : 'ellipsis',
          wordBreak: 'break-all',
          cursor: isLong ? 'pointer' : 'default',
        }}
        title={isLong && !expanded ? trimmed : undefined}
        onClick={() => isLong && setExpanded((v) => !v)}
      >
        {trimmed || '(no output)'}
      </span>
    </div>
  )
}

export function HealthCheck({ containerId }: HealthCheckProps) {
  const { data, isLoading, isError, error } = useContainerHealth(containerId)

  const is502 =
    isError &&
    typeof error === 'object' &&
    error !== null &&
    'status' in error &&
    (error as { status: number }).status === 502

  return (
    <div
      style={{
        backgroundColor: 'var(--bg-surface)',
        border: '1px solid var(--border-subtle)',
        borderRadius: '3px',
        maxWidth: '640px',
      }}
    >
      {/* Header */}
      <div
        className="px-3 py-2 flex items-center gap-2"
        style={{ borderBottom: '1px solid var(--border-subtle)' }}
      >
        <HeartPulse size={12} style={{ color: 'var(--text-muted)' }} />
        <p
          className="text-xs font-medium uppercase tracking-wider"
          style={{ color: 'var(--text-muted)' }}
        >
          Health Check
        </p>
      </div>

      {/* Loading */}
      {isLoading && (
        <div className="flex items-center gap-2 px-3 py-4">
          <Loader size={12} className="animate-spin" style={{ color: 'var(--accent)' }} />
          <span className="text-xs" style={{ color: 'var(--text-muted)' }}>
            Loading health...
          </span>
        </div>
      )}

      {/* Error */}
      {isError && (
        <div className="px-3 py-3 text-xs font-mono" style={{ color: 'var(--status-error)' }}>
          {is502
            ? 'Docker daemon unreachable — cannot fetch health data.'
            : 'Failed to load health check data.'}
        </div>
      )}

      {/* Data */}
      {data && (
        <>
          {/* Status row */}
          <div className="px-3 py-3">
            <StatusBadge status={data.status} failingStreak={data.failing_streak} />
          </div>

          {/* Config (only when configured) */}
          {data.configured && (
            <div
              className="px-3 py-3 flex flex-col gap-1.5"
              style={{ borderTop: '1px solid var(--border-subtle)' }}
            >
              <span
                className="text-xs font-medium uppercase tracking-wider"
                style={{ color: 'var(--text-muted)' }}
              >
                Configuration
              </span>
              <div className="flex items-baseline gap-3 mt-1">
                <span
                  className="text-xs shrink-0 w-28"
                  style={{ color: 'var(--text-muted)' }}
                >
                  Command
                </span>
                <span
                  className="font-mono text-xs truncate"
                  style={{ color: 'var(--text-primary)' }}
                  title={formatTestCommand(data.test)}
                >
                  {formatTestCommand(data.test)}
                </span>
              </div>
              <div className="flex items-baseline gap-3">
                <span
                  className="text-xs shrink-0 w-28"
                  style={{ color: 'var(--text-muted)' }}
                >
                  Interval
                </span>
                <span
                  className="font-mono text-xs"
                  style={{ color: 'var(--text-secondary)' }}
                >
                  {fmtSeconds(data.interval_sec)}
                </span>
              </div>
              <div className="flex items-baseline gap-3">
                <span
                  className="text-xs shrink-0 w-28"
                  style={{ color: 'var(--text-muted)' }}
                >
                  Timeout
                </span>
                <span
                  className="font-mono text-xs"
                  style={{ color: 'var(--text-secondary)' }}
                >
                  {fmtSeconds(data.timeout_sec)}
                </span>
              </div>
              <div className="flex items-baseline gap-3">
                <span
                  className="text-xs shrink-0 w-28"
                  style={{ color: 'var(--text-muted)' }}
                >
                  Start period
                </span>
                <span
                  className="font-mono text-xs"
                  style={{ color: 'var(--text-secondary)' }}
                >
                  {fmtSeconds(data.start_period_sec)}
                </span>
              </div>
              <div className="flex items-baseline gap-3">
                <span
                  className="text-xs shrink-0 w-28"
                  style={{ color: 'var(--text-muted)' }}
                >
                  Retries
                </span>
                <span
                  className="font-mono text-xs"
                  style={{ color: 'var(--text-secondary)' }}
                >
                  {data.retries}
                </span>
              </div>
            </div>
          )}

          {/* Log results */}
          {data.configured && (
            <div style={{ borderTop: '1px solid var(--border-subtle)' }}>
              <div
                className="px-3 py-2 flex items-center justify-between"
                style={{ borderBottom: '1px solid var(--border-subtle)' }}
              >
                <span
                  className="text-xs font-medium uppercase tracking-wider"
                  style={{ color: 'var(--text-muted)' }}
                >
                  Recent probes
                </span>
                {data.log.length > 0 && (
                  <span className="text-xs" style={{ color: 'var(--text-muted)' }}>
                    {data.log.length}
                  </span>
                )}
              </div>

              {data.log.length === 0 ? (
                <div className="px-3 py-3 text-xs" style={{ color: 'var(--text-muted)' }}>
                  No probe results yet.
                </div>
              ) : (
                /* Reverse so newest is first */
                [...data.log].reverse().map((entry, i) => (
                  <LogRow key={`${entry.start}-${i}`} entry={entry} />
                ))
              )}
            </div>
          )}
        </>
      )}
    </div>
  )
}
