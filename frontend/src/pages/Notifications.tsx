import { useState } from 'react'
import {
  Bell,
  Plus,
  Pencil,
  Trash2,
  Send,
  Loader,
  Check,
  X,
  Shield,
  ChevronDown,
  ChevronUp,
  Clock,
  AlertTriangle,
} from 'lucide-react'
import { AppShell } from '../components/layout/AppShell'
import { useMe } from '../hooks/useMe'
import { ApiError } from '../lib/api'
import {
  useWebhooks,
  useCreateWebhook,
  useUpdateWebhook,
  useDeleteWebhook,
  useTestWebhook,
} from '../lib/api/webhooks'
import type { Webhook, WebhookProvider, WebhookRequest, TriggerDef } from '../types/api'
import {
  useAlertPolicies,
  useAlertDeliveries,
  useCreateAlertPolicy,
  useUpdateAlertPolicy,
  useDeleteAlertPolicy,
} from '../lib/api/alertpolicy'
import type {
  AlertPolicy,
  AlertPolicyRequest,
  AlertSeverity,
  QuietHours,
  PolicyEscalation,
  PolicyMatch,
  DeliveryStatus,
} from '../lib/api/alertpolicy'

// ---- Trigger helpers ----

/** Build a label/description lookup from TriggerDef array (server-driven). */
function buildTriggerIndex(defs: TriggerDef[]): Map<string, TriggerDef> {
  const m = new Map<string, TriggerDef>()
  for (const d of defs) m.set(d.key, d)
  return m
}

function triggerLabel(key: string, index: Map<string, TriggerDef>): string {
  return index.get(key)?.label ?? key
}

// ---- Provider badge ----

function ProviderBadge({ provider }: { provider: WebhookProvider }) {
  const color = provider === 'slack' ? '#4A154B' : '#5865F2'
  const bg = provider === 'slack' ? 'rgba(74,21,75,0.18)' : 'rgba(88,101,242,0.18)'
  return (
    <span
      className="font-mono text-xs px-1.5 py-0.5 uppercase tracking-wider shrink-0"
      style={{
        background: bg,
        border: `1px solid ${color}`,
        color,
        borderRadius: '3px',
        fontSize: '12px',
      }}
    >
      {provider}
    </span>
  )
}

// ---- Trigger chip ----

function TriggerChip({ label }: { label: string }) {
  return (
    <span
      className="font-mono text-xs px-1.5 py-0.5"
      style={{
        background: 'var(--accent-dim)',
        border: '1px solid var(--accent-glow)',
        color: 'var(--accent)',
        borderRadius: '3px',
        fontSize: '12px',
      }}
    >
      {label}
    </span>
  )
}

// ---- Masked URL helper ----

function maskedUrl(url: string): string {
  try {
    const u = new URL(url)
    return `${u.protocol}//${u.host}/…`
  } catch {
    return url.length > 30 ? url.slice(0, 30) + '…' : url
  }
}

// ---- Toggle switch ----

interface ToggleProps {
  checked: boolean
  onChange: (v: boolean) => void
  disabled?: boolean
}

function Toggle({ checked, onChange, disabled }: ToggleProps) {
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
        flexShrink: 0,
        transition: 'background 0.15s',
        opacity: disabled ? 0.5 : 1,
      }}
    >
      <span
        style={{
          position: 'absolute',
          top: '2px',
          left: checked ? '16px' : '2px',
          width: '14px',
          height: '14px',
          borderRadius: '50%',
          background: 'var(--text-primary)',
          transition: 'left 0.15s',
        }}
      />
    </button>
  )
}

// ---- Delete confirm dialog ----

interface DeleteDialogProps {
  name: string
  onConfirm: () => void
  onCancel: () => void
}

function DeleteDialog({ name, onConfirm, onCancel }: DeleteDialogProps) {
  return (
    <div
      style={{
        position: 'fixed',
        inset: 0,
        background: 'rgba(0,0,0,0.55)',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        zIndex: 50,
      }}
    >
      <div
        style={{
          background: 'var(--bg-elevated)',
          border: '1px solid var(--border-default)',
          borderRadius: '3px',
          padding: '24px',
          width: '360px',
          maxWidth: '90vw',
        }}
      >
        <p className="text-sm mb-1" style={{ color: 'var(--text-primary)' }}>
          Delete webhook
        </p>
        <p className="text-xs mb-5" style={{ color: 'var(--text-muted)' }}>
          Remove <strong style={{ color: 'var(--text-secondary)' }}>{name}</strong>? This cannot be undone.
        </p>
        <div className="flex gap-2 justify-end">
          <button
            type="button"
            onClick={onCancel}
            className="text-xs px-3 py-1.5"
            style={{
              background: 'var(--bg-surface)',
              border: '1px solid var(--border-default)',
              color: 'var(--text-secondary)',
              borderRadius: '3px',
              cursor: 'pointer',
            }}
          >
            Cancel
          </button>
          <button
            type="button"
            onClick={onConfirm}
            className="text-xs px-3 py-1.5"
            style={{
              background: 'rgba(232,64,64,0.15)',
              border: '1px solid var(--status-error)',
              color: 'var(--status-error)',
              borderRadius: '3px',
              cursor: 'pointer',
            }}
          >
            Delete
          </button>
        </div>
      </div>
    </div>
  )
}

// ---- Alert policy helpers ----

const SEVERITY_ORDER: AlertSeverity[] = ['info', 'warning', 'critical']

function severityColor(s: AlertSeverity): string {
  if (s === 'critical') return 'var(--status-error)'
  if (s === 'warning') return '#f5a623'
  return 'var(--text-muted)'
}

function severityBg(s: AlertSeverity): string {
  if (s === 'critical') return 'rgba(232,64,64,0.15)'
  if (s === 'warning') return 'rgba(245,166,35,0.15)'
  return 'var(--bg-surface)'
}

function SeverityBadge({ severity }: { severity: AlertSeverity }) {
  return (
    <span
      className="font-mono text-xs px-1.5 py-0.5 uppercase tracking-wider shrink-0"
      style={{
        background: severityBg(severity),
        border: `1px solid ${severityColor(severity)}`,
        color: severityColor(severity),
        borderRadius: '3px',
        fontSize: '11px',
      }}
    >
      {severity}
    </span>
  )
}

function DeliveryStatusBadge({ status }: { status: DeliveryStatus }) {
  const label = status === 'delivered'
    ? 'delivered'
    : status === 'suppressed_dedup'
    ? 'dedup'
    : 'quiet'
  const color = status === 'delivered' ? 'var(--status-ok, #40c878)' : 'var(--text-muted)'
  const bg = status === 'delivered' ? 'rgba(64,200,120,0.12)' : 'var(--bg-surface)'
  const border = status === 'delivered' ? '1px solid rgba(64,200,120,0.4)' : '1px solid var(--border-subtle)'
  return (
    <span
      className="font-mono text-xs px-1.5 py-0.5"
      style={{ background: bg, border, color, borderRadius: '3px', fontSize: '10px' }}
    >
      {label}
    </span>
  )
}

