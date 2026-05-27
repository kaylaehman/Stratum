import { useState } from 'react'
import { BookText, Plus, X, Pencil, Trash2, Save, Loader, ListChecks } from 'lucide-react'
import { useCan } from '../../lib/roles'
import {
  useRunbooks,
  useCreateRunbook,
  useUpdateRunbook,
  useDeleteRunbook,
} from '../../lib/api/runbooks'
import { ApiError } from '../../lib/api'
import type { Runbook, RunbookRequest } from '../../types/api'

// ── Helpers ───────────────────────────────────────────────────────────────────

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
    fontFamily: 'monospace',
    fontSize: '0.75rem',
  }
}

function actionBtnStyle(
  variant: 'ok' | 'err' | 'neutral' | 'accent',
  disabled: boolean,
): React.CSSProperties {
  const colors = {
    ok: { bg: 'rgba(64,200,120,0.10)', border: 'var(--status-ok)', color: 'var(--status-ok)' },
    err: { bg: 'rgba(232,64,64,0.10)', border: 'var(--status-error)', color: 'var(--status-error)' },
    neutral: { bg: 'var(--bg-elevated)', border: 'var(--border-default)', color: 'var(--text-secondary)' },
    accent: { bg: 'var(--accent)', border: 'var(--accent)', color: '#fff' },
  }[variant]
  return {
    background: colors.bg,
    border: `1px solid ${colors.border}`,
    color: disabled ? 'var(--text-muted)' : colors.color,
    borderRadius: '3px',
    padding: '4px 12px',
    fontSize: '0.7rem',
    fontFamily: 'monospace',
    cursor: disabled ? 'not-allowed' : 'pointer',
    opacity: disabled ? 0.6 : 1,
    display: 'flex',
    alignItems: 'center',
    gap: '4px',
  }
}

function runbookErrMsg(err: unknown): string {
  if (err instanceof ApiError) {
    const code = (err.body as { error?: string })?.error
    if (code === 'name_required') return 'Name is required.'
    return `Error: ${code ?? err.status}`
  }
  return 'An unexpected error occurred.'
}

// ── StringListEditor ──────────────────────────────────────────────────────────

interface StringListEditorProps {
  label: string
  items: string[]
  onChange: (items: string[]) => void
  placeholder: string
  ordered?: boolean
}

function StringListEditor({ label, items, onChange, placeholder, ordered }: StringListEditorProps) {
  const [input, setInput] = useState('')
  const [focused, setFocused] = useState(false)
  const [focusedIdx, setFocusedIdx] = useState<number | null>(null)

  function add() {
    const v = input.trim()
    if (!v) return
    onChange([...items, v])
    setInput('')
  }

  function remove(idx: number) {
    onChange(items.filter((_, i) => i !== idx))
  }

  function update(idx: number, val: string) {
    const next = [...items]
    next[idx] = val
    onChange(next)
  }

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: '4px' }}>
      <span className="text-xs font-medium" style={{ color: 'var(--text-secondary)' }}>{label}</span>

      {items.length > 0 && (
        <div
          style={{
            border: '1px solid var(--border-subtle)',
            borderRadius: '3px',
            overflow: 'hidden',
          }}
        >
          {items.map((item, idx) => (
            <div
              key={idx}
              style={{
                display: 'flex',
                alignItems: 'center',
                gap: '6px',
                padding: '4px 6px',
                backgroundColor: idx % 2 === 0 ? 'var(--bg-elevated)' : 'var(--bg-surface)',
                borderBottom: idx < items.length - 1 ? '1px solid var(--border-subtle)' : 'none',
              }}
            >
              {ordered && (
                <span
                  className="font-mono text-xs"
                  style={{
                    color: 'var(--text-muted)',
                    minWidth: '18px',
                    textAlign: 'right',
                    flexShrink: 0,
                    userSelect: 'none',
                  }}
                >
                  {idx + 1}.
                </span>
              )}
              <input
                type="text"
                value={item}
                onChange={(e) => update(idx, e.target.value)}
                onFocus={() => setFocusedIdx(idx)}
                onBlur={() => setFocusedIdx(null)}
                className="font-mono text-xs"
                style={{
                  ...inputStyle(focusedIdx === idx),
                  padding: '2px 6px',
                }}
              />
              <button
                type="button"
                onClick={() => remove(idx)}
                title="Remove"
                style={{
                  background: 'transparent',
                  border: 'none',
                  cursor: 'pointer',
                  color: 'var(--text-muted)',
                  padding: '2px',
                  display: 'flex',
                  alignItems: 'center',
                  lineHeight: 1,
                  flexShrink: 0,
                }}
              >
                <X size={11} />
              </button>
            </div>
          ))}
        </div>
      )}

      <div style={{ display: 'flex', gap: '6px' }}>
        <input
          type="text"
          value={input}
          placeholder={placeholder}
          onChange={(e) => setInput(e.target.value)}
          onFocus={() => setFocused(true)}
          onBlur={() => setFocused(false)}
          onKeyDown={(e) => { if (e.key === 'Enter') { e.preventDefault(); add() } }}
          className="font-mono text-xs"
          style={inputStyle(focused)}
        />
        <button
          type="button"
          onClick={add}
          disabled={!input.trim()}
          style={{
            ...actionBtnStyle(input.trim() ? 'accent' : 'neutral', !input.trim()),
            whiteSpace: 'nowrap',
            flexShrink: 0,
          }}
        >
          <Plus size={11} />
          Add
        </button>
      </div>
    </div>
  )
}

