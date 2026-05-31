import { Bot, Play, Loader, CheckCircle, XCircle, SkipForward, AlertTriangle } from 'lucide-react'
import { AppShell } from '../components/layout/AppShell'
import { useCan } from '../lib/roles'
import {
  useAutomations,
  useUpdateAutomation,
  useRunAutomation,
} from '../lib/api/automations'
import type { AutomationView, AutomationCategory, AutomationStatus } from '../lib/api/automations'

// ---- Interval options ----

interface IntervalOption {
  label: string
  seconds: number
}

const INTERVAL_OPTIONS: IntervalOption[] = [
  { label: '5 min', seconds: 300 },
  { label: '10 min', seconds: 600 },
  { label: '15 min', seconds: 900 },
  { label: '30 min', seconds: 1800 },
  { label: '1 hour', seconds: 3600 },
  { label: '6 hours', seconds: 21600 },
  { label: '24 hours', seconds: 86400 },
  { label: '7 days', seconds: 604800 },
]

// humanizeInterval renders a raw second count as a readable duration
// (e.g. 300 -> "5 min", 3600 -> "1 hour", 604800 -> "7 days") so the interval
// selector never shows bare seconds.
function humanizeInterval(seconds: number): string {
  const named = INTERVAL_OPTIONS.find((o) => o.seconds === seconds)
  if (named) return named.label
  const units: [number, string][] = [
    [86400, 'day'],
    [3600, 'hour'],
    [60, 'min'],
  ]
  for (const [size, unit] of units) {
    if (seconds >= size && seconds % size === 0) {
      const n = seconds / size
      return `${n} ${unit}${unit !== 'min' && n !== 1 ? 's' : ''}`
    }
  }
  return `${seconds}s`
}

// ---- Destructive automation keys ----

const DESTRUCTIVE_KEYS = new Set([
  'auto_update_containers',
  'prune_unused_volumes',
  'fix_bind_mount_perms',
  'patch_critical_cves',
  'prune_disk_pressure',
])

// ---- Category display helpers ----

const CATEGORY_LABELS: Record<AutomationCategory, string> = {
  self_heal: 'Self-heal',
  update: 'Update',
  security: 'Security',
  maintenance: 'Maintenance',
}

const CATEGORY_ORDER: AutomationCategory[] = ['self_heal', 'update', 'security', 'maintenance']

// ---- Status badge ----

interface StatusBadgeProps {
  status: AutomationStatus
  lastRun: string | null
  lastDetail: string
}

function relativeTime(iso: string): string {
  const ms = Date.now() - new Date(iso).getTime()
  const s = Math.floor(ms / 1000)
  if (s < 60) return `${s}s ago`
  const m = Math.floor(s / 60)
  if (m < 60) return `${m}m ago`
  const h = Math.floor(m / 60)
  if (h < 24) return `${h}h ago`
  return `${Math.floor(h / 24)}d ago`
}

function StatusBadge({ status, lastRun, lastDetail }: StatusBadgeProps) {
  if (!status) {
    return (
      <span className="text-xs font-mono" style={{ color: 'var(--text-muted)' }}>
        Never run
      </span>
    )
  }

  const configs: Record<
    Exclude<AutomationStatus, ''>,
    { icon: React.ReactNode; color: string; bg: string; border: string; label: string }
  > = {
    ok: {
      icon: <CheckCircle size={10} />,
      color: 'var(--status-ok, #40c878)',
      bg: 'rgba(64,200,120,0.1)',
      border: 'rgba(64,200,120,0.3)',
      label: 'OK',
    },
    error: {
      icon: <XCircle size={10} />,
      color: 'var(--status-error, #f05050)',
      bg: 'rgba(240,80,80,0.1)',
      border: 'rgba(240,80,80,0.3)',
      label: 'Error',
    },
    running: {
      icon: <Loader size={10} className="animate-spin" />,
      color: 'var(--accent)',
      bg: 'rgba(99,102,241,0.1)',
      border: 'rgba(99,102,241,0.3)',
      label: 'Running',
    },
    skipped: {
      icon: <SkipForward size={10} />,
      color: 'var(--text-muted)',
      bg: 'transparent',
      border: 'var(--border-subtle)',
      label: 'Skipped',
    },
  }

  const cfg = configs[status]

  return (
    <div className="flex flex-col gap-1">
      <div className="flex items-center gap-2">
        <span
          className="flex items-center gap-1 font-mono text-xs px-1.5 py-0.5 uppercase tracking-wider"
          style={{
            color: cfg.color,
            background: cfg.bg,
            border: `1px solid ${cfg.border}`,
            borderRadius: '3px',
            fontSize: '11px',
            whiteSpace: 'nowrap',
          }}
        >
          {cfg.icon}
          {cfg.label}
        </span>
        {lastRun && (
          <span className="text-xs" style={{ color: 'var(--text-muted)' }}>
            {relativeTime(lastRun)}
          </span>
        )}
      </div>
      {lastDetail && (
        <span
          className="text-xs font-mono"
          style={{ color: 'var(--text-muted)', fontSize: '11px' }}
        >
          {lastDetail}
        </span>
      )}
    </div>
  )
}