function minutesToTime(min: number): string {
  const h = Math.floor(min / 60) % 24
  const m = min % 60
  return `${String(h).padStart(2, '0')}:${String(m).padStart(2, '0')}`
}

function timeToMinutes(t: string): number {
  const [h, m] = t.split(':').map(Number)
  return (h ?? 0) * 60 + (m ?? 0)
}

// ---- Alert policy form ----

interface PolicyFormState {
  name: string
  enabled: boolean
  min_severity: AlertSeverity
  channels: string[]
  match_sources: string
  match_key_glob: string
  dedup_window_sec: number
  quiet_enabled: boolean
  quiet_start: string
  quiet_end: string
  quiet_tz: string
  quiet_allow_critical: boolean
  escalate_enabled: boolean
  escalate_after_sec: number
  escalate_channels: string[]
}

const EMPTY_POLICY_FORM: PolicyFormState = {
  name: '',
  enabled: true,
  min_severity: 'warning',
  channels: [],
  match_sources: '',
  match_key_glob: '',
  dedup_window_sec: 300,
  quiet_enabled: false,
  quiet_start: '22:00',
  quiet_end: '08:00',
  quiet_tz: 'UTC',
  quiet_allow_critical: true,
  escalate_enabled: false,
  escalate_after_sec: 900,
  escalate_channels: [],
}

function policyToForm(p: AlertPolicy): PolicyFormState {
  return {
    name: p.name,
    enabled: p.enabled,
    min_severity: p.min_severity,
    channels: p.channels,
    match_sources: p.match.sources.join(', '),
    match_key_glob: p.match.key_glob,
    dedup_window_sec: p.dedup_window_sec,
    quiet_enabled: p.quiet_hours !== null,
    quiet_start: p.quiet_hours ? minutesToTime(p.quiet_hours.start_min) : '22:00',
    quiet_end: p.quiet_hours ? minutesToTime(p.quiet_hours.end_min) : '08:00',
    quiet_tz: p.quiet_hours?.tz ?? 'UTC',
    quiet_allow_critical: p.quiet_hours?.allow_critical ?? true,
    escalate_enabled: p.escalate !== null,
    escalate_after_sec: p.escalate?.after_sec ?? 900,
    escalate_channels: p.escalate?.channels ?? [],
  }
}

function formToRequest(f: PolicyFormState): AlertPolicyRequest {
  const sources = f.match_sources
    .split(',')
    .map((s) => s.trim())
    .filter(Boolean)

  const quiet_hours: QuietHours | null = f.quiet_enabled
    ? {
        start_min: timeToMinutes(f.quiet_start),
        end_min: timeToMinutes(f.quiet_end),
        tz: f.quiet_tz || 'UTC',
        allow_critical: f.quiet_allow_critical,
      }
    : null

  const escalate: PolicyEscalation | null = f.escalate_enabled
    ? { after_sec: f.escalate_after_sec, channels: f.escalate_channels }
    : null

  const match: PolicyMatch = {
    sources,
    key_glob: f.match_key_glob,
  }

  return {
    name: f.name,
    enabled: f.enabled,
    min_severity: f.min_severity,
    channels: f.channels,
    match,
    quiet_hours,
    dedup_window_sec: f.dedup_window_sec,
    escalate,
  }
}

interface PolicyFormProps {
  initial: PolicyFormState
  webhooks: Webhook[]
  onSubmit: (req: AlertPolicyRequest) => void
  onClose: () => void
  isPending: boolean
  serverError?: string | null
  title: string
}

