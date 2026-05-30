import { useState } from 'react'
import { Brain, Check, X, Pencil, Trash2, Plus, Loader } from 'lucide-react'
import { useCan } from '../../lib/roles'
import { useMemory, useCreateMemory, useUpdateMemory, useDeleteMemory } from '../../lib/api/memory'
import { ApiError } from '../../lib/api'
import type { Memory, MemoryScope, MemorySource } from '../../types/api'

// ── Types ─────────────────────────────────────────────────────────────────────

interface MemoryPanelProps {
  scope: MemoryScope
  scopeId?: string
}

// ── Helpers ───────────────────────────────────────────────────────────────────

const SOURCE_LABELS: Record<MemorySource, string> = {
  user: 'user',
  ai: 'ai',
  observed: 'observed',
}

function sourceBadgeStyle(source: MemorySource, suggested: boolean): React.CSSProperties {
  if (suggested) {
    return {
      background: 'rgba(240,160,32,0.13)',
      border: '1px solid var(--status-warn)',
      color: 'var(--status-warn)',
      borderRadius: '3px',
      padding: '1px 6px',
      fontSize: '12px',
      fontFamily: 'monospace',
      textTransform: 'uppercase',
      letterSpacing: '0.04em',
    }
  }
  return {
    background: source === 'user'
      ? 'rgba(64,120,200,0.10)'
      : source === 'ai'
      ? 'rgba(120,80,200,0.12)'
      : 'rgba(74,82,104,0.20)',
    border: `1px solid ${source === 'user' ? 'rgba(64,120,200,0.3)' : source === 'ai' ? 'rgba(120,80,200,0.3)' : 'var(--border-subtle)'}`,
    color: source === 'user' ? 'var(--accent)' : source === 'ai' ? '#a07de8' : 'var(--text-muted)',
    borderRadius: '3px',
    padding: '1px 6px',
    fontSize: '12px',
    fontFamily: 'monospace',
    textTransform: 'uppercase',
    letterSpacing: '0.04em',
  }
}

function apiErrMsg(err: unknown): string {
  if (err instanceof ApiError) {
    const code = (err.body as { error?: string })?.error
    if (code === 'memory_exists_or_failed') return 'Key already exists in this scope.'
    if (code === 'scope_key_value_required') return 'Scope, key, and value are required.'
    if (code === 'scope_id_required') return 'A scope ID is required for node/container memory.'
    return `Error: ${code ?? err.status}`
  }
  return 'An unexpected error occurred.'
}

// ── MemoryRow ─────────────────────────────────────────────────────────────────

interface MemoryRowProps {
  memory: Memory
  scope: MemoryScope
  scopeId?: string
  isOperator: boolean
}