// ---- Toggle ----

interface ToggleProps {
  checked: boolean
  disabled: boolean
  onChange: (next: boolean) => void
}

function Toggle({ checked, disabled, onChange }: ToggleProps) {
  return (
    <button
      type="button"
      role="switch"
      aria-checked={checked}
      disabled={disabled}
      onClick={() => onChange(!checked)}
      style={{
        width: '32px',
        height: '18px',
        borderRadius: '9px',
        border: 'none',
        cursor: disabled ? 'not-allowed' : 'pointer',
        background: checked ? 'var(--accent)' : 'var(--border-default)',
        position: 'relative',
        transition: 'background 0.15s',
        flexShrink: 0,
        opacity: disabled ? 0.5 : 1,
        padding: 0,
      }}
    >
      <span
        style={{
          display: 'block',
          width: '12px',
          height: '12px',
          borderRadius: '50%',
          background: '#fff',
          position: 'absolute',
          top: '3px',
          left: checked ? '17px' : '3px',
          transition: 'left 0.15s',
        }}
      />
    </button>
  )
}

// ---- Automation card ----

interface AutomationCardProps {
  automation: AutomationView
  isAdmin: boolean
  isOperator: boolean
}

function AutomationCard({ automation, isAdmin, isOperator }: AutomationCardProps) {
  const { mutate: update, isPending: isUpdating } = useUpdateAutomation()
  const { mutate: run, isPending: isRunning } = useRunAutomation()

  const isDestructive = DESTRUCTIVE_KEYS.has(automation.key)

  const handleToggle = (enabled: boolean) => {
    update({ key: automation.key, body: { enabled } })
  }

  const handleIntervalChange = (e: React.ChangeEvent<HTMLSelectElement>) => {
    update({ key: automation.key, body: { interval_seconds: Number(e.target.value) } })
  }

  const handleRun = () => {
    run(automation.key)
  }

  return (
    <div
      style={{
        backgroundColor: 'var(--bg-surface)',
        border: `1px solid ${isDestructive ? 'rgba(240,160,32,0.25)' : 'var(--border-subtle)'}`,
        borderRadius: '3px',
        padding: '14px 16px',
        display: 'flex',
        flexDirection: 'column',
        gap: '10px',
      }}
    >
      {/* Header row */}
      <div className="flex items-start justify-between gap-3">
        <div className="flex flex-col gap-1 min-w-0">
          <div className="flex items-center gap-2">
            <span
              className="text-sm font-medium"
              style={{ color: 'var(--text-primary)' }}
            >
              {automation.label}
            </span>
            {isDestructive && (
              <span
                className="flex items-center gap-1 font-mono text-xs px-1.5 py-0.5 uppercase tracking-wider"
                title="This automation is destructive and disabled by default"
                style={{
                  color: 'var(--status-warn)',
                  background: 'rgba(240,160,32,0.1)',
                  border: '1px solid rgba(240,160,32,0.3)',
                  borderRadius: '3px',
                  fontSize: '10px',
                  whiteSpace: 'nowrap',
                  flexShrink: 0,
                }}
              >
                <AlertTriangle size={9} />
                Destructive
              </span>
            )}
          </div>
          <span className="text-xs" style={{ color: 'var(--text-muted)', lineHeight: '1.5' }}>
            {automation.description}
          </span>
        </div>

        {/* Toggle (admin only) */}
        <div style={{ flexShrink: 0, marginTop: '2px' }}>
          <Toggle
            checked={automation.enabled}
            disabled={!isAdmin || isUpdating}
            onChange={handleToggle}
          />
        </div>
      </div>

      {/* Controls row */}
      <div className="flex items-center justify-between gap-3 flex-wrap">
        <div className="flex items-center gap-3">
          {/* Interval selector (admin only) */}
          {isAdmin && (
            <div className="flex items-center gap-2">
              <span className="text-xs" style={{ color: 'var(--text-muted)', whiteSpace: 'nowrap' }}>
                Every
              </span>
              <select
                value={automation.interval_seconds}
                onChange={handleIntervalChange}
                disabled={isUpdating}
                className="font-mono text-xs px-2 py-1"
                style={{
                  background: 'var(--bg-elevated)',
                  border: '1px solid var(--border-default)',
                  color: 'var(--text-secondary)',
                  borderRadius: '3px',
                  cursor: isUpdating ? 'not-allowed' : 'pointer',
                  opacity: isUpdating ? 0.6 : 1,
                }}
              >
                {INTERVAL_OPTIONS.map((opt) => (
                  <option key={opt.seconds} value={opt.seconds}>
                    {opt.label}
                  </option>
                ))}
                {/* Keep any custom value that doesn't match options */}
                {!INTERVAL_OPTIONS.some((o) => o.seconds === automation.interval_seconds) && (
                  <option value={automation.interval_seconds}>
                    {humanizeInterval(automation.interval_seconds)}
                  </option>
                )}
              </select>
            </div>
          )}
        </div>

        {/* Run now button (operator) */}
        {isOperator && (
          <button
            type="button"
            onClick={handleRun}
            disabled={isRunning || automation.last_status === 'running'}
            className="flex items-center gap-1.5 font-mono text-xs px-2.5 py-1"
            style={{
              background: 'var(--bg-elevated)',
              border: '1px solid var(--border-default)',
              color:
                isRunning || automation.last_status === 'running'
                  ? 'var(--text-muted)'
                  : 'var(--accent)',
              borderRadius: '3px',
              cursor:
                isRunning || automation.last_status === 'running' ? 'not-allowed' : 'pointer',
              opacity: isRunning || automation.last_status === 'running' ? 0.6 : 1,
              whiteSpace: 'nowrap',
              flexShrink: 0,
            }}
          >
            {isRunning ? (
              <Loader size={11} className="animate-spin" />
            ) : (
              <Play size={11} />
            )}
            Run now
          </button>
        )}
      </div>

      {/* Status row */}
      <StatusBadge
        status={automation.last_status}
        lastRun={automation.last_run}
        lastDetail={automation.last_detail}
      />
    </div>
  )
}