// ── RunbookForm ───────────────────────────────────────────────────────────────

interface RunbookFormProps {
  initial?: Partial<RunbookRequest>
  isPending: boolean
  error: unknown
  onSubmit: (data: RunbookRequest) => void
  onCancel: () => void
  submitLabel: string
}

function RunbookForm({ initial, isPending, error, onSubmit, onCancel, submitLabel }: RunbookFormProps) {
  const [name, setName] = useState(initial?.name ?? '')
  const [description, setDescription] = useState(initial?.description ?? '')
  const [triggerConditions, setTriggerConditions] = useState<string[]>(initial?.trigger_conditions ?? [])
  const [steps, setSteps] = useState<string[]>(initial?.steps ?? [])
  const [requiresApproval, setRequiresApproval] = useState(initial?.requires_approval ?? false)
  const [nameFocused, setNameFocused] = useState(false)
  const [descFocused, setDescFocused] = useState(false)

  function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    onSubmit({
      name: name.trim(),
      description: description.trim(),
      trigger_conditions: triggerConditions,
      steps,
      requires_approval: requiresApproval,
    })
  }

  const nameErr = error ? runbookErrMsg(error) : null

  return (
    <form
      onSubmit={handleSubmit}
      style={{
        display: 'flex',
        flexDirection: 'column',
        gap: '12px',
        backgroundColor: 'var(--bg-elevated)',
        border: '1px solid var(--border-default)',
        borderRadius: '3px',
        padding: '14px',
      }}
    >
      {/* Name */}
      <div style={{ display: 'flex', flexDirection: 'column', gap: '3px' }}>
        <label className="text-xs font-medium" style={{ color: 'var(--text-secondary)' }}>
          Name <span style={{ color: 'var(--status-error)' }}>*</span>
        </label>
        <input
          required
          type="text"
          value={name}
          onChange={(e) => setName(e.target.value)}
          onFocus={() => setNameFocused(true)}
          onBlur={() => setNameFocused(false)}
          placeholder="e.g. Jellyfin permission reset"
          className="font-mono text-xs"
          style={inputStyle(nameFocused)}
        />
        {nameErr && (
          <span className="font-mono text-xs" style={{ color: 'var(--status-error)' }}>
            {nameErr}
          </span>
        )}
      </div>

      {/* Description */}
      <div style={{ display: 'flex', flexDirection: 'column', gap: '3px' }}>
        <label className="text-xs font-medium" style={{ color: 'var(--text-secondary)' }}>
          Description
        </label>
        <textarea
          value={description}
          onChange={(e) => setDescription(e.target.value)}
          onFocus={() => setDescFocused(true)}
          onBlur={() => setDescFocused(false)}
          rows={2}
          placeholder="When and why this runbook applies"
          className="font-mono text-xs"
          style={{ ...inputStyle(descFocused), resize: 'vertical' }}
        />
      </div>

      {/* Trigger conditions */}
      <StringListEditor
        label="Trigger Conditions"
        items={triggerConditions}
        onChange={setTriggerConditions}
        placeholder="e.g. Jellyfin exits with permission denied"
      />

      {/* Steps */}
      <StringListEditor
        label="Steps (ordered)"
        items={steps}
        onChange={setSteps}
        placeholder="e.g. chown -R 1000:1000 /config"
        ordered
      />

      {/* Requires approval */}
      <label
        style={{
          display: 'flex',
          alignItems: 'center',
          gap: '8px',
          cursor: 'pointer',
          userSelect: 'none',
        }}
      >
        <input
          type="checkbox"
          checked={requiresApproval}
          onChange={(e) => setRequiresApproval(e.target.checked)}
          style={{ accentColor: 'var(--accent)', cursor: 'pointer' }}
        />
        <span className="text-xs" style={{ color: 'var(--text-secondary)' }}>
          Requires approval before executing
        </span>
      </label>

      {/* Actions */}
      <div style={{ display: 'flex', gap: '8px' }}>
        <button
          type="submit"
          disabled={isPending || !name.trim()}
          style={actionBtnStyle('accent', isPending || !name.trim())}
        >
          {isPending ? <Loader size={11} className="animate-spin" /> : <Save size={11} />}
          {submitLabel}
        </button>
        <button type="button" onClick={onCancel} style={actionBtnStyle('neutral', false)}>
          <X size={11} />
          Cancel
        </button>
      </div>
    </form>
  )
}

