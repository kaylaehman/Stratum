import { useState, useMemo, useEffect } from 'react'
import {
  Layers,
  Loader,
  ChevronDown,
  ChevronRight,
  ExternalLink,
  Pencil,
  AlertTriangle,
  Check,
  X,
  KeyRound,
  Plus,
  Trash2,
  Play,
  Square,
  RotateCw,
} from 'lucide-react'
import { useNavigate } from 'react-router-dom'
import CodeMirror from '@uiw/react-codemirror'
import type { LanguageSupport } from '@codemirror/language'
import { AppShell } from '../components/layout/AppShell'
import { useTree } from '../lib/api/tree'
import { useMe } from '../hooks/useMe'
import { useSecrets } from '../lib/api/secrets'
import { useStackCompose, useRedeployStack, useStackLifecycle } from '../lib/api/stacks'
import type { StackLifecycleAction } from '../lib/api/stacks'
import { resolveLanguage } from '../lib/codemirror'
import { useCan } from '../lib/roles'
import type { Container, TreeNode, SecretGroup, StackEnvVar } from '../types/api'

// ── Types ─────────────────────────────────────────────────────────────────────

interface Stack {
  key: string
  projectName: string
  node: TreeNode
  containers: Container[]
}

// ── Helpers ───────────────────────────────────────────────────────────────────

const UNGROUPED = '__ungrouped__'

function statusColor(status: Container['status']): string {
  switch (status) {
    case 'running':
      return 'var(--status-ok)'
    case 'paused':
    case 'restarting':
      return 'var(--status-warn)'
    default:
      return 'var(--status-error)'
  }
}

function buildStacks(nodes: TreeNode[]): Stack[] {
  const stacks: Stack[] = []

  for (const node of nodes) {
    const byProject = new Map<string, Container[]>()

    for (const c of node.containers) {
      const key = c.compose_project ?? UNGROUPED
      const existing = byProject.get(key)
      if (existing) {
        existing.push(c)
      } else {
        byProject.set(key, [c])
      }
    }

    for (const [project, containers] of byProject) {
      if (project === UNGROUPED) continue
      stacks.push({
        key: `${node.id}::${project}`,
        projectName: project,
        node,
        containers,
      })
    }

    const ungrouped = byProject.get(UNGROUPED)
    if (ungrouped && ungrouped.length > 0) {
      stacks.push({
        key: `${node.id}::${UNGROUPED}`,
        projectName: UNGROUPED,
        node,
        containers: ungrouped,
      })
    }
  }

  return stacks
}

// ── Sub-components ────────────────────────────────────────────────────────────

function StatusDot({ status }: { status: Container['status'] }) {
  return (
    <span
      style={{
        display: 'inline-block',
        width: '6px',
        height: '6px',
        borderRadius: '50%',
        backgroundColor: statusColor(status),
        flexShrink: 0,
      }}
    />
  )
}

function Stat({ label, value }: { label: string; value: number | string }) {
  return (
    <div className="flex items-baseline gap-1.5">
      <span className="font-mono text-sm" style={{ color: 'var(--text-primary)' }}>
        {value}
      </span>
      <span
        className="text-xs uppercase tracking-wider"
        style={{ color: 'var(--text-muted)', fontSize: '11px' }}
      >
        {label}
      </span>
    </div>
  )
}

// ── Env Var editor row ────────────────────────────────────────────────────────

interface EnvVarRowProps {
  envVar: StackEnvVar
  secretGroups: SecretGroup[]
  onDelete: (key: string) => void
}

function EnvVarRow({ envVar, secretGroups, onDelete }: EnvVarRowProps) {
  const allSecrets = secretGroups.flatMap((g) => g.secrets)
  const secretEntry = allSecrets.find((s) => s.id === envVar.secret_id)

  return (
    <div
      className="flex items-center gap-2 px-3 py-1.5"
      style={{ borderBottom: '1px solid var(--border-subtle)' }}
    >
      <span className="font-mono text-xs flex-1 truncate" style={{ color: 'var(--text-primary)' }}>
        {envVar.key}
      </span>
      {envVar.secret_id ? (
        <span
          className="flex items-center gap-1 text-xs px-1.5 py-0.5 shrink-0"
          style={{
            backgroundColor: 'var(--accent-glow)',
            border: '1px solid var(--accent-dim)',
            color: 'var(--accent)',
            borderRadius: '3px',
          }}
        >
          <KeyRound size={9} />
          {secretEntry ? secretEntry.key : 'secret'}
        </span>
      ) : (
        <span
          className="font-mono text-xs shrink-0"
          style={{ color: 'var(--text-muted)', letterSpacing: '0.12em' }}
        >
          {'••••••'}
        </span>
      )}
      <button
        type="button"
        onClick={() => onDelete(envVar.key)}
        title={`Remove ${envVar.key}`}
        style={{
          background: 'transparent',
          border: 'none',
          cursor: 'pointer',
          color: 'var(--status-error)',
          padding: '2px',
          flexShrink: 0,
        }}
      >
        <Trash2 size={10} />
      </button>
    </div>
  )
}

