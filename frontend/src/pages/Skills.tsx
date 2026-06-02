import { useEffect, useMemo, useState } from 'react'
import { Link } from 'react-router-dom'
import {
  Wrench,
  Loader,
  Search,
  ChevronDown,
  ChevronRight,
  ExternalLink,
  X,
  AlertTriangle,
  Terminal,
  ShieldAlert,
  Plus,
  Pencil,
  Trash2,
  Copy,
  Sparkles,
  Save,
} from 'lucide-react'
import CodeMirror from '@uiw/react-codemirror'
import type { LanguageSupport } from '@codemirror/language'
import { AppShell } from '../components/layout/AppShell'
import { ApiError } from '../lib/api'
import {
  useSkills,
  useSkill,
  fetchSkillRaw,
  useCreateSkill,
  useUpdateSkill,
  useDeleteSkill,
  useGenerateSkill,
} from '../lib/api/skills'
import { useTree } from '../lib/api/tree'
import { resolveLanguage } from '../lib/codemirror'
import type { SkillSummary, SkillStep, Container } from '../types/api'

// Minimal commented YAML scaffold shown when creating a brand-new skill.
const NEW_SKILL_TEMPLATE = `# Stratum troubleshooting skill
# Change "id" to something unique before saving.
id: my-skill
name: My Skill
version: "1.0"
category: Custom
description: Short summary of what this skill helps troubleshoot.
docs_url: ""

container_match:
  # Image name substrings that auto-match this skill to a container.
  image_patterns:
    - myorg/app
  # Exposed container ports that hint at this skill.
  port_hints:
    - 8080

common_issues:
  - id: example-issue
    name: Example issue
    symptoms:
      - Describe a user-visible symptom here.
    trigger_conditions:
      - log_pattern: "ERROR.*connection refused"
    steps:
      - id: step-1
        description: Explain what this step checks or does.
        type: command
        command: "echo 'replace with a real command'"
        requires_approval: false
`

// Extract a human-readable error detail from an ApiError body, falling back to message.
function apiErrorMessage(err: unknown): { error: string; detail: string } {
  if (err instanceof ApiError) {
    const body = (err.body ?? {}) as Record<string, unknown>
    const error = typeof body.error === 'string' ? body.error : ''
    const detail = typeof body.detail === 'string' ? body.detail : ''
    return { error, detail }
  }
  return { error: '', detail: err instanceof Error ? err.message : String(err) }
}

// ---- Chip ----

function Chip({ label }: { label: string }) {
  return (
    <span
      className="font-mono px-1.5 py-0.5"
      style={{
        background: 'var(--accent-glow)',
        border: '1px solid var(--accent-dim)',
        color: 'var(--accent)',
        borderRadius: '3px',
        fontSize: '12px',
      }}
    >
      {label}
    </span>
  )
}

function PortChip({ port }: { port: number }) {
  return (
    <span
      className="font-mono px-1.5 py-0.5"
      style={{
        background: 'rgba(74,82,104,0.2)',
        border: '1px solid var(--border-default)',
        color: 'var(--text-muted)',
        borderRadius: '3px',
        fontSize: '12px',
      }}
    >
      :{port}
    </span>
  )
}

function CustomBadge() {
  return (
    <span
      className="font-mono shrink-0 px-1.5 py-0.5 uppercase tracking-wider"
      style={{
        background: 'rgba(124,156,232,0.12)',
        border: '1px solid var(--accent-dim)',
        color: 'var(--accent)',
        borderRadius: '3px',
        fontSize: '11px',
      }}
    >
      Custom
    </span>
  )
}

// ---- Skill card (list item) ----

interface SkillCardProps {
  skill: SkillSummary
  selected: boolean
  onSelect: (id: string) => void
}

function SkillCard({ skill, selected, onSelect }: SkillCardProps) {
  return (
    <button
      type="button"
      onClick={() => onSelect(skill.id)}
      className="text-left w-full"
      style={{
        backgroundColor: selected ? 'var(--accent-glow)' : 'var(--bg-surface)',
        border: `1px solid ${selected ? 'var(--accent-dim)' : 'var(--border-subtle)'}`,
        borderRadius: '3px',
        padding: '12px 14px',
        display: 'flex',
        flexDirection: 'column',
        gap: '8px',
        cursor: 'pointer',
      }}
    >
      <div className="flex items-start justify-between gap-2">
        <span
          className="text-sm font-medium"
          style={{ color: selected ? 'var(--accent)' : 'var(--text-primary)' }}
        >
          {skill.name}
        </span>
        <div className="flex items-center gap-1 shrink-0">
          {skill.source === 'custom' && <CustomBadge />}
          {/* Count of catalogued troubleshooting guides, not live alarms — so use
              calm neutral styling and the word "guides", not amber "issues". */}
          <span
            className="font-mono px-1.5 py-0.5"
            title="Troubleshooting guides catalogued for this skill"
            style={{
              background: 'rgba(74,82,104,0.2)',
              border: '1px solid var(--border-default)',
              color: 'var(--text-muted)',
              borderRadius: '3px',
              fontSize: '12px',
            }}
          >
            {skill.issue_count} {skill.issue_count === 1 ? 'guide' : 'guides'}
          </span>
        </div>
      </div>

      {skill.description && (
        <p className="text-sm" style={{ color: 'var(--text-secondary)', lineHeight: '1.5' }}>
          {skill.description}
        </p>
      )}

      {(skill.image_patterns.length > 0 || skill.port_hints.length > 0) && (
        <div className="flex flex-wrap items-center gap-1">
          {skill.image_patterns.map((p) => (
            <Chip key={p} label={p} />
          ))}
          {skill.port_hints.map((p) => (
            <PortChip key={p} port={p} />
          ))}
        </div>
      )}
    </button>
  )
}

