import { useState } from 'react'
import {
  HeartPulse,
  CheckCircle,
  XCircle,
  Clock,
  MinusCircle,
  Loader,
  Pencil,
  Save,
  X,
} from 'lucide-react'
import { useContainerHealth, useSetHealthcheck } from '../../lib/api/health'
import { ApiError } from '../../lib/api'
import { useCan } from '../../lib/roles'
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

// ---- Number input helper ----

interface NumInputProps {
  label: string
  value: number
  min: number
  onChange: (v: number) => void
}

function NumInput({ label, value, min, onChange }: NumInputProps) {
  return (
    <div className="flex items-center gap-3">
      <span
        className="text-xs shrink-0 w-28"
        style={{ color: 'var(--text-muted)' }}
      >
        {label}
      </span>
      <input
        type="number"
        min={min}
        value={value}
        onChange={(e) => {
          const n = parseInt(e.target.value, 10)
          if (!isNaN(n) && n >= min) onChange(n)
        }}
        className="font-mono text-xs w-20 px-2 py-1"
        style={{
          background: 'var(--bg-elevated)',
          border: '1px solid var(--border-default)',
          borderRadius: '3px',
          color: 'var(--text-primary)',
          outline: 'none',
        }}
      />
    </div>
  )
}

// ---- Edit form ----

interface EditFormState {
  command: string
  interval: number
  timeout: number
  startPeriod: number
  retries: number
}

interface HealthEditFormProps {
  containerId: string
  initialState: EditFormState
  onCancel: () => void
}

function HealthEditForm({ containerId, initialState, onCancel }: HealthEditFormProps) {
  const [form, setForm] = useState<EditFormState>(initialState)
  const [confirming, setConfirming] = useState<'save' | 'disable' | null>(null)

  const { mutate, isPending, error, reset: resetMutation } = useSetHealthcheck(containerId)

  const apiErr = error as ApiError | null
  const errCode = apiErr?.body != null && typeof apiErr.body === 'object'
    ? (apiErr.body as Record<string, string>).error
    : null
  const inlineError =
    errCode === 'test_required' ? 'Command is required when not disabling.'
    : errCode === 'healthcheck_failed' ? 'Healthcheck apply failed — container could not be recreated.'
    : apiErr ? 'An unexpected error occurred.'
    : null

  function submit(disable: boolean) {
    resetMutation()
    const req = disable
      ? { disable: true as const }
      : {
          disable: false as const,
          test: ['CMD-SHELL', form.command.trim()],
          interval_sec: form.interval,
          timeout_sec: form.timeout,
          start_period_sec: form.startPeriod,
          retries: form.retries,
        }
    mutate(req, { onSuccess: () => onCancel() })
  }

  return (
    <div
      className="flex flex-col gap-3 px-3 py-3"
      style={{ borderTop: '1px solid var(--border-subtle)' }}
    >
      <span
        className="text-xs font-medium uppercase tracking-wider"
        style={{ color: 'var(--text-muted)' }}
      >
        Edit Healthcheck
      </span>

      {/* Command input */}
      <div className="flex items-center gap-3">
        <span
          className="text-xs shrink-0 w-28"
          style={{ color: 'var(--text-muted)' }}
        >
          Command
        </span>
        <input
          type="text"
          value={form.command}
          placeholder="curl -f http://localhost/health || exit 1"
          onChange={(e) => setForm((f) => ({ ...f, command: e.target.value }))}
          className="font-mono text-xs flex-1 px-2 py-1"
          style={{
            background: 'var(--bg-elevated)',
            border: '1px solid var(--border-default)',
            borderRadius: '3px',
            color: 'var(--text-primary)',
            outline: 'none',
          }}
        />
      </div>

      <NumInput
        label="Interval (s)"
        value={form.interval}
        min={1}
        onChange={(v) => setForm((f) => ({ ...f, interval: v }))}
      />
      <NumInput
        label="Timeout (s)"
        value={form.timeout}
        min={1}
        onChange={(v) => setForm((f) => ({ ...f, timeout: v }))}
      />
      <NumInput
        label="Start period (s)"
        value={form.startPeriod}
        min={0}
        onChange={(v) => setForm((f) => ({ ...f, startPeriod: v }))}
      />
      <NumInput
        label="Retries"
        value={form.retries}
        min={1}
        onChange={(v) => setForm((f) => ({ ...f, retries: v }))}
      />

      {/* Inline error */}
      {inlineError && !confirming && (
        <span className="font-mono text-xs" style={{ color: 'var(--status-error)' }}>
          {inlineError}
        </span>
      )}

      {/* Action row (pre-confirm) */}
      {!confirming && (
        <div className="flex items-center gap-2 mt-1">
          <button
            type="button"
            disabled={isPending}
            onClick={() => { resetMutation(); setConfirming('save') }}
            className="flex items-center gap-1.5 font-mono text-xs px-2.5 py-1"
            style={{
              background: 'var(--bg-elevated)',
              border: '1px solid var(--accent)',
              color: isPending ? 'var(--text-muted)' : 'var(--accent)',
              borderRadius: '3px',
              cursor: isPending ? 'not-allowed' : 'pointer',
              opacity: isPending ? 0.6 : 1,
            }}
          >
            <Save size={11} />
            Save
          </button>
          <button
            type="button"
            disabled={isPending}
            onClick={() => { resetMutation(); setConfirming('disable') }}
            className="flex items-center gap-1.5 font-mono text-xs px-2.5 py-1"
            style={{
              background: 'var(--bg-elevated)',
              border: '1px solid var(--status-error)',
              color: isPending ? 'var(--text-muted)' : 'var(--status-error)',
              borderRadius: '3px',
              cursor: isPending ? 'not-allowed' : 'pointer',
              opacity: isPending ? 0.6 : 1,
            }}
          >
            <X size={11} />
            Disable healthcheck
          </button>
          <button
            type="button"
            disabled={isPending}
            onClick={onCancel}
            className="font-mono text-xs px-2.5 py-1"
            style={{
              background: 'var(--bg-elevated)',
              border: '1px solid var(--border-default)',
              color: 'var(--text-secondary)',
              borderRadius: '3px',
              cursor: isPending ? 'not-allowed' : 'pointer',
            }}
          >
            Cancel
          </button>
        </div>
      )}

      {/* Confirm dialog (inline, matches SnapshotsPanel pattern) */}
      {confirming && (
        <div
          className="flex flex-col gap-2 p-3"
          style={{
            background: 'var(--bg-elevated)',
            border: '1px solid var(--status-warn)',
            borderRadius: '3px',
          }}
        >
          <p className="font-mono text-xs" style={{ color: 'var(--text-primary)' }}>
            {confirming === 'disable'
              ? 'Disable the healthcheck on this container?'
              : 'Apply healthcheck changes to this container?'}
          </p>
          <p className="text-xs" style={{ color: 'var(--text-muted)' }}>
            This will <strong>stop and recreate</strong> the container to apply the change.
            The operation may take several seconds.
          </p>
          {inlineError && (
            <span className="font-mono text-xs" style={{ color: 'var(--status-error)' }}>
              {inlineError}
            </span>
          )}
          <div className="flex items-center gap-2 mt-1">
            <button
              type="button"
              disabled={isPending}
              onClick={() => submit(confirming === 'disable')}
              className="flex items-center gap-1.5 font-mono text-xs px-2.5 py-1"
              style={{
                background: 'rgba(240,160,32,0.15)',
                border: '1px solid var(--status-warn)',
                color: isPending ? 'var(--text-muted)' : 'var(--status-warn)',
                borderRadius: '3px',
                cursor: isPending ? 'not-allowed' : 'pointer',
                opacity: isPending ? 0.6 : 1,
              }}
            >
              {isPending
                ? <Loader size={11} className="animate-spin" />
                : <Save size={11} />}
              {confirming === 'disable' ? 'Disable & recreate' : 'Apply & recreate'}
            </button>
            <button
              type="button"
              disabled={isPending}
              onClick={() => { resetMutation(); setConfirming(null) }}
              className="font-mono text-xs px-2.5 py-1"
              style={{
                background: 'var(--bg-elevated)',
                border: '1px solid var(--border-default)',
                color: 'var(--text-secondary)',
                borderRadius: '3px',
                cursor: isPending ? 'not-allowed' : 'pointer',
              }}
            >
              Cancel
            </button>
          </div>
        </div>
      )}
    </div>
  )
}

