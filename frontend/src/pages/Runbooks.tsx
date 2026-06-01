import { useState } from 'react'
import {
  BookText,
  Plus,
  X,
  Pencil,
  Trash2,
  Save,
  Loader,
  ListChecks,
  ChevronDown,
  ChevronRight,
  CheckCircle,
  AlertCircle,
  ShieldAlert,
  Download,
  Upload,
  RefreshCw,
} from 'lucide-react'
import { AppShell } from '../components/layout/AppShell'
import { ApiError } from '../lib/api'
import {
  useRunbooks,
  useCreateRunbook,
  useUpdateRunbook,
  useDeleteRunbook,
  useValidateRunbook,
} from '../lib/api/runbooks'
import { useCan } from '../lib/roles'
import type { Runbook, RunbookRequest, RunbookValidationResult } from '../lib/api/runbooks'

// ── Styles ────────────────────────────────────────────────────────────────────

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
  variant: 'ok' | 'err' | 'neutral' | 'accent' | 'warn',
  disabled: boolean,
): React.CSSProperties {
  const colors = {
    ok: { bg: 'rgba(64,200,120,0.10)', border: 'var(--status-ok)', color: 'var(--status-ok)' },
    err: { bg: 'rgba(232,64,64,0.10)', border: 'var(--status-error)', color: 'var(--status-error)' },
    warn: { bg: 'rgba(240,160,32,0.10)', border: 'var(--status-warn)', color: 'var(--status-warn)' },
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

function riskBadgeStyle(risk: string): React.CSSProperties {
  const colorMap: Record<string, string> = {
    low: 'var(--status-ok)',
    medium: 'var(--status-warn)',
    high: 'var(--status-warn)',
    destructive: 'var(--status-error)',
  }
  const color = colorMap[risk] ?? 'var(--text-muted)'
  return {
    padding: '1px 6px',
    borderRadius: '3px',
    border: `1px solid ${color}`,
    color,
    backgroundColor: `${color}18`,
    fontSize: '0.6rem',
    fontWeight: 600,
    textTransform: 'uppercase' as const,
    letterSpacing: '0.05em',
    flexShrink: 0,
    fontFamily: 'monospace',
  }
}

function runbookErrMsg(err: unknown): string {
  if (err instanceof ApiError) {
    const code = (err.body as { error?: string })?.error
    if (code === 'name_required') return 'Name is required.'
    return `Error: ${code ?? err.status}`
  }
  return err instanceof Error ? err.message : 'An unexpected error occurred.'
}

// ── StringListEditor ──────────────────────────────────────────────────────────

interface StringListEditorProps {
  label: string
  items: string[]
  onChange: (items: string[]) => void
  placeholder: string
  ordered?: boolean
  stepRisks?: string[]
}

function StringListEditor({ label, items, onChange, placeholder, ordered, stepRisks }: StringListEditorProps) {
  const [input, setInput] = useState('')
  const [focused, setFocused] = useState(false)
  const [focusedIdx, setFocusedIdx] = useState<number | null>(null)

  function add() {
    const v = input.trim()
    if (!v) return
    onChange([...items, v])
    setInput('')
  }

  function remove(idx: number) { onChange(items.filter((_, i) => i !== idx)) }
  function update(idx: number, val: string) {
    const next = [...items]; next[idx] = val; onChange(next)
  }

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: '4px' }}>
      <span className="text-xs font-medium" style={{ color: 'var(--text-secondary)' }}>{label}</span>
      {items.length > 0 && (
        <div style={{ border: '1px solid var(--border-subtle)', borderRadius: '3px', overflow: 'hidden' }}>
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
                <span className="font-mono text-xs" style={{ color: 'var(--text-muted)', minWidth: '18px', textAlign: 'right', flexShrink: 0 }}>
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
                style={{ ...inputStyle(focusedIdx === idx), padding: '2px 6px' }}
              />
              {stepRisks && stepRisks[idx] && (
                <span style={riskBadgeStyle(stepRisks[idx])}>{stepRisks[idx]}</span>
              )}
              <button
                type="button"
                onClick={() => remove(idx)}
                title="Remove"
                style={{ background: 'transparent', border: 'none', cursor: 'pointer', color: 'var(--text-muted)', padding: '2px', display: 'flex', alignItems: 'center', flexShrink: 0 }}
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
          style={{ ...actionBtnStyle(input.trim() ? 'accent' : 'neutral', !input.trim()), whiteSpace: 'nowrap', flexShrink: 0 }}
        >
          <Plus size={11} /> Add
        </button>
      </div>
    </div>
  )
}

// ── ValidationPanel ──────────────────────────────────────────────────────────

interface ValidationPanelProps {
  result: RunbookValidationResult
}

function ValidationPanel({ result }: ValidationPanelProps) {
  return (
    <div
      style={{
        backgroundColor: result.valid ? 'rgba(34,201,122,0.06)' : 'rgba(232,64,64,0.06)',
        border: `1px solid ${result.valid ? 'rgba(34,201,122,0.3)' : 'rgba(232,64,64,0.3)'}`,
        borderRadius: '3px',
        padding: '10px 12px',
        display: 'flex',
        flexDirection: 'column',
        gap: '6px',
      }}
    >
      <div style={{ display: 'flex', alignItems: 'center', gap: '6px' }}>
        {result.valid
          ? <CheckCircle size={13} style={{ color: 'var(--status-ok)', flexShrink: 0 }} />
          : <AlertCircle size={13} style={{ color: 'var(--status-error)', flexShrink: 0 }} />
        }
        <span className="font-mono text-xs font-semibold" style={{ color: result.valid ? 'var(--status-ok)' : 'var(--status-error)' }}>
          {result.valid ? 'Runbook is valid' : `${result.errors.length} lint error${result.errors.length !== 1 ? 's' : ''}`}
        </span>
      </div>
      {result.errors.map((e, i) => (
        <div key={i} style={{ display: 'flex', gap: '6px', paddingLeft: '19px' }}>
          <span style={riskBadgeStyle(e.risk)}>{e.risk}</span>
          <span className="font-mono text-xs" style={{ color: 'var(--status-error)', lineHeight: 1.4 }}>
            {e.message}
          </span>
        </div>
      ))}
      {result.warnings.length > 0 && (
        <>
          <span className="font-mono text-xs" style={{ color: 'var(--status-warn)', paddingLeft: '19px', fontWeight: 600 }}>
            {result.warnings.length} warning{result.warnings.length !== 1 ? 's' : ''}
          </span>
          {result.warnings.map((w, i) => (
            <div key={i} style={{ display: 'flex', gap: '6px', paddingLeft: '19px' }}>
              <span style={riskBadgeStyle(w.risk)}>{w.risk}</span>
              <span className="font-mono text-xs" style={{ color: 'var(--status-warn)', lineHeight: 1.4 }}>
                {w.message}
              </span>
            </div>
          ))}
        </>
      )}
    </div>
  )
}

// ── RunbookForm ───────────────────────────────────────────────────────────────

interface RunbookFormProps {
  initial?: Partial<RunbookRequest>
  runbookId?: string
  isPending: boolean
  error: unknown
  onSubmit: (data: RunbookRequest) => void
  onCancel: () => void
  submitLabel: string
}

function RunbookForm({ initial, runbookId, isPending, error, onSubmit, onCancel, submitLabel }: RunbookFormProps) {
  const [name, setName] = useState(initial?.name ?? '')
  const [description, setDescription] = useState(initial?.description ?? '')
  const [triggerConditions, setTriggerConditions] = useState<string[]>(initial?.trigger_conditions ?? [])
  const [steps, setSteps] = useState<string[]>(initial?.steps ?? [])
  const [requiresApproval, setRequiresApproval] = useState(initial?.requires_approval ?? false)
  const [nameFocused, setNameFocused] = useState(false)
  const [descFocused, setDescFocused] = useState(false)
  const [validationResult, setValidationResult] = useState<RunbookValidationResult | null>(null)

  const validate = useValidateRunbook()

  function handleValidate() {
    if (!runbookId) return
    validate.mutate(runbookId, {
      onSuccess: (res) => setValidationResult(res),
    })
  }

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
        {nameErr && <span className="font-mono text-xs" style={{ color: 'var(--status-error)' }}>{nameErr}</span>}
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
        stepRisks={validationResult?.step_risks}
      />

      {/* Requires approval */}
      <label style={{ display: 'flex', alignItems: 'center', gap: '8px', cursor: 'pointer', userSelect: 'none' }}>
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

      {/* Validation result */}
      {validationResult && <ValidationPanel result={validationResult} />}

      {/* Actions */}
      <div style={{ display: 'flex', gap: '8px', flexWrap: 'wrap' }}>
        <button
          type="submit"
          disabled={isPending || !name.trim()}
          style={actionBtnStyle('accent', isPending || !name.trim())}
        >
          {isPending ? <Loader size={11} className="animate-spin" /> : <Save size={11} />}
          {submitLabel}
        </button>
        {runbookId && (
          <button
            type="button"
            onClick={handleValidate}
            disabled={validate.isPending}
            style={actionBtnStyle('warn', validate.isPending)}
          >
            {validate.isPending ? <Loader size={11} className="animate-spin" /> : <ShieldAlert size={11} />}
            Lint
          </button>
        )}
        <button type="button" onClick={onCancel} style={actionBtnStyle('neutral', false)}>
          <X size={11} /> Cancel
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
  const [validationResult, setValidationResult] = useState<RunbookValidationResult | null>(null)
  const update = useUpdateRunbook()
  const del = useDeleteRunbook()
  const validate = useValidateRunbook()

  function handleUpdate(data: RunbookRequest) {
    update.mutate({ id: runbook.id, body: data }, { onSuccess: () => setEditing(false) })
  }

  function handleDelete() {
    del.mutate(runbook.id, { onSuccess: () => setConfirmDelete(false) })
  }

  function handleLint() {
    validate.mutate(runbook.id, { onSuccess: (res) => { setValidationResult(res); setExpanded(true) } })
  }

  function handleExport() {
    const data = JSON.stringify(runbook, null, 2)
    const blob = new Blob([data], { type: 'application/json' })
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = `runbook-${runbook.name.replace(/\s+/g, '-').toLowerCase()}.json`
    a.click()
    URL.revokeObjectURL(url)
  }

  if (editing) {
    return (
      <RunbookForm
        initial={runbook}
        runbookId={runbook.id}
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
        border: `1px solid ${validationResult && !validationResult.valid ? 'rgba(232,64,64,0.4)' : 'var(--border-subtle)'}`,
        borderRadius: '3px',
        overflow: 'hidden',
      }}
    >
      {/* Header row */}
      <div
        style={{ display: 'flex', alignItems: 'flex-start', gap: '10px', padding: '10px 12px', cursor: 'pointer' }}
        onClick={() => setExpanded((v) => !v)}
      >
        <span style={{ color: 'var(--text-muted)', flexShrink: 0, marginTop: '2px' }}>
          {expanded ? <ChevronDown size={13} /> : <ChevronRight size={13} />}
        </span>
        <ListChecks size={13} style={{ color: 'var(--accent)', flexShrink: 0, marginTop: '1px' }} />

        <div style={{ flex: 1, minWidth: 0 }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: '8px', flexWrap: 'wrap' }}>
            <span className="text-xs font-medium" style={{ color: 'var(--text-primary)' }}>
              {runbook.name}
            </span>
            {runbook.requires_approval && (
              <span style={{ padding: '1px 6px', borderRadius: '3px', border: '1px solid var(--status-warn)', color: 'var(--status-warn)', backgroundColor: 'rgba(240,160,32,0.10)', fontSize: '0.6rem', fontWeight: 600, textTransform: 'uppercase', letterSpacing: '0.05em', flexShrink: 0, fontFamily: 'monospace' }}>
                approval required
              </span>
            )}
            {validationResult && (
              <span style={{ padding: '1px 6px', borderRadius: '3px', border: `1px solid ${validationResult.valid ? 'var(--status-ok)' : 'var(--status-error)'}`, color: validationResult.valid ? 'var(--status-ok)' : 'var(--status-error)', backgroundColor: validationResult.valid ? 'rgba(34,201,122,0.10)' : 'rgba(232,64,64,0.10)', fontSize: '0.6rem', fontWeight: 600, textTransform: 'uppercase', letterSpacing: '0.05em', flexShrink: 0, fontFamily: 'monospace' }}>
                {validationResult.valid ? 'lint ok' : `${validationResult.errors.length} lint err`}
              </span>
            )}
          </div>
          {runbook.description && (
            <p className="text-xs" style={{ color: 'var(--text-muted)', margin: '2px 0 0', lineHeight: 1.4 }}>
              {runbook.description}
            </p>
          )}
          {runbook.trigger_conditions.length > 0 && (
            <div style={{ display: 'flex', flexWrap: 'wrap', gap: '4px', marginTop: '6px' }}>
              {runbook.trigger_conditions.map((t, i) => (
                <span key={i} className="font-mono" style={{ padding: '1px 6px', borderRadius: '3px', border: '1px solid var(--border-subtle)', color: 'var(--text-secondary)', backgroundColor: 'var(--bg-surface)', fontSize: '0.65rem' }}>
                  {t}
                </span>
              ))}
            </div>
          )}
        </div>

        {/* Operator controls */}
        {isOperator && (
          <div style={{ display: 'flex', alignItems: 'center', gap: '4px', flexShrink: 0 }} onClick={(e) => e.stopPropagation()}>
            <button
              type="button"
              title="Lint runbook"
              onClick={handleLint}
              disabled={validate.isPending}
              style={{ background: 'transparent', border: 'none', cursor: 'pointer', color: 'var(--status-warn)', padding: '3px', display: 'flex', alignItems: 'center', borderRadius: '3px' }}
            >
              {validate.isPending ? <Loader size={12} className="animate-spin" /> : <ShieldAlert size={12} />}
            </button>
            <button type="button" title="Export runbook" onClick={handleExport} style={{ background: 'transparent', border: 'none', cursor: 'pointer', color: 'var(--text-muted)', padding: '3px', display: 'flex', alignItems: 'center', borderRadius: '3px' }}>
              <Download size={12} />
            </button>
            <button type="button" title="Edit runbook" onClick={() => setEditing(true)} style={{ background: 'transparent', border: 'none', cursor: 'pointer', color: 'var(--text-muted)', padding: '3px', display: 'flex', alignItems: 'center', borderRadius: '3px' }}>
              <Pencil size={12} />
            </button>
            <button type="button" title="Delete runbook" onClick={() => setConfirmDelete(true)} style={{ background: 'transparent', border: 'none', cursor: 'pointer', color: 'var(--status-error)', padding: '3px', display: 'flex', alignItems: 'center', borderRadius: '3px' }}>
              <Trash2 size={12} />
            </button>
          </div>
        )}
      </div>

      {/* Expanded body */}
      {expanded && (
        <div style={{ borderTop: '1px solid var(--border-subtle)', padding: '10px 12px', display: 'flex', flexDirection: 'column', gap: '10px' }}>
          {/* Steps */}
          {runbook.steps.length > 0 ? (
            <div>
              <p className="text-xs font-medium uppercase tracking-wider" style={{ color: 'var(--text-muted)', marginBottom: '6px' }}>
                Steps
              </p>
              <ol style={{ margin: 0, paddingLeft: '20px', display: 'flex', flexDirection: 'column', gap: '4px' }}>
                {runbook.steps.map((step, i) => (
                  <li key={i} style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
                    <code className="font-mono text-xs" style={{ color: 'var(--text-secondary)', lineHeight: 1.5, flex: 1 }}>
                      {step}
                    </code>
                    {validationResult?.step_risks[i] && (
                      <span style={riskBadgeStyle(validationResult.step_risks[i])}>
                        {validationResult.step_risks[i]}
                      </span>
                    )}
                  </li>
                ))}
              </ol>
            </div>
          ) : (
            <span className="font-mono text-xs" style={{ color: 'var(--text-muted)' }}>No steps defined.</span>
          )}

          {/* Lint result */}
          {validationResult && <ValidationPanel result={validationResult} />}
        </div>
      )}

      {/* Delete confirm */}
      {confirmDelete && (
        <div style={{ borderTop: '1px solid var(--status-error)', padding: '10px 12px', backgroundColor: 'rgba(232,64,64,0.07)', display: 'flex', flexDirection: 'column', gap: '8px' }}>
          <span className="font-mono text-xs" style={{ color: 'var(--text-primary)' }}>
            Delete runbook <strong>{runbook.name}</strong>?
          </span>
          {del.isError && (
            <span className="font-mono text-xs" style={{ color: 'var(--status-error)' }}>
              {runbookErrMsg(del.error)}
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

// ── Import modal ──────────────────────────────────────────────────────────────

interface ImportModalProps {
  onImport: (data: RunbookRequest) => void
  onClose: () => void
}

function ImportModal({ onImport, onClose }: ImportModalProps) {
  const [raw, setRaw] = useState('')
  const [parseErr, setParseErr] = useState('')
  const [focused, setFocused] = useState(false)

  function handleImport() {
    try {
      const obj = JSON.parse(raw)
      const req: RunbookRequest = {
        name: String(obj.name ?? ''),
        description: String(obj.description ?? ''),
        trigger_conditions: Array.isArray(obj.trigger_conditions) ? obj.trigger_conditions.map(String) : [],
        steps: Array.isArray(obj.steps) ? obj.steps.map(String) : [],
        requires_approval: Boolean(obj.requires_approval),
      }
      if (!req.name) { setParseErr('Imported JSON must have a non-empty "name" field.'); return }
      setParseErr('')
      onImport(req)
    } catch {
      setParseErr('Invalid JSON. Paste the exported runbook JSON.')
    }
  }

  return (
    <div
      style={{
        position: 'fixed', inset: 0, zIndex: 200,
        backgroundColor: 'rgba(0,0,0,0.5)',
        display: 'flex', alignItems: 'center', justifyContent: 'center',
      }}
      onClick={onClose}
    >
      <div
        onClick={(e) => e.stopPropagation()}
        style={{
          backgroundColor: 'var(--bg-surface)',
          border: '1px solid var(--border-default)',
          borderRadius: '4px',
          padding: '20px',
          width: '520px',
          maxWidth: '90vw',
          display: 'flex',
          flexDirection: 'column',
          gap: '12px',
        }}
      >
        <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
          <Upload size={14} style={{ color: 'var(--accent)' }} />
          <span className="text-sm font-semibold" style={{ color: 'var(--text-primary)' }}>Import Runbook</span>
          <button type="button" onClick={onClose} style={{ marginLeft: 'auto', background: 'none', border: 'none', cursor: 'pointer', color: 'var(--text-muted)' }}>
            <X size={14} />
          </button>
        </div>
        <p className="text-xs" style={{ color: 'var(--text-muted)', margin: 0 }}>
          Paste the JSON from a previously exported runbook.
        </p>
        <textarea
          value={raw}
          onChange={(e) => setRaw(e.target.value)}
          onFocus={() => setFocused(true)}
          onBlur={() => setFocused(false)}
          rows={8}
          placeholder='{ "name": "...", "steps": [...], ... }'
          className="font-mono text-xs"
          style={{ ...inputStyle(focused), resize: 'vertical' }}
        />
        {parseErr && <span className="font-mono text-xs" style={{ color: 'var(--status-error)' }}>{parseErr}</span>}
        <div style={{ display: 'flex', gap: '8px' }}>
          <button type="button" onClick={handleImport} disabled={!raw.trim()} style={actionBtnStyle('accent', !raw.trim())}>
            <Upload size={11} /> Import
          </button>
          <button type="button" onClick={onClose} style={actionBtnStyle('neutral', false)}>
            Cancel
          </button>
        </div>
      </div>
    </div>
  )
}

// ── Page ──────────────────────────────────────────────────────────────────────

export default function RunbooksPage() {
  const { isOperator } = useCan()
  const { data, isLoading, refetch, isFetching } = useRunbooks()
  const create = useCreateRunbook()
  const [showCreate, setShowCreate] = useState(false)
  const [showImport, setShowImport] = useState(false)
  const [search, setSearch] = useState('')
  const [searchFocused, setSearchFocused] = useState(false)

  const runbooks = data?.runbooks ?? []
  const filtered = search.trim()
    ? runbooks.filter(
        (rb) =>
          rb.name.toLowerCase().includes(search.toLowerCase()) ||
          rb.description.toLowerCase().includes(search.toLowerCase()) ||
          rb.trigger_conditions.some((t) => t.toLowerCase().includes(search.toLowerCase())),
      )
    : runbooks

  function handleCreate(body: RunbookRequest) {
    create.mutate(body, { onSuccess: () => setShowCreate(false) })
  }

  function handleImport(body: RunbookRequest) {
    create.mutate(body, { onSuccess: () => setShowImport(false) })
  }

  return (
    <AppShell>
      <div style={{ padding: '28px', maxWidth: '860px', margin: '0 auto', display: 'flex', flexDirection: 'column', gap: '20px' }}>
        {/* Page header */}
        <div style={{ display: 'flex', alignItems: 'flex-start', gap: '12px' }}>
          <BookText size={20} style={{ color: 'var(--accent)', flexShrink: 0, marginTop: '2px' }} />
          <div style={{ flex: 1, minWidth: 0 }}>
            <h1 className="text-base font-semibold" style={{ color: 'var(--text-primary)', margin: 0 }}>
              Runbooks
            </h1>
            <p className="text-xs" style={{ color: 'var(--text-muted)', margin: '2px 0 0' }}>
              Saved diagnostic and remediation procedures. Steps are lint-checked via the risk classifier — destructive commands require approval.
            </p>
          </div>
          <div style={{ display: 'flex', gap: '6px', flexShrink: 0 }}>
            <button
              type="button"
              title="Refresh"
              onClick={() => void refetch()}
              disabled={isFetching}
              style={{ ...actionBtnStyle('neutral', isFetching), padding: '4px 8px' }}
            >
              <RefreshCw size={12} style={{ animation: isFetching ? 'spin 1s linear infinite' : 'none' }} />
            </button>
            {isOperator && (
              <>
                <button type="button" onClick={() => setShowImport(true)} style={{ ...actionBtnStyle('neutral', false), padding: '4px 10px' }}>
                  <Upload size={11} /> Import
                </button>
                <button type="button" onClick={() => { setShowCreate(true); setShowImport(false) }} style={{ ...actionBtnStyle('accent', false), padding: '4px 12px' }}>
                  <Plus size={12} /> New Runbook
                </button>
              </>
            )}
          </div>
        </div>

        {/* Search */}
        {runbooks.length > 0 && (
          <input
            type="search"
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            onFocus={() => setSearchFocused(true)}
            onBlur={() => setSearchFocused(false)}
            placeholder="Search runbooks…"
            className="font-mono text-xs"
            style={{ ...inputStyle(searchFocused), maxWidth: '360px' }}
          />
        )}

        {/* Create form */}
        {showCreate && (
          <RunbookForm
            isPending={create.isPending}
            error={create.isError ? create.error : null}
            onSubmit={handleCreate}
            onCancel={() => setShowCreate(false)}
            submitLabel="Create"
          />
        )}

        {/* Loading */}
        {isLoading && (
          <div style={{ display: 'flex', alignItems: 'center', gap: '8px', color: 'var(--text-muted)' }}>
            <Loader size={14} className="animate-spin" />
            <span className="text-xs">Loading runbooks…</span>
          </div>
        )}

        {/* List */}
        {!isLoading && filtered.length > 0 && (
          <div style={{ display: 'flex', flexDirection: 'column', gap: '8px' }}>
            {filtered.map((rb) => (
              <RunbookRow key={rb.id} runbook={rb} isOperator={isOperator} />
            ))}
          </div>
        )}

        {/* No results from search */}
        {!isLoading && search.trim() && filtered.length === 0 && (
          <p className="text-xs" style={{ color: 'var(--text-muted)' }}>
            No runbooks match &ldquo;{search}&rdquo;.
          </p>
        )}

        {/* Empty state */}
        {!isLoading && runbooks.length === 0 && !showCreate && (
          <div style={{ backgroundColor: 'var(--bg-elevated)', border: '1px solid var(--border-subtle)', borderRadius: '3px', padding: '20px', display: 'flex', flexDirection: 'column', gap: '8px' }}>
            <BookText size={24} style={{ color: 'var(--text-muted)' }} />
            <p className="text-xs" style={{ color: 'var(--text-muted)', margin: 0 }}>
              No runbooks yet. Create a procedure (e.g.{' '}
              <em style={{ color: 'var(--text-secondary)' }}>&ldquo;Jellyfin permission reset&rdquo;</em>
              ) so the AI assistant can reference it when diagnosing issues.
            </p>
            <p className="text-xs" style={{ color: 'var(--text-muted)', margin: 0, lineHeight: 1.5 }}>
              You can also save a runbook directly from a{' '}
              <span style={{ color: 'var(--accent)' }}>diagnostic result</span>{' '}
              using the &ldquo;Save as runbook&rdquo; action.
            </p>
            {isOperator && (
              <button type="button" onClick={() => setShowCreate(true)} style={{ ...actionBtnStyle('accent', false), alignSelf: 'flex-start', marginTop: '4px' }}>
                <Plus size={11} /> Create your first runbook
              </button>
            )}
          </div>
        )}

        {/* Footer info */}
        {!isLoading && runbooks.length > 0 && (
          <p className="text-xs" style={{ color: 'var(--text-muted)' }}>
            {runbooks.length} runbook{runbooks.length !== 1 ? 's' : ''} total
            {search.trim() && filtered.length !== runbooks.length
              ? ` — ${filtered.length} shown`
              : ''}
            . Use the lint button (<ShieldAlert size={11} style={{ display: 'inline', verticalAlign: 'middle' }} />) to validate risk classification.
          </p>
        )}
      </div>

      {/* Import modal */}
      {showImport && <ImportModal onImport={handleImport} onClose={() => setShowImport(false)} />}
    </AppShell>
  )
}