// ---- Category group ----

interface CategoryGroupProps {
  category: string
  skills: SkillSummary[]
  selectedId: string | null
  onSelect: (id: string) => void
}

function CategoryGroup({ category, skills, selectedId, onSelect }: CategoryGroupProps) {
  const [open, setOpen] = useState(true)

  return (
    <div className="mb-5">
      <button
        type="button"
        onClick={() => setOpen((v) => !v)}
        className="flex items-center gap-2 w-full px-0 py-2 mb-3"
        style={{
          background: 'none',
          border: 'none',
          borderBottom: '1px solid var(--border-subtle)',
          cursor: 'pointer',
        }}
      >
        {open ? (
          <ChevronDown size={13} style={{ color: 'var(--text-muted)' }} />
        ) : (
          <ChevronRight size={13} style={{ color: 'var(--text-muted)' }} />
        )}
        <span
          className="text-xs font-medium uppercase tracking-wider"
          style={{ color: 'var(--text-muted)' }}
        >
          {category}
        </span>
        <span className="text-xs font-mono" style={{ color: 'var(--text-muted)' }}>
          ({skills.length})
        </span>
      </button>
      {open && (
        <div
          style={{
            display: 'grid',
            gridTemplateColumns: 'repeat(auto-fill, minmax(280px, 1fr))',
            gap: '12px',
          }}
        >
          {skills.map((s) => (
            <SkillCard key={s.id} skill={s} selected={s.id === selectedId} onSelect={onSelect} />
          ))}
        </div>
      )}
    </div>
  )
}

// ---- Fix step ----

function StepRow({ step }: { step: SkillStep }) {
  return (
    <div
      className="flex flex-col gap-1.5 px-3 py-2.5"
      style={{ borderBottom: '1px solid var(--border-subtle)' }}
    >
      <div className="flex items-start gap-2">
        <Terminal size={12} style={{ color: 'var(--text-muted)', flexShrink: 0, marginTop: '2px' }} />
        <span className="text-sm flex-1" style={{ color: 'var(--text-secondary)', lineHeight: '1.5' }}>
          {step.description}
        </span>
        {step.requires_approval && (
          <span
            className="flex items-center gap-1 font-mono shrink-0 px-1.5 py-0.5"
            style={{
              background: 'rgba(232,64,64,0.12)',
              border: '1px solid rgba(232,64,64,0.35)',
              color: 'var(--status-error)',
              borderRadius: '3px',
              fontSize: '12px',
            }}
            title="This step requires explicit approval before running"
          >
            <ShieldAlert size={10} />
            approval
          </span>
        )}
      </div>
      {step.command && (
        <pre
          className="font-mono text-xs px-2.5 py-2 m-0"
          style={{
            backgroundColor: 'var(--bg-surface)',
            border: '1px solid var(--border-subtle)',
            borderRadius: '3px',
            color: 'var(--text-primary)',
            overflowX: 'auto',
            whiteSpace: 'pre-wrap',
            wordBreak: 'break-all',
          }}
        >
          {step.command}
        </pre>
      )}
    </div>
  )
}

// ---- Skill editor modal (create / edit / clone) ----

type EditorMode =
  | { kind: 'create' }
  | { kind: 'clone'; fromId: string }
  | { kind: 'edit'; id: string }

interface SkillEditorModalProps {
  mode: EditorMode
  containers: Container[]
  onClose: () => void
  onSaved: (id: string) => void
}