// ── Add env var form ──────────────────────────────────────────────────────────

interface AddEnvFormProps {
  secretGroups: SecretGroup[]
  onAdd: (key: string, value: string, secretId: string) => void
  onCancel: () => void
}

function AddEnvForm({ secretGroups, onAdd, onCancel }: AddEnvFormProps) {
  const [key, setKey] = useState('')
  const [value, setValue] = useState('')
  const [secretId, setSecretId] = useState('')
  const [error, setError] = useState<string | null>(null)

  const allSecrets = secretGroups.flatMap((g) =>
    g.secrets.map((s) => ({ ...s, groupName: g.name })),
  )

  function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    if (!key.trim()) {
      setError('Key is required.')
      return
    }
    setError(null)
    onAdd(key.trim(), secretId ? '' : value, secretId)
  }

  return (
    <form
      onSubmit={handleSubmit}
      className="flex flex-col gap-2 px-3 py-2"
      style={{ borderTop: '1px solid var(--border-subtle)', backgroundColor: 'var(--bg-elevated)' }}
    >
      <div className="flex items-center gap-2 flex-wrap">
        <input
          type="text"
          placeholder="KEY_NAME"
          value={key}
          onChange={(e) => setKey(e.target.value)}
          className="font-mono text-xs px-2 py-1.5"
          style={{
            width: '140px',
            backgroundColor: 'var(--bg-surface)',
            border: '1px solid var(--border-default)',
            color: 'var(--text-primary)',
            borderRadius: '3px',
            outline: 'none',
          }}
          autoCapitalize="none"
          autoCorrect="off"
        />
        {secretId === '' ? (
          <input
            type="password"
            placeholder="plaintext value"
            value={value}
            onChange={(e) => setValue(e.target.value)}
            className="font-mono text-xs px-2 py-1.5 flex-1"
            style={{
              minWidth: '120px',
              backgroundColor: 'var(--bg-surface)',
              border: '1px solid var(--border-default)',
              color: 'var(--text-primary)',
              borderRadius: '3px',
              outline: 'none',
            }}
          />
        ) : (
          <span className="flex-1 text-xs" style={{ color: 'var(--text-muted)' }}>
            (backed by secret vault)
          </span>
        )}
        {allSecrets.length > 0 && (
          <select
            value={secretId}
            onChange={(e) => setSecretId(e.target.value)}
            className="text-xs px-1.5 py-1.5"
            style={{
              backgroundColor: 'var(--bg-surface)',
              border: '1px solid var(--border-default)',
              color: 'var(--text-primary)',
              borderRadius: '3px',
              outline: 'none',
              maxWidth: '160px',
            }}
          >
            <option value="">— plaintext —</option>
            {allSecrets.map((s) => (
              <option key={s.id} value={s.id}>
                {s.groupName}/{s.key}
              </option>
            ))}
          </select>
        )}
        <button
          type="submit"
          className="flex items-center gap-1 text-xs px-2 py-1.5 shrink-0"
          style={{
            backgroundColor: 'var(--bg-elevated)',
            border: '1px solid var(--border-default)',
            color: 'var(--text-secondary)',
            borderRadius: '3px',
            cursor: 'pointer',
          }}
        >
          <Plus size={10} />
          Add
        </button>
        <button
          type="button"
          onClick={onCancel}
          className="text-xs px-2 py-1.5 shrink-0"
          style={{
            background: 'transparent',
            border: 'none',
            color: 'var(--text-muted)',
            cursor: 'pointer',
          }}
        >
          Cancel
        </button>
      </div>
      {error && (
        <span className="text-xs" style={{ color: 'var(--status-error)' }}>
          {error}
        </span>
      )}
    </form>
  )
}

