import { useState, useEffect, useCallback } from 'react'
import { Terminal, Plus, Pencil, Trash2, Play, Loader, AlertTriangle, X } from 'lucide-react'
import CodeMirror from '@uiw/react-codemirror'
import type { LanguageSupport } from '@codemirror/language'
import { AppShell } from '../components/layout/AppShell'
import { useMe } from '../hooks/useMe'
import { useTree } from '../lib/api/tree'
import {
  useScripts,
  useCreateScript,
  useUpdateScript,
  useDeleteScript,
  useRunScript,
} from '../lib/api/scripts'
import { ApiError } from '../lib/api'
import { resolveLanguage } from '../lib/codemirror'
import type { Script, ScriptRunResult } from '../types/api'

// ---- Shared helpers ----

function ErrorBanner({ message }: { message: string }) {
  return (
    <div
      className="flex items-center gap-2 px-3 py-2 text-xs"
      style={{
        backgroundColor: 'rgba(232,64,64,0.1)',
        border: '1px solid rgba(232,64,64,0.3)',
        borderRadius: '3px',
        color: 'var(--status-error)',
      }}
    >
      <AlertTriangle size={11} style={{ flexShrink: 0 }} />
      {message}
    </div>
  )
}

function SectionHeader({ label, count }: { label: string; count?: number }) {
  return (
    <div
      className="px-0 py-2 mb-3 flex items-center gap-2"
      style={{ borderBottom: '1px solid var(--border-subtle)' }}
    >
      <span className="text-xs font-medium uppercase tracking-wider" style={{ color: 'var(--text-muted)' }}>
        {label}
      </span>
      {count !== undefined && (
        <span className="text-xs font-mono" style={{ color: 'var(--text-muted)' }}>({count})</span>
      )}
    </div>
  )
}

function extractError(err: unknown): string {
  if (err instanceof ApiError) {
    const body = err.body as Record<string, unknown>
    return String(body?.error ?? err.message)
  }
  return err instanceof Error ? err.message : 'Unknown error'
}

// ---- Confirm delete dialog ----

interface ConfirmDeleteProps {
  name: string
  onConfirm: () => void
  onCancel: () => void
  isPending: boolean
  error: string | null
}