function SkillEditorModal({ mode, containers, onClose, onSaved }: SkillEditorModalProps) {
  const [yaml, setYaml] = useState('')
  const [loadingSource, setLoadingSource] = useState(mode.kind !== 'create')
  const [sourceError, setSourceError] = useState<string | null>(null)
  const [saveError, setSaveError] = useState<string | null>(null)
  const [langExt, setLangExt] = useState<LanguageSupport | null>(null)

  // Generate-with-AI panel state
  const [genOpen, setGenOpen] = useState(false)
  const [genTarget, setGenTarget] = useState<'container' | 'image'>(
    containers.length > 0 ? 'container' : 'image',
  )
  const [genContainerId, setGenContainerId] = useState(containers[0]?.id ?? '')
  const [genImage, setGenImage] = useState('')
  const [genNotes, setGenNotes] = useState('')
  const [genWarning, setGenWarning] = useState<string | null>(null)
  const [genError, setGenError] = useState<{ text: string; toSettings: boolean } | null>(null)

  const createMut = useCreateSkill()
  const updateMut = useUpdateSkill()
  const generateMut = useGenerateSkill()

  const isEdit = mode.kind === 'edit'

  useEffect(() => {
    void resolveLanguage('skill.yaml').then((l) => setLangExt(l))
  }, [])

  // Load initial YAML: scaffold for create, raw source for edit/clone.
  useEffect(() => {
    let cancelled = false
    if (mode.kind === 'create') {
      setYaml(NEW_SKILL_TEMPLATE)
      setLoadingSource(false)
      return
    }
    const id = mode.kind === 'edit' ? mode.id : mode.fromId
    setLoadingSource(true)
    setSourceError(null)
    void fetchSkillRaw(id)
      .then((raw) => {
        if (cancelled) return
        if (mode.kind === 'clone') {
          setYaml(
            `# Cloned from built-in "${id}". Change the "id" field to a new unique value before saving.\n` +
              raw.yaml,
          )
        } else {
          setYaml(raw.yaml)
        }
        setLoadingSource(false)
      })
      .catch((e: unknown) => {
        if (cancelled) return
        setSourceError(e instanceof Error ? e.message : 'Failed to load skill source')
        setLoadingSource(false)
      })
    return () => {
      cancelled = true
    }
  }, [mode])

  const title =
    mode.kind === 'edit' ? 'Edit skill' : mode.kind === 'clone' ? 'Clone skill' : 'New skill'

  async function handleSave() {
    setSaveError(null)
    try {
      if (mode.kind === 'edit') {
        const detail = await updateMut.mutateAsync({ id: mode.id, yaml })
        onSaved(detail.id)
      } else {
        const detail = await createMut.mutateAsync(yaml)
        onSaved(detail.id)
      }
    } catch (e) {
      const { error, detail } = apiErrorMessage(e)
      if (error === 'id_exists') {
        setSaveError(
          detail || 'A skill with that id already exists. Change the "id" field to a unique value.',
        )
      } else if (error === 'builtin_readonly') {
        setSaveError('Built-in skills are read-only. Clone it instead.')
      } else if (error === 'id_mismatch') {
        setSaveError('The "id" in the YAML must match the skill being edited.')
      } else if (error === 'invalid_skill') {
        setSaveError(detail || 'The skill YAML is invalid.')
      } else {
        setSaveError(detail || 'Failed to save skill.')
      }
    }
  }

  async function handleGenerate() {
    setGenError(null)
    setGenWarning(null)
    const req =
      genTarget === 'container'
        ? { container_id: genContainerId, notes: genNotes.trim() || undefined }
        : { image: genImage.trim(), notes: genNotes.trim() || undefined }

    if (genTarget === 'container' && !genContainerId) {
      setGenError({ text: 'Select a container to generate from.', toSettings: false })
      return
    }
    if (genTarget === 'image' && !genImage.trim()) {
      setGenError({ text: 'Enter an image reference (e.g. myorg/app:latest).', toSettings: false })
      return
    }

    try {
      const result = await generateMut.mutateAsync(req)
      setYaml(result.yaml)
      setSaveError(null)
      if (!result.valid) {
        setGenWarning(
          `The AI returned YAML that did not parse cleanly${
            result.parse_error ? `: ${result.parse_error}` : ''
          }. Review and fix it before saving.`,
        )
      }
      setGenOpen(false)
    } catch (e) {
      const { error, detail } = apiErrorMessage(e)
      if (error === 'ai_not_configured') {
        setGenError({
          text: 'No AI provider is configured. Set one up in Settings, then try again.',
          toSettings: true,
        })
      } else if (error === 'image_or_container_required') {
        setGenError({ text: 'An image or container is required.', toSettings: false })
      } else if (error === 'ai_request_failed') {
        setGenError({
          text: detail ? `AI request failed: ${detail}` : 'The AI request failed. Try again.',
          toSettings: false,
        })
      } else {
        setGenError({ text: detail || 'Generation failed.', toSettings: false })
      }
    }
  }

  const saving = createMut.isPending || updateMut.isPending

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center p-6"
      style={{ background: 'rgba(0,0,0,0.6)' }}
      onClick={onClose}
    >
      <div
        onClick={(e) => e.stopPropagation()}
        className="flex flex-col"
        style={{
          backgroundColor: 'var(--bg-elevated)',
          border: '1px solid var(--border-default)',
          borderRadius: '3px',
          width: '900px',
          maxWidth: '100%',
          maxHeight: 'calc(100vh - 48px)',
        }}
      >
        {/* Header */}
        <div
          className="flex items-center gap-2 px-4 py-3 shrink-0"
          style={{ borderBottom: '1px solid var(--border-subtle)' }}
        >
          <Wrench size={13} style={{ color: 'var(--text-secondary)' }} />
          <span
            className="text-xs font-medium uppercase tracking-wider flex-1"
            style={{ color: 'var(--text-primary)' }}
          >
            {title}
          </span>
          <button
            type="button"
            onClick={() => setGenOpen((v) => !v)}
            className="flex items-center gap-1 text-xs px-2 py-1"
            style={{
              background: genOpen ? 'var(--accent-glow)' : 'var(--bg-surface)',
              border: `1px solid ${genOpen ? 'var(--accent-dim)' : 'var(--border-default)'}`,
              color: 'var(--accent)',
              borderRadius: '3px',
              cursor: 'pointer',
            }}
          >
            <Sparkles size={11} />
            Generate with AI
          </button>
          <button
            type="button"
            onClick={onClose}
            style={{ background: 'none', border: 'none', color: 'var(--text-muted)', cursor: 'pointer', padding: '2px' }}
            title="Close"
          >
            <X size={14} />
          </button>
        </div>

        {/* Generate-with-AI panel */}
        {genOpen && (
          <div
            className="px-4 py-3 shrink-0 flex flex-col gap-2.5"
            style={{ borderBottom: '1px solid var(--border-subtle)', backgroundColor: 'var(--bg-surface)' }}
          >
            <span className="text-xs uppercase tracking-wider font-medium" style={{ color: 'var(--text-muted)' }}>
              Draft from a container or image
            </span>

            {/* Target toggle */}
            <div className="flex items-center gap-3 flex-wrap">
              {containers.length > 0 && (
                <label className="flex items-center gap-1.5 text-sm" style={{ color: 'var(--text-secondary)', cursor: 'pointer' }}>
                  <input
                    type="radio"
                    checked={genTarget === 'container'}
                    onChange={() => setGenTarget('container')}
                  />
                  Running container
                </label>
              )}
              <label className="flex items-center gap-1.5 text-sm" style={{ color: 'var(--text-secondary)', cursor: 'pointer' }}>
                <input
                  type="radio"
                  checked={genTarget === 'image'}
                  onChange={() => setGenTarget('image')}
                />
                Image reference
              </label>
            </div>

            {genTarget === 'container' && containers.length > 0 ? (
              <select
                value={genContainerId}
                onChange={(e) => setGenContainerId(e.target.value)}
                className="text-sm px-2 py-1.5"
                style={{
                  background: 'var(--bg-elevated)',
                  border: '1px solid var(--border-default)',
                  color: 'var(--text-primary)',
                  borderRadius: '3px',
                  outline: 'none',
                }}
              >
                {containers.map((c) => (
                  <option key={c.id} value={c.id}>
                    {c.name} — {c.image}
                  </option>
                ))}
              </select>
            ) : (
              <input
                type="text"
                value={genImage}
                onChange={(e) => setGenImage(e.target.value)}
                placeholder="myorg/app:latest"
                className="text-sm font-mono px-2 py-1.5"
                style={{
                  background: 'var(--bg-elevated)',
                  border: '1px solid var(--border-default)',
                  color: 'var(--text-primary)',
                  borderRadius: '3px',
                  outline: 'none',
                }}
              />
            )}

            <textarea
              value={genNotes}
              onChange={(e) => setGenNotes(e.target.value)}
              placeholder="Optional notes for the AI (symptoms you're seeing, what to focus on)..."
              rows={2}
              className="text-sm px-2 py-1.5"
              style={{
                background: 'var(--bg-elevated)',
                border: '1px solid var(--border-default)',
                color: 'var(--text-primary)',
                borderRadius: '3px',
                outline: 'none',
                resize: 'vertical',
                fontFamily: 'inherit',
              }}
            />

            {genError && (
              <div className="text-xs flex items-center gap-1.5" style={{ color: 'var(--status-error)' }}>
                <AlertTriangle size={12} style={{ flexShrink: 0 }} />
                <span className="flex-1">{genError.text}</span>
                {genError.toSettings && (
                  <Link
                    to="/settings"
                    className="flex items-center gap-1 shrink-0"
                    style={{ color: 'var(--accent)', textDecoration: 'none' }}
                  >
                    Open Settings
                    <ExternalLink size={10} />
                  </Link>
                )}
              </div>
            )}

            <button
              type="button"
              onClick={() => void handleGenerate()}
              disabled={generateMut.isPending}
              className="flex items-center gap-1.5 text-xs self-start px-3 py-1.5"
              style={{
                background: 'var(--accent-glow)',
                border: '1px solid var(--accent-dim)',
                color: 'var(--accent)',
                borderRadius: '3px',
                cursor: generateMut.isPending ? 'default' : 'pointer',
              }}
            >
              {generateMut.isPending ? <Loader size={11} className="animate-spin" /> : <Sparkles size={11} />}
              {generateMut.isPending ? 'Generating...' : 'Generate draft'}
            </button>
          </div>
        )}

        {/* Generate validity warning (non-blocking) */}
        {genWarning && (
          <div
            className="flex items-start gap-2 px-4 py-2 text-xs shrink-0"
            style={{ backgroundColor: 'rgba(240,160,32,0.1)', borderBottom: '1px solid var(--border-subtle)', color: 'var(--status-warn)' }}
          >
            <AlertTriangle size={12} style={{ flexShrink: 0, marginTop: '1px' }} />
            <span className="flex-1">{genWarning}</span>
            <button
              type="button"
              onClick={() => setGenWarning(null)}
              style={{ background: 'none', border: 'none', color: 'var(--status-warn)', cursor: 'pointer', padding: 0 }}
            >
              <X size={12} />
            </button>
          </div>
        )}

        {/* Editor body */}
        <div className="flex-1 overflow-auto" style={{ minHeight: '300px' }}>
          {loadingSource ? (
            <div className="flex items-center gap-2 px-4 py-6">
              <Loader size={13} className="animate-spin" style={{ color: 'var(--accent)' }} />
              <span className="text-xs" style={{ color: 'var(--text-muted)' }}>Loading source...</span>
            </div>
          ) : sourceError ? (
            <div className="px-4 py-4 text-xs" style={{ color: 'var(--status-error)' }}>
              {sourceError}
            </div>
          ) : (
            <CodeMirror
              value={yaml}
              onChange={setYaml}
              extensions={langExt ? [langExt] : []}
              editable={true}
              basicSetup={{ lineNumbers: true, foldGutter: true }}
              theme="dark"
              style={{ fontSize: '12px', fontFamily: "'Space Mono', monospace" }}
            />
          )}
        </div>

        {/* Save error (inline, keeps editor open) */}
        {saveError && (
          <div
            className="flex items-start gap-2 px-4 py-2 text-xs shrink-0"
            style={{ backgroundColor: 'rgba(232,64,64,0.1)', borderTop: '1px solid var(--border-subtle)', color: 'var(--status-error)' }}
          >
            <AlertTriangle size={12} style={{ flexShrink: 0, marginTop: '1px' }} />
            <span className="flex-1" style={{ whiteSpace: 'pre-wrap', wordBreak: 'break-word' }}>{saveError}</span>
          </div>
        )}

        {/* Footer */}
        <div
          className="flex items-center justify-end gap-2 px-4 py-3 shrink-0"
          style={{ borderTop: '1px solid var(--border-subtle)' }}
        >
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
            type="button"
            onClick={() => void handleSave()}
            disabled={saving || loadingSource || !!sourceError}
            className="flex items-center gap-1.5 text-xs px-3 py-1.5"
            style={{
              background: 'var(--accent-glow)',
              border: '1px solid var(--accent-dim)',
              color: 'var(--accent)',
              borderRadius: '3px',
              cursor: saving || loadingSource ? 'default' : 'pointer',
            }}
          >
            {saving ? <Loader size={11} className="animate-spin" /> : <Save size={11} />}
            {isEdit ? 'Save changes' : 'Create skill'}
          </button>
        </div>
      </div>
    </div>
  )
}