// ---- Main component ----

export function HealthCheck({ containerId }: HealthCheckProps) {
  const { data, isLoading, isError, error } = useContainerHealth(containerId)
  const { isAdmin } = useCan()
  const [editing, setEditing] = useState(false)

  const is502 =
    isError &&
    typeof error === 'object' &&
    error !== null &&
    'status' in error &&
    (error as { status: number }).status === 502

  function buildInitialState(): EditFormState {
    if (!data || !data.configured) {
      return { command: '', interval: 30, timeout: 10, startPeriod: 0, retries: 3 }
    }
    return {
      command: formatTestCommand(data.test),
      interval: data.interval_sec,
      timeout: data.timeout_sec,
      startPeriod: data.start_period_sec,
      retries: data.retries,
    }
  }

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
        className="px-3 py-2 flex items-center justify-between gap-2"
        style={{ borderBottom: '1px solid var(--border-subtle)' }}
      >
        <div className="flex items-center gap-2">
          <HeartPulse size={12} style={{ color: 'var(--text-muted)' }} />
          <p
            className="text-xs font-medium uppercase tracking-wider"
            style={{ color: 'var(--text-muted)' }}
          >
            Health Check
          </p>
        </div>
        {isAdmin && data && !isLoading && !editing && (
          <button
            type="button"
            onClick={() => setEditing(true)}
            className="flex items-center gap-1.5 font-mono text-xs px-2 py-0.5"
            style={{
              background: 'var(--bg-elevated)',
              border: '1px solid var(--border-default)',
              color: 'var(--text-secondary)',
              borderRadius: '3px',
              cursor: 'pointer',
            }}
          >
            <Pencil size={11} />
            {data.configured ? 'Edit' : 'Add'}
          </button>
        )}
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
          {data.configured && !editing && (
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

          {/* Edit form (admin only) */}
          {editing && (
            <HealthEditForm
              containerId={containerId}
              initialState={buildInitialState()}
              onCancel={() => setEditing(false)}
            />
          )}

          {/* Log results */}
          {data.configured && !editing && (
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
