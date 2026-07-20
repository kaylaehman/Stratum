import { useState, useEffect, useCallback } from 'react'
import {
  LayoutTemplate,
  Plus,
  Pencil,
  Trash2,
  Rocket,
  Eye,
  Loader,
  Download,
  X,
  AlertTriangle,
  ChevronDown,
  ChevronRight,
} from 'lucide-react'
import CodeMirror from '@uiw/react-codemirror'
import type { LanguageSupport } from '@codemirror/language'
import { AppShell } from '../components/layout/AppShell'
import { useMe } from '../hooks/useMe'
import { useTree } from '../lib/api/tree'
import {
  useTemplates,
  useTemplate,
  useCreateTemplate,
  useUpdateTemplate,
  useDeleteTemplate,
  useRenderTemplate,
  useDeployTemplate,
} from '../lib/api/templates'
import { ApiError } from '../lib/api'
import { resolveLanguage } from '../lib/codemirror'
import type { Template, TemplateVar, TemplateVersion } from '../types/api'

// ---- Helpers ----

function downloadYml(filename: string, content: string) {
  const blob = new Blob([content], { type: 'text/yaml' })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = filename.endsWith('.yml') ? filename : `${filename}.yml`
  a.click()
  URL.revokeObjectURL(url)
}

function formatDate(iso: string): string {
  try {
    return new Date(iso).toLocaleString()
  } catch {
    return iso
  }
}

// ---- Tag chip ----