function MemoryRow({ memory, scope, scopeId, isOperator }: MemoryRowProps) {
  const [editing, setEditing] = useState(false)
  const [editValue, setEditValue] = useState(memory.value)
  const [editFocused, setEditFocused] = useState(false)
  const [confirmDelete, setConfirmDelete] = useState(false)

  const update = useUpdateMemory(scope, scopeId)
  const del = useDeleteMemory(scope, scopeId)
  const isSuggested = !memory.confirmed

  function handleSave() {
    update.mutate(
      { id: memory.id, body: { value: editValue } },
      { onSuccess: () => setEditing(false) },
    )
  }

  function handleAccept() {
    update.mutate({ id: memory.id, body: { confirmed: true } })
  }

  function handleDismiss() {
    del.mutate(memory.id)
  }

  function handleDelete() {
    del.mutate(memory.id, { onSuccess: () => setConfirmDelete(false) })
  }

  const rowBorder: React.CSSProperties = isSuggested
    ? { border: '1px solid var(--status-warn)', borderRadius: '3px', padding: '8px 10px' }
    : { borderBottom: '1px solid var(--border-subtle)', paddingBottom: '8px', marginBottom: '6px' }

  return (
    <div style={{ ...rowBorder, display: 'flex', flexDirection: 'column', gap: '4px' }}>
      {/* Header row */}
      <div style={{ display: 'flex', alignItems: 'flex-start', justifyContent: 'space-between', gap: '8px' }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: '6px', minWidth: 0, flexWrap: 'wrap' }}>
          <code
            className="font-mono text-xs"
            style={{ color: 'var(--text-primary)', wordBreak: 'break-all' }}
          >
            {memory.key}
          </code>
          <span style={sourceBadgeStyle(memory.source, isSuggested)}>
            {isSuggested ? 'ai suggestion' : SOURCE_LABELS[memory.source]}
          </span>
        </div>

        {/* Controls */}
        {isOperator && !editing && (
          <div style={{ display: 'flex', alignItems: 'center', gap: '4px', flexShrink: 0 }}>
            {isSuggested ? (
              <>
                <button
                  type="button"
                  title="Accept suggestion"
                  disabled={update.isPending || del.isPending}
                  onClick={handleAccept}
                  style={actionBtnStyle('ok', update.isPending)}
                >
                  {update.isPending ? <Loader size={11} className="animate-spin" /> : <Check size={11} />}
                  Accept
                </button>
                <button
                  type="button"
                  title="Dismiss suggestion"
                  disabled={del.isPending || update.isPending}
                  onClick={handleDismiss}
                  style={actionBtnStyle('err', del.isPending)}
                >
                  {del.isPending ? <Loader size={11} className="animate-spin" /> : <X size={11} />}
                  Dismiss
                </button>
              </>
            ) : (
              <>
                <button
                  type="button"
                  title="Edit value"
                  onClick={() => { setEditValue(memory.value); setEditing(true) }}
                  style={iconBtnStyle()}
                >
                  <Pencil size={11} />
                </button>
                <button
                  type="button"
                  title="Delete"
                  onClick={() => setConfirmDelete(true)}
                  style={iconBtnStyle('err')}
                >
                  <Trash2 size={11} />
                </button>
              </>
            )}
          </div>
        )}
      </div>

      {/* Value */}
      {!editing && (
        <span
          className="font-mono text-xs"
          style={{ color: 'var(--text-secondary)', wordBreak: 'break-all', paddingLeft: '2px' }}
        >
          {memory.value}
        </span>
      )}

      {/* Inline edit */}
      {editing && (
        <div style={{ display: 'flex', flexDirection: 'column', gap: '4px' }}>
          <textarea
            value={editValue}
            onChange={(e) => setEditValue(e.target.value)}
            onFocus={() => setEditFocused(true)}
            onBlur={() => setEditFocused(false)}
            rows={2}
            className="font-mono text-xs"
            style={{
              background: 'var(--bg-elevated)',
              border: `1px solid ${editFocused ? 'var(--accent)' : 'var(--border-default)'}`,
              color: 'var(--text-primary)',
              borderRadius: '3px',
              outline: 'none',
              padding: '5px 8px',
              resize: 'vertical',
              width: '100%',
              boxSizing: 'border-box',
            }}
          />
          {update.error && (
            <span className="font-mono text-xs" style={{ color: 'var(--status-error)' }}>
              {apiErrMsg(update.error)}
            </span>
          )}
          <div style={{ display: 'flex', gap: '6px' }}>
            <button type="button" onClick={handleSave} disabled={update.isPending} style={actionBtnStyle('ok', update.isPending)}>
              {update.isPending ? <Loader size={11} className="animate-spin" /> : <Check size={11} />}
              Save
            </button>
            <button type="button" onClick={() => setEditing(false)} style={actionBtnStyle('neutral', false)}>
              <X size={11} />
              Cancel
            </button>
          </div>
        </div>
      )}

      {/* Inline delete confirm */}
      {confirmDelete && (
        <div
          style={{
            background: 'var(--bg-elevated)',
            border: '1px solid var(--status-error)',
            borderRadius: '3px',
            padding: '8px 10px',
            display: 'flex',
            flexDirection: 'column',
            gap: '6px',
          }}
        >
          <span className="font-mono text-xs" style={{ color: 'var(--text-primary)' }}>
            Delete <strong>{memory.key}</strong>?
          </span>
          {del.error && (
            <span className="font-mono text-xs" style={{ color: 'var(--status-error)' }}>
              {apiErrMsg(del.error)}
            </span>
          )}
          <div style={{ display: 'flex', gap: '6px' }}>
            <button type="button" onClick={handleDelete} disabled={del.isPending} style={actionBtnStyle('err', del.isPending)}>
              {del.isPending ? <Loader size={11} className="animate-spin" /> : <Trash2 size={11} />}
              Delete
            </button>
            <button type="button" onClick={() => setConfirmDelete(false)} style={actionBtnStyle('neutral', false)}>
              Cancel
            </button>
          </div>
        </div>
      )}
    </div>
  )
}