// ---- Delete confirm ----

function DeleteConfirm({
  name,
  pending,
  error,
  onConfirm,
  onCancel,
}: {
  name: string
  pending: boolean
  error: string | null
  onConfirm: () => void
  onCancel: () => void
}) {
  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center p-6"
      style={{ background: 'rgba(0,0,0,0.6)' }}
      onClick={onCancel}
    >
      <div
        onClick={(e) => e.stopPropagation()}
        className="flex flex-col gap-3 p-4"
        style={{
          backgroundColor: 'var(--bg-elevated)',
          border: '1px solid var(--border-default)',
          borderRadius: '3px',
          width: '420px',
          maxWidth: '100%',
        }}
      >
        <div className="flex items-center gap-2">
          <Trash2 size={14} style={{ color: 'var(--status-error)' }} />
          <span className="text-sm font-medium" style={{ color: 'var(--text-primary)' }}>
            Delete skill
          </span>
        </div>
        <p className="text-sm" style={{ color: 'var(--text-secondary)', lineHeight: '1.5' }}>
          Delete <strong style={{ color: 'var(--text-primary)' }}>{name}</strong>? This cannot be undone.
        </p>
        {error && (
          <p className="text-xs" style={{ color: 'var(--status-error)' }}>{error}</p>
        )}
        <div className="flex items-center justify-end gap-2">
          <button
            type="button"
            onClick={onCancel}
            className="text-xs px-3 py-1.5"
            style={{ background: 'var(--bg-surface)', border: '1px solid var(--border-default)', color: 'var(--text-secondary)', borderRadius: '3px', cursor: 'pointer' }}
          >
            Cancel
          </button>
          <button
            type="button"
            onClick={onConfirm}
            disabled={pending}
            className="flex items-center gap-1.5 text-xs px-3 py-1.5"
            style={{
              background: 'rgba(232,64,64,0.12)',
              border: '1px solid rgba(232,64,64,0.4)',
              color: 'var(--status-error)',
              borderRadius: '3px',
              cursor: pending ? 'default' : 'pointer',
            }}
          >
            {pending ? <Loader size={11} className="animate-spin" /> : <Trash2 size={11} />}
            Delete
          </button>
        </div>
      </div>
    </div>
  )
}