function PolicyForm({
  initial,
  webhooks,
  onSubmit,
  onClose,
  isPending,
  serverError,
  title,
}: PolicyFormProps) {
  const [form, setForm] = useState<PolicyFormState>(initial)
  const [clientError, setClientError] = useState<string | null>(null)

  function set<K extends keyof PolicyFormState>(key: K, val: PolicyFormState[K]) {
    setForm((p) => ({ ...p, [key]: val }))
  }

  function toggleChannel(id: string) {
    setForm((p) => ({
      ...p,
      channels: p.channels.includes(id)
        ? p.channels.filter((c) => c !== id)
        : [...p.channels, id],
    }))
  }

  function toggleEscalateChannel(id: string) {
    setForm((p) => ({
      ...p,
      escalate_channels: p.escalate_channels.includes(id)
        ? p.escalate_channels.filter((c) => c !== id)
        : [...p.escalate_channels, id],
    }))
  }

  function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    if (!form.name.trim()) {
      setClientError('Name is required.')
      return
    }
    if (form.channels.length === 0) {
      setClientError('Select at least one channel.')
      return
    }
    setClientError(null)
    onSubmit(formToRequest(form))
  }

  const inputStyle: React.CSSProperties = {
    background: 'var(--bg-surface)',
    border: '1px solid var(--border-default)',
    borderRadius: '3px',
    color: 'var(--text-primary)',
    fontSize: '12px',
    padding: '5px 8px',
    outline: 'none',
    width: '100%',
  }

  const labelStyle: React.CSSProperties = {
    fontSize: '10px',
    color: 'var(--text-muted)',
    textTransform: 'uppercase',
    letterSpacing: '0.06em',
    fontFamily: 'monospace',
    marginBottom: '4px',
    display: 'block',
  }

  const sectionStyle: React.CSSProperties = {
    background: 'var(--bg-surface)',
    border: '1px solid var(--border-subtle)',
    borderRadius: '3px',
    padding: '12px',
    display: 'flex',
    flexDirection: 'column',
    gap: '10px',
  }

  const errorMsg = clientError ?? serverError ?? null

  return (
    <div
      style={{
        position: 'fixed',
        inset: 0,
        background: 'rgba(0,0,0,0.55)',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        zIndex: 50,
      }}
    >
      <div
        style={{
          background: 'var(--bg-elevated)',
          border: '1px solid var(--border-default)',
          borderRadius: '3px',
          padding: '24px',
          width: '520px',
          maxWidth: '96vw',
          maxHeight: '92vh',
          overflowY: 'auto',
          display: 'flex',
          flexDirection: 'column',
          gap: '0',
        }}
      >
        <div className="flex items-center justify-between mb-4">
          <span className="text-sm font-medium" style={{ color: 'var(--text-primary)' }}>
            {title}
          </span>
          <button
            type="button"
            onClick={onClose}
            style={{ background: 'none', border: 'none', cursor: 'pointer', color: 'var(--text-muted)' }}
          >
            <X size={14} />
          </button>
        </div>

        <form onSubmit={handleSubmit} className="flex flex-col gap-4">
          {/* Name + enabled */}
          <div className="flex gap-3 items-end">
            <div className="flex flex-col gap-1 flex-1">
              <span style={labelStyle}>Name</span>
              <input
                type="text"
                value={form.name}
                onChange={(e) => set('name', e.target.value)}
                placeholder="e.g. critical-ops"
                style={inputStyle}
              />
            </div>
            <label className="flex items-center gap-2 cursor-pointer pb-1">
              <Toggle checked={form.enabled} onChange={(v) => set('enabled', v)} />
              <span className="text-xs" style={{ color: 'var(--text-secondary)' }}>Enabled</span>
            </label>
          </div>

          {/* Min severity */}
          <div className="flex flex-col gap-1">
            <span style={labelStyle}>Minimum severity</span>
            <div className="flex gap-1.5">
              {SEVERITY_ORDER.map((s) => (
                <button
                  key={s}
                  type="button"
                  onClick={() => set('min_severity', s)}
                  style={{
                    background: form.min_severity === s ? severityBg(s) : 'var(--bg-surface)',
                    border: `1px solid ${form.min_severity === s ? severityColor(s) : 'var(--border-default)'}`,
                    color: form.min_severity === s ? severityColor(s) : 'var(--text-muted)',
                    borderRadius: '3px',
                    padding: '3px 10px',
                    fontSize: '11px',
                    fontFamily: 'monospace',
                    textTransform: 'uppercase',
                    letterSpacing: '0.05em',
                    cursor: 'pointer',
                  }}
                >
                  {s}
                </button>
              ))}
            </div>
            <span className="text-xs" style={{ color: 'var(--text-muted)', fontSize: '10px' }}>
              Alerts below this severity are ignored by this policy.
            </span>
          </div>

          {/* Channels */}
          <div className="flex flex-col gap-1">
            <span style={labelStyle}>Channels (webhooks)</span>
            {webhooks.length === 0 ? (
              <span className="text-xs font-mono" style={{ color: 'var(--text-muted)' }}>
                No webhooks configured — add one above first.
              </span>
            ) : (
              <div className="flex flex-col gap-1">
                {webhooks.map((wh) => (
                  <label key={wh.id} className="flex items-center gap-2 cursor-pointer">
                    <input
                      type="checkbox"
                      checked={form.channels.includes(wh.id)}
                      onChange={() => toggleChannel(wh.id)}
                      style={{ accentColor: 'var(--accent)', cursor: 'pointer' }}
                    />
                    <span className="text-xs font-mono" style={{ color: 'var(--text-secondary)' }}>
                      {wh.name}
                    </span>
                    <ProviderBadge provider={wh.provider} />
                  </label>
                ))}
              </div>
            )}
          </div>

          {/* Match filter */}
          <div style={sectionStyle}>
            <span style={{ ...labelStyle, marginBottom: 0, color: 'var(--text-secondary)' }}>
              Match filter (optional)
            </span>
            <div className="flex flex-col gap-1">
              <span style={labelStyle}>Sources (comma-separated, empty = all)</span>
              <input
                type="text"
                value={form.match_sources}
                onChange={(e) => set('match_sources', e.target.value)}
                placeholder="container.crash, cve.critical"
                style={inputStyle}
              />
            </div>
            <div className="flex flex-col gap-1">
              <span style={labelStyle}>Key glob (empty = all)</span>
              <input
                type="text"
                value={form.match_key_glob}
                onChange={(e) => set('match_key_glob', e.target.value)}
                placeholder="container.*"
                style={inputStyle}
              />
            </div>
          </div>

          {/* Dedup window */}
          <div className="flex flex-col gap-1">
            <span style={labelStyle}>Dedup window</span>
            <div className="flex items-center gap-2">
              <input
                type="number"
                min={0}
                value={form.dedup_window_sec}
                onChange={(e) => set('dedup_window_sec', Number(e.target.value))}
                style={{ ...inputStyle, width: '80px' }}
              />
              <span className="text-xs" style={{ color: 'var(--text-muted)' }}>seconds</span>
              <span className="text-xs font-mono" style={{ color: 'var(--text-muted)' }}>
                ({Math.round(form.dedup_window_sec / 60)} min)
              </span>
            </div>
            <span style={{ fontSize: '10px', color: 'var(--text-muted)' }}>
              Identical alerts within this window are suppressed (shown as &ldquo;dedup&rdquo; in deliveries).
            </span>
          </div>

          {/* Quiet hours */}
          <div style={sectionStyle}>
            <label className="flex items-center gap-2 cursor-pointer">
              <input
                type="checkbox"
                checked={form.quiet_enabled}
                onChange={(e) => set('quiet_enabled', e.target.checked)}
                style={{ accentColor: 'var(--accent)', cursor: 'pointer' }}
              />
              <span className="text-xs font-medium" style={{ color: 'var(--text-secondary)' }}>
                Quiet hours
              </span>
            </label>
            {form.quiet_enabled && (
              <div className="flex flex-col gap-2 pl-5">
                <div className="flex gap-3 flex-wrap">
                  <div className="flex flex-col gap-1">
                    <span style={labelStyle}>Start</span>
                    <input
                      type="time"
                      value={form.quiet_start}
                      onChange={(e) => set('quiet_start', e.target.value)}
                      style={{ ...inputStyle, width: 'auto' }}
                    />
                  </div>
                  <div className="flex flex-col gap-1">
                    <span style={labelStyle}>End</span>
                    <input
                      type="time"
                      value={form.quiet_end}
                      onChange={(e) => set('quiet_end', e.target.value)}
                      style={{ ...inputStyle, width: 'auto' }}
                    />
                  </div>
                  <div className="flex flex-col gap-1 flex-1 min-w-0">
                    <span style={labelStyle}>Timezone</span>
                    <input
                      type="text"
                      value={form.quiet_tz}
                      onChange={(e) => set('quiet_tz', e.target.value)}
                      placeholder="UTC"
                      style={inputStyle}
                    />
                  </div>
                </div>
                <label className="flex items-center gap-2 cursor-pointer">
                  <input
                    type="checkbox"
                    checked={form.quiet_allow_critical}
                    onChange={(e) => set('quiet_allow_critical', e.target.checked)}
                    style={{ accentColor: 'var(--accent)', cursor: 'pointer' }}
                  />
                  <span className="text-xs" style={{ color: 'var(--text-secondary)' }}>
                    Always deliver <strong>critical</strong> alerts even during quiet hours
                  </span>
                </label>
              </div>
            )}
          </div>

          {/* Escalation */}
          <div style={sectionStyle}>
            <label className="flex items-center gap-2 cursor-pointer">
              <input
                type="checkbox"
                checked={form.escalate_enabled}
                onChange={(e) => set('escalate_enabled', e.target.checked)}
                style={{ accentColor: 'var(--accent)', cursor: 'pointer' }}
              />
              <span className="text-xs font-medium" style={{ color: 'var(--text-secondary)' }}>
                Escalation
              </span>
            </label>
            {form.escalate_enabled && (
              <div className="flex flex-col gap-2 pl-5">
                <div className="flex items-center gap-2">
                  <span style={labelStyle}>Escalate after</span>
                  <input
                    type="number"
                    min={60}
                    value={form.escalate_after_sec}
                    onChange={(e) => set('escalate_after_sec', Number(e.target.value))}
                    style={{ ...inputStyle, width: '80px' }}
                  />
                  <span className="text-xs" style={{ color: 'var(--text-muted)' }}>
                    seconds ({Math.round(form.escalate_after_sec / 60)} min)
                  </span>
                </div>
                <div className="flex flex-col gap-1">
                  <span style={labelStyle}>Escalation channels</span>
                  {webhooks.map((wh) => (
                    <label key={wh.id} className="flex items-center gap-2 cursor-pointer">
                      <input
                        type="checkbox"
                        checked={form.escalate_channels.includes(wh.id)}
                        onChange={() => toggleEscalateChannel(wh.id)}
                        style={{ accentColor: 'var(--accent)', cursor: 'pointer' }}
                      />
                      <span className="text-xs font-mono" style={{ color: 'var(--text-secondary)' }}>
                        {wh.name}
                      </span>
                    </label>
                  ))}
                </div>
              </div>
            )}
          </div>

          {errorMsg && (
            <p className="text-xs" style={{ color: 'var(--status-error)' }}>
              {errorMsg}
            </p>
          )}

          <div className="flex gap-2 justify-end pt-1">
            <button
              type="button"
              onClick={onClose}
              className="text-xs px-3 py-1.5"
              style={{
                background: 'var(--bg-surface)',
                border: '1px solid var(--border-default)',
                color: 'var(--text-secondary)',
                borderRadius: '3px',
                cursor: 'pointer',
              }}
            >
              Cancel
            </button>
            <button
              type="submit"
              disabled={isPending}
              className="text-xs px-3 py-1.5 flex items-center gap-1.5"
              style={{
                background: 'var(--accent-glow)',
                border: '1px solid var(--accent)',
                color: 'var(--accent)',
                borderRadius: '3px',
                cursor: isPending ? 'not-allowed' : 'pointer',
                opacity: isPending ? 0.7 : 1,
              }}
            >
              {isPending && <Loader size={11} className="animate-spin" />}
              Save
            </button>
          </div>
        </form>
      </div>
    </div>
  )
}

