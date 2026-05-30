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