// ── AddMemoryForm ─────────────────────────────────────────────────────────────

interface AddMemoryFormProps {
  scope: MemoryScope
  scopeId?: string
  onDone: () => void
}

function AddMemoryForm({ scope, scopeId, onDone }: AddMemoryFormProps) {
  const [key, setKey] = useState('')
  const [value, setValue] = useState('')
  const [keyFocused, setKeyFocused] = useState(false)
  const [valueFocused, setValueFocused] = useState(false)
  const create = useCreateMemory(scope, scopeId)

  function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    const trimKey = key.trim()
    const trimVal = value.trim()
    if (!trimKey || !trimVal) return
    create.mutate(
      { scope, scope_id: scopeId, key: trimKey, value: trimVal },
      {
        onSuccess: () => {
          setKey('')
          setValue('')
          onDone()
        },
      },
    )
  }

  const keyEmpty = !key.trim()
  const valEmpty = !value.trim()

  return (
    <form
      onSubmit={handleSubmit}
      style={{ display: 'flex', flexDirection: 'column', gap: '6px', paddingTop: '8px', borderTop: '1px solid var(--border-subtle)' }}
    >
      <span className="text-xs" style={{ color: 'var(--text-muted)' }}>Add memory</span>
      <input
        type="text"
        placeholder="key (e.g. always-uid)"
        value={key}
        onChange={(e) => setKey(e.target.value)}
        onFocus={() => setKeyFocused(true)}
        onBlur={() => setKeyFocused(false)}
        className="font-mono text-xs"
        style={inputStyle(keyFocused)}
      />
      <textarea
        placeholder="value (e.g. container needs UID 1000 for /config)"
        value={value}
        onChange={(e) => setValue(e.target.value)}
        onFocus={() => setValueFocused(true)}
        onBlur={() => setValueFocused(false)}
        rows={2}
        className="font-mono text-xs"
        style={{ ...inputStyle(valueFocused), resize: 'vertical' }}
      />
      {create.error && (
        <span className="font-mono text-xs" style={{ color: 'var(--status-error)' }}>
          {apiErrMsg(create.error)}
        </span>
      )}
      <div style={{ display: 'flex', gap: '6px' }}>
        <button
          type="submit"
          disabled={create.isPending || keyEmpty || valEmpty}
          style={actionBtnStyle('ok', create.isPending || keyEmpty || valEmpty)}
        >
          {create.isPending ? <Loader size={11} className="animate-spin" /> : <Plus size={11} />}
          Add
        </button>
        <button type="button" onClick={onDone} style={actionBtnStyle('neutral', false)}>
          Cancel
        </button>
      </div>
    </form>
  )
}

// ── Style helpers ─────────────────────────────────────────────────────────────

function inputStyle(focused: boolean): React.CSSProperties {
  return {
    background: 'var(--bg-elevated)',
    border: `1px solid ${focused ? 'var(--accent)' : 'var(--border-default)'}`,
    color: 'var(--text-primary)',
    borderRadius: '3px',
    outline: 'none',
    padding: '5px 8px',
    width: '100%',
    boxSizing: 'border-box',
  }
}