// ---- Detail panel ----

interface SkillDetailPanelProps {
  id: string
  onClose: () => void
  onEdit: (id: string) => void
  onClone: (id: string) => void
  onDelete: (skill: { id: string; name: string }) => void
}

function SkillDetailPanel({ id, onClose, onEdit, onClone, onDelete }: SkillDetailPanelProps) {
  const { data: skill, isLoading } = useSkill(id, true)
  const [openIssue, setOpenIssue] = useState<string | null>(null)

  return (
    <div
      style={{
        position: 'sticky',
        top: '24px',
        alignSelf: 'flex-start',
        backgroundColor: 'var(--bg-elevated)',
        border: '1px solid var(--border-default)',
        borderRadius: '3px',
        display: 'flex',
        flexDirection: 'column',
        maxHeight: 'calc(100vh - 120px)',
      }}
    >
      {/* Header */}
      <div
        className="flex items-center gap-2 px-4 py-3 shrink-0"
        style={{ borderBottom: '1px solid var(--border-subtle)' }}
      >
        <Wrench size={13} style={{ color: 'var(--text-secondary)' }} />
        <span
          className="text-xs font-medium uppercase tracking-wider flex-1 truncate"
          style={{ color: 'var(--text-primary)' }}
        >
          {skill?.name ?? 'Skill'}
        </span>
        {skill?.source === 'custom' && <CustomBadge />}
        {skill?.version && (
          <span className="font-mono text-xs shrink-0" style={{ color: 'var(--text-muted)' }}>
            v{skill.version}
          </span>
        )}
        <button
          type="button"
          onClick={onClose}
          style={{ background: 'none', border: 'none', color: 'var(--text-muted)', cursor: 'pointer', padding: '2px' }}
          title="Close"
        >
          <X size={14} />
        </button>
      </div>

      <div className="flex-1 overflow-auto p-4 flex flex-col gap-4">
        {isLoading && (
          <div className="flex items-center gap-2 py-4">
            <Loader size={13} className="animate-spin" style={{ color: 'var(--accent)' }} />
            <span className="text-xs" style={{ color: 'var(--text-muted)' }}>Loading skill...</span>
          </div>
        )}

        {!isLoading && skill && (
          <>
            {/* Action bar */}
            <div className="flex items-center gap-2 flex-wrap">
              {skill.editable ? (
                <>
                  <button
                    type="button"
                    onClick={() => onEdit(skill.id)}
                    className="flex items-center gap-1.5 text-xs px-2.5 py-1"
                    style={{ background: 'var(--bg-surface)', border: '1px solid var(--border-default)', color: 'var(--text-primary)', borderRadius: '3px', cursor: 'pointer' }}
                  >
                    <Pencil size={11} />
                    Edit
                  </button>
                  <button
                    type="button"
                    onClick={() => onDelete({ id: skill.id, name: skill.name })}
                    className="flex items-center gap-1.5 text-xs px-2.5 py-1"
                    style={{ background: 'rgba(232,64,64,0.1)', border: '1px solid rgba(232,64,64,0.35)', color: 'var(--status-error)', borderRadius: '3px', cursor: 'pointer' }}
                  >
                    <Trash2 size={11} />
                    Delete
                  </button>
                </>
              ) : (
                <button
                  type="button"
                  onClick={() => onClone(skill.id)}
                  className="flex items-center gap-1.5 text-xs px-2.5 py-1"
                  style={{ background: 'var(--bg-surface)', border: '1px solid var(--border-default)', color: 'var(--text-primary)', borderRadius: '3px', cursor: 'pointer' }}
                  title="Built-in skills are read-only. Clone to make an editable copy."
                >
                  <Copy size={11} />
                  Clone
                </button>
              )}
            </div>

            {skill.description && (
              <p className="text-sm" style={{ color: 'var(--text-secondary)', lineHeight: '1.6' }}>
                {skill.description}
              </p>
            )}

            {/* Meta chips */}
            {(skill.image_patterns.length > 0 || skill.port_hints.length > 0) && (
              <div className="flex flex-col gap-2">
                {skill.image_patterns.length > 0 && (
                  <div className="flex flex-col gap-1.5">
                    <span className="text-xs uppercase tracking-wider font-medium" style={{ color: 'var(--text-muted)' }}>
                      Image Patterns
                    </span>
                    <div className="flex flex-wrap gap-1">
                      {skill.image_patterns.map((p) => <Chip key={p} label={p} />)}
                    </div>
                  </div>
                )}
                {skill.port_hints.length > 0 && (
                  <div className="flex flex-col gap-1.5">
                    <span className="text-xs uppercase tracking-wider font-medium" style={{ color: 'var(--text-muted)' }}>
                      Port Hints
                    </span>
                    <div className="flex flex-wrap gap-1">
                      {skill.port_hints.map((p) => <PortChip key={p} port={p} />)}
                    </div>
                  </div>
                )}
              </div>
            )}

            {skill.docs_url && (
              <a
                href={skill.docs_url}
                target="_blank"
                rel="noreferrer noopener"
                className="flex items-center gap-1.5 text-xs self-start px-2 py-1"
                style={{
                  backgroundColor: 'var(--bg-surface)',
                  border: '1px solid var(--border-default)',
                  color: 'var(--accent)',
                  borderRadius: '3px',
                  textDecoration: 'none',
                }}
              >
                <ExternalLink size={11} />
                Documentation
              </a>
            )}

            {/* Common issues */}
            <div className="flex flex-col gap-2">
              <span className="text-xs uppercase tracking-wider font-medium" style={{ color: 'var(--text-muted)' }}>
                Common Issues ({skill.common_issues.length})
              </span>
              {skill.common_issues.length === 0 ? (
                <span className="text-xs" style={{ color: 'var(--text-muted)' }}>
                  No common issues documented.
                </span>
              ) : (
                skill.common_issues.map((issue) => {
                  const isOpen = openIssue === issue.id
                  return (
                    <div
                      key={issue.id}
                      style={{
                        backgroundColor: 'var(--bg-surface)',
                        border: '1px solid var(--border-subtle)',
                        borderRadius: '3px',
                      }}
                    >
                      <button
                        type="button"
                        onClick={() => setOpenIssue(isOpen ? null : issue.id)}
                        className="flex items-center gap-2 w-full px-3 py-2 text-left"
                        style={{ background: 'none', border: 'none', cursor: 'pointer' }}
                      >
                        {isOpen ? (
                          <ChevronDown size={12} style={{ color: 'var(--text-muted)', flexShrink: 0 }} />
                        ) : (
                          <ChevronRight size={12} style={{ color: 'var(--text-muted)', flexShrink: 0 }} />
                        )}
                        <AlertTriangle size={12} style={{ color: 'var(--status-warn)', flexShrink: 0 }} />
                        <span className="text-sm font-medium flex-1" style={{ color: 'var(--text-primary)' }}>
                          {issue.name}
                        </span>
                        <span className="font-mono text-xs shrink-0" style={{ color: 'var(--text-muted)' }}>
                          {issue.steps.length} {issue.steps.length === 1 ? 'step' : 'steps'}
                        </span>
                      </button>

                      {isOpen && (
                        <div style={{ borderTop: '1px solid var(--border-subtle)' }}>
                          {/* Symptoms */}
                          {issue.symptoms.length > 0 && (
                            <div className="px-3 py-2.5" style={{ borderBottom: '1px solid var(--border-subtle)' }}>
                              <span className="text-xs uppercase tracking-wider font-medium block mb-1.5" style={{ color: 'var(--text-muted)' }}>
                                Symptoms
                              </span>
                              <ul className="flex flex-col gap-1" style={{ listStyle: 'none', padding: 0, margin: 0 }}>
                                {issue.symptoms.map((s, i) => (
                                  <li
                                    key={i}
                                    className="text-sm flex items-start gap-1.5"
                                    style={{ color: 'var(--text-secondary)', lineHeight: '1.5' }}
                                  >
                                    <span style={{ color: 'var(--text-muted)', flexShrink: 0 }}>•</span>
                                    {s}
                                  </li>
                                ))}
                              </ul>
                            </div>
                          )}

                          {/* Fix steps */}
                          <div>
                            <div className="px-3 pt-2.5 pb-1">
                              <span className="text-xs uppercase tracking-wider font-medium" style={{ color: 'var(--text-muted)' }}>
                                Fix Steps
                              </span>
                            </div>
                            {issue.steps.length === 0 ? (
                              <div className="px-3 py-2 text-xs" style={{ color: 'var(--text-muted)' }}>
                                No steps documented.
                              </div>
                            ) : (
                              issue.steps.map((step) => <StepRow key={step.id} step={step} />)
                            )}
                          </div>
                        </div>
                      )}
                    </div>
                  )
                })
              )}
            </div>
          </>
        )}
      </div>
    </div>
  )
}