// ---- Policy row ----

interface PolicyRowProps {
  policy: AlertPolicy
  webhooks: Webhook[]
  onEdit: () => void
  onDelete: () => void
  onToggle: () => void
  isToggling: boolean
}

function PolicyRow({ policy, webhooks, onEdit, onDelete, onToggle, isToggling }: PolicyRowProps) {
  const webhookMap = new Map(webhooks.map((w) => [w.id, w]))

  return (
    <div
      style={{
        background: 'var(--bg-elevated)',
        border: '1px solid var(--border-subtle)',
        borderRadius: '3px',
        padding: '12px 16px',
        display: 'flex',
        flexDirection: 'column',
        gap: '6px',
      }}
    >
      <div className="flex items-center gap-3 flex-wrap">
        <Toggle checked={policy.enabled} onChange={onToggle} disabled={isToggling} />
        <span
          className="font-mono text-xs font-medium flex-1 min-w-0 truncate"
          style={{ color: 'var(--text-primary)' }}
        >
          {policy.name}
        </span>
        <SeverityBadge severity={policy.min_severity} />
        {policy.quiet_hours && (
          <span
            className="flex items-center gap-1 font-mono text-xs"
            style={{ color: 'var(--text-muted)', fontSize: '11px' }}
            title="Quiet hours active"
          >
            <Clock size={10} />
            {minutesToTime(policy.quiet_hours.start_min)}–{minutesToTime(policy.quiet_hours.end_min)}
          </span>
        )}
        {policy.escalate && (
          <span
            className="flex items-center gap-1 font-mono text-xs"
            style={{ color: '#f5a623', fontSize: '11px' }}
            title={`Escalates after ${policy.escalate.after_sec}s`}
          >
            <AlertTriangle size={10} />
            esc
          </span>
        )}
        <div className="flex items-center gap-1 shrink-0 ml-auto">
          <button
            type="button"
            onClick={onEdit}
            title="Edit"
            style={{ background: 'none', border: 'none', color: 'var(--text-muted)', cursor: 'pointer', padding: '4px' }}
          >
            <Pencil size={12} />
          </button>
          <button
            type="button"
            onClick={onDelete}
            title="Delete"
            style={{ background: 'none', border: 'none', color: 'var(--text-muted)', cursor: 'pointer', padding: '4px' }}
          >
            <Trash2 size={12} />
          </button>
        </div>
      </div>

      {policy.channels.length > 0 && (
        <div className="flex flex-wrap gap-1 pl-10">
          {policy.channels.map((id) => {
            const wh = webhookMap.get(id)
            return wh ? (
              <TriggerChip key={id} label={wh.name} />
            ) : (
              <span key={id} className="font-mono text-xs" style={{ color: 'var(--text-muted)' }}>{id}</span>
            )
          })}
        </div>
      )}
    </div>
  )
}

// ---- Deliveries list ----

function DeliveriesList({ webhooks }: { webhooks: Webhook[] }) {
  const [expanded, setExpanded] = useState(false)
  const { data, isLoading } = useAlertDeliveries(50)
  const webhookMap = new Map(webhooks.map((w) => [w.id, w]))

  const deliveries = data?.deliveries ?? []

  return (
    <div
      style={{
        border: '1px solid var(--border-subtle)',
        borderRadius: '3px',
        overflow: 'hidden',
      }}
    >
      <button
        type="button"
        onClick={() => setExpanded((v) => !v)}
        className="w-full flex items-center justify-between px-4 py-2.5"
        style={{
          background: 'var(--bg-elevated)',
          border: 'none',
          cursor: 'pointer',
          color: 'var(--text-secondary)',
        }}
      >
        <span className="text-xs font-mono flex items-center gap-2" style={{ color: 'var(--text-secondary)' }}>
          <Clock size={12} style={{ color: 'var(--accent)' }} />
          Recent deliveries
          {deliveries.length > 0 && (
            <span
              className="font-mono text-xs"
              style={{
                background: 'var(--accent-glow)',
                border: '1px solid var(--accent)',
                color: 'var(--accent)',
                borderRadius: '3px',
                padding: '1px 6px',
                fontSize: '10px',
              }}
            >
              {deliveries.length}
            </span>
          )}
        </span>
        {expanded ? <ChevronUp size={12} /> : <ChevronDown size={12} />}
      </button>

      {expanded && (
        <div style={{ background: 'var(--bg-surface)', borderTop: '1px solid var(--border-subtle)' }}>
          {isLoading && (
            <div className="flex items-center gap-2 p-4" style={{ color: 'var(--text-muted)' }}>
              <Loader size={11} className="animate-spin" />
              <span className="text-xs font-mono">Loading…</span>
            </div>
          )}
          {!isLoading && deliveries.length === 0 && (
            <p className="text-xs font-mono p-4" style={{ color: 'var(--text-muted)' }}>
              No deliveries recorded yet.
            </p>
          )}
          {!isLoading && deliveries.length > 0 && (
            <div style={{ overflowX: 'auto' }}>
              <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: '11px', fontFamily: 'monospace' }}>
                <thead>
                  <tr style={{ borderBottom: '1px solid var(--border-subtle)' }}>
                    {['Time', 'Alert key', 'Severity', 'Channel', 'Status'].map((h) => (
                      <th
                        key={h}
                        style={{
                          padding: '6px 12px',
                          textAlign: 'left',
                          color: 'var(--text-muted)',
                          fontWeight: 500,
                          fontSize: '10px',
                          textTransform: 'uppercase',
                          letterSpacing: '0.05em',
                        }}
                      >
                        {h}
                      </th>
                    ))}
                  </tr>
                </thead>
                <tbody>
                  {deliveries.map((d, i) => {
                    const wh = webhookMap.get(d.channel)
                    return (
                      <tr
                        key={i}
                        style={{
                          borderBottom: '1px solid var(--border-subtle)',
                          opacity: d.status !== 'delivered' ? 0.65 : 1,
                        }}
                      >
                        <td style={{ padding: '5px 12px', color: 'var(--text-muted)', whiteSpace: 'nowrap' }}>
                          {new Date(d.created_at).toLocaleTimeString()}
                        </td>
                        <td style={{ padding: '5px 12px', color: 'var(--text-secondary)', maxWidth: '200px', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                          {d.alert_key}
                        </td>
                        <td style={{ padding: '5px 12px' }}>
                          <SeverityBadge severity={d.severity} />
                        </td>
                        <td style={{ padding: '5px 12px', color: 'var(--text-secondary)' }}>
                          {wh?.name ?? d.channel}
                        </td>
                        <td style={{ padding: '5px 12px' }}>
                          <DeliveryStatusBadge status={d.status} />
                        </td>
                      </tr>
                    )
                  })}
                </tbody>
              </table>
            </div>
          )}
          <p className="text-xs px-4 pb-3 pt-1" style={{ color: 'var(--text-muted)', fontSize: '10px' }}>
            Showing last 50 deliveries. Policy engine is active — default routes all alerts to all channels.
          </p>
        </div>
      )}
    </div>
  )
}