function TagChip({ label }: { label: string }) {
  return (
    <span
      className="font-mono text-xs px-1.5 py-0.5 uppercase tracking-wider"
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

// ---- Section header ----

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

// ---- Inline error banner ----

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

// ---- Variables editor ----

interface VarsEditorProps {
  vars: TemplateVar[]
  onChange: (vars: TemplateVar[]) => void
  readOnly?: boolean
}

function VarsEditor({ vars, onChange, readOnly }: VarsEditorProps) {
  function update(i: number, field: keyof TemplateVar, value: string) {
    const next = vars.map((v, idx) => (idx === i ? { ...v, [field]: value } : v))
    onChange(next)
  }

  function add() {
    onChange([...vars, { name: '', description: '', default: '' }])
  }

  function remove(i: number) {
    onChange(vars.filter((_, idx) => idx !== i))
  }

  return (
    <div className="flex flex-col gap-2">
      {vars.length === 0 && (
        <span className="text-xs" style={{ color: 'var(--text-muted)' }}>No variables defined.</span>
      )}
      {vars.map((v, i) => (
        <div key={i} className="flex items-start gap-2">
          <input
            type="text"
            placeholder="NAME"
            value={v.name}
            onChange={(e) => update(i, 'name', e.target.value)}
            readOnly={readOnly}
            className="font-mono text-xs px-2 py-1"
            style={{
              width: '120px',
              backgroundColor: 'var(--bg-elevated)',
              border: '1px solid var(--border-default)',
              color: 'var(--text-primary)',
              borderRadius: '3px',
              outline: 'none',
            }}
          />
          <input
            type="text"
            placeholder="description"
            value={v.description}
            onChange={(e) => update(i, 'description', e.target.value)}
            readOnly={readOnly}
            className="text-xs px-2 py-1"
            style={{
              flex: 1,
              backgroundColor: 'var(--bg-elevated)',
              border: '1px solid var(--border-default)',
              color: 'var(--text-primary)',
              borderRadius: '3px',
              outline: 'none',
            }}
          />
          <input
            type="text"
            placeholder="default"
            value={v.default}
            onChange={(e) => update(i, 'default', e.target.value)}
            readOnly={readOnly}
            className="font-mono text-xs px-2 py-1"
            style={{
              width: '120px',
              backgroundColor: 'var(--bg-elevated)',
              border: '1px solid var(--border-default)',
              color: 'var(--text-primary)',
              borderRadius: '3px',
              outline: 'none',
            }}
          />
          {!readOnly && (
            <button
              type="button"
              onClick={() => remove(i)}
              style={{ background: 'none', border: 'none', color: 'var(--text-muted)', cursor: 'pointer', padding: '4px' }}
              title="Remove variable"
            >
              <X size={12} />
            </button>
          )}
        </div>
      ))}
      {!readOnly && (
        <button
          type="button"
          onClick={add}
          className="flex items-center gap-1.5 text-xs px-2 py-1 self-start"
          style={{
            backgroundColor: 'var(--bg-elevated)',
            border: '1px solid var(--border-default)',
            color: 'var(--text-secondary)',
            borderRadius: '3px',
            cursor: 'pointer',
          }}
        >
          <Plus size={11} />
          Add variable
        </button>
      )}
    </div>
  )
}

// ---- Version history ----

function VersionHistory({ versions }: { versions: TemplateVersion[] }) {
  const [open, setOpen] = useState(false)

  if (versions.length === 0) return null

  return (
    <div className="mt-4">
      <button
        type="button"
        onClick={() => setOpen((v) => !v)}
        className="flex items-center gap-1.5 text-xs"
        style={{ background: 'none', border: 'none', color: 'var(--text-secondary)', cursor: 'pointer', padding: 0 }}
      >
        {open ? <ChevronDown size={12} /> : <ChevronRight size={12} />}
        <span className="uppercase tracking-wider font-medium" style={{ color: 'var(--text-muted)' }}>
          Version History ({versions.length})
        </span>
      </button>
      {open && (
        <div
          className="mt-2"
          style={{
            backgroundColor: 'var(--bg-surface)',
            border: '1px solid var(--border-subtle)',
            borderRadius: '3px',
          }}
        >
          <table style={{ width: '100%', borderCollapse: 'collapse' }}>
            <thead>
              <tr>
                {['Version', 'Created'].map((col) => (
                  <th
                    key={col}
                    className="px-3 py-2 text-left text-xs uppercase tracking-wider font-medium"
                    style={{ color: 'var(--text-muted)', borderBottom: '1px solid var(--border-subtle)' }}
                  >
                    {col}
                  </th>
                ))}
              </tr>
            </thead>
            <tbody>
              {versions.map((v) => (
                <tr key={v.version}>
                  <td className="px-3 py-2 font-mono text-xs" style={{ color: 'var(--text-primary)', borderBottom: '1px solid var(--border-subtle)' }}>
                    v{v.version}
                  </td>
                  <td className="px-3 py-2 text-xs" style={{ color: 'var(--text-secondary)', borderBottom: '1px solid var(--border-subtle)' }}>
                    {formatDate(v.created_at)}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  )
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
            Delete Template
          </span>
        </div>
        <p className="text-xs mb-4" style={{ color: 'var(--text-secondary)', lineHeight: '1.6' }}>
          Delete template{' '}
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

// ---- Render preview panel ----

interface RenderPanelProps {
  templateId: string
  templateName: string
  vars: TemplateVar[]
  isAdmin: boolean
  onDeploy: (values: Record<string, string>) => void
  onClose: () => void
}

function RenderPanel({ templateId, templateName, vars, isAdmin, onDeploy, onClose }: RenderPanelProps) {
  const [values, setValues] = useState<Record<string, string>>(() =>
    Object.fromEntries(vars.map((v) => [v.name, v.default]))
  )
  const [rendered, setRendered] = useState<string | null>(null)
  const [unresolved, setUnresolved] = useState<string[]>([])
  const [langExt, setLangExt] = useState<LanguageSupport | null>(null)
  const { mutate: render, isPending, error } = useRenderTemplate()

  useEffect(() => {
    void resolveLanguage('compose.yml').then((l) => setLangExt(l))
  }, [])

  function handleRender() {
    render(
      { id: templateId, req: { variables: values } },
      {
        onSuccess: (data) => {
          setRendered(data.rendered)
          setUnresolved(data.unresolved)
        },
      },
    )
  }

  const renderError = error instanceof ApiError
    ? String((error.body as Record<string, unknown>)?.error ?? error.message)
    : error instanceof Error ? error.message : null

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
          width: '700px',
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
          <Eye size={13} style={{ color: 'var(--text-secondary)' }} />
          <span className="text-xs font-medium uppercase tracking-wider flex-1" style={{ color: 'var(--text-primary)' }}>
            Preview — {templateName}
          </span>
          <button
            type="button"
            onClick={onClose}
            style={{ background: 'none', border: 'none', color: 'var(--text-muted)', cursor: 'pointer', padding: '2px' }}
          >
            <X size={14} />
          </button>
        </div>

        <div className="flex-1 overflow-auto p-4 flex flex-col gap-4">
          {/* Variable inputs */}
          {vars.length > 0 && (
            <div>
              <div className="mb-2">
                <span className="text-xs uppercase tracking-wider font-medium" style={{ color: 'var(--text-muted)' }}>
                  Variables
                </span>
              </div>
              <div className="flex flex-col gap-2">
                {vars.map((v) => (
                  <div key={v.name} className="flex items-center gap-2">
                    <label
                      className="font-mono text-xs"
                      style={{ color: 'var(--text-primary)', width: '140px', flexShrink: 0 }}
                    >
                      {v.name}
                    </label>
                    <input
                      type="text"
                      value={values[v.name] ?? ''}
                      onChange={(e) => setValues((prev) => ({ ...prev, [v.name]: e.target.value }))}
                      placeholder={v.description || v.default}
                      className="font-mono text-xs px-2 py-1 flex-1"
                      style={{
                        backgroundColor: 'var(--bg-surface)',
                        border: '1px solid var(--border-default)',
                        color: 'var(--text-primary)',
                        borderRadius: '3px',
                        outline: 'none',
                      }}
                    />
                  </div>
                ))}
              </div>
            </div>
          )}

          {/* Render action */}
          <div className="flex items-center gap-2">
            <button
              type="button"
              onClick={handleRender}
              disabled={isPending}
              className="flex items-center gap-1.5 text-xs px-3 py-1.5"
              style={{
                backgroundColor: 'var(--accent-glow)',
                border: '1px solid var(--accent-dim)',
                color: 'var(--accent)',
                borderRadius: '3px',
                cursor: isPending ? 'default' : 'pointer',
                opacity: isPending ? 0.7 : 1,
              }}
            >
              {isPending ? <Loader size={11} className="animate-spin" /> : <Eye size={11} />}
              Render
            </button>
            {rendered && (
              <button
                type="button"
                onClick={() => downloadYml(templateName, rendered)}
                className="flex items-center gap-1.5 text-xs px-3 py-1.5"
                style={{
                  backgroundColor: 'var(--bg-surface)',
                  border: '1px solid var(--border-default)',
                  color: 'var(--text-secondary)',
                  borderRadius: '3px',
                  cursor: 'pointer',
                }}
              >
                <Download size={11} />
                Download
              </button>
            )}
            {rendered && isAdmin && (
              <button
                type="button"
                onClick={() => onDeploy(values)}
                className="flex items-center gap-1.5 text-xs px-3 py-1.5"
                style={{
                  backgroundColor: 'rgba(64,200,120,0.1)',
                  border: '1px solid rgba(64,200,120,0.35)',
                  color: 'var(--status-ok, #40c878)',
                  borderRadius: '3px',
                  cursor: 'pointer',
                }}
              >
                <Rocket size={11} />
                Deploy...
              </button>
            )}
          </div>

          {renderError && <ErrorBanner message={renderError} />}

          {unresolved.length > 0 && (
            <div
              className="flex items-start gap-2 px-3 py-2 text-xs"
              style={{
                backgroundColor: 'rgba(232,64,64,0.1)',
                border: '1px solid rgba(232,64,64,0.3)',
                borderRadius: '3px',
                color: 'var(--status-error)',
              }}
            >
              <AlertTriangle size={11} style={{ flexShrink: 0, marginTop: '1px' }} />
              <span>
                Unresolved variables:{' '}
                {unresolved.map((u) => (
                  <strong key={u} className="font-mono">{`{{${u}}}`} </strong>
                ))}
              </span>
            </div>
          )}

          {rendered && (
            <div>
              <div className="mb-2">
                <span className="text-xs uppercase tracking-wider font-medium" style={{ color: 'var(--text-muted)' }}>
                  Rendered Output
                </span>
              </div>
              <div
                style={{
                  border: '1px solid var(--border-subtle)',
                  borderRadius: '3px',
                  overflow: 'hidden',
                  maxHeight: '320px',
                  overflowY: 'auto',
                }}
              >
                <CodeMirror
                  value={rendered}
                  extensions={langExt ? [langExt] : []}
                  editable={false}
                  basicSetup={{ lineNumbers: true }}
                  theme="dark"
                  style={{ fontSize: '12px', fontFamily: "'IBM Plex Mono', monospace" }}
                />
              </div>
            </div>
          )}
        </div>
      </div>
    </div>
  )
}

// ---- Deploy panel ----

interface DeployPanelProps {
  templateId: string
  templateName: string
  initialVarValues: Record<string, string>
  onClose: () => void
}

function DeployPanel({ templateId, templateName, initialVarValues, onClose }: DeployPanelProps) {
  const { data: tree } = useTree()
  const dockerNodes = (tree?.nodes ?? []).filter((n) => n.capabilities?.docker)

  const [nodeId, setNodeId] = useState(dockerNodes[0]?.id ?? '')
  const [dir, setDir] = useState('')
  const [varValues] = useState<Record<string, string>>(initialVarValues)
  const [confirmed, setConfirmed] = useState(false)
  const [deployResult, setDeployResult] = useState<{ path: string; output: string } | null>(null)
  const [deployError, setDeployError] = useState<string | null>(null)

  const { mutate: deploy, isPending } = useDeployTemplate()

  const targetNode = dockerNodes.find((n) => n.id === nodeId)

  function handleDeploy() {
    setDeployError(null)
    deploy(
      {
        id: templateId,
        req: {
          node_id: nodeId,
          dir: dir || undefined,
          variables: varValues,
        },
      },
      {
        onSuccess: (data) => {
          setDeployResult(data)
          setConfirmed(false)
        },
        onError: (err) => {
          if (err instanceof ApiError) {
            const body = err.body as Record<string, unknown>
            if (err.status === 400 && body?.error === 'unresolved_variables') {
              const unresolved = (body.unresolved as string[]) ?? []
              setDeployError(`Unresolved variables: ${unresolved.join(', ')}`)
            } else if (err.status === 409) {
              setDeployError('Docker is not available on the selected node.')
            } else if (err.status === 502) {
              const output = body?.output ? `\n\n${String(body.output)}` : ''
              setDeployError(`Compose failed.${output}`)
            } else {
              setDeployError(String(body?.error ?? err.message))
            }
          } else {
            setDeployError(err instanceof Error ? err.message : 'Deploy failed')
          }
          setConfirmed(false)
        },
      },
    )
  }

  return (
    <div
      style={{
        position: 'fixed',
        inset: 0,
        backgroundColor: 'rgba(0,0,0,0.6)',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        zIndex: 60,
      }}
    >
      <div
        style={{
          backgroundColor: 'var(--bg-elevated)',
          border: '1px solid var(--border-default)',
          borderRadius: '3px',
          width: '520px',
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
          <Rocket size={13} style={{ color: 'var(--text-secondary)' }} />
          <span className="text-xs font-medium uppercase tracking-wider flex-1" style={{ color: 'var(--text-primary)' }}>
            Deploy — {templateName}
          </span>
          <button
            type="button"
            onClick={onClose}
            style={{ background: 'none', border: 'none', color: 'var(--text-muted)', cursor: 'pointer', padding: '2px' }}
          >
            <X size={14} />
          </button>
        </div>

        <div className="flex-1 overflow-auto p-4 flex flex-col gap-4">
          {deployResult ? (
            <div className="flex flex-col gap-3">
              <div
                className="flex items-center gap-2 px-3 py-2 text-xs"
                style={{
                  backgroundColor: 'rgba(64,200,120,0.1)',
                  border: '1px solid rgba(64,200,120,0.3)',
                  borderRadius: '3px',
                  color: 'var(--status-ok, #40c878)',
                }}
              >
                Deployed to <strong className="font-mono">{deployResult.path}</strong>
              </div>
              {deployResult.output && (
                <pre
                  className="text-xs p-3 overflow-auto"
                  style={{
                    backgroundColor: 'var(--bg-surface)',
                    border: '1px solid var(--border-subtle)',
                    borderRadius: '3px',
                    color: 'var(--text-secondary)',
                    maxHeight: '240px',
                    whiteSpace: 'pre-wrap',
                    wordBreak: 'break-all',
                  }}
                >
                  {deployResult.output}
                </pre>
              )}
              <button
                type="button"
                onClick={onClose}
                className="text-xs px-3 py-1.5 self-end"
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
          ) : confirmed ? (
            <div className="flex flex-col gap-3">
              <div
                className="px-3 py-3 text-xs"
                style={{
                  backgroundColor: 'rgba(64,200,120,0.08)',
                  border: '1px solid rgba(64,200,120,0.25)',
                  borderRadius: '3px',
                  color: 'var(--text-secondary)',
                  lineHeight: '1.6',
                }}
              >
                Deploy <strong style={{ color: 'var(--text-primary)' }}>{templateName}</strong> to{' '}
                <strong style={{ color: 'var(--text-primary)' }}>{targetNode?.name ?? nodeId}</strong>?
                This runs <span className="font-mono">docker compose up -d</span> on the host.
              </div>
              {deployError && <ErrorBanner message={deployError} />}
              <div className="flex items-center gap-2 justify-end">
                <button
                  type="button"
                  onClick={() => setConfirmed(false)}
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
                  Back
                </button>
                <button
                  type="button"
                  onClick={handleDeploy}
                  disabled={isPending}
                  className="flex items-center gap-1.5 text-xs px-3 py-1.5"
                  style={{
                    backgroundColor: 'rgba(64,200,120,0.12)',
                    border: '1px solid rgba(64,200,120,0.4)',
                    color: 'var(--status-ok, #40c878)',
                    borderRadius: '3px',
                    cursor: isPending ? 'default' : 'pointer',
                    opacity: isPending ? 0.7 : 1,
                  }}
                >
                  {isPending ? <Loader size={11} className="animate-spin" /> : <Rocket size={11} />}
                  {isPending ? 'Deploying...' : 'Confirm Deploy'}
                </button>
              </div>
            </div>
          ) : (
            <div className="flex flex-col gap-4">
              {/* Node select */}
              <div>
                <label className="text-xs uppercase tracking-wider font-medium block mb-1.5" style={{ color: 'var(--text-muted)' }}>
                  Target Node
                </label>
                {dockerNodes.length === 0 ? (
                  <span className="text-xs" style={{ color: 'var(--status-error)' }}>No Docker-capable nodes found.</span>
                ) : (
                  <select
                    value={nodeId}
                    onChange={(e) => setNodeId(e.target.value)}
                    className="text-xs px-2 py-1.5"
                    style={{
                      backgroundColor: 'var(--bg-surface)',
                      border: '1px solid var(--border-default)',
                      color: 'var(--text-primary)',
                      borderRadius: '3px',
                      outline: 'none',
                      width: '100%',
                    }}
                  >
                    {dockerNodes.map((n) => (
                      <option key={n.id} value={n.id}>{n.name} ({n.host})</option>
                    ))}
                  </select>
                )}
              </div>

              {/* Optional dir */}
              <div>
                <label className="text-xs uppercase tracking-wider font-medium block mb-1.5" style={{ color: 'var(--text-muted)' }}>
                  Deploy Directory (optional)
                </label>
                <input
                  type="text"
                  placeholder="/opt/stacks/my-stack"
                  value={dir}
                  onChange={(e) => setDir(e.target.value)}
                  className="font-mono text-xs px-2 py-1.5"
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

              {deployError && <ErrorBanner message={deployError} />}

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
                  onClick={() => { setDeployError(null); setConfirmed(true) }}
                  disabled={!nodeId}
                  className="flex items-center gap-1.5 text-xs px-3 py-1.5"
                  style={{
                    backgroundColor: 'rgba(64,200,120,0.1)',
                    border: '1px solid rgba(64,200,120,0.35)',
                    color: 'var(--status-ok, #40c878)',
                    borderRadius: '3px',
                    cursor: nodeId ? 'pointer' : 'default',
                    opacity: nodeId ? 1 : 0.5,
                  }}
                >
                  <Rocket size={11} />
                  Review &amp; Deploy
                </button>
              </div>
            </div>
          )}
        </div>
      </div>
    </div>
  )
}

// ---- Template editor/detail ----

interface TemplateEditorProps {
  templateId: string | null  // null = new
  isAdmin: boolean
  onClose: () => void
  onSaved: (id: string) => void
}

function TemplateEditor({ templateId, isAdmin, onClose, onSaved }: TemplateEditorProps) {
  const isNew = templateId === null
  const { data: templateData, isLoading } = useTemplate(templateId ?? '')

  const [name, setName] = useState('')
  const [description, setDescription] = useState('')
  const [tagsInput, setTagsInput] = useState('')
  const [vars, setVars] = useState<TemplateVar[]>([])
  const [compose, setCompose] = useState('')
  const [langExt, setLangExt] = useState<LanguageSupport | null>(null)
  const [importUrl, setImportUrl] = useState('')
  const [importLoading, setImportLoading] = useState(false)
  const [importError, setImportError] = useState<string | null>(null)
  const [saveError, setSaveError] = useState<string | null>(null)
  const [showRender, setShowRender] = useState(false)
  const [deployVarValues, setDeployVarValues] = useState<Record<string, string> | null>(null)

  const { mutate: create, isPending: creating } = useCreateTemplate()
  const { mutate: update, isPending: updating } = useUpdateTemplate()

  useEffect(() => {
    void resolveLanguage('compose.yml').then((l) => setLangExt(l))
  }, [])

  useEffect(() => {
    if (templateData && !isNew) {
      setName(templateData.name)
      setDescription(templateData.description ?? '')
      setTagsInput((templateData.tags ?? []).join(', '))
      setVars(templateData.variables ?? [])
      setCompose(templateData.compose_yaml)
    }
  }, [templateData, isNew])

  const isPending = creating || updating

  function parseTags(input: string): string[] {
    return input
      .split(',')
      .map((t) => t.trim())
      .filter(Boolean)
  }

  function handleSave() {
    setSaveError(null)
    const req = {
      name,
      description: description || undefined,
      tags: parseTags(tagsInput),
      compose_yaml: compose,
      variables: vars,
    }

    if (isNew) {
      create(req, {
        onSuccess: (data) => onSaved(data.id),
        onError: (err) => {
          if (err instanceof ApiError) {
            const body = err.body as Record<string, unknown>
            setSaveError(String(body?.error ?? err.message))
          } else {
            setSaveError(err instanceof Error ? err.message : 'Save failed')
          }
        },
      })
    } else {
      update(
        { id: templateId!, req },
        {
          onSuccess: (data) => onSaved(data.id),
          onError: (err) => {
            if (err instanceof ApiError) {
              const body = err.body as Record<string, unknown>
              setSaveError(String(body?.error ?? err.message))
            } else {
              setSaveError(err instanceof Error ? err.message : 'Save failed')
            }
          },
        },
      )
    }
  }

  async function handleImportUrl() {
    setImportError(null)
    setImportLoading(true)
    try {
      const res = await fetch(importUrl)
      if (!res.ok) throw new Error(`HTTP ${res.status}`)
      const text = await res.text()
      setCompose(text)
    } catch (e) {
      setImportError(
        e instanceof TypeError
          ? "Couldn't fetch — likely a CORS error. Paste the content manually."
          : e instanceof Error
          ? e.message
          : 'Fetch failed — paste manually.',
      )
    } finally {
      setImportLoading(false)
    }
  }

  const handleComposeChange = useCallback((val: string) => setCompose(val), [])

  if (!isNew && isLoading) {
    return (
      <div className="flex items-center gap-2 py-8">
        <Loader size={13} className="animate-spin" style={{ color: 'var(--accent)' }} />
        <span className="text-xs" style={{ color: 'var(--text-muted)' }}>Loading template...</span>
      </div>
    )
  }

  const canEdit = isNew || isAdmin

  return (
    <>
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
            {isNew ? 'New Template' : (canEdit ? `Edit: ${name}` : `Template: ${name}`)}
          </span>
          {!isNew && (
            <span className="font-mono text-xs" style={{ color: 'var(--text-muted)' }}>
              v{templateData?.version ?? '—'}
            </span>
          )}
          <button
            type="button"
            onClick={() => setShowRender(true)}
            disabled={!compose}
            className="flex items-center gap-1.5 text-xs px-2 py-1"
            style={{
              backgroundColor: 'var(--bg-elevated)',
              border: '1px solid var(--border-default)',
              color: compose ? 'var(--text-secondary)' : 'var(--text-muted)',
              borderRadius: '3px',
              cursor: compose ? 'pointer' : 'default',
            }}
          >
            <Eye size={11} />
            Preview
          </button>
          {!isNew && (
            <button
              type="button"
              onClick={() => downloadYml(name, compose)}
              disabled={!compose}
              className="flex items-center gap-1.5 text-xs px-2 py-1"
              style={{
                backgroundColor: 'var(--bg-elevated)',
                border: '1px solid var(--border-default)',
                color: 'var(--text-secondary)',
                borderRadius: '3px',
                cursor: 'pointer',
              }}
            >
              <Download size={11} />
              Export
            </button>
          )}
          {canEdit && (
            <button
              type="button"
              onClick={handleSave}
              disabled={isPending || !name || !compose}
              className="flex items-center gap-1.5 text-xs px-3 py-1"
              style={{
                backgroundColor: 'var(--accent-glow)',
                border: '1px solid var(--accent-dim)',
                color: (!name || !compose) ? 'var(--text-muted)' : 'var(--accent)',
                borderRadius: '3px',
                cursor: isPending || !name || !compose ? 'default' : 'pointer',
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
            readOnly={!canEdit}
            placeholder="my-stack"
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
            readOnly={!canEdit}
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

        {/* Tags */}
        <div>
          <label className="text-xs uppercase tracking-wider font-medium block mb-1.5" style={{ color: 'var(--text-muted)' }}>
            Tags (comma-separated)
          </label>
          <input
            type="text"
            value={tagsInput}
            onChange={(e) => setTagsInput(e.target.value)}
            readOnly={!canEdit}
            placeholder="monitoring, web, infra"
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

        {/* Variables */}
        <div>
          <label className="text-xs uppercase tracking-wider font-medium block mb-1.5" style={{ color: 'var(--text-muted)' }}>
            Variables
          </label>
          <VarsEditor vars={vars} onChange={setVars} readOnly={!canEdit} />
        </div>

        {/* Import from URL */}
        {canEdit && (
          <div>
            <label className="text-xs uppercase tracking-wider font-medium block mb-1.5" style={{ color: 'var(--text-muted)' }}>
              Import from URL
            </label>
            <div className="flex items-center gap-2">
              <input
                type="url"
                value={importUrl}
                onChange={(e) => setImportUrl(e.target.value)}
                placeholder="https://raw.githubusercontent.com/..."
                className="font-mono text-xs px-2 py-1.5 flex-1"
                style={{
                  backgroundColor: 'var(--bg-surface)',
                  border: '1px solid var(--border-default)',
                  color: 'var(--text-primary)',
                  borderRadius: '3px',
                  outline: 'none',
                }}
              />
              <button
                type="button"
                onClick={() => void handleImportUrl()}
                disabled={importLoading || !importUrl}
                className="flex items-center gap-1.5 text-xs px-2 py-1.5"
                style={{
                  backgroundColor: 'var(--bg-elevated)',
                  border: '1px solid var(--border-default)',
                  color: 'var(--text-secondary)',
                  borderRadius: '3px',
                  cursor: importLoading || !importUrl ? 'default' : 'pointer',
                  opacity: importLoading || !importUrl ? 0.6 : 1,
                  whiteSpace: 'nowrap',
                }}
              >
                {importLoading ? <Loader size={11} className="animate-spin" /> : null}
                Fetch
              </button>
            </div>
            {importError && (
              <div className="mt-1.5">
                <ErrorBanner message={importError} />
              </div>
            )}
          </div>
        )}

        {/* Compose YAML editor */}
        <div>
          <label className="text-xs uppercase tracking-wider font-medium block mb-1.5" style={{ color: 'var(--text-muted)' }}>
            Compose YAML *
          </label>
          <div
            style={{
              border: '1px solid var(--border-subtle)',
              borderRadius: '3px',
              overflow: 'hidden',
              minHeight: '200px',
            }}
          >
            <CodeMirror
              value={compose}
              onChange={canEdit ? handleComposeChange : undefined}
              extensions={langExt ? [langExt] : []}
              editable={canEdit}
              basicSetup={{ lineNumbers: true, foldGutter: true }}
              theme="dark"
              style={{ fontSize: '12px', fontFamily: "'IBM Plex Mono', monospace" }}
            />
          </div>
        </div>

        {/* Version history */}
        {!isNew && templateData?.versions && (
          <VersionHistory versions={templateData.versions} />
        )}
      </div>

      {/* Render preview overlay */}
      {showRender && (
        <RenderPanel
          templateId={templateId!}
          templateName={name}
          vars={vars}
          isAdmin={isAdmin}
          onDeploy={(values) => {
            setShowRender(false)
            setDeployVarValues(values)
          }}
          onClose={() => setShowRender(false)}
        />
      )}

      {/* Deploy overlay */}
      {deployVarValues !== null && (
        <DeployPanel
          templateId={templateId!}
          templateName={name}
          initialVarValues={deployVarValues}
          onClose={() => setDeployVarValues(null)}
        />
      )}
    </>
  )
}

// ---- Template list card ----

interface TemplateCardProps {
  template: Template
  isAdmin: boolean
  onOpen: (id: string) => void
  onDelete: (id: string) => void
}

function TemplateCard({ template, isAdmin, onOpen, onDelete }: TemplateCardProps) {
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
          onClick={() => onOpen(template.id)}
          className="text-xs font-medium text-left"
          style={{
            background: 'none',
            border: 'none',
            color: 'var(--accent)',
            cursor: 'pointer',
            padding: 0,
          }}
        >
          {template.name}
        </button>
        <span className="font-mono text-xs shrink-0" style={{ color: 'var(--text-muted)' }}>
          v{template.version}
        </span>
      </div>

      {template.description && (
        <p className="text-xs" style={{ color: 'var(--text-secondary)', lineHeight: '1.5' }}>
          {template.description}
        </p>
      )}

      {template.tags && template.tags.length > 0 && (
        <div className="flex flex-wrap gap-1">
          {template.tags.map((tag) => <TagChip key={tag} label={tag} />)}
        </div>
      )}

      <div className="flex items-center gap-1.5 mt-1">
        <button
          type="button"
          onClick={() => onOpen(template.id)}
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
            onClick={() => onDelete(template.id)}
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

type View = { kind: 'list' } | { kind: 'editor'; id: string | null }

export default function Templates() {
  const { data: me } = useMe()
  const isAdmin = me?.role === 'admin'

  const { data, isLoading } = useTemplates()
  const templates = data?.templates ?? []

  const { mutate: deleteTemplate, isPending: deleting } = useDeleteTemplate()

  const [view, setView] = useState<View>({ kind: 'list' })
  const [deleteTarget, setDeleteTarget] = useState<Template | null>(null)
  const [deleteError, setDeleteError] = useState<string | null>(null)

  function handleDeleteConfirm() {
    if (!deleteTarget) return
    setDeleteError(null)
    deleteTemplate(deleteTarget.id, {
      onSuccess: () => {
        setDeleteTarget(null)
        if (view.kind === 'editor' && view.id === deleteTarget.id) {
          setView({ kind: 'list' })
        }
      },
      onError: (err) => {
        setDeleteError(err instanceof Error ? err.message : 'Delete failed')
      },
    })
  }

  return (
    <AppShell>
      <div
        className="flex flex-col flex-1 min-h-0 h-full w-full p-6"
        style={{ maxWidth: '1000px', margin: '0 auto' }}
      >
        {/* Page header */}
        <div className="flex items-center gap-2 mb-6">
          <LayoutTemplate size={16} style={{ color: 'var(--text-secondary)' }} />
          <h1 className="text-sm font-medium uppercase tracking-wider" style={{ color: 'var(--text-primary)' }}>
            Template Library
          </h1>
          <div className="flex-1" />
          {view.kind === 'list' && isAdmin && (
            <button
              type="button"
              onClick={() => setView({ kind: 'editor', id: null })}
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
              New Template
            </button>
          )}
        </div>

        {/* Page subtitle — explains what the template library is for. */}
        {view.kind === 'list' && (
          <p className="text-xs mb-6" style={{ color: 'var(--text-secondary)', maxWidth: '640px', lineHeight: 1.6 }}>
            Templates are reusable, pre-built Docker Compose service definitions you can deploy to any
            connected node. Define variable substitution tokens (e.g.{' '}
            <code className="font-mono" style={{ color: 'var(--text-primary)' }}>{'{{DOMAIN}}'}</code>) to
            fill in per-deployment, then deploy the stack without hand-writing YAML each time.
          </p>
        )}

        {/* Loading */}
        {isLoading && view.kind === 'list' && (
          <div className="flex items-center gap-2 py-8">
            <Loader size={13} className="animate-spin" style={{ color: 'var(--accent)' }} />
            <span className="text-xs" style={{ color: 'var(--text-muted)' }}>Loading templates...</span>
          </div>
        )}

        {/* List view */}
        {!isLoading && view.kind === 'list' && (
          <>
            <SectionHeader label="Templates" count={templates.length} />
            {templates.length === 0 ? (
              <div
                className="px-3 py-4 text-xs"
                style={{
                  color: 'var(--text-muted)',
                  backgroundColor: 'var(--bg-surface)',
                  border: '1px solid var(--border-subtle)',
                  borderRadius: '3px',
                }}
              >
                {isAdmin ? 'No templates yet. Create one with the button above.' : 'No templates found.'}
              </div>
            ) : (
              <div
                style={{
                  display: 'grid',
                  gridTemplateColumns: 'repeat(auto-fill, minmax(280px, 1fr))',
                  gap: '12px',
                }}
              >
                {templates.map((t) => (
                  <TemplateCard
                    key={t.id}
                    template={t}
                    isAdmin={isAdmin}
                    onOpen={(id) => setView({ kind: 'editor', id })}
                    onDelete={(id) => {
                      setDeleteError(null)
                      setDeleteTarget(templates.find((t) => t.id === id) ?? null)
                    }}
                  />
                ))}
              </div>
            )}
          </>
        )}

        {/* Editor view */}
        {view.kind === 'editor' && (
          <TemplateEditor
            templateId={view.id}
            isAdmin={isAdmin}
            onClose={() => setView({ kind: 'list' })}
            onSaved={(id) => setView({ kind: 'editor', id })}
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
      </div>
    </AppShell>
  )
}