// ---- Main page ----

export default function Skills() {
  const { data, isLoading } = useSkills()
  const skills = useMemo(() => data?.skills ?? [], [data])

  const { data: treeData } = useTree()
  const containers = useMemo<Container[]>(() => {
    const list: Container[] = []
    for (const node of treeData?.nodes ?? []) {
      for (const c of node.containers) list.push(c)
    }
    return list
  }, [treeData])

  const [filter, setFilter] = useState('')
  const [selectedId, setSelectedId] = useState<string | null>(null)

  const [editorMode, setEditorMode] = useState<EditorMode | null>(null)
  const [deleteTarget, setDeleteTarget] = useState<{ id: string; name: string } | null>(null)
  const [deleteError, setDeleteError] = useState<string | null>(null)
  const deleteMut = useDeleteSkill()

  const filtered = useMemo(() => {
    const q = filter.trim().toLowerCase()
    if (!q) return skills
    return skills.filter((s) => {
      return (
        s.name.toLowerCase().includes(q) ||
        s.description.toLowerCase().includes(q) ||
        s.image_patterns.some((p) => p.toLowerCase().includes(q))
      )
    })
  }, [skills, filter])

  // Group by category, sorted alphabetically.
  const grouped = useMemo(() => {
    const map = new Map<string, SkillSummary[]>()
    for (const s of filtered) {
      const cat = s.category || 'Uncategorized'
      const list = map.get(cat)
      if (list) list.push(s)
      else map.set(cat, [s])
    }
    return Array.from(map.entries()).sort(([a], [b]) => a.localeCompare(b))
  }, [filtered])

  function handleConfirmDelete() {
    if (!deleteTarget) return
    setDeleteError(null)
    deleteMut.mutate(deleteTarget.id, {
      onSuccess: () => {
        if (selectedId === deleteTarget.id) setSelectedId(null)
        setDeleteTarget(null)
      },
      onError: (e) => {
        const { error, detail } = apiErrorMessage(e)
        setDeleteError(
          error === 'builtin_readonly'
            ? 'Built-in skills cannot be deleted.'
            : detail || 'Failed to delete skill.',
        )
      },
    })
  }

  return (
    <AppShell>
      <div
        className="flex flex-col flex-1 min-h-0 h-full w-full p-6"
        style={{ maxWidth: selectedId ? '1200px' : '1000px', margin: '0 auto' }}
      >
        {/* Page header */}
        <div className="flex items-center gap-2 mb-6">
          <Wrench size={16} style={{ color: 'var(--text-secondary)' }} />
          <h1 className="text-sm font-medium uppercase tracking-wider" style={{ color: 'var(--text-primary)' }}>
            Skills
          </h1>
          <span className="text-xs font-mono" style={{ color: 'var(--text-muted)' }}>
            ({skills.length})
          </span>
          <div className="flex-1" />
          {/* Search */}
          <div
            className="flex items-center gap-2 px-2.5 py-1.5"
            style={{
              backgroundColor: 'var(--bg-surface)',
              border: '1px solid var(--border-default)',
              borderRadius: '3px',
              width: '280px',
            }}
          >
            <Search size={12} style={{ color: 'var(--text-muted)', flexShrink: 0 }} />
            <input
              type="text"
              value={filter}
              onChange={(e) => setFilter(e.target.value)}
              placeholder="Filter by name, description, image..."
              className="text-sm flex-1"
              style={{
                background: 'transparent',
                border: 'none',
                color: 'var(--text-primary)',
                outline: 'none',
                minWidth: 0,
              }}
            />
            {filter && (
              <button
                type="button"
                onClick={() => setFilter('')}
                style={{ background: 'none', border: 'none', color: 'var(--text-muted)', cursor: 'pointer', padding: 0 }}
                title="Clear"
              >
                <X size={12} />
              </button>
            )}
          </div>
          {/* New skill */}
          <button
            type="button"
            onClick={() => setEditorMode({ kind: 'create' })}
            className="flex items-center gap-1.5 text-xs px-3 py-1.5 shrink-0"
            style={{
              background: 'var(--accent-glow)',
              border: '1px solid var(--accent-dim)',
              color: 'var(--accent)',
              borderRadius: '3px',
              cursor: 'pointer',
            }}
          >
            <Plus size={12} />
            New skill
          </button>
        </div>

        {/* Loading */}
        {isLoading && (
          <div className="flex items-center gap-2 py-8">
            <Loader size={13} className="animate-spin" style={{ color: 'var(--accent)' }} />
            <span className="text-xs" style={{ color: 'var(--text-muted)' }}>Loading skills...</span>
          </div>
        )}

        {/* Body: list + optional detail panel */}
        {!isLoading && (
          <div
            style={{
              display: 'grid',
              gridTemplateColumns: selectedId ? 'minmax(0, 1fr) 420px' : 'minmax(0, 1fr)',
              gap: '20px',
              alignItems: 'start',
            }}
          >
            {/* List column */}
            <div>
              {skills.length === 0 ? (
                <div
                  className="px-3 py-4 text-xs"
                  style={{
                    color: 'var(--text-muted)',
                    backgroundColor: 'var(--bg-surface)',
                    border: '1px solid var(--border-subtle)',
                    borderRadius: '3px',
                  }}
                >
                  No skills available.
                </div>
              ) : grouped.length === 0 ? (
                <div
                  className="px-3 py-4 text-xs"
                  style={{
                    color: 'var(--text-muted)',
                    backgroundColor: 'var(--bg-surface)',
                    border: '1px solid var(--border-subtle)',
                    borderRadius: '3px',
                  }}
                >
                  No skills match “{filter}”.
                </div>
              ) : (
                grouped.map(([category, items]) => (
                  <CategoryGroup
                    key={category}
                    category={category}
                    skills={items}
                    selectedId={selectedId}
                    onSelect={setSelectedId}
                  />
                ))
              )}
            </div>

            {/* Detail column */}
            {selectedId && (
              <SkillDetailPanel
                id={selectedId}
                onClose={() => setSelectedId(null)}
                onEdit={(id) => setEditorMode({ kind: 'edit', id })}
                onClone={(id) => setEditorMode({ kind: 'clone', fromId: id })}
                onDelete={(s) => {
                  setDeleteError(null)
                  setDeleteTarget(s)
                }}
              />
            )}
          </div>
        )}
      </div>

      {editorMode && (
        <SkillEditorModal
          mode={editorMode}
          containers={containers}
          onClose={() => setEditorMode(null)}
          onSaved={(id) => {
            setEditorMode(null)
            setSelectedId(id)
          }}
        />
      )}

      {deleteTarget && (
        <DeleteConfirm
          name={deleteTarget.name}
          pending={deleteMut.isPending}
          error={deleteError}
          onConfirm={handleConfirmDelete}
          onCancel={() => {
            setDeleteTarget(null)
            setDeleteError(null)
          }}
        />
      )}
    </AppShell>
  )
}