// ---- Alert policies section ----

interface AlertPoliciesSectionProps {
  isAdmin: boolean
  webhooks: Webhook[]
}

function AlertPoliciesSection({ isAdmin, webhooks }: AlertPoliciesSectionProps) {
  const { data, isLoading, error } = useAlertPolicies()
  const { mutate: create, isPending: creating, error: createErr } = useCreateAlertPolicy()
  const { mutate: update, isPending: updating, error: updateErr } = useUpdateAlertPolicy()
  const { mutate: del } = useDeleteAlertPolicy()

  const [showCreate, setShowCreate] = useState(false)
  const [editTarget, setEditTarget] = useState<AlertPolicy | null>(null)
  const [deleteTarget, setDeleteTarget] = useState<AlertPolicy | null>(null)
  const [togglingIds, setTogglingIds] = useState<Set<string>>(new Set())

  const policies = data?.policies ?? []

  function serverErrMsg(err: unknown): string | null {
    if (!err) return null
    if (err instanceof ApiError) {
      const body = err.body as { error?: string }
      return body?.error ?? 'An error occurred.'
    }
    return 'An error occurred.'
  }

  function handleToggle(policy: AlertPolicy) {
    setTogglingIds((prev) => new Set([...prev, policy.id]))
    const req = formToRequest(policyToForm(policy))
    update(
      { id: policy.id, body: { ...req, enabled: !policy.enabled } },
      {
        onSettled: () => {
          setTogglingIds((prev) => {
            const next = new Set(prev)
            next.delete(policy.id)
            return next
          })
        },
      },
    )
  }

  return (
    <>
      {/* Section header */}
      <div className="flex items-center justify-between pt-2">
        <div className="flex items-center gap-2.5">
          <Shield size={14} style={{ color: 'var(--accent)' }} />
          <span
            className="text-xs uppercase tracking-wider font-mono font-medium"
            style={{ color: 'var(--text-primary)' }}
          >
            Alert Policies
          </span>
        </div>
        {isAdmin && (
          <button
            type="button"
            onClick={() => setShowCreate(true)}
            className="flex items-center gap-1.5 text-xs px-3 py-1.5"
            style={{
              background: 'var(--accent-glow)',
              border: '1px solid var(--accent)',
              color: 'var(--accent)',
              borderRadius: '3px',
              cursor: 'pointer',
            }}
          >
            <Plus size={12} />
            Add Policy
          </button>
        )}
      </div>

      <p className="text-xs" style={{ color: 'var(--text-muted)', marginTop: '-8px' }}>
        Control which alerts reach which channels, with severity filtering, quiet hours, dedup, and escalation.
        By default, all alerts are routed to all enabled channels.
      </p>

      {/* Loading */}
      {isLoading && (
        <div className="flex items-center gap-2" style={{ color: 'var(--text-muted)' }}>
          <Loader size={12} className="animate-spin" />
          <span className="text-xs font-mono">Loading policies…</span>
        </div>
      )}

      {/* Error */}
      {error && (
        <div
          className="text-xs font-mono"
          style={{ color: 'var(--status-error)', padding: '10px 14px', border: '1px solid rgba(232,64,64,0.3)', borderRadius: '3px' }}
        >
          Failed to load alert policies.
        </div>
      )}

      {/* Empty state */}
      {!isLoading && !error && policies.length === 0 && (
        <div className="text-xs font-mono py-4" style={{ color: 'var(--text-muted)' }}>
          No policies configured — all alerts route to all channels by default.
          {isAdmin && (
            <button
              type="button"
              onClick={() => setShowCreate(true)}
              style={{
                background: 'none',
                border: 'none',
                color: 'var(--accent)',
                cursor: 'pointer',
                fontSize: '12px',
                fontFamily: 'monospace',
                paddingLeft: '6px',
                textDecoration: 'underline',
              }}
            >
              Add a policy
            </button>
          )}
        </div>
      )}

      {/* Policy list */}
      {!isLoading && policies.length > 0 && (
        <div className="flex flex-col gap-2">
          {policies.map((p) => (
            <PolicyRow
              key={p.id}
              policy={p}
              webhooks={webhooks}
              onEdit={() => setEditTarget(p)}
              onDelete={() => setDeleteTarget(p)}
              onToggle={() => handleToggle(p)}
              isToggling={togglingIds.has(p.id)}
            />
          ))}
        </div>
      )}

      {/* Deliveries */}
      <DeliveriesList webhooks={webhooks} />

      {/* Divider */}
      <div style={{ height: '1px', background: 'var(--border-subtle)' }} />

      {/* Create modal */}
      {showCreate && (
        <PolicyForm
          title="Add Alert Policy"
          initial={EMPTY_POLICY_FORM}
          webhooks={webhooks}
          isPending={creating}
          serverError={serverErrMsg(createErr)}
          onClose={() => setShowCreate(false)}
          onSubmit={(req) => {
            create(req, { onSuccess: () => setShowCreate(false) })
          }}
        />
      )}

      {/* Edit modal */}
      {editTarget && (
        <PolicyForm
          title="Edit Alert Policy"
          initial={policyToForm(editTarget)}
          webhooks={webhooks}
          isPending={updating}
          serverError={serverErrMsg(updateErr)}
          onClose={() => setEditTarget(null)}
          onSubmit={(req) => {
            update(
              { id: editTarget.id, body: req },
              { onSuccess: () => setEditTarget(null) },
            )
          }}
        />
      )}

      {/* Delete confirm */}
      {deleteTarget && (
        <DeleteDialog
          name={deleteTarget.name}
          onConfirm={() => {
            del(deleteTarget.id)
            setDeleteTarget(null)
          }}
          onCancel={() => setDeleteTarget(null)}
        />
      )}
    </>
  )
}