function actionBtnStyle(
  variant: 'ok' | 'err' | 'neutral',
  disabled: boolean,
): React.CSSProperties {
  const colors = {
    ok: { bg: 'rgba(64,200,120,0.10)', border: 'var(--status-ok)', color: 'var(--status-ok)' },
    err: { bg: 'rgba(232,64,64,0.10)', border: 'var(--status-error)', color: 'var(--status-error)' },
    neutral: { bg: 'var(--bg-elevated)', border: 'var(--border-default)', color: 'var(--text-secondary)' },
  }[variant]
  return {
    background: colors.bg,
    border: `1px solid ${colors.border}`,
    color: disabled ? 'var(--text-muted)' : colors.color,
    borderRadius: '3px',
    padding: '3px 10px',
    fontSize: '0.7rem',
    fontFamily: 'monospace',
    cursor: disabled ? 'not-allowed' : 'pointer',
    opacity: disabled ? 0.6 : 1,
    display: 'flex',
    alignItems: 'center',
    gap: '4px',
  }
}

function iconBtnStyle(variant?: 'err'): React.CSSProperties {
  return {
    background: 'transparent',
    border: 'none',
    color: variant === 'err' ? 'var(--status-error)' : 'var(--text-muted)',
    cursor: 'pointer',
    padding: '2px 4px',
    lineHeight: 1,
    borderRadius: '3px',
    display: 'flex',
    alignItems: 'center',
  }
}

// ── MemoryPanel (exported) ────────────────────────────────────────────────────

export function MemoryPanel({ scope, scopeId }: MemoryPanelProps) {
  const { isOperator } = useCan()
  const { data, isLoading } = useMemory(scope, scopeId)
  const [showAdd, setShowAdd] = useState(false)

  const memories = data?.memories ?? []
  const suggested = memories.filter((m) => !m.confirmed)
  const confirmed = memories.filter((m) => m.confirmed)

  return (
    <div
      className="flex flex-col gap-3"
      style={{
        backgroundColor: 'var(--bg-surface)',
        border: '1px solid var(--border-subtle)',
        borderRadius: '3px',
        padding: '16px',
        maxWidth: '640px',
      }}
    >
      {/* Header */}
      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
          <Brain size={13} style={{ color: 'var(--text-muted)' }} />
          <span
            className="text-xs font-medium uppercase tracking-wider"
            style={{ color: 'var(--text-muted)' }}
          >
            AI Memory
          </span>
        </div>
        {isOperator && !showAdd && (
          <button
            type="button"
            onClick={() => setShowAdd(true)}
            style={actionBtnStyle('neutral', false)}
          >
            <Plus size={11} />
            Add
          </button>
        )}
      </div>

      {/* Loading */}
      {isLoading && (
        <div style={{ display: 'flex', alignItems: 'center', gap: '8px', padding: '4px 0' }}>
          <Loader size={12} className="animate-spin" style={{ color: 'var(--accent)' }} />
          <span className="text-xs" style={{ color: 'var(--text-muted)' }}>Loading…</span>
        </div>
      )}

      {/* AI-suggested entries (unconfirmed) */}
      {!isLoading && suggested.length > 0 && (
        <div style={{ display: 'flex', flexDirection: 'column', gap: '6px' }}>
          <span className="text-xs" style={{ color: 'var(--status-warn)' }}>
            Suggested by AI — review before use
          </span>
          {suggested.map((m) => (
            <MemoryRow key={m.id} memory={m} scope={scope} scopeId={scopeId} isOperator={isOperator} />
          ))}
        </div>
      )}

      {/* Confirmed entries */}
      {!isLoading && confirmed.length > 0 && (
        <div style={{ display: 'flex', flexDirection: 'column' }}>
          {confirmed.map((m) => (
            <MemoryRow key={m.id} memory={m} scope={scope} scopeId={scopeId} isOperator={isOperator} />
          ))}
        </div>
      )}

      {/* Empty state */}
      {!isLoading && memories.length === 0 && (
        <p className="text-xs" style={{ color: 'var(--text-muted)' }}>
          No memory yet. Add notes the assistant should remember (e.g. &lsquo;always needs UID 1000 for /config&rsquo;).
        </p>
      )}

      {/* Add form */}
      {showAdd && (
        <AddMemoryForm scope={scope} scopeId={scopeId} onDone={() => setShowAdd(false)} />
      )}
    </div>
  )
}