// ── Stack Edit Modal ──────────────────────────────────────────────────────────

interface StackEditModalProps {
  stack: Stack
  isAdmin: boolean
  secretGroups: SecretGroup[]
  onClose: () => void
}

function StackEditModal({ stack, isAdmin, secretGroups, onClose }: StackEditModalProps) {
  const nodeId = stack.node.id
  const project = stack.projectName

  const { data: composeData, isLoading: composeLoading, error: composeError } =
    useStackCompose(nodeId, project)

  const { mutate: redeploy, isPending: redeploying, error: deployError } = useRedeployStack()

  const [yaml, setYaml] = useState('')
  const [originalYaml, setOriginalYaml] = useState('')
  const [envVars, setEnvVars] = useState<StackEnvVar[]>([])
  const [showAddEnv, setShowAddEnv] = useState(false)
  const [deployOutput, setDeployOutput] = useState<string | null>(null)
  const [deploySuccess, setDeploySuccess] = useState(false)
  const [langExt, setLangExt] = useState<LanguageSupport | null>(null)

  useEffect(() => {
    void resolveLanguage('compose.yml').then((l) => setLangExt(l))
  }, [])

  useEffect(() => {
    if (composeData) {
      setYaml(composeData.compose_yaml)
      setOriginalYaml(composeData.compose_yaml)
      setEnvVars(composeData.env_vars ?? [])
    }
  }, [composeData])

  const yamlChanged = yaml !== originalYaml

  const diffLines = useMemo(() => {
    if (!yamlChanged) return []
    const orig = originalYaml.split('\n')
    const next = yaml.split('\n')
    const changed: string[] = []
    const maxLen = Math.max(orig.length, next.length)
    for (let i = 0; i < maxLen; i++) {
      if (orig[i] !== next[i]) {
        if (orig[i] !== undefined) changed.push(`- ${orig[i]}`)
        if (next[i] !== undefined) changed.push(`+ ${next[i]}`)
      }
    }
    return changed.slice(0, 20)
  }, [yaml, originalYaml, yamlChanged])

  function handleAddEnvVar(key: string, _value: string, secretId: string) {
    setEnvVars((prev) => {
      const filtered = prev.filter((v) => v.key !== key)
      return [
        ...filtered,
        { key, masked: true, secret_id: secretId || undefined },
      ]
    })
    setShowAddEnv(false)
  }

  function handleDeleteEnvVar(key: string) {
    setEnvVars((prev) => prev.filter((v) => v.key !== key))
  }

  function handleDeploy() {
    if (!composeData?.compose_path) return
    setDeployOutput(null)
    setDeploySuccess(false)

    const envPayload = envVars.map((v) => ({
      key: v.key,
      value: '',
      secret_id: v.secret_id ?? '',
      masked: v.masked,
    }))

    redeploy(
      {
        nodeId,
        project,
        req: {
          compose_path: composeData.compose_path,
          compose_yaml: yamlChanged ? yaml : '',
          env_vars: envPayload,
          secret_groups: [],
        },
      },
      {
        onSuccess: (data) => {
          setDeployOutput(data.output)
          setDeploySuccess(true)
          setOriginalYaml(yaml)
        },
        onError: () => {
          setDeploySuccess(false)
        },
      },
    )
  }

  const notAvailable = !composeLoading && !composeData?.found

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
          borderRadius: '4px',
          width: 'min(820px, 96vw)',
          maxHeight: '90vh',
          display: 'flex',
          flexDirection: 'column',
          overflow: 'hidden',
        }}
      >
        {/* Header */}
        <div
          className="flex items-center justify-between px-4 py-3 shrink-0"
          style={{ borderBottom: '1px solid var(--border-subtle)' }}
        >
          <div className="flex items-center gap-2">
            <Layers size={13} style={{ color: 'var(--text-secondary)' }} />
            <span className="font-mono text-xs font-medium" style={{ color: 'var(--text-primary)' }}>
              {project}
            </span>
            <span className="text-xs" style={{ color: 'var(--text-muted)' }}>
              on {stack.node.name}
            </span>
          </div>
          <button
            type="button"
            onClick={onClose}
            style={{ background: 'transparent', border: 'none', cursor: 'pointer', padding: '4px' }}
          >
            <X size={14} style={{ color: 'var(--text-muted)' }} />
          </button>
        </div>

        {/* Body */}
        <div className="flex flex-col flex-1 min-h-0 overflow-auto p-4 gap-4">
          {composeLoading && (
            <div className="flex items-center gap-2 py-4">
              <Loader size={13} className="animate-spin" style={{ color: 'var(--accent)' }} />
              <span className="text-xs" style={{ color: 'var(--text-muted)' }}>
                Loading compose file...
              </span>
            </div>
          )}

          {composeError && (
            <div
              className="flex items-center gap-2 text-xs px-3 py-2"
              style={{
                backgroundColor: 'rgba(232,64,64,0.08)',
                border: '1px solid rgba(232,64,64,0.25)',
                borderRadius: '3px',
                color: 'var(--status-error)',
              }}
            >
              <AlertTriangle size={11} />
              Failed to load compose file. The node may lack Docker capability or the stack may
              have no compose file at a known path.
            </div>
          )}

          {notAvailable && !composeError && !composeLoading && (
            <div
              className="flex items-center gap-2 text-xs px-3 py-2"
              style={{
                backgroundColor: 'rgba(240,160,32,0.07)',
                border: '1px solid rgba(240,160,32,0.2)',
                borderRadius: '3px',
                color: 'var(--text-secondary)',
              }}
            >
              <AlertTriangle size={11} style={{ color: 'var(--status-warn)' }} />
              No compose file could be located for this project on this host.
            </div>
          )}

          {composeData?.found && (
            <>
              <div className="flex items-center gap-2">
                <span className="text-xs" style={{ color: 'var(--text-muted)' }}>
                  Compose file:
                </span>
                <span className="font-mono text-xs" style={{ color: 'var(--text-secondary)' }}>
                  {composeData.compose_path}
                </span>
              </div>

              {/* YAML editor */}
              <div className="flex flex-col gap-1">
                <span
                  className="text-xs uppercase tracking-wider"
                  style={{ color: 'var(--text-muted)' }}
                >
                  Compose YAML
                </span>
                <div
                  style={{
                    border: '1px solid var(--border-default)',
                    borderRadius: '3px',
                    overflow: 'hidden',
                    maxHeight: '300px',
                    overflowY: 'auto',
                  }}
                >
                  <CodeMirror
                    value={yaml}
                    onChange={(v) => setYaml(v)}
                    extensions={langExt ? [langExt] : []}
                    editable={isAdmin}
                    basicSetup={{ lineNumbers: true }}
                    theme="dark"
                    style={{ fontSize: '12px', fontFamily: "'Space Mono', monospace" }}
                  />
                </div>
              </div>

              {/* Diff preview */}
              {yamlChanged && diffLines.length > 0 && (
                <div className="flex flex-col gap-1">
                  <span
                    className="text-xs uppercase tracking-wider"
                    style={{ color: 'var(--text-muted)' }}
                  >
                    Changes (preview)
                  </span>
                  <div
                    style={{
                      backgroundColor: 'var(--bg-surface)',
                      border: '1px solid var(--border-subtle)',
                      borderRadius: '3px',
                      padding: '8px',
                      maxHeight: '120px',
                      overflowY: 'auto',
                    }}
                  >
                    {diffLines.map((line, i) => (
                      <div
                        key={i}
                        className="font-mono text-xs"
                        style={{
                          color: line.startsWith('+')
                            ? 'var(--status-ok)'
                            : line.startsWith('-')
                              ? 'var(--status-error)'
                              : 'var(--text-muted)',
                          whiteSpace: 'pre',
                        }}
                      >
                        {line}
                      </div>
                    ))}
                    {diffLines.length >= 20 && (
                      <span className="text-xs" style={{ color: 'var(--text-muted)' }}>
                        ... (diff truncated)
                      </span>
                    )}
                  </div>
                </div>
              )}

              {/* Env vars */}
              <div className="flex flex-col gap-1">
                <div className="flex items-center justify-between">
                  <span
                    className="text-xs uppercase tracking-wider"
                    style={{ color: 'var(--text-muted)' }}
                  >
                    Environment Variables
                  </span>
                  {isAdmin && (
                    <button
                      type="button"
                      onClick={() => setShowAddEnv((v) => !v)}
                      className="flex items-center gap-1 text-xs px-2 py-1"
                      style={{
                        backgroundColor: 'var(--bg-surface)',
                        border: '1px solid var(--border-default)',
                        color: showAddEnv ? 'var(--accent)' : 'var(--text-secondary)',
                        borderRadius: '3px',
                        cursor: 'pointer',
                      }}
                    >
                      <Plus size={10} />
                      Add
                    </button>
                  )}
                </div>
                <div
                  style={{
                    backgroundColor: 'var(--bg-surface)',
                    border: '1px solid var(--border-subtle)',
                    borderRadius: '3px',
                    overflow: 'hidden',
                  }}
                >
                  {envVars.length === 0 && !showAddEnv && (
                    <div className="px-3 py-2 text-xs" style={{ color: 'var(--text-muted)' }}>
                      No env vars. Secrets are injected at deploy time — never written to disk.
                    </div>
                  )}
                  {envVars.map((v) => (
                    <EnvVarRow
                      key={v.key}
                      envVar={v}
                      secretGroups={secretGroups}
                      onDelete={handleDeleteEnvVar}
                    />
                  ))}
                  {showAddEnv && (
                    <AddEnvForm
                      secretGroups={secretGroups}
                      onAdd={handleAddEnvVar}
                      onCancel={() => setShowAddEnv(false)}
                    />
                  )}
                </div>
                <span className="text-xs" style={{ color: 'var(--text-muted)' }}>
                  Secret values pass as <code>--env</code> at deploy time and never touch disk.
                </span>
              </div>

              {/* Deploy output */}
              {deployOutput !== null && (
                <div
                  style={{
                    backgroundColor: 'var(--bg-surface)',
                    border: `1px solid ${deploySuccess ? 'rgba(74,200,80,0.3)' : 'rgba(232,64,64,0.3)'}`,
                    borderRadius: '3px',
                    padding: '8px',
                  }}
                >
                  <div className="flex items-center gap-1.5 mb-1.5">
                    {deploySuccess ? (
                      <Check size={11} style={{ color: 'var(--status-ok)' }} />
                    ) : (
                      <AlertTriangle size={11} style={{ color: 'var(--status-error)' }} />
                    )}
                    <span
                      className="text-xs font-medium uppercase tracking-wider"
                      style={{ color: deploySuccess ? 'var(--status-ok)' : 'var(--status-error)' }}
                    >
                      {deploySuccess ? 'Deployed' : 'Deploy failed'}
                    </span>
                  </div>
                  <pre
                    className="text-xs"
                    style={{
                      color: 'var(--text-secondary)',
                      whiteSpace: 'pre-wrap',
                      maxHeight: '120px',
                      overflowY: 'auto',
                      fontFamily: "'Space Mono', monospace",
                    }}
                  >
                    {deployOutput}
                  </pre>
                </div>
              )}

              {deployError && deployOutput === null && (
                <div
                  className="flex items-center gap-2 text-xs px-3 py-2"
                  style={{
                    backgroundColor: 'rgba(232,64,64,0.08)',
                    border: '1px solid rgba(232,64,64,0.25)',
                    borderRadius: '3px',
                    color: 'var(--status-error)',
                  }}
                >
                  <AlertTriangle size={11} />
                  Deploy failed. Check server logs.
                </div>
              )}
            </>
          )}
        </div>

        {/* Footer */}
        <div
          className="flex items-center justify-end gap-2 px-4 py-3 shrink-0"
          style={{ borderTop: '1px solid var(--border-subtle)' }}
        >
          {!isAdmin && (
            <span className="text-xs flex-1" style={{ color: 'var(--text-muted)' }}>
              Read-only — admin required to redeploy.
            </span>
          )}
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
          {isAdmin && composeData?.found && (
            <button
              type="button"
              onClick={handleDeploy}
              disabled={redeploying}
              className="flex items-center gap-1.5 text-xs px-3 py-1.5"
              style={{
                backgroundColor: 'rgba(74,200,80,0.1)',
                border: '1px solid rgba(74,200,80,0.35)',
                color: redeploying ? 'var(--text-muted)' : 'var(--status-ok)',
                borderRadius: '3px',
                cursor: redeploying ? 'default' : 'pointer',
                opacity: redeploying ? 0.7 : 1,
              }}
            >
              {redeploying ? (
                <Loader size={10} className="animate-spin" />
              ) : (
                <Check size={10} />
              )}
              {redeploying ? 'Deploying...' : 'Redeploy'}
            </button>
          )}
        </div>
      </div>
    </div>
  )
}