// ── RunbookRow ────────────────────────────────────────────────────────────────

interface RunbookRowProps {
  runbook: Runbook
  isOperator: boolean
}

function RunbookRow({ runbook, isOperator }: RunbookRowProps) {
  const [expanded, setExpanded] = useState(false)
  const [editing, setEditing] = useState(false)
  const [confirmDelete, setConfirmDelete] = useState(false)
  const update = useUpdateRunbook()
  const del = useDeleteRunbook()

  function handleUpdate(data: RunbookRequest) {
    update.mutate(
      { id: runbook.id, body: data },
      { onSuccess: () => setEditing(false) },
    )
  }

  function handleDelete() {
    del.mutate(runbook.id, { onSuccess: () => setConfirmDelete(false) })
  }

  if (editing) {
    return (
      <RunbookForm
        initial={runbook}
        isPending={update.isPending}
        error={update.isError ? update.error : null}
        onSubmit={handleUpdate}
        onCancel={() => setEditing(false)}
        submitLabel="Save"
      />
    )
  }

  return (
    <div
      style={{
        backgroundColor: 'var(--bg-elevated)',
        border: '1px solid var(--border-subtle)',
        borderRadius: '3px',
        overflow: 'hidden',
      }}
    >
      {/* Header */}
      <div
        style={{
          display: 'flex',
          alignItems: 'flex-start',
          gap: '10px',
          padding: '10px 12px',
          cursor: 'pointer',
        }}
        onClick={() => setExpanded((v) => !v)}
      >
        <ListChecks size={13} style={{ color: 'var(--accent)', flexShrink: 0, marginTop: '1px' }} />

        <div style={{ flex: 1, minWidth: 0 }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: '8px', flexWrap: 'wrap' }}>
            <span className="text-xs font-medium" style={{ color: 'var(--text-primary)' }}>
              {runbook.name}
            </span>

            {runbook.requires_approval && (
              <span
                className="font-mono text-xs"
                style={{
                  padding: '1px 6px',
                  borderRadius: '3px',
                  border: '1px solid var(--status-warn)',
                  color: 'var(--status-warn)',
                  backgroundColor: 'rgba(240,160,32,0.10)',
                  fontSize: '0.6rem',
                  fontWeight: 600,
                  textTransform: 'uppercase',
                  letterSpacing: '0.05em',
                  flexShrink: 0,
                }}
              >
                approval required
              </span>
            )}
          </div>

          {runbook.description && (
            <p
              className="text-xs"
              style={{ color: 'var(--text-muted)', margin: '2px 0 0', lineHeight: 1.4 }}
            >
              {runbook.description}
            </p>
          )}

          {/* Trigger chips */}
          {runbook.trigger_conditions.length > 0 && (
            <div style={{ display: 'flex', flexWrap: 'wrap', gap: '4px', marginTop: '6px' }}>
              {runbook.trigger_conditions.map((t, i) => (
                <span
                  key={i}
                  className="font-mono text-xs"
                  style={{
                    padding: '1px 6px',
                    borderRadius: '3px',
                    border: '1px solid var(--border-subtle)',
                    color: 'var(--text-secondary)',
                    backgroundColor: 'var(--bg-surface)',
                    fontSize: '0.65rem',
                  }}
                >
                  {t}
                </span>
              ))}
            </div>
          )}
        </div>

        {/* Operator controls */}
        {isOperator && (
          <div
            style={{ display: 'flex', alignItems: 'center', gap: '4px', flexShrink: 0 }}
            onClick={(e) => e.stopPropagation()}
          >
            <button
              type="button"
              title="Edit runbook"
              onClick={() => setEditing(true)}
              style={{
                background: 'transparent',
                border: 'none',
                cursor: 'pointer',
                color: 'var(--text-muted)',
                padding: '3px',
                display: 'flex',
                alignItems: 'center',
                borderRadius: '3px',
              }}
            >
              <Pencil size={12} />
            </button>
            <button
              type="button"
              title="Delete runbook"
              onClick={() => setConfirmDelete(true)}
              style={{
                background: 'transparent',
                border: 'none',
                cursor: 'pointer',
                color: 'var(--status-error)',
                padding: '3px',
                display: 'flex',
                alignItems: 'center',
                borderRadius: '3px',
              }}
            >
              <Trash2 size={12} />
            </button>
          </div>
        )}
      </div>

      {/* Expanded steps */}
      {expanded && runbook.steps.length > 0 && (
        <div
          style={{
            borderTop: '1px solid var(--border-subtle)',
            padding: '8px 12px',
          }}
        >
          <ol
            style={{
              margin: 0,
              paddingLeft: '20px',
              display: 'flex',
              flexDirection: 'column',
              gap: '4px',
            }}
          >
            {runbook.steps.map((step, i) => (
              <li
                key={i}
                className="font-mono text-xs"
                style={{ color: 'var(--text-secondary)', lineHeight: 1.5 }}
              >
                {step}
              </li>
            ))}
          </ol>
        </div>
      )}

      {expanded && runbook.steps.length === 0 && (
        <div style={{ borderTop: '1px solid var(--border-subtle)', padding: '8px 12px' }}>
          <span className="font-mono text-xs" style={{ color: 'var(--text-muted)' }}>
            No steps defined.
          </span>
        </div>
      )}

      {/* Delete confirm */}
      {confirmDelete && (
        <div
          style={{
            borderTop: '1px solid var(--status-error)',
            padding: '10px 12px',
            backgroundColor: 'rgba(232,64,64,0.07)',
            display: 'flex',
            flexDirection: 'column',
            gap: '8px',
          }}
        >
          <span className="font-mono text-xs" style={{ color: 'var(--text-primary)' }}>
            Delete runbook <strong>{runbook.name}</strong>?
          </span>
          {del.isError && (
            <span className="font-mono text-xs" style={{ color: 'var(--status-error)' }}>
              {runbookErrMsg(del.error)}
            </span>
          )}
          <div style={{ display: 'flex', gap: '6px' }}>
            <button
              type="button"
              onClick={handleDelete}
              disabled={del.isPending}
              style={actionBtnStyle('err', del.isPending)}
            >
              {del.isPending ? <Loader size={11} className="animate-spin" /> : <Trash2 size={11} />}
              Delete
            </button>
            <button
              type="button"
              onClick={() => setConfirmDelete(false)}
              style={actionBtnStyle('neutral', false)}
            >
              Cancel
            </button>
          </div>
        </div>
      )}
    </div>
  )
}