function ConfirmDelete({ name, onConfirm, onCancel, isPending, error }: ConfirmDeleteProps) {
  return (
    <div
      style={{
        position: 'fixed',
        inset: 0,
        backgroundColor: 'rgba(0,0,0,0.55)',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        zIndex: 50,
      }}
    >
      <div
        style={{
          backgroundColor: 'var(--bg-elevated)',
          border: '1px solid var(--border-default)',
          borderRadius: '3px',
          padding: '20px 24px',
          maxWidth: '400px',
          width: '100%',
        }}
      >
        <div className="flex items-center gap-2 mb-3">
          <Trash2 size={14} style={{ color: 'var(--status-error)', flexShrink: 0 }} />
          <span className="text-xs font-medium uppercase tracking-wider" style={{ color: 'var(--text-primary)' }}>
            Delete Script
          </span>
        </div>
        <p className="text-xs mb-4" style={{ color: 'var(--text-secondary)', lineHeight: '1.6' }}>
          Delete script{' '}
          <strong className="font-mono" style={{ color: 'var(--text-primary)' }}>{name}</strong>?
          This cannot be undone.
        </p>
        {error && <div className="mb-3"><ErrorBanner message={error} /></div>}
        <div className="flex items-center gap-2 justify-end">
          <button
            type="button"
            onClick={onCancel}
            disabled={isPending}
            className="text-xs px-3 py-1.5"
            style={{
              backgroundColor: 'var(--bg-surface)',
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
            disabled={isPending}
            className="text-xs px-3 py-1.5"
            style={{
              backgroundColor: 'rgba(232,64,64,0.15)',
              border: '1px solid rgba(232,64,64,0.4)',
              color: 'var(--status-error)',
              borderRadius: '3px',
              cursor: isPending ? 'default' : 'pointer',
              opacity: isPending ? 0.6 : 1,
            }}
          >
            {isPending ? 'Deleting...' : 'Delete'}
          </button>
        </div>
      </div>
    </div>
  )
}

// ---- Run dialog (node selection + confirm + results) ----

type RunPhase = 'select' | 'confirm' | 'running' | 'results'

interface RunDialogProps {
  script: Script
  onClose: () => void
}

function RunDialog({ script, onClose }: RunDialogProps) {
  const { data: tree } = useTree()
  const allNodes = tree?.nodes ?? []

  const [phase, setPhase] = useState<RunPhase>('select')
  const [selectedIds, setSelectedIds] = useState<Set<string>>(new Set())
  const [results, setResults] = useState<ScriptRunResult[]>([])
  const [runError, setRunError] = useState<string | null>(null)

  const { mutate: runScript, isPending } = useRunScript()

  function toggleNode(id: string) {
    setSelectedIds((prev) => {
      const next = new Set(prev)
      if (next.has(id)) next.delete(id)
      else next.add(id)
      return next
    })
  }

  function handleRun() {
    setRunError(null)
    setPhase('running')
    runScript(
      { id: script.id, req: { node_ids: Array.from(selectedIds) } },
      {
        onSuccess: (data) => {
          setResults(data.results)
          setPhase('results')
        },
        onError: (err) => {
          setRunError(extractError(err))
          setPhase('confirm')
        },
      },
    )
  }

  const selectedCount = selectedIds.size
  const nodeLabel = (id: string) => allNodes.find((n) => n.id === id)?.name ?? id

  return (
    <div
      style={{
        position: 'fixed',
        inset: 0,
        backgroundColor: 'rgba(0,0,0,0.6)',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        zIndex: 50,
      }}
    >
      <div
        style={{
          backgroundColor: 'var(--bg-elevated)',
          border: '1px solid var(--border-default)',
          borderRadius: '3px',
          width: '560px',
          maxWidth: '95vw',
          maxHeight: '90vh',
          display: 'flex',
          flexDirection: 'column',
        }}
      >
        {/* Header */}
        <div
          className="flex items-center gap-2 px-4 py-3 shrink-0"
          style={{ borderBottom: '1px solid var(--border-subtle)' }}
        >
          <Terminal size={13} style={{ color: 'var(--text-secondary)' }} />
          <span className="text-xs font-medium uppercase tracking-wider flex-1" style={{ color: 'var(--text-primary)' }}>
            Run — {script.name}
          </span>
          {phase !== 'running' && (
            <button
              type="button"
              onClick={onClose}
              style={{ background: 'none', border: 'none', color: 'var(--text-muted)', cursor: 'pointer', padding: '2px' }}
            >
              <X size={14} />
            </button>
          )}
        </div>

        <div className="flex-1 overflow-auto p-4 flex flex-col gap-4">

          {/* Phase: select nodes */}
          {phase === 'select' && (
            <>
              <div>
                <span className="text-xs uppercase tracking-wider font-medium block mb-2" style={{ color: 'var(--text-muted)' }}>
                  Target Hosts
                </span>
                {allNodes.length === 0 ? (
                  <span className="text-xs" style={{ color: 'var(--text-muted)' }}>No nodes registered.</span>
                ) : (
                  <div className="flex flex-col gap-1">
                    {allNodes.map((n) => (
                      <label
                        key={n.id}
                        className="flex items-center gap-2 px-3 py-2 text-xs cursor-pointer"
                        style={{
                          backgroundColor: selectedIds.has(n.id) ? 'var(--accent-glow)' : 'var(--bg-surface)',
                          border: `1px solid ${selectedIds.has(n.id) ? 'var(--accent-dim)' : 'var(--border-subtle)'}`,
                          borderRadius: '3px',
                          color: selectedIds.has(n.id) ? 'var(--accent)' : 'var(--text-secondary)',
                        }}
                      >
                        <input
                          type="checkbox"
                          checked={selectedIds.has(n.id)}
                          onChange={() => toggleNode(n.id)}
                          style={{ accentColor: 'var(--accent)', cursor: 'pointer' }}
                        />
                        <span className="font-medium" style={{ color: 'var(--text-primary)' }}>{n.name}</span>
                        <span style={{ color: 'var(--text-muted)' }}>{n.host}</span>
                        <span
                          className="font-mono ml-auto"
                          style={{ fontSize: '12px', color: 'var(--text-muted)' }}
                        >
                          {n.type}
                        </span>
                      </label>
                    ))}
                  </div>
                )}
              </div>
              <div className="flex items-center gap-2 justify-end">
                <button
                  type="button"
                  onClick={onClose}
                  className="text-xs px-3 py-1.5"
                  style={{
                    backgroundColor: 'var(--bg-surface)',
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
                  onClick={() => setPhase('confirm')}
                  disabled={selectedCount === 0}
                  className="flex items-center gap-1.5 text-xs px-3 py-1.5"
                  style={{
                    backgroundColor: selectedCount > 0 ? 'var(--accent-glow)' : 'var(--bg-surface)',
                    border: `1px solid ${selectedCount > 0 ? 'var(--accent-dim)' : 'var(--border-default)'}`,
                    color: selectedCount > 0 ? 'var(--accent)' : 'var(--text-muted)',
                    borderRadius: '3px',
                    cursor: selectedCount > 0 ? 'pointer' : 'default',
                  }}
                >
                  <Play size={11} />
                  Review Run
                </button>
              </div>
            </>
          )}

          {/* Phase: confirm */}
          {phase === 'confirm' && (
            <>
              <div
                className="px-3 py-3 text-xs"
                style={{
                  backgroundColor: 'rgba(64,200,120,0.08)',
                  border: '1px solid rgba(64,200,120,0.25)',
                  borderRadius: '3px',
                  color: 'var(--text-secondary)',
                  lineHeight: '1.7',
                }}
              >
                Run <strong style={{ color: 'var(--text-primary)' }}>{script.name}</strong> on{' '}
                <strong style={{ color: 'var(--text-primary)' }}>{selectedCount} host{selectedCount !== 1 ? 's' : ''}</strong>?
                <div className="mt-1.5" style={{ color: 'var(--text-muted)' }}>
                  {Array.from(selectedIds).map((id) => nodeLabel(id)).join(', ')}
                </div>
              </div>
              {runError && <ErrorBanner message={runError} />}
              <div className="flex items-center gap-2 justify-end">
                <button
                  type="button"
                  onClick={() => { setRunError(null); setPhase('select') }}
                  className="text-xs px-3 py-1.5"
                  style={{
                    backgroundColor: 'var(--bg-surface)',
                    border: '1px solid var(--border-default)',
                    color: 'var(--text-secondary)',
                    borderRadius: '3px',
                    cursor: 'pointer',
                  }}
                >
                  Back
                </button>
                <button
                  type="button"
                  onClick={handleRun}
                  disabled={isPending}
                  className="flex items-center gap-1.5 text-xs px-3 py-1.5"
                  style={{
                    backgroundColor: 'rgba(64,200,120,0.12)',
                    border: '1px solid rgba(64,200,120,0.4)',
                    color: 'var(--status-ok, #40c878)',
                    borderRadius: '3px',
                    cursor: 'pointer',
                  }}
                >
                  <Play size={11} />
                  Run
                </button>
              </div>
            </>
          )}

          {/* Phase: running */}
          {phase === 'running' && (
            <div className="flex flex-col items-center gap-3 py-8">
              <Loader size={20} className="animate-spin" style={{ color: 'var(--accent)' }} />
              <span className="text-xs" style={{ color: 'var(--text-secondary)' }}>
                Running on {selectedCount} host{selectedCount !== 1 ? 's' : ''}…
              </span>
              <span className="text-xs" style={{ color: 'var(--text-muted)' }}>
                This may take a moment. Please wait.
              </span>
            </div>
          )}

          {/* Phase: results */}
          {phase === 'results' && (
            <>
              <div>
                <span className="text-xs uppercase tracking-wider font-medium block mb-3" style={{ color: 'var(--text-muted)' }}>
                  Results ({results.length} host{results.length !== 1 ? 's' : ''})
                </span>
                <div className="flex flex-col gap-3">
                  {results.map((r) => (
                    <ResultBlock key={r.node_id} result={r} nodeName={nodeLabel(r.node_id)} />
                  ))}
                </div>
              </div>
              <div className="flex justify-end">
                <button
                  type="button"
                  onClick={onClose}
                  className="text-xs px-3 py-1.5"
                  style={{
                    backgroundColor: 'var(--bg-surface)',
                    border: '1px solid var(--border-default)',
                    color: 'var(--text-secondary)',
                    borderRadius: '3px',
                    cursor: 'pointer',
                  }}
                >
                  Close
                </button>
              </div>
            </>
          )}

        </div>
      </div>
    </div>
  )
}

// ---- Result block ----

function ResultBlock({ result, nodeName }: { result: ScriptRunResult; nodeName: string }) {
  return (
    <div
      style={{
        backgroundColor: 'var(--bg-surface)',
        border: `1px solid ${result.ok ? 'rgba(64,200,120,0.25)' : 'rgba(232,64,64,0.25)'}`,
        borderRadius: '3px',
        overflow: 'hidden',
      }}
    >
      <div
        className="flex items-center gap-2 px-3 py-2"
        style={{ borderBottom: '1px solid var(--border-subtle)' }}
      >
        <span className="text-xs font-medium" style={{ color: 'var(--text-primary)', flex: 1 }}>
          {nodeName}
        </span>
        <span
          className="font-mono text-xs px-1.5 py-0.5 uppercase tracking-wider"
          style={{
            backgroundColor: result.ok ? 'rgba(64,200,120,0.12)' : 'rgba(232,64,64,0.12)',
            border: `1px solid ${result.ok ? 'rgba(64,200,120,0.35)' : 'rgba(232,64,64,0.35)'}`,
            color: result.ok ? 'var(--status-ok, #40c878)' : 'var(--status-error)',
            borderRadius: '3px',
            fontSize: '12px',
          }}
        >
          {result.ok ? 'ok' : 'failed'}
        </span>
      </div>
      <pre
        className="font-mono text-xs p-3 overflow-auto"
        style={{
          color: 'var(--text-secondary)',
          maxHeight: '220px',
          whiteSpace: 'pre-wrap',
          wordBreak: 'break-all',
          margin: 0,
          backgroundColor: 'transparent',
        }}
      >
        {result.output || <span style={{ color: 'var(--text-muted)' }}>(no output)</span>}
      </pre>
    </div>
  )
}

// ---- Script editor panel ----

interface ScriptEditorProps {
  script: Script | null  // null = new
  isAdmin: boolean
  onClose: () => void
  onSaved: (s: Script) => void
  onDelete: (s: Script) => void
  onRun: (s: Script) => void
}

function ScriptEditor({ script, isAdmin, onClose, onSaved, onDelete, onRun }: ScriptEditorProps) {
  const isNew = script === null

  const [name, setName] = useState(script?.name ?? '')
  const [description, setDescription] = useState(script?.description ?? '')
  const [content, setContent] = useState(script?.content ?? '')
  const [langExt, setLangExt] = useState<LanguageSupport | null>(null)
  const [saveError, setSaveError] = useState<string | null>(null)

  const { mutate: create, isPending: creating } = useCreateScript()
  const { mutate: update, isPending: updating } = useUpdateScript()

  const isPending = creating || updating

  useEffect(() => {
    void resolveLanguage('script.sh').then((l) => setLangExt(l))
  }, [])

  const handleContentChange = useCallback((val: string) => setContent(val), [])

  function handleSave() {
    setSaveError(null)
    const req = { name, description: description || undefined, content }

    if (isNew) {
      create(req, {
        onSuccess: (data) => onSaved(data),
        onError: (err) => setSaveError(extractError(err)),
      })
    } else {
      update(
        { id: script!.id, req },
        {
          onSuccess: (data) => onSaved(data),
          onError: (err) => setSaveError(extractError(err)),
        },
      )
    }
  }

  return (
    <div className="flex flex-col gap-5">
      {/* Header bar */}
      <div className="flex items-center gap-2">
        <button
          type="button"
          onClick={onClose}
          style={{ background: 'none', border: 'none', color: 'var(--text-muted)', cursor: 'pointer', padding: '2px' }}
          title="Back to list"
        >
          <X size={14} />
        </button>
        <span className="text-xs font-medium uppercase tracking-wider flex-1" style={{ color: 'var(--text-primary)' }}>
          {isNew ? 'New Script' : (isAdmin ? `Edit: ${script!.name}` : `Script: ${script!.name}`)}
        </span>
        {!isNew && isAdmin && (
          <button
            type="button"
            onClick={() => onRun(script!)}
            className="flex items-center gap-1.5 text-xs px-2 py-1"
            style={{
              backgroundColor: 'rgba(64,200,120,0.1)',
              border: '1px solid rgba(64,200,120,0.35)',
              color: 'var(--status-ok, #40c878)',
              borderRadius: '3px',
              cursor: 'pointer',
            }}
          >
            <Play size={11} />
            Run
          </button>
        )}
        {!isNew && isAdmin && (
          <button
            type="button"
            onClick={() => onDelete(script!)}
            className="flex items-center gap-1.5 text-xs px-2 py-1"
            style={{
              backgroundColor: 'var(--bg-elevated)',
              border: '1px solid var(--border-default)',
              color: 'var(--status-error)',
              borderRadius: '3px',
              cursor: 'pointer',
            }}
          >
            <Trash2 size={11} />
            Delete
          </button>
        )}
        {isAdmin && (
          <button
            type="button"
            onClick={handleSave}
            disabled={isPending || !name || !content}
            className="flex items-center gap-1.5 text-xs px-3 py-1"
            style={{
              backgroundColor: 'var(--accent-glow)',
              border: '1px solid var(--accent-dim)',
              color: !name || !content ? 'var(--text-muted)' : 'var(--accent)',
              borderRadius: '3px',
              cursor: isPending || !name || !content ? 'default' : 'pointer',
              opacity: isPending ? 0.7 : 1,
            }}
          >
            {isPending ? <Loader size={11} className="animate-spin" /> : null}
            {isPending ? 'Saving...' : 'Save'}
          </button>
        )}
      </div>

      {saveError && <ErrorBanner message={saveError} />}

      {/* Name */}
      <div>
        <label className="text-xs uppercase tracking-wider font-medium block mb-1.5" style={{ color: 'var(--text-muted)' }}>
          Name *
        </label>
        <input
          type="text"
          value={name}
          onChange={(e) => setName(e.target.value)}
          readOnly={!isAdmin}
          placeholder="my-script"
          className="text-xs px-2 py-1.5"
          style={{
            width: '100%',
            backgroundColor: 'var(--bg-surface)',
            border: '1px solid var(--border-default)',
            color: 'var(--text-primary)',
            borderRadius: '3px',
            outline: 'none',
            boxSizing: 'border-box',
          }}
        />
      </div>

      {/* Description */}
      <div>
        <label className="text-xs uppercase tracking-wider font-medium block mb-1.5" style={{ color: 'var(--text-muted)' }}>
          Description
        </label>
        <input
          type="text"
          value={description}
          onChange={(e) => setDescription(e.target.value)}
          readOnly={!isAdmin}
          placeholder="Optional description"
          className="text-xs px-2 py-1.5"
          style={{
            width: '100%',
            backgroundColor: 'var(--bg-surface)',
            border: '1px solid var(--border-default)',
            color: 'var(--text-primary)',
            borderRadius: '3px',
            outline: 'none',
            boxSizing: 'border-box',
          }}
        />
      </div>

      {/* Script content editor */}
      <div>
        <label className="text-xs uppercase tracking-wider font-medium block mb-1.5" style={{ color: 'var(--text-muted)' }}>
          Script Content *
        </label>
        <div
          style={{
            border: '1px solid var(--border-subtle)',
            borderRadius: '3px',
            overflow: 'hidden',
            minHeight: '240px',
          }}
        >
          <CodeMirror
            value={content}
            onChange={isAdmin ? handleContentChange : undefined}
            extensions={langExt ? [langExt] : []}
            editable={isAdmin}
            basicSetup={{ lineNumbers: true, foldGutter: false }}
            theme="dark"
            style={{ fontSize: '12px', fontFamily: "'IBM Plex Mono', monospace" }}
          />
        </div>
        <p className="text-xs mt-1.5" style={{ color: 'var(--text-muted)' }}>
          Scripts run as the SSH user on the selected hosts.
        </p>
      </div>
    </div>
  )
}

// ---- Script list card ----

interface ScriptCardProps {
  script: Script
  isAdmin: boolean
  onOpen: (s: Script) => void
  onDelete: (s: Script) => void
  onRun: (s: Script) => void
}

function ScriptCard({ script, isAdmin, onOpen, onDelete, onRun }: ScriptCardProps) {
  return (
    <div
      style={{
        backgroundColor: 'var(--bg-surface)',
        border: '1px solid var(--border-subtle)',
        borderRadius: '3px',
        padding: '12px 14px',
        display: 'flex',
        flexDirection: 'column',
        gap: '8px',
      }}
    >
      <div className="flex items-start justify-between gap-2">
        <button
          type="button"
          onClick={() => onOpen(script)}
          className="text-xs font-medium text-left"
          style={{
            background: 'none',
            border: 'none',
            color: 'var(--accent)',
            cursor: 'pointer',
            padding: 0,
          }}
        >
          {script.name}
        </button>
      </div>

      {script.description && (
        <p className="text-xs" style={{ color: 'var(--text-secondary)', lineHeight: '1.5' }}>
          {script.description}
        </p>
      )}

      <div className="flex items-center gap-1.5 mt-1">
        <button
          type="button"
          onClick={() => onOpen(script)}
          className="flex items-center gap-1 text-xs px-2 py-0.5"
          style={{
            backgroundColor: 'var(--bg-elevated)',
            border: '1px solid var(--border-default)',
            color: 'var(--text-secondary)',
            borderRadius: '3px',
            cursor: 'pointer',
          }}
        >
          <Pencil size={10} />
          {isAdmin ? 'Edit' : 'View'}
        </button>
        {isAdmin && (
          <button
            type="button"
            onClick={() => onRun(script)}
            className="flex items-center gap-1 text-xs px-2 py-0.5"
            style={{
              backgroundColor: 'rgba(64,200,120,0.08)',
              border: '1px solid rgba(64,200,120,0.3)',
              color: 'var(--status-ok, #40c878)',
              borderRadius: '3px',
              cursor: 'pointer',
            }}
          >
            <Play size={10} />
            Run
          </button>
        )}
        {isAdmin && (
          <button
            type="button"
            onClick={() => onDelete(script)}
            className="flex items-center gap-1 text-xs px-2 py-0.5"
            style={{
              backgroundColor: 'var(--bg-elevated)',
              border: '1px solid var(--border-default)',
              color: 'var(--status-error)',
              borderRadius: '3px',
              cursor: 'pointer',
            }}
          >
            <Trash2 size={10} />
            Delete
          </button>
        )}
      </div>
    </div>
  )
}

// ---- Main page ----

type View = { kind: 'list' } | { kind: 'editor'; script: Script | null }

export default function Scripts() {
  const { data: me } = useMe()
  const isAdmin = me?.role === 'admin'

  const { data, isLoading } = useScripts()
  const scripts = data?.scripts ?? []

  const { mutate: deleteScript, isPending: deleting } = useDeleteScript()

  const [view, setView] = useState<View>({ kind: 'list' })
  const [deleteTarget, setDeleteTarget] = useState<Script | null>(null)
  const [deleteError, setDeleteError] = useState<string | null>(null)
  const [runTarget, setRunTarget] = useState<Script | null>(null)

  function handleDeleteConfirm() {
    if (!deleteTarget) return
    setDeleteError(null)
    deleteScript(deleteTarget.id, {
      onSuccess: () => {
        setDeleteTarget(null)
        if (view.kind === 'editor' && view.script?.id === deleteTarget.id) {
          setView({ kind: 'list' })
        }
      },
      onError: (err) => {
        setDeleteError(extractError(err))
      },
    })
  }

  if (!isAdmin && me !== undefined) {
    return (
      <AppShell>
        <div className="flex flex-col flex-1 min-h-0 h-full w-full p-6" style={{ maxWidth: '1000px', margin: '0 auto' }}>
          <div className="flex items-center gap-2 mb-6">
            <Terminal size={16} style={{ color: 'var(--text-secondary)' }} />
            <h1 className="text-sm font-medium uppercase tracking-wider" style={{ color: 'var(--text-primary)' }}>
              Script Runner
            </h1>
          </div>
          <div
            className="px-3 py-4 text-xs"
            style={{
              color: 'var(--status-error)',
              backgroundColor: 'rgba(232,64,64,0.08)',
              border: '1px solid rgba(232,64,64,0.25)',
              borderRadius: '3px',
            }}
          >
            Admin access required to use Script Runner.
          </div>
        </div>
      </AppShell>
    )
  }

  return (
    <AppShell>
      <div
        className="flex flex-col flex-1 min-h-0 h-full w-full p-6"
        style={{ maxWidth: '1000px', margin: '0 auto' }}
      >
        {/* Page header */}
        <div className="flex items-center gap-2 mb-6">
          <Terminal size={16} style={{ color: 'var(--text-secondary)' }} />
          <h1 className="text-sm font-medium uppercase tracking-wider" style={{ color: 'var(--text-primary)' }}>
            Script Runner
          </h1>
          <div className="flex-1" />
          {view.kind === 'list' && isAdmin && (
            <button
              type="button"
              onClick={() => setView({ kind: 'editor', script: null })}
              className="flex items-center gap-1.5 text-xs px-3 py-1.5"
              style={{
                backgroundColor: 'var(--accent-glow)',
                border: '1px solid var(--accent-dim)',
                color: 'var(--accent)',
                borderRadius: '3px',
                cursor: 'pointer',
              }}
            >
              <Plus size={11} />
              New Script
            </button>
          )}
        </div>

        {/* Loading */}
        {isLoading && view.kind === 'list' && (
          <div className="flex items-center gap-2 py-8">
            <Loader size={13} className="animate-spin" style={{ color: 'var(--accent)' }} />
            <span className="text-xs" style={{ color: 'var(--text-muted)' }}>Loading scripts...</span>
          </div>
        )}

        {/* List view */}
        {!isLoading && view.kind === 'list' && (
          <>
            <SectionHeader label="Scripts" count={scripts.length} />
            {scripts.length === 0 ? (
              <div
                className="px-3 py-4 text-xs"
                style={{
                  color: 'var(--text-muted)',
                  backgroundColor: 'var(--bg-surface)',
                  border: '1px solid var(--border-subtle)',
                  borderRadius: '3px',
                }}
              >
                {isAdmin ? 'No scripts yet. Create one with the button above.' : 'No scripts found.'}
              </div>
            ) : (
              <div
                style={{
                  display: 'grid',
                  gridTemplateColumns: 'repeat(auto-fill, minmax(280px, 1fr))',
                  gap: '12px',
                }}
              >
                {scripts.map((s) => (
                  <ScriptCard
                    key={s.id}
                    script={s}
                    isAdmin={isAdmin}
                    onOpen={(sc) => setView({ kind: 'editor', script: sc })}
                    onDelete={(sc) => { setDeleteError(null); setDeleteTarget(sc) }}
                    onRun={(sc) => setRunTarget(sc)}
                  />
                ))}
              </div>
            )}
          </>
        )}

        {/* Editor view */}
        {view.kind === 'editor' && (
          <ScriptEditor
            script={view.script}
            isAdmin={isAdmin}
            onClose={() => setView({ kind: 'list' })}
            onSaved={(s) => setView({ kind: 'editor', script: s })}
            onDelete={(s) => { setDeleteError(null); setDeleteTarget(s) }}
            onRun={(s) => setRunTarget(s)}
          />
        )}

        {/* Delete confirm dialog */}
        {deleteTarget && (
          <ConfirmDelete
            name={deleteTarget.name}
            onConfirm={handleDeleteConfirm}
            onCancel={() => { setDeleteTarget(null); setDeleteError(null) }}
            isPending={deleting}
            error={deleteError}
          />
        )}

        {/* Run dialog */}
        {runTarget && (
          <RunDialog
            script={runTarget}
            onClose={() => setRunTarget(null)}
          />
        )}
      </div>
    </AppShell>
  )
}