// ── StackCard ─────────────────────────────────────────────────────────────────

interface StackCardProps {
  stack: Stack
  isAdmin: boolean
  secretGroups: SecretGroup[]
  defaultOpen?: boolean
}

function StackCard({ stack, isAdmin, secretGroups, defaultOpen = false }: StackCardProps) {
  const [open, setOpen] = useState(defaultOpen)
  const [editOpen, setEditOpen] = useState(false)
  const navigate = useNavigate()
  const { isOperator } = useCan()
  const { mutate: lifecycle, isPending: lifecyclePending, variables: lifecycleVars } = useStackLifecycle()
  const [pendingConfirm, setPendingConfirm] = useState<StackLifecycleAction | null>(null)

  const running = stack.containers.filter((c) => c.status === 'running').length
  const total = stack.containers.length
  const isUngrouped = stack.projectName === UNGROUPED
  const displayName = isUngrouped ? 'Ungrouped / standalone' : stack.projectName
  const canEdit = !isUngrouped && stack.node.type !== 'ssh'
  const canLifecycle = isOperator && !isUngrouped && stack.node.type !== 'ssh'

  const nodeId = stack.node.id
  const project = stack.projectName

  function triggerLifecycle(action: StackLifecycleAction) {
    if ((action === 'stop' || action === 'restart') && pendingConfirm !== action) {
      setPendingConfirm(action)
      return
    }
    setPendingConfirm(null)
    lifecycle({ nodeId, project, action })
  }

  function lifecycleBtn(action: StackLifecycleAction, icon: React.ReactNode, label: string) {
    const loading =
      lifecyclePending &&
      lifecycleVars?.nodeId === nodeId &&
      lifecycleVars?.project === project &&
      lifecycleVars?.action === action
    const confirming = pendingConfirm === action
    return (
      <button
        key={action}
        type="button"
        disabled={lifecyclePending}
        onClick={(e) => { e.stopPropagation(); triggerLifecycle(action) }}
        title={confirming ? `Click again to confirm ${label}` : label}
        className="flex items-center justify-center shrink-0"
        style={{
          background: confirming ? 'rgba(232,64,64,0.10)' : 'transparent',
          border: `1px solid ${confirming ? 'var(--status-error)' : 'var(--border-subtle)'}`,
          color: confirming ? 'var(--status-error)' : 'var(--text-muted)',
          borderRadius: '3px',
          padding: '2px 5px',
          cursor: lifecyclePending ? 'not-allowed' : 'pointer',
          opacity: lifecyclePending && !loading ? 0.4 : 1,
          gap: '3px',
          fontSize: '10px',
        }}
      >
        {loading ? <Loader size={10} className="animate-spin" /> : icon}
      </button>
    )
  }

  return (
    <>
      <div
        style={{
          backgroundColor: 'var(--bg-surface)',
          border: '1px solid var(--border-subtle)',
          borderRadius: '3px',
          overflow: 'hidden',
          flexShrink: 0,
        }}
      >
        {/* Header row */}
        <div
          className="flex items-center gap-2.5 w-full px-3 py-2"
          style={{ borderBottom: open ? '1px solid var(--border-subtle)' : 'none' }}
        >
          <button
            type="button"
            onClick={() => setOpen((o) => !o)}
            style={{ background: 'transparent', border: 'none', cursor: 'pointer', padding: 0 }}
          >
            {open ? (
              <ChevronDown size={13} style={{ color: 'var(--text-muted)', flexShrink: 0 }} />
            ) : (
              <ChevronRight size={13} style={{ color: 'var(--text-muted)', flexShrink: 0 }} />
            )}
          </button>

          <span
            className="font-mono text-xs flex-1 truncate"
            style={{ color: isUngrouped ? 'var(--text-muted)' : 'var(--text-primary)' }}
          >
            {displayName}
          </span>

          <span
            className="font-mono text-xs px-1.5 py-0.5 shrink-0"
            style={{
              backgroundColor: 'var(--bg-elevated)',
              border: '1px solid var(--border-subtle)',
              borderRadius: '3px',
              color:
                running === total
                  ? 'var(--status-ok)'
                  : running === 0
                    ? 'var(--status-error)'
                    : 'var(--status-warn)',
            }}
          >
            {running}/{total}
          </span>

          {canLifecycle && (
            <div className="flex items-center gap-1 shrink-0">
              {lifecycleBtn('start', <Play size={10} />, 'Start')}
              {lifecycleBtn('stop', <Square size={10} />, 'Stop')}
              {lifecycleBtn('restart', <RotateCw size={10} />, 'Restart')}
            </div>
          )}

          <span
            className="text-xs truncate shrink-0"
            style={{ color: 'var(--text-muted)', maxWidth: '120px' }}
          >
            {stack.node.name}
          </span>

          {canEdit && (
            <button
              type="button"
              title="Edit / redeploy stack"
              onClick={() => setEditOpen(true)}
              className="flex items-center justify-center shrink-0"
              style={{
                background: 'transparent',
                border: 'none',
                cursor: 'pointer',
                color: 'var(--text-muted)',
                padding: '2px',
                borderRadius: '3px',
              }}
            >
              <Pencil size={11} />
            </button>
          )}
        </div>

        {/* Container list */}
        {open && (
          <ul className="flex flex-col">
            {stack.containers.map((c) => (
              <li
                key={c.id}
                className="flex items-center gap-2 px-4 py-1.5"
                style={{ borderBottom: '1px solid var(--border-subtle)' }}
              >
                <StatusDot status={c.status} />
                <span
                  className="font-mono text-xs flex-1 truncate"
                  style={{ color: 'var(--text-primary)' }}
                >
                  {c.name}
                </span>
                <span
                  className="text-xs truncate"
                  style={{ color: 'var(--text-muted)', maxWidth: '180px' }}
                >
                  {c.image}
                </span>
                <button
                  type="button"
                  title="Open in Resources"
                  onClick={() => navigate('/resources')}
                  className="flex items-center justify-center shrink-0"
                  style={{
                    background: 'transparent',
                    border: 'none',
                    cursor: 'pointer',
                    color: 'var(--text-muted)',
                    padding: '2px',
                    borderRadius: '3px',
                  }}
                >
                  <ExternalLink size={11} />
                </button>
              </li>
            ))}
          </ul>
        )}
      </div>

      {editOpen && (
        <StackEditModal
          stack={stack}
          isAdmin={isAdmin}
          secretGroups={secretGroups}
          onClose={() => setEditOpen(false)}
        />
      )}
    </>
  )
}