// ---- Category section ----

interface CategorySectionProps {
  category: AutomationCategory
  automations: AutomationView[]
  isAdmin: boolean
  isOperator: boolean
}

function CategorySection({ category, automations, isAdmin, isOperator }: CategorySectionProps) {
  if (automations.length === 0) return null

  return (
    <div className="flex flex-col gap-3">
      <h2
        className="text-xs font-medium uppercase tracking-wider"
        style={{ color: 'var(--text-muted)' }}
      >
        {CATEGORY_LABELS[category]}
      </h2>
      <div className="flex flex-col gap-3">
        {automations.map((a) => (
          <AutomationCard
            key={a.key}
            automation={a}
            isAdmin={isAdmin}
            isOperator={isOperator}
          />
        ))}
      </div>
    </div>
  )
}

// ---- Main page ----

export default function Automations() {
  const { isAdmin, isOperator } = useCan()
  const { data, isLoading } = useAutomations()

  const automations = data?.automations ?? []

  // Group by category in stable display order
  const grouped = CATEGORY_ORDER.reduce<Record<AutomationCategory, AutomationView[]>>(
    (acc, cat) => {
      acc[cat] = automations.filter((a) => a.category === cat)
      return acc
    },
    { self_heal: [], update: [], security: [], maintenance: [] },
  )

  return (
    <AppShell>
      <div
        className="flex flex-col flex-1 min-h-0 h-full w-full max-w-full p-6 pb-20"
        style={{ maxWidth: '900px', margin: '0 auto' }}
      >
        {/* Page header */}
        <div className="flex items-center gap-2 mb-6">
          <Bot size={16} style={{ color: 'var(--text-secondary)' }} />
          <h1
            className="text-sm font-medium uppercase tracking-wider"
            style={{ color: 'var(--text-primary)' }}
          >
            Automations
          </h1>
        </div>

        {/* Loading */}
        {isLoading && (
          <div className="flex items-center gap-2 py-8">
            <Loader size={13} className="animate-spin" style={{ color: 'var(--accent)' }} />
            <span className="text-xs" style={{ color: 'var(--text-muted)' }}>
              Loading automations…
            </span>
          </div>
        )}

        {/* Content */}
        {!isLoading && automations.length === 0 && (
          <div
            className="px-3 py-4 text-xs"
            style={{
              color: 'var(--text-muted)',
              backgroundColor: 'var(--bg-surface)',
              border: '1px solid var(--border-subtle)',
              borderRadius: '3px',
            }}
          >
            No automations configured.
          </div>
        )}

        {!isLoading && automations.length > 0 && (
          <div className="flex flex-col gap-8">
            {CATEGORY_ORDER.map((cat) => (
              <CategorySection
                key={cat}
                category={cat}
                automations={grouped[cat]}
                isAdmin={isAdmin}
                isOperator={isOperator}
              />
            ))}
          </div>
        )}

        {/* Admin role hint */}
        {!isLoading && !isAdmin && automations.length > 0 && (
          <div
            className="flex items-start gap-2 mt-6 px-3 py-2.5 text-xs"
            style={{
              backgroundColor: 'rgba(74,82,104,0.12)',
              border: '1px solid var(--border-subtle)',
              borderRadius: '3px',
              color: 'var(--text-muted)',
              lineHeight: '1.6',
            }}
          >
            <Bot size={12} style={{ flexShrink: 0, marginTop: '1px' }} />
            <span>
              Toggle and interval controls are admin-only. As an operator you can trigger any automation manually with Run now.
            </span>
          </div>
        )}
      </div>
    </AppShell>
  )
}