// ── RunbooksSection (exported) ────────────────────────────────────────────────

export function RunbooksSection() {
  const { isOperator } = useCan()
  const { data, isLoading } = useRunbooks()
  const create = useCreateRunbook()
  const [showCreate, setShowCreate] = useState(false)

  const runbooks = data?.runbooks ?? []

  function handleCreate(body: RunbookRequest) {
    create.mutate(body, { onSuccess: () => setShowCreate(false) })
  }

  return (
    <section
      style={{
        backgroundColor: 'var(--bg-surface)',
        border: '1px solid var(--border-default)',
        borderRadius: '3px',
        padding: '20px',
      }}
    >
      {/* Header */}
      <div style={{ display: 'flex', alignItems: 'center', gap: '10px', marginBottom: '16px' }}>
        <BookText size={16} style={{ color: 'var(--accent)' }} />
        <div style={{ flex: 1 }}>
          <h2 className="text-sm font-semibold" style={{ color: 'var(--text-primary)', margin: 0 }}>
            AI Runbooks
          </h2>
          <p className="text-xs" style={{ color: 'var(--text-muted)', margin: 0 }}>
            Saved diagnostic and remediation procedures the assistant can reference
          </p>
        </div>
        {isOperator && !showCreate && (
          <button
            type="button"
            onClick={() => setShowCreate(true)}
            style={{
              display: 'flex',
              alignItems: 'center',
              gap: '4px',
              backgroundColor: 'var(--accent)',
              color: '#fff',
              border: 'none',
              borderRadius: '3px',
              padding: '4px 12px',
              fontSize: '0.7rem',
              fontWeight: 500,
              cursor: 'pointer',
            }}
          >
            <Plus size={12} />
            New Runbook
          </button>
        )}
      </div>

      {/* Create form */}
      {showCreate && (
        <div style={{ marginBottom: '14px' }}>
          <RunbookForm
            isPending={create.isPending}
            error={create.isError ? create.error : null}
            onSubmit={handleCreate}
            onCancel={() => setShowCreate(false)}
            submitLabel="Create"
          />
        </div>
      )}

      {/* Loading */}
      {isLoading && (
        <div style={{ display: 'flex', alignItems: 'center', gap: '8px', color: 'var(--text-muted)' }}>
          <Loader size={14} className="animate-spin" />
          <span className="text-xs">Loading runbooks…</span>
        </div>
      )}

      {/* List */}
      {!isLoading && runbooks.length > 0 && (
        <div style={{ display: 'flex', flexDirection: 'column', gap: '8px' }}>
          {runbooks.map((rb) => (
            <RunbookRow key={rb.id} runbook={rb} isOperator={isOperator} />
          ))}
        </div>
      )}

      {/* Empty state */}
      {!isLoading && runbooks.length === 0 && !showCreate && (
        <div
          style={{
            backgroundColor: 'var(--bg-elevated)',
            border: '1px solid var(--border-subtle)',
            borderRadius: '3px',
            padding: '16px',
            display: 'flex',
            flexDirection: 'column',
            gap: '6px',
          }}
        >
          <p className="text-xs" style={{ color: 'var(--text-muted)', margin: 0 }}>
            No runbooks yet. Save a procedure (e.g.{' '}
            <span style={{ color: 'var(--text-secondary)', fontStyle: 'italic' }}>
              &ldquo;Jellyfin permission reset&rdquo;
            </span>
            ) so the assistant can suggest it when diagnosing issues.
          </p>
          <p className="text-xs" style={{ color: 'var(--text-muted)', margin: 0, lineHeight: 1.5 }}>
            The assistant references runbooks automatically when a trigger condition matches what it observes in logs or a diagnostic result.
          </p>
        </div>
      )}
    </section>
  )
}