// ── Main page ─────────────────────────────────────────────────────────────────

export default function Stacks() {
  const { data: tree, isLoading } = useTree()
  const { data: me } = useMe()
  const { data: secretsData } = useSecrets()
  const isAdmin = me?.role === 'admin'
  const secretGroups = secretsData?.groups ?? []

  const nodes = useMemo(() => tree?.nodes ?? [], [tree])
  const stacks = useMemo(() => buildStacks(nodes), [nodes])

  const namedStacks = useMemo(
    () => stacks.filter((s) => s.projectName !== UNGROUPED),
    [stacks],
  )

  const totalContainers = useMemo(
    () => stacks.reduce((sum, s) => sum + s.containers.length, 0),
    [stacks],
  )

  return (
    <AppShell>
      <div
        className="flex flex-col flex-1 min-h-0 h-full w-full p-6"
        style={{ maxWidth: '900px', margin: '0 auto' }}
      >
        {/* Page header */}
        <div className="flex items-center justify-between gap-4 mb-5 flex-wrap shrink-0">
          <div className="flex items-center gap-2">
            <Layers size={16} style={{ color: 'var(--text-secondary)' }} />
            <h1
              className="text-sm font-medium uppercase tracking-wider"
              style={{ color: 'var(--text-primary)' }}
            >
              Stacks
            </h1>
          </div>

          {tree && (
            <div className="flex items-center gap-4 flex-wrap">
              <Stat label="stacks" value={namedStacks.length} />
              <Stat label="containers" value={totalContainers} />
            </div>
          )}
        </div>

        {isLoading && (
          <div className="flex items-center gap-2 py-8">
            <Loader size={13} className="animate-spin" style={{ color: 'var(--accent)' }} />
            <span className="text-xs" style={{ color: 'var(--text-muted)' }}>
              Loading stacks...
            </span>
          </div>
        )}

        {tree && stacks.length === 0 && (
          <p className="text-xs" style={{ color: 'var(--text-muted)' }}>
            No deployed stacks found.
          </p>
        )}

        {stacks.length > 0 && (
          <div className="flex flex-col gap-2 overflow-auto flex-1 min-h-0">
            {stacks.map((stack) => (
              <StackCard
                key={stack.key}
                stack={stack}
                isAdmin={isAdmin}
                secretGroups={secretGroups}
              />
            ))}
          </div>
        )}
      </div>
    </AppShell>
  )
}