// ---- Webhook form modal ----

interface FormState {
  name: string
  url: string
  provider: WebhookProvider
  triggers: string[]
  enabled: boolean
}

interface WebhookFormProps {
  initial: FormState
  triggerDefs: TriggerDef[]
  onSubmit: (data: FormState) => void
  onClose: () => void
  isPending: boolean
  serverError?: string | null
  title: string
}

function WebhookForm({
  initial,
  triggerDefs,
  onSubmit,
  onClose,
  isPending,
  serverError,
  title,
}: WebhookFormProps) {
  const [form, setForm] = useState<FormState>(initial)
  const [clientError, setClientError] = useState<string | null>(null)

  function toggleTrigger(key: string) {
    setForm((prev) => ({
      ...prev,
      triggers: prev.triggers.includes(key)
        ? prev.triggers.filter((t) => t !== key)
        : [...prev.triggers, key],
    }))
  }

  function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    if (!form.name.trim() || !form.url.trim()) {
      setClientError('Name and URL are required.')
      return
    }
    setClientError(null)
    onSubmit(form)
  }

  const errorMsg = clientError ?? serverError ?? null

  return (
    <div
      style={{
        position: 'fixed',
        inset: 0,
        background: 'rgba(0,0,0,0.55)',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        zIndex: 50,
      }}
    >
      <div
        style={{
          background: 'var(--bg-elevated)',
          border: '1px solid var(--border-default)',
          borderRadius: '3px',
          padding: '24px',
          width: '480px',
          maxWidth: '95vw',
          maxHeight: '90vh',
          overflowY: 'auto',
        }}
      >
        <div className="flex items-center justify-between mb-4">
          <span className="text-sm font-medium" style={{ color: 'var(--text-primary)' }}>
            {title}
          </span>
          <button
            type="button"
            onClick={onClose}
            style={{ background: 'none', border: 'none', cursor: 'pointer', color: 'var(--text-muted)' }}
          >
            <X size={14} />
          </button>
        </div>

        <form onSubmit={handleSubmit} className="flex flex-col gap-4">
          {/* Name */}
          <label className="flex flex-col gap-1">
            <span
              className="text-xs uppercase tracking-wider font-mono"
              style={{ color: 'var(--text-muted)' }}
            >
              Name
            </span>
            <input
              type="text"
              value={form.name}
              onChange={(e) => setForm((p) => ({ ...p, name: e.target.value }))}
              placeholder="e.g. ops-alerts"
              style={{
                background: 'var(--bg-surface)',
                border: '1px solid var(--border-default)',
                borderRadius: '3px',
                color: 'var(--text-primary)',
                fontSize: '13px',
                padding: '6px 8px',
                outline: 'none',
              }}
            />
          </label>

          {/* URL */}
          <label className="flex flex-col gap-1">
            <span
              className="text-xs uppercase tracking-wider font-mono"
              style={{ color: 'var(--text-muted)' }}
            >
              Webhook URL
            </span>
            <input
              type="url"
              value={form.url}
              onChange={(e) => setForm((p) => ({ ...p, url: e.target.value }))}
              placeholder="https://hooks.slack.com/…"
              style={{
                background: 'var(--bg-surface)',
                border: '1px solid var(--border-default)',
                borderRadius: '3px',
                color: 'var(--text-primary)',
                fontSize: '13px',
                padding: '6px 8px',
                outline: 'none',
              }}
            />
          </label>

          {/* Provider */}
          <div className="flex flex-col gap-1">
            <span
              className="text-xs uppercase tracking-wider font-mono"
              style={{ color: 'var(--text-muted)' }}
            >
              Provider
            </span>
            <div className="flex gap-2">
              {(['slack', 'discord'] as WebhookProvider[]).map((p) => (
                <button
                  key={p}
                  type="button"
                  onClick={() => setForm((prev) => ({ ...prev, provider: p }))}
                  style={{
                    background:
                      form.provider === p ? 'var(--accent-glow)' : 'var(--bg-surface)',
                    border: `1px solid ${form.provider === p ? 'var(--accent)' : 'var(--border-default)'}`,
                    color: form.provider === p ? 'var(--accent)' : 'var(--text-secondary)',
                    borderRadius: '3px',
                    padding: '4px 12px',
                    fontSize: '12px',
                    fontFamily: 'monospace',
                    textTransform: 'uppercase',
                    letterSpacing: '0.05em',
                    cursor: 'pointer',
                  }}
                >
                  {p}
                </button>
              ))}
            </div>
          </div>

          {/* Triggers — rendered from server registry (adds automatically as new triggers register) */}
          <div className="flex flex-col gap-2">
            <span
              className="text-xs uppercase tracking-wider font-mono"
              style={{ color: 'var(--text-muted)' }}
            >
              Triggers
            </span>
            <div className="flex flex-col gap-2">
              {triggerDefs.map((def) => (
                <div key={def.key}>
                  <label
                    className="flex items-start gap-2 cursor-pointer"
                    style={{ fontSize: '12px', color: 'var(--text-secondary)' }}
                  >
                    <input
                      type="checkbox"
                      checked={form.triggers.includes(def.key)}
                      onChange={() => toggleTrigger(def.key)}
                      style={{ accentColor: 'var(--accent)', cursor: 'pointer', marginTop: '2px', flexShrink: 0 }}
                    />
                    <div className="flex flex-col gap-0.5 min-w-0">
                      <div className="flex items-center gap-2 flex-wrap">
                        <span className="font-mono" style={{ color: 'var(--text-muted)', fontSize: '11px' }}>
                          {def.key}
                        </span>
                        <span style={{ color: 'var(--text-primary)', fontWeight: 500 }}>{def.label}</span>
                        {def.requires_capability && (
                          <span
                            className="font-mono"
                            style={{
                              fontSize: '10px',
                              padding: '1px 5px',
                              background: 'var(--bg-surface)',
                              border: '1px solid var(--border-subtle)',
                              color: 'var(--text-muted)',
                              borderRadius: '3px',
                            }}
                          >
                            requires: {def.requires_capability}
                          </span>
                        )}
                      </div>
                      <span style={{ color: 'var(--text-muted)', fontSize: '11px' }}>{def.description}</span>
                      {/* Config schema inputs — only shown when trigger is enabled */}
                      {form.triggers.includes(def.key) && def.config_schema && def.config_schema.length > 0 && (
                        <div className="flex flex-col gap-1 mt-1 pl-1" style={{ borderLeft: '2px solid var(--border-subtle)' }}>
                          {def.config_schema.map((field) => (
                            <div key={field.key} className="flex items-center gap-2">
                              <span style={{ color: 'var(--text-muted)', fontSize: '11px', minWidth: '120px' }}>
                                {field.label}
                              </span>
                              {field.type === 'select' ? (
                                <select
                                  defaultValue={field.default}
                                  style={{
                                    background: 'var(--bg-surface)',
                                    border: '1px solid var(--border-default)',
                                    borderRadius: '3px',
                                    color: 'var(--text-primary)',
                                    fontSize: '11px',
                                    padding: '2px 6px',
                                  }}
                                >
                                  {(field.options ?? []).map((opt) => (
                                    <option key={opt} value={opt}>{opt}</option>
                                  ))}
                                </select>
                              ) : (
                                <input
                                  type={field.type === 'number' ? 'number' : 'text'}
                                  defaultValue={field.default}
                                  style={{
                                    background: 'var(--bg-surface)',
                                    border: '1px solid var(--border-default)',
                                    borderRadius: '3px',
                                    color: 'var(--text-primary)',
                                    fontSize: '11px',
                                    padding: '2px 6px',
                                    width: '80px',
                                  }}
                                />
                              )}
                            </div>
                          ))}
                        </div>
                      )}
                    </div>
                  </label>
                </div>
              ))}
            </div>
          </div>

          {/* Enabled */}
          <label className="flex items-center gap-3 cursor-pointer">
            <Toggle
              checked={form.enabled}
              onChange={(v) => setForm((p) => ({ ...p, enabled: v }))}
            />
            <span className="text-xs" style={{ color: 'var(--text-secondary)' }}>
              Enabled
            </span>
          </label>

          {errorMsg && (
            <p className="text-xs" style={{ color: 'var(--status-error)' }}>
              {errorMsg}
            </p>
          )}

          <div className="flex gap-2 justify-end pt-1">
            <button
              type="button"
              onClick={onClose}
              className="text-xs px-3 py-1.5"
              style={{
                background: 'var(--bg-surface)',
                border: '1px solid var(--border-default)',
                color: 'var(--text-secondary)',
                borderRadius: '3px',
                cursor: 'pointer',
              }}
            >
              Cancel
            </button>
            <button
              type="submit"
              disabled={isPending}
              className="text-xs px-3 py-1.5 flex items-center gap-1.5"
              style={{
                background: 'var(--accent-glow)',
                border: '1px solid var(--accent)',
                color: 'var(--accent)',
                borderRadius: '3px',
                cursor: isPending ? 'not-allowed' : 'pointer',
                opacity: isPending ? 0.7 : 1,
              }}
            >
              {isPending && <Loader size={11} className="animate-spin" />}
              Save
            </button>
          </div>
        </form>
      </div>
    </div>
  )
}

// ---- Test status badge ----

type TestStatus = 'idle' | 'pending' | 'ok' | 'error'

interface TestBadgeProps {
  status: TestStatus
}

function TestBadge({ status }: TestBadgeProps) {
  if (status === 'idle') return null
  if (status === 'pending')
    return <Loader size={11} className="animate-spin" style={{ color: 'var(--text-muted)' }} />
  if (status === 'ok')
    return (
      <span
        className="font-mono text-xs flex items-center gap-1"
        style={{ color: 'var(--status-ok, #40c878)', fontSize: '12px' }}
      >
        <Check size={10} /> Sent
      </span>
    )
  return (
    <span
      className="font-mono text-xs flex items-center gap-1"
      style={{ color: 'var(--status-error)', fontSize: '12px' }}
    >
      <X size={10} /> Delivery failed
    </span>
  )
}

// ---- Webhook row ----

interface WebhookRowProps {
  webhook: Webhook
  triggerIndex: Map<string, TriggerDef>
  onEdit: () => void
  onDelete: () => void
  testStatus: TestStatus
  onTest: () => void
  onToggleEnabled: () => void
  isTogglingEnabled: boolean
}

function WebhookRow({
  webhook,
  triggerIndex,
  onEdit,
  onDelete,
  testStatus,
  onTest,
  onToggleEnabled,
  isTogglingEnabled,
}: WebhookRowProps) {
  return (
    <div
      style={{
        background: 'var(--bg-elevated)',
        border: '1px solid var(--border-subtle)',
        borderRadius: '3px',
        padding: '12px 16px',
        display: 'flex',
        flexDirection: 'column',
        gap: '8px',
      }}
    >
      {/* Top row: name + provider + enabled toggle + actions */}
      <div className="flex items-center gap-3 flex-wrap">
        <Toggle
          checked={webhook.enabled}
          onChange={onToggleEnabled}
          disabled={isTogglingEnabled}
        />
        <span
          className="font-mono text-xs font-medium flex-1 min-w-0 truncate"
          style={{ color: 'var(--text-primary)' }}
        >
          {webhook.name}
        </span>
        <ProviderBadge provider={webhook.provider} />
        <span
          className="font-mono text-xs truncate"
          style={{ color: 'var(--text-muted)', maxWidth: '200px' }}
          title={webhook.url}
        >
          {maskedUrl(webhook.url)}
        </span>
        {/* Actions */}
        <div className="flex items-center gap-2 shrink-0 ml-auto">
          <TestBadge status={testStatus} />
          <button
            type="button"
            onClick={onTest}
            title="Send test"
            disabled={testStatus === 'pending'}
            className="flex items-center justify-center"
            style={{
              background: 'none',
              border: 'none',
              color: 'var(--text-muted)',
              cursor: testStatus === 'pending' ? 'not-allowed' : 'pointer',
              padding: '4px',
            }}
          >
            <Send size={12} />
          </button>
          <button
            type="button"
            onClick={onEdit}
            title="Edit"
            className="flex items-center justify-center"
            style={{ background: 'none', border: 'none', color: 'var(--text-muted)', cursor: 'pointer', padding: '4px' }}
          >
            <Pencil size={12} />
          </button>
          <button
            type="button"
            onClick={onDelete}
            title="Delete"
            className="flex items-center justify-center"
            style={{ background: 'none', border: 'none', color: 'var(--text-muted)', cursor: 'pointer', padding: '4px' }}
          >
            <Trash2 size={12} />
          </button>
        </div>
      </div>

      {/* Trigger chips */}
      {webhook.triggers.length > 0 && (
        <div className="flex flex-wrap gap-1.5 pl-10">
          {webhook.triggers.map((t) => (
            <TriggerChip key={t} label={triggerLabel(t, triggerIndex)} />
          ))}
        </div>
      )}
    </div>
  )
}

// ---- Page ----

const EMPTY_FORM: FormState = {
  name: '',
  url: '',
  provider: 'slack',
  triggers: [],
  enabled: true,
}

export default function Notifications() {
  const { data: me, isLoading: meLoading } = useMe()
  const { data, isLoading, error } = useWebhooks()
  const { mutate: create, isPending: creating, error: createErr } = useCreateWebhook()
  const { mutate: update, isPending: updating, error: updateErr } = useUpdateWebhook()
  const { mutate: del } = useDeleteWebhook()
  const { mutate: test } = useTestWebhook()

  const [showCreate, setShowCreate] = useState(false)
  const [editTarget, setEditTarget] = useState<Webhook | null>(null)
  const [deleteTarget, setDeleteTarget] = useState<Webhook | null>(null)
  const [testStatuses, setTestStatuses] = useState<Record<string, TestStatus>>({})
  const [togglingIds, setTogglingIds] = useState<Set<string>>(new Set())

  // Build a lookup index from the server-driven trigger registry.
  const triggerDefs = data?.trigger_defs ?? []
  const triggerIndex = buildTriggerIndex(triggerDefs)

  function setTestStatus(id: string, s: TestStatus) {
    setTestStatuses((prev) => ({ ...prev, [id]: s }))
  }

  function handleTest(webhook: Webhook) {
    setTestStatus(webhook.id, 'pending')
    test(webhook.id, {
      onSuccess: () => {
        setTestStatus(webhook.id, 'ok')
        setTimeout(() => setTestStatus(webhook.id, 'idle'), 3000)
      },
      onError: () => {
        setTestStatus(webhook.id, 'error')
        setTimeout(() => setTestStatus(webhook.id, 'idle'), 4000)
      },
    })
  }

  function handleToggleEnabled(webhook: Webhook) {
    setTogglingIds((prev) => new Set([...prev, webhook.id]))
    update(
      {
        id: webhook.id,
        body: {
          name: webhook.name,
          url: webhook.url,
          provider: webhook.provider,
          triggers: webhook.triggers,
          enabled: !webhook.enabled,
        },
      },
      {
        onSettled: () => {
          setTogglingIds((prev) => {
            const next = new Set(prev)
            next.delete(webhook.id)
            return next
          })
        },
      },
    )
  }

  function serverErrMsg(err: unknown): string | null {
    if (!err) return null
    if (err instanceof ApiError) {
      const body = err.body as { error?: string }
      if (body?.error === 'name_url_provider_required') return 'Name, URL, and provider are required.'
      return body?.error ?? 'An error occurred.'
    }
    return 'An error occurred.'
  }

  const isAdmin = me?.role === 'admin'

  return (
    <AppShell>
      <div
        style={{
          flex: 1,
          overflowY: 'auto',
          padding: '24px',
          display: 'flex',
          flexDirection: 'column',
          gap: '20px',
        }}
      >
        {/* Header */}
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-2.5">
            <Bell size={16} style={{ color: 'var(--accent)' }} />
            <span
              className="text-xs uppercase tracking-wider font-mono font-medium"
              style={{ color: 'var(--text-primary)' }}
            >
              Notification Hooks
            </span>
          </div>
          {isAdmin && (
            <button
              type="button"
              onClick={() => setShowCreate(true)}
              className="flex items-center gap-1.5 text-xs px-3 py-1.5"
              style={{
                background: 'var(--accent-glow)',
                border: '1px solid var(--accent)',
                color: 'var(--accent)',
                borderRadius: '3px',
                cursor: 'pointer',
              }}
            >
              <Plus size={12} />
              Add Webhook
            </button>
          )}
        </div>

        {/* Admin gate */}
        {!meLoading && !isAdmin && (
          <div
            className="text-xs font-mono"
            style={{
              color: 'var(--status-error)',
              background: 'rgba(232,64,64,0.08)',
              border: '1px solid rgba(232,64,64,0.25)',
              borderRadius: '3px',
              padding: '12px 16px',
            }}
          >
            Admin access required to manage notification hooks.
          </div>
        )}

        {/* Loading */}
        {isLoading && isAdmin && (
          <div className="flex items-center gap-2" style={{ color: 'var(--text-muted)' }}>
            <Loader size={13} className="animate-spin" />
            <span className="text-xs font-mono">Loading webhooks…</span>
          </div>
        )}

        {/* Error */}
        {error && isAdmin && (
          <div
            className="text-xs font-mono"
            style={{ color: 'var(--status-error)', padding: '10px 14px', border: '1px solid rgba(232,64,64,0.3)', borderRadius: '3px' }}
          >
            Failed to load webhooks.
          </div>
        )}

        {/* Empty state */}
        {!isLoading && !error && isAdmin && data?.webhooks.length === 0 && (
          <div
            className="flex flex-col items-center justify-center gap-3 py-16 text-center"
            style={{ color: 'var(--text-muted)' }}
          >
            <Bell size={28} style={{ opacity: 0.3 }} />
            <p className="text-xs font-mono">No webhooks configured.</p>
            <button
              type="button"
              onClick={() => setShowCreate(true)}
              className="text-xs px-3 py-1.5 flex items-center gap-1.5"
              style={{
                background: 'var(--accent-glow)',
                border: '1px solid var(--accent)',
                color: 'var(--accent)',
                borderRadius: '3px',
                cursor: 'pointer',
              }}
            >
              <Plus size={11} />
              Add your first webhook
            </button>
          </div>
        )}

        {/* Webhook list */}
        {!isLoading && isAdmin && data && data.webhooks.length > 0 && (
          <div className="flex flex-col gap-2">
            {data.webhooks.map((wh) => (
              <WebhookRow
                key={wh.id}
                webhook={wh}
                triggerIndex={triggerIndex}
                testStatus={testStatuses[wh.id] ?? 'idle'}
                onTest={() => handleTest(wh)}
                onEdit={() => setEditTarget(wh)}
                onDelete={() => setDeleteTarget(wh)}
                onToggleEnabled={() => handleToggleEnabled(wh)}
                isTogglingEnabled={togglingIds.has(wh.id)}
              />
            ))}
          </div>
        )}

        {/* Divider between webhooks and alert policies */}
        {isAdmin && <div style={{ height: '1px', background: 'var(--border-subtle)' }} />}

        {/* Alert policies section */}
        {isAdmin && (
          <AlertPoliciesSection
            isAdmin={isAdmin}
            webhooks={data?.webhooks ?? []}
          />
        )}
      </div>

      {/* Create modal */}
      {showCreate && (
        <WebhookForm
          title="Add Webhook"
          initial={EMPTY_FORM}
          triggerDefs={triggerDefs}
          isPending={creating}
          serverError={serverErrMsg(createErr)}
          onClose={() => setShowCreate(false)}
          onSubmit={(form) => {
            create(form as WebhookRequest, {
              onSuccess: () => setShowCreate(false),
            })
          }}
        />
      )}

      {/* Edit modal */}
      {editTarget && (
        <WebhookForm
          title="Edit Webhook"
          initial={{
            name: editTarget.name,
            url: editTarget.url,
            provider: editTarget.provider,
            triggers: editTarget.triggers,
            enabled: editTarget.enabled,
          }}
          triggerDefs={triggerDefs}
          isPending={updating}
          serverError={serverErrMsg(updateErr)}
          onClose={() => setEditTarget(null)}
          onSubmit={(form) => {
            update(
              { id: editTarget.id, body: form as WebhookRequest },
              { onSuccess: () => setEditTarget(null) },
            )
          }}
        />
      )}

      {/* Delete confirm */}
      {deleteTarget && (
        <DeleteDialog
          name={deleteTarget.name}
          onCancel={() => setDeleteTarget(null)}
          onConfirm={() => {
            del(deleteTarget.id)
            setDeleteTarget(null)
          }}
        />
      )}
    </AppShell>
  )
}
